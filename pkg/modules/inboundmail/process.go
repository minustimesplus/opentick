// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package inboundmail

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	vmail "code.vikunja.io/api/pkg/mail"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
)

// sendReply delivers the answer to a processed mail. Swapped out in tests.
var sendReply = vmail.SendMail

// now is the clock commands are read against. Swapped out in tests.
var now = time.Now

var subjectPrefixRe = regexp.MustCompile(`(?i)^\s*((re|fwd?|aw|wg|tr|sv)\s*:\s*|opentick task\s*:\s*)+`)

// Process applies one received mail. It returns an error only when the mail
// should be tried again later; mail that can't or mustn't be applied is dropped.
func Process(raw []byte) error {
	in, err := parseMessage(raw)
	if err != nil {
		log.Warningf("[inbound mail] dropping a mail that can't be read: %s", err)
		return nil
	}

	var secret string
	var taskID int64
	for _, r := range in.Recipients {
		if sec, id, ok := models.ParseInboundTag(r); ok {
			secret, taskID = sec, id
			break
		}
	}
	if secret == "" {
		log.Debugf("[inbound mail] mail %s is not addressed to a private address, ignoring it", in.MessageID)
		return nil
	}
	if ownAddress(in.From) {
		return nil
	}

	s := db.NewSession()
	defer s.Close()
	cfg, err := models.GetAgentConfigByInboundSecret(s, secret)
	if err != nil {
		return err
	}
	if cfg == nil {
		log.Infof("[inbound mail] mail %s is addressed to an unknown or replaced address, ignoring it", in.MessageID)
		return nil
	}
	// With the email address: the account address may always send.
	u, err := user.GetUserWithEmail(s, &user.User{ID: cfg.UserID})
	if err != nil {
		var disabled *user.ErrAccountDisabled
		var locked *user.ErrAccountLocked
		if user.IsErrUserDoesNotExist(err) || errors.As(err, &disabled) || errors.As(err, &locked) {
			log.Infof("[inbound mail] mail %s is for user %d, who can't sign in; ignoring it", in.MessageID, cfg.UserID)
			return nil
		}
		return err
	}
	if in.SpoofedFrom || !cfg.InboundSenderAllowed(u, in.From) {
		// No answer: replying would tell a stranger the address works.
		log.Warningf("[inbound mail] mail %s for user %d came from %q, which may not use the address; ignoring it", in.MessageID, u.ID, in.From)
		return nil
	}

	claimed, err := models.ClaimInboundMessage(s, models.InboundMessageKey(in.MessageID, raw), u.ID)
	if err != nil {
		_ = s.Rollback()
		return err
	}
	if err := s.Commit(); err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	ex := &executor{user: u, cfg: cfg, loc: userLocation(u), now: now(), contextTaskID: taskID}
	body := cleanBody(in.Text)
	cmds := parseCommands(body)
	var outcomes []outcome
	switch {
	case len(cmds) > 0:
		outcomes = ex.run(cmds)
	case taskID > 0 && body != "":
		// A plain reply to a task: it's a comment.
		outcomes = ex.run([]*command{{Verb: "comment", Fields: map[string]string{"text": body}}})
	case taskID > 0:
		outcomes = []outcome{{command: "reply", text: "The reply was empty, so nothing was added to the task."}}
	default:
		title := strings.TrimSpace(subjectPrefixRe.ReplaceAllString(in.Subject, ""))
		if title == "" {
			outcomes = []outcome{{command: "mail", text: "There were no commands and no subject, so nothing was done. Send \"help\" for the commands."}}
			break
		}
		add := &command{Verb: "add", Arg: title, Fields: map[string]string{}}
		if body != "" {
			add.Fields["notes"] = body
		}
		outcomes = ex.run([]*command{add})
	}
	log.Infof("[inbound mail] applied mail %s for user %d: %d command(s)", in.MessageID, u.ID, len(outcomes))

	if in.Automatic || !config.MailerEnabled.GetBool() || in.From == "" {
		return nil
	}
	sendReply(&vmail.Opts{
		To:          in.From,
		Subject:     replySubject(in.Subject),
		Message:     replyText(outcomes),
		ContentType: vmail.ContentTypePlain,
		ReplyTo:     models.InboundAddressFor(secret, taskID),
		InReplyTo:   in.MessageID,
		AutoReply:   true,
	})
	return nil
}

// ownAddress reports whether mail comes from OpenTick itself, which is never applied.
func ownAddress(from string) bool {
	for _, own := range []string{config.MailerFromEmail.GetString(), config.InboundMailAddress.GetString()} {
		if own != "" && strings.EqualFold(strings.TrimSpace(own), from) {
			return true
		}
	}
	_, _, isTagged := models.ParseInboundTag(from)
	return isTagged
}

func userLocation(u *user.User) *time.Location {
	if u.Timezone != "" {
		if loc, err := time.LoadLocation(u.Timezone); err == nil {
			return loc
		}
	}
	return config.GetTimeZone()
}

func replySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "OpenTick"
	}
	if strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

func replyText(outcomes []outcome) string {
	var b strings.Builder
	failed := false
	for i, o := range outcomes {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if o.ok {
			b.WriteString("OK " + o.text)
			continue
		}
		failed = true
		b.WriteString("FAILED " + o.command + ": " + o.text)
	}
	if failed {
		b.WriteString("\n\nCommands that failed were not applied; the others were. Send \"help\" for all commands.")
	}
	return b.String()
}
