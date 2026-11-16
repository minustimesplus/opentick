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
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Google Calendar and others stuff their HTML markup into the plain-text
// DESCRIPTION (often next to an X-ALT-DESC twin), while the API contract for
// events is plain text. Angle brackets alone don't make a description HTML
// ("a < b"), so only convert when something tag-shaped is present.
var htmlTag = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)

// blockTags force a line break at their boundary; every other tag only
// disappears. Lists keep their items on separate lines but get no bullets.
var blockTags = map[string]bool{
	"address":    true,
	"article":    true,
	"aside":      true,
	"blockquote": true,
	"br":         true,
	"dd":         true,
	"details":    true,
	"div":        true,
	"dl":         true,
	"dt":         true,
	"fieldset":   true,
	"figcaption": true,
	"figure":     true,
	"footer":     true,
	"form":       true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"header":     true,
	"hr":         true,
	"li":         true,
	"main":       true,
	"nav":        true,
	"ol":         true,
	"p":          true,
	"pre":        true,
	"section":    true,
	"table":      true,
	"td":         true,
	"th":         true,
	"tr":         true,
	"ul":         true,
}

// skipTags never contribute text: script and style would leak their source,
// head its title and metadata.
var skipTags = map[string]bool{
	"head":   true,
	"script": true,
	"style":  true,
}

func descriptionText(s string) string {
	if !htmlTag.MatchString(s) {
		return s
	}
	return htmlToText(s)
}

var whitespaceAroundNewline = regexp.MustCompile(`\s*\n\s*`)

func htmlToText(s string) string {
	var out strings.Builder
	var skip int
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case html.ErrorToken:
			text := whitespaceAroundNewline.ReplaceAllString(out.String(), "\n")
			return strings.TrimSpace(strings.ReplaceAll(text, "\u00a0", " "))
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			if skipTags[string(name)] {
				skip++
			} else if blockTags[string(name)] {
				out.WriteByte('\n')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if skipTags[string(name)] {
				if skip > 0 {
					skip--
				}
			} else if blockTags[string(name)] {
				out.WriteByte('\n')
			}
		case html.TextToken:
			if skip == 0 {
				// Text() decodes entities (&amp; and friends) already.
				out.Write(z.Text())
			}
		case html.CommentToken, html.DoctypeToken:
		}
	}
}
