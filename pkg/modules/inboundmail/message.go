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
	"bytes"
	"errors"
	"io"
	"mime"
	"strings"

	"code.vikunja.io/api/pkg/richtext"

	// Registers decoders for non-UTF-8 charsets.
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
)

const (
	maxMailBytes = 1 << 20
	maxBodyBytes = 256 << 10
)

// inbound is the part of a received mail OpenTick looks at.
type inbound struct {
	MessageID  string
	Subject    string
	From       string
	Recipients []string
	Text       string
	// Automatic is set for auto-replies, bounces and list mail, which never get an answer.
	// Mail marked "Auto-Submitted: auto-generated" doesn't count: that is how AI agents
	// such as Instinct send, and only senders the user allowed get this far anyway.
	// Answering "auto-replied" mail (out-of-office and the like) is what causes loops,
	// and OpenTick's own replies are marked auto-replied, so a well-behaved agent
	// never answers them in turn.
	Automatic bool
	// SpoofedFrom is set when the receiving server reports a failed DMARC check of the sender.
	SpoofedFrom bool
}

// parseMessage reads a raw RFC 5322 message.
func parseMessage(raw []byte) (*inbound, error) {
	if len(raw) > maxMailBytes {
		return nil, errors.New("mail is too large")
	}
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil && mr == nil {
		return nil, err
	}
	defer mr.Close()
	h := mr.Header

	in := &inbound{}
	in.MessageID, _ = h.MessageID()
	in.Subject, _ = h.Subject()
	if from, err := h.AddressList("From"); err == nil && len(from) > 0 {
		in.From = strings.ToLower(from[0].Address)
	}
	for _, key := range []string{"To", "Cc", "Delivered-To", "X-Original-To", "X-Forwarded-To", "Envelope-To"} {
		if list, err := h.AddressList(key); err == nil {
			for _, a := range list {
				in.Recipients = append(in.Recipients, a.Address)
			}
			continue
		}
		// Delivered-To and friends are often bare addresses that don't parse as an address list.
		for _, v := range h.Values(key) {
			in.Recipients = append(in.Recipients, strings.Trim(strings.TrimSpace(v), "<>"))
		}
	}

	autoSubmitted := strings.ToLower(strings.TrimSpace(h.Get("Auto-Submitted")))
	precedence := strings.ToLower(strings.TrimSpace(h.Get("Precedence")))
	in.Automatic = (autoSubmitted != "" && autoSubmitted != "no" && autoSubmitted != "auto-generated") ||
		precedence == "bulk" || precedence == "list" || precedence == "junk" ||
		h.Get("List-Id") != "" || strings.HasPrefix(strings.ToLower(in.From), "mailer-daemon@")

	// The receiving server puts its verdict on top, so only the first header counts.
	if results := h.Values("Authentication-Results"); len(results) > 0 {
		in.SpoofedFrom = strings.Contains(strings.ToLower(results[0]), "dmarc=fail")
	}

	in.Text, err = readText(mr)
	if err != nil {
		return nil, err
	}
	return in, nil
}

// readText returns the plain text of a mail, converting the HTML part when there is no plain one.
func readText(mr *mail.Reader) (string, error) {
	var plain, html string
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if plain != "" || html != "" {
				break
			}
			return "", err
		}
		inline, ok := p.Header.(*mail.InlineHeader)
		if !ok {
			continue
		}
		ct, _, _ := mime.ParseMediaType(inline.Get("Content-Type"))
		if ct == "" {
			ct = "text/plain"
		}
		body, err := io.ReadAll(io.LimitReader(p.Body, maxBodyBytes))
		if err != nil {
			continue
		}
		switch {
		case ct == "text/plain" && plain == "":
			plain = string(body)
		case ct == "text/html" && html == "":
			html = string(body)
		}
	}
	if strings.TrimSpace(plain) == "" && html != "" {
		if md, err := richtext.HTMLToMarkdown(html); err == nil {
			return md, nil
		}
	}
	return plain, nil
}
