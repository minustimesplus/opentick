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
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// SkipTaskOccurrence drops only the current occurrence of a repeating task: the
// task moves on to its next occurrence the way completing it would, but nothing
// is recorded as done. When the current occurrence is the last one of its
// series, nothing is left, so the task goes to the trash and deleted is true.
// The caller checks write access to the task.
func SkipTaskOccurrence(s *xorm.Session, a web.Auth, taskID int64) (task *Task, deleted bool, err error) {
	ot := &Task{ID: taskID}
	if err := ot.ReadOne(s, a); err != nil {
		return nil, false, err
	}
	if !ot.isRepeating() {
		return nil, false, ErrTaskNotRepeating{TaskID: taskID}
	}

	loc := repeatLocation(s, a)
	if repeatSeriesEndsOnCompletion(ot, time.Now(), loc) {
		if err := (&Task{ID: taskID}).Delete(s, a); err != nil {
			return nil, false, err
		}
		return ot, true, nil
	}

	// The repeat helpers shift the reminders in place, so they get a copy and ot stays untouched.
	next := *ot
	next.Reminders = make([]*TaskReminder, 0, len(ot.Reminders))
	for _, r := range ot.Reminders {
		rc := *r
		next.Reminders = append(next.Reminders, &rc)
	}
	prev := *ot
	prev.Reminders = next.Reminders
	next.Done = true
	updateDoneIn(&prev, &next, loc)
	// Skipping is not completing: the task stays open and keeps its done state.
	next.Done = false
	next.DoneAt = time.Time{}

	fields := []string{"due_date", "start_date", "end_date", "repeat_rule", "description"}
	if err := next.updateSingleTask(s, a, fields); err != nil {
		return nil, false, err
	}
	if err := next.ReadOne(s, a); err != nil {
		return nil, false, err
	}
	return &next, false, nil
}

// IsRepeating reports whether the task repeats, by rule, interval or monthly.
func (t *Task) IsRepeating() bool {
	return t.isRepeating()
}

// UpdateColumns saves only the given columns of t, the way a partial update does.
// Reminders and assignees are always taken from t, so t should be fully loaded.
// The caller checks write access.
func (t *Task) UpdateColumns(s *xorm.Session, a web.Auth, columns []string) error {
	return t.updateSingleTask(s, a, columns)
}
