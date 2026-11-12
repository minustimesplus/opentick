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

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

const inboundSecretLength = 12

// InboundMailEnabled reports whether the server reads a mailbox for task commands.
func InboundMailEnabled() bool {
	return config.InboundMailEnabled.GetBool() && config.InboundMailHost.GetString() != "" &&
		strings.Contains(config.InboundMailAddress.GetString(), "@")
}

func newInboundSecret() (string, error) {
	// Lowercase only: some providers lowercase the local part on the way in.
	secret, err := utils.CryptoRandomString(inboundSecretLength)
	return strings.ToLower(secret), err
}

// InboundAddressFor builds a user's private address, or the reply address of one
// of their tasks when taskID is set: the configured address plus a "+tag".
func InboundAddressFor(secret string, taskID int64) string {
	base := strings.TrimSpace(config.InboundMailAddress.GetString())
	at := strings.LastIndex(base, "@")
	if at < 0 || secret == "" {
		return ""
	}
	local, domain := base[:at], base[at+1:]
	if plus := strings.Index(local, "+"); plus >= 0 {
		local = local[:plus]
	}
	tag := secret
	if taskID > 0 {
		tag += "-t" + strconv.FormatInt(taskID, 10)
	}
	return local + "+" + tag + "@" + domain
}

// ParseInboundTag reads the secret and the optional task out of an address built by InboundAddressFor.
func ParseInboundTag(address string) (secret string, taskID int64, ok bool) {
	address = strings.ToLower(strings.TrimSpace(address))
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return "", 0, false
	}
	local := address[:at]
	plus := strings.Index(local, "+")
	if plus < 0 {
		return "", 0, false
	}
	tag := local[plus+1:]
	if dash := strings.Index(tag, "-t"); dash >= 0 {
		id, err := strconv.ParseInt(tag[dash+2:], 10, 64)
		if err != nil || id <= 0 {
			return "", 0, false
		}
		tag, taskID = tag[:dash], id
	}
	if len(tag) != inboundSecretLength {
		return "", 0, false
	}
	return tag, taskID, true
}

// GetAgentConfigByInboundSecret returns the agent config owning an inbound secret, or nil when none does.
func GetAgentConfigByInboundSecret(s *xorm.Session, secret string) (*AgentConfig, error) {
	if secret == "" {
		return nil, nil
	}
	c := &AgentConfig{}
	has, err := s.Where("inbound_secret = ?", secret).Get(c)
	if err != nil || !has {
		return nil, err
	}
	return c, nil
}

// RotateInboundAddress gives the authenticated user a new inbound address; the old one stops working.
func RotateInboundAddress(s *xorm.Session, a web.Auth) (*AgentConfig, error) {
	c := &AgentConfig{}
	can, err := c.CanUpdate(s, a)
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, ErrGenericForbidden{}
	}
	stored, err := getAgentConfig(s, c.UserID)
	if err != nil {
		return nil, err
	}
	if stored.InboundSecret, err = newInboundSecret(); err != nil {
		return nil, err
	}
	if stored.ID == 0 {
		_, err = s.Insert(stored)
	} else {
		_, err = s.ID(stored.ID).Cols("inbound_secret").Update(stored)
	}
	if err != nil {
		return nil, err
	}
	stored.fillComputed()
	return stored, nil
}

// normalizeInboundSenders validates a comma- or newline-separated list of addresses and "@domain" entries.
func normalizeInboundSenders(raw string) (string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == ' ' })
	out := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" || seen[f] {
			continue
		}
		if strings.HasPrefix(f, "@") {
			if len(f) < 4 || !strings.Contains(f[1:], ".") || strings.Contains(f[1:], "@") {
				return "", ErrInvalidAgentEmail{}
			}
		} else if _, err := mail.ParseAddress(f); err != nil {
			return "", ErrInvalidAgentEmail{}
		}
		seen[f] = true
		out = append(out, f)
	}
	return strings.Join(out, ", "), nil
}

// InboundSenderAllowed reports whether mail from the given address may act for the config's user:
// their own account address, their Instinct address, or anything on their sender list.
func (c *AgentConfig) InboundSenderAllowed(u *user.User, from string) bool {
	from = strings.ToLower(strings.TrimSpace(from))
	if from == "" {
		return false
	}
	if u != nil && u.Email != "" && strings.EqualFold(u.Email, from) {
		return true
	}
	if c.InboundEmailMatches(from) {
		return true
	}
	for _, allowed := range strings.Split(c.InboundSenders, ",") {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		switch {
		case allowed == "":
		case strings.HasPrefix(allowed, "@") && strings.HasSuffix(from, allowed):
			return true
		case allowed == from:
			return true
		}
	}
	return false
}

