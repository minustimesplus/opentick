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

package handler

import (
	"bytes"
	"io"
	"os"

	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/user"

	"xorm.io/xorm"
)

// ValidateAndSaveUserBackground stores an uploaded image as the user's
// app-wide background, replacing (and deleting) any previous one. It goes
// through the same validation and resizing as project backgrounds.
func ValidateAndSaveUserBackground(s *xorm.Session, u *user.User, srcf io.ReadSeeker, filename string) error {
	if err := validateBackgroundMime(srcf); err != nil {
		return err
	}
	buf, _, err := encodeBackgroundImage(srcf)
	if err != nil {
		return err
	}

	f, err := files.CreateWithSession(s, bytes.NewReader(buf.Bytes()), filename, uint64(buf.Len()), u)
	if err != nil {
		return err
	}

	if err := deleteUserBackgroundFile(s, u); err != nil {
		return err
	}
	return setUserBackgroundFileID(s, u, f.ID)
}

// RemoveUserBackground deletes the user's background. It is a no-op when the
// user has none.
func RemoveUserBackground(s *xorm.Session, u *user.User) error {
	if u.BackgroundFileID == 0 {
		return nil
	}
	if err := deleteUserBackgroundFile(s, u); err != nil {
		return err
	}
	return setUserBackgroundFileID(s, u, 0)
}

// LoadUserBackgroundForDownload opens the user's background file. The caller
// must close bgFile.File.
func LoadUserBackgroundForDownload(u *user.User) (bgFile *files.File, stat os.FileInfo, err error) {
	if u.BackgroundFileID == 0 {
		return nil, nil, &user.ErrUserHasNoBackground{UserID: u.ID}
	}

	bgFile = &files.File{ID: u.BackgroundFileID}
	if err := bgFile.LoadFileByID(); err != nil {
		return nil, nil, err
	}
	stat, err = files.FileStat(bgFile)
	if err != nil {
		_ = bgFile.File.Close()
		return nil, nil, err
	}
	return bgFile, stat, nil
}

func deleteUserBackgroundFile(s *xorm.Session, u *user.User) error {
	if u.BackgroundFileID == 0 {
		return nil
	}
	f := &files.File{ID: u.BackgroundFileID}
	if err := f.Delete(s); err != nil && !files.IsErrFileDoesNotExist(err) {
		return err
	}
	return nil
}

func setUserBackgroundFileID(s *xorm.Session, u *user.User, fileID int64) error {
	u.BackgroundFileID = fileID
	_, err := s.
		Where("id = ?", u.ID).
		Cols("background_file_id").
		NoAutoTime().
		Update(u)
	return err
}
