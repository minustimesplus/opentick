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
	"strings"

	"xorm.io/xorm"
)

// The functions in this file change a contact's history and notes. Callers
// must have checked CanUpdate on the contact.

func (e *ContactExperience) clean() {
	e.Company = strings.TrimSpace(e.Company)
	e.Title = strings.TrimSpace(e.Title)
	e.Location = strings.TrimSpace(e.Location)
	e.StartDate = strings.TrimSpace(e.StartDate)
	e.EndDate = strings.TrimSpace(e.EndDate)
	e.Description = strings.TrimSpace(e.Description)
}

func (e *ContactEducation) clean() {
	e.School = strings.TrimSpace(e.School)
	e.Degree = strings.TrimSpace(e.Degree)
	e.FieldOfStudy = strings.TrimSpace(e.FieldOfStudy)
	e.StartYear = strings.TrimSpace(e.StartYear)
	e.EndYear = strings.TrimSpace(e.EndYear)
}

// handEdited locks one of the contact's lists against imports and bumps its
// updated timestamp.
func (c *Contact) handEdited(s *xorm.Session, lock string) error {
	stored := &Contact{ID: c.ID}
	if err := stored.load(s); err != nil {
		return err
	}
	if !stored.isLocked(lock) {
		stored.lock(lock)
		if err := stored.saveLocks(s); err != nil {
			return err
		}
	}
	return stored.touch(s)
}

// AddExperience appends a job to the contact's history.
func (c *Contact) AddExperience(s *xorm.Session, e *ContactExperience) error {
	e.clean()
	if e.Company == "" && e.Title == "" {
		return ErrContactEntryEmpty{}
	}
	count, err := s.Where("contact_id = ?", c.ID).Count(&ContactExperience{})
	if err != nil {
		return err
	}
	e.ID = 0
	e.ContactID = c.ID
	e.SortOrder = int(count)
	// A current job belongs on top of the list.
	if e.EndDate == "" || strings.EqualFold(e.EndDate, "present") {
		if _, err := s.Where("contact_id = ?", c.ID).Incr("sort_order").Update(&ContactExperience{}); err != nil {
			return err
		}
		e.SortOrder = 0
	}
	if _, err := s.Insert(e); err != nil {
		return err
	}
	return c.handEdited(s, ContactLockExperiences)
}

// UpdateExperience changes one job of the contact's history.
func (c *Contact) UpdateExperience(s *xorm.Session, e *ContactExperience) error {
	e.clean()
	if e.Company == "" && e.Title == "" {
		return ErrContactEntryEmpty{}
	}
	updated, err := s.
		Where("id = ? AND contact_id = ?", e.ID, c.ID).
		Cols("company", "title", "location", "start_date", "end_date", "description").
		Update(e)
	if err != nil {
		return err
	}
	if updated == 0 {
		exists, err := s.Where("id = ? AND contact_id = ?", e.ID, c.ID).Exist(&ContactExperience{})
		if err != nil {
			return err
		}
		if !exists {
			return ErrContactEntryDoesNotExist{ID: e.ID}
		}
	}
	if _, err := s.ID(e.ID).Get(e); err != nil {
		return err
	}
	return c.handEdited(s, ContactLockExperiences)
}

// DeleteExperience removes one job from the contact's history.
func (c *Contact) DeleteExperience(s *xorm.Session, id int64) error {
	deleted, err := s.Where("id = ? AND contact_id = ?", id, c.ID).Delete(&ContactExperience{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrContactEntryDoesNotExist{ID: id}
	}
	return c.handEdited(s, ContactLockExperiences)
}

// AddEducation appends a school to the contact's education.
func (c *Contact) AddEducation(s *xorm.Session, e *ContactEducation) error {
	e.clean()
	if e.School == "" {
		return ErrContactEntryEmpty{}
	}
	count, err := s.Where("contact_id = ?", c.ID).Count(&ContactEducation{})
	if err != nil {
		return err
	}
	e.ID = 0
	e.ContactID = c.ID
	e.SortOrder = int(count)
	if _, err := s.Insert(e); err != nil {
		return err
	}
	return c.handEdited(s, ContactLockEducations)
}

// UpdateEducation changes one school of the contact's education.
func (c *Contact) UpdateEducation(s *xorm.Session, e *ContactEducation) error {
	e.clean()
	if e.School == "" {
		return ErrContactEntryEmpty{}
	}
	updated, err := s.
		Where("id = ? AND contact_id = ?", e.ID, c.ID).
		Cols("school", "degree", "field_of_study", "start_year", "end_year").
		Update(e)
	if err != nil {
		return err
	}
	if updated == 0 {
		exists, err := s.Where("id = ? AND contact_id = ?", e.ID, c.ID).Exist(&ContactEducation{})
		if err != nil {
			return err
		}
		if !exists {
			return ErrContactEntryDoesNotExist{ID: e.ID}
		}
	}
	if _, err := s.ID(e.ID).Get(e); err != nil {
		return err
	}
	return c.handEdited(s, ContactLockEducations)
}

// DeleteEducation removes one school from the contact's education.
func (c *Contact) DeleteEducation(s *xorm.Session, id int64) error {
	deleted, err := s.Where("id = ? AND contact_id = ?", id, c.ID).Delete(&ContactEducation{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrContactEntryDoesNotExist{ID: id}
	}
	return c.handEdited(s, ContactLockEducations)
}

// AddNote adds a note to the contact.
func (c *Contact) AddNote(s *xorm.Session, n *ContactNote) error {
	n.Body = strings.TrimSpace(n.Body)
	if n.Body == "" {
		return ErrContactEntryEmpty{}
	}
	n.ID = 0
	n.ContactID = c.ID
	if _, err := s.Insert(n); err != nil {
		return err
	}
	return c.touch(s)
}

// UpdateNote changes the text of one of the contact's notes.
func (c *Contact) UpdateNote(s *xorm.Session, n *ContactNote) error {
	n.Body = strings.TrimSpace(n.Body)
	if n.Body == "" {
		return ErrContactEntryEmpty{}
	}
	exists, err := s.Where("id = ? AND contact_id = ?", n.ID, c.ID).Exist(&ContactNote{})
	if err != nil {
		return err
	}
	if !exists {
		return ErrContactEntryDoesNotExist{ID: n.ID}
	}
	if _, err := s.ID(n.ID).Cols("body").Update(n); err != nil {
		return err
	}
	if _, err := s.ID(n.ID).Get(n); err != nil {
		return err
	}
	return c.touch(s)
}

// DeleteNote removes one of the contact's notes.
func (c *Contact) DeleteNote(s *xorm.Session, id int64) error {
	deleted, err := s.Where("id = ? AND contact_id = ?", id, c.ID).Delete(&ContactNote{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrContactEntryDoesNotExist{ID: id}
	}
	return c.touch(s)
}
