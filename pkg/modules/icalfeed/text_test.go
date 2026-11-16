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

package icalfeed

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescriptionText(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text passes through",
			in:   "Bring a < b and c > d",
			want: "Bring a < b and c > d",
		},
		{
			name: "inline tags vanish",
			in:   "A <b>bold</b> <i>word</i>",
			want: "A bold word",
		},
		{
			name: "br and blocks become newlines",
			in:   "<p>one<br>two</p><p>three</p><ul><li>four</li><li>five</li></ul>",
			want: "one\ntwo\nthree\nfour\nfive",
		},
		{
			name: "self-closing br",
			in:   "one<br/>two<br />three",
			want: "one\ntwo\nthree",
		},
		{
			name: "entities are decoded",
			in:   `<p>Fish &amp; Chips &lt;3 "quoted" &nbsp;end</p>`,
			want: `Fish & Chips <3 "quoted"  end`,
		},
		{
			name: "script and style content is dropped",
			in:   `<p>keep</p><style>body{color:red}</style><script>alert(1)</script>`,
			want: "keep",
		},
		{
			name: "blank lines collapse",
			in:   "<p>one</p>\n\n\n<p>two</p>",
			want: "one\ntwo",
		},
		{
			name: "malformed html still yields its text",
			in:   "<b>open<div>mixed</b>up",
			want: "open\nmixedup",
		},
		{
			name: "full document keeps only the body",
			in:   "<html><head><title>t</title></head><body><p>body text</p></body></html>",
			want: "body text",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, descriptionText(c.in))
		})
	}
}