// InboundEmailMatches reports whether the address is the user's Instinct address.
func (c *AgentConfig) InboundEmailMatches(from string) bool {
	return c.InstinctEmail != "" && strings.EqualFold(strings.TrimSpace(c.InstinctEmail), from)
}

// InboundMailMessage remembers a processed inbound mail, so it is never applied twice,
// even when the mailbox could not be told it was read or two servers read the same mailbox.
type InboundMailMessage struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	MessageKey string    `xorm:"varchar(64) not null unique 'message_key'"`
	UserID     int64     `xorm:"bigint not null default 0 INDEX"`
	Created    time.Time `xorm:"created not null"`
}

// TableName returns the table name for processed inbound mails.
func (*InboundMailMessage) TableName() string {
	return "inbound_mail_messages"
}

// InboundMessageKey derives the dedupe key of a mail from its Message-ID, or from its raw bytes when it has none.
func InboundMessageKey(messageID string, raw []byte) string {
	var sum [32]byte
	if id := strings.TrimSpace(messageID); id != "" {
		sum = sha256.Sum256([]byte("id:" + id))
	} else {
		sum = sha256.Sum256(append([]byte("raw:"), raw...))
	}
	return hex.EncodeToString(sum[:])
}

// ClaimInboundMessage records a mail as processed. It returns false when it already was.
func ClaimInboundMessage(s *xorm.Session, key string, userID int64) (bool, error) {
	exists, err := s.Where("message_key = ?", key).Exist(&InboundMailMessage{})
	if err != nil || exists {
		return false, err
	}
	// A unique violation here means another server claimed the mail since the check; the caller rolls back.
	if _, err := s.Insert(&InboundMailMessage{MessageKey: key, UserID: userID}); err != nil {
		return false, fmt.Errorf("could not record inbound mail: %w", err)
	}
	return true, nil
}

// InboundCommandReferenceMarker starts the command reference in mails OpenTick sends.
// Mail reading stops there, so a reply quoting the reference never runs its examples.
const InboundCommandReferenceMarker = "== OpenTick email commands =="

// InboundCommandReference explains the commands the inbound address understands.
const InboundCommandReference = InboundCommandReferenceMarker + `
Email commands to your OpenTick address. Put each command on its own line as
"command: value", followed by its "field: value" lines. Separate commands with
an empty line. You get a reply with what was done, including task IDs.

add: <title>                 Create a task.
  list: <list name or ID>    Default: your inbox list. Sub-lists: Parent/Child.
  date: <when>               2026-11-20 (all day), 2026-11-20 17:00, today,
                             tomorrow 9am, friday 14:30.
  duration: <length>         45m, 1h30m, 2h, 3d. Needs a date; a timed date
                             becomes a time block, an all-day date spans days.
  end: <when>                Instead of duration: when the block ends.
  repeat: <rule>             daily, weekly, monthly, yearly, weekdays,
                             every 2 weeks, every monday and thursday,
                             every 2 weeks on friday, or an RRULE such as
                             FREQ=MONTHLY;BYDAY=-1FR. Add "until 2026-12-31"
                             or "10 times" to end it. Needs a date.
  repeat from: due|completion  Count the next date from the due date
                             (default) or from when it was completed.
  priority: none|low|medium|high
  labels: <label>, <label>   Missing labels are created.
  notes: <text>              May continue over several lines.

update: <task ID>            Change a task. Takes all fields of add, plus
  title: <new title>         Use "none" to clear date, duration, repeat,
                             labels or notes.
done: <task ID>              Complete a task. A repeating task moves on to
                             its next date.
undone: <task ID>            Reopen a completed task.
delete: <task ID>            Move a task to the trash. For a repeating task
  scope: this|future         say which: "this" skips only the current
                             occurrence, "future" deletes it and all later ones.
comment: <task ID>           Comment on a task.
  text: <text>
find: <words>                Reply with the open tasks matching the words.
  list: <list>               Optional: only search this list.
lists                        Reply with all your lists.
help                         Reply with this text.

Task IDs are numbers (123 or #123). In a reply to a task OpenTick sent you,
the task ID may be left out; plain text without commands becomes a comment.
An email without commands creates a task from its subject and text.`

// InboundReplyInstructions is appended to tasks emailed to an agent, so it can answer by replying.
func InboundReplyInstructions(taskID int64) string {
	return "You can update OpenTick by replying to this email (task ID " + strconv.FormatInt(taskID, 10) +
		"). A plain reply is added to the task as a comment; to change tasks, use the commands below.\n\n" +
		InboundCommandReference
}
