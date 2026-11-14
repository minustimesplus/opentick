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
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"time"

	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/modules/imageutils"
	"code.vikunja.io/api/pkg/web"

	"github.com/disintegration/imaging"
	"xorm.io/xorm"
)

// contactPhotoSize is the edge length photos are scaled down to.
const contactPhotoSize = 512

// MaxContactPhotoBytes is the largest photo upload accepted.
const MaxContactPhotoBytes = 8 * 1024 * 1024

func deleteContactPhotoFile(s *xorm.Session, fileID int64) error {
	f := &files.File{ID: fileID}
	if err := f.Delete(s); err != nil && !files.IsErrFileDoesNotExist(err) {
		return err
	}
	return nil
}

// SetPhoto stores an image as the contact's photo, scaled down and
// re-encoded as JPEG, replacing the previous one. byHand locks the photo
// against enrichment. Callers must have checked CanUpdate.
func (c *Contact) SetPhoto(s *xorm.Session, a web.Auth, src io.Reader, byHand bool) error {
	data, err := io.ReadAll(io.LimitReader(src, MaxContactPhotoBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxContactPhotoBytes {
		return ErrContactPhotoInvalid{}
	}
	// Checks the dimensions before decoding, so a decompression bomb is refused.
	if _, err := imageutils.ValidateReader(bytes.NewReader(data)); err != nil {
		return ErrContactPhotoInvalid{}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ErrContactPhotoInvalid{}
	}
	buf := &bytes.Buffer{}
	fitted := imaging.Fit(img, contactPhotoSize, contactPhotoSize, imaging.Lanczos)
	// JPEG has no alpha: flatten transparent images onto white.
	flat := imaging.New(fitted.Bounds().Dx(), fitted.Bounds().Dy(), image.White)
	flat = imaging.Overlay(flat, fitted, image.Point{}, 1)
	if err := jpeg.Encode(buf, flat, &jpeg.Options{Quality: 86}); err != nil {
		return err
	}

	stored := &Contact{ID: c.ID}
	if err := stored.load(s); err != nil {
		return err
	}
	if stored.ImageFileID != 0 {
		if err := deleteContactPhotoFile(s, stored.ImageFileID); err != nil {
			return err
		}
	}
	f, err := files.CreateWithMimeAndSession(s, bytes.NewReader(buf.Bytes()), "contact.jpg", uint64(buf.Len()), a, "image/jpeg", true)
	if err != nil {
		return err
	}
	stored.ImageFileID = f.ID
	if byHand {
		stored.lock(ContactLockPhoto)
	}
	stored.Updated = time.Now()
	if _, err := s.ID(stored.ID).Cols("image_file_id", "locked_fields", "updated").Update(stored); err != nil {
		return err
	}
	c.ImageFileID = stored.ImageFileID
	c.LockedFields = stored.LockedFields
	c.present()
	return nil
}

// RemovePhoto deletes the contact's photo. Callers must have checked CanUpdate.
func (c *Contact) RemovePhoto(s *xorm.Session) error {
	stored := &Contact{ID: c.ID}
	if err := stored.load(s); err != nil {
		return err
	}
	if stored.ImageFileID == 0 {
		return nil
	}
	if err := deleteContactPhotoFile(s, stored.ImageFileID); err != nil {
		return err
	}
	stored.ImageFileID = 0
	stored.lock(ContactLockPhoto)
	stored.Updated = time.Now()
	_, err := s.ID(stored.ID).Cols("image_file_id", "locked_fields", "updated").Update(stored)
	return err
}

// Photo returns the contact's photo as JPEG bytes. Callers must have checked CanRead.
func (c *Contact) Photo() ([]byte, error) {
	if c.ImageFileID == 0 {
		return nil, ErrContactHasNoPhoto{ID: c.ID}
	}
	f := &files.File{ID: c.ImageFileID}
	if err := f.LoadFileByID(); err != nil {
		if files.IsErrFileDoesNotExist(err) {
			return nil, ErrContactHasNoPhoto{ID: c.ID}
		}
		return nil, err
	}
	defer f.File.Close()
	return io.ReadAll(f.File)
}
