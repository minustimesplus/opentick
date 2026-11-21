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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// LinkedinPhotoURLAllowed reports whether a profile photo URL points at
// LinkedIn's image CDN. The server fetches these URLs itself, so anything
// else is refused to keep the extension's payload from making the server
// request arbitrary hosts.
func LinkedinPhotoURLAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "licdn.com" || strings.HasSuffix(host, ".licdn.com")
}

var linkedinPhotoClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !LinkedinPhotoURLAllowed(req.URL.String()) {
			return errors.New("redirect away from LinkedIn's image CDN")
		}
		return nil
	},
}

// SaveLinkedinPhoto downloads a LinkedIn profile photo and stores it as the
// contact's photo, unless the contact already has one or its photo is locked.
// It reports whether a photo was stored.
func SaveLinkedinPhoto(ctx context.Context, s *xorm.Session, a web.Auth, contactID int64, photoURL string) (bool, error) {
	if !LinkedinPhotoURLAllowed(photoURL) {
		return false, nil
	}
	c, err := LoadOwnedContact(s, a, contactID)
	if err != nil {
		return false, err
	}
	if c.ImageFileID != 0 || c.isLocked(ContactLockPhoto) {
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, nil)
	if err != nil {
		return false, err
	}
	res, err := linkedinPhotoClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("photo download failed with status %d", res.StatusCode)
	}
	if err := c.SetPhoto(s, a, io.LimitReader(res.Body, MaxContactPhotoBytes+1), false); err != nil {
		return false, err
	}
	return true, nil
}
