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

package caldav

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoRrule(t *testing.T) {
	due := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	assert.Empty(t, todoRrule("", 0, models.TaskRepeatModeDefault, due))
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,FR", todoRrule("FREQ=WEEKLY;BYDAY=MO,FR", 0, models.TaskRepeatModeDefault, due))
	assert.Equal(t, "FREQ=DAILY;INTERVAL=3", todoRrule("FREQ=DAILY;INTERVAL=3", 0, models.TaskRepeatModeFromCurrentDate, due), "after completion exports as the plain rule")
	assert.Equal(t, "FREQ=MONTHLY;COUNT=2", todoRrule("FREQ=MONTHLY;COUNT=2", 86400, models.TaskRepeatModeMonth, due), "the rule wins over legacy fields")
	assert.Equal(t, "FREQ=MONTHLY;BYMONTHDAY=05", todoRrule("", 0, models.TaskRepeatModeMonth, due))
	assert.Equal(t, "FREQ=WEEKLY;INTERVAL=2", todoRrule("", 14*86400, models.TaskRepeatModeDefault, due))
}

func TestParseVTODORrule(t *testing.T) {
	tests := []struct {
		in        string
		wantRule  string
		wantAfter int64
		wantErr   bool
	}{
		{in: "FREQ=WEEKLY;INTERVAL=2;BYDAY=FR,MO", wantRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,FR"},
		{in: "FREQ=MONTHLY;BYDAY=-1FR;COUNT=4", wantRule: "FREQ=MONTHLY;BYDAY=-1FR;COUNT=4"},
		{in: "FREQ=DAILY;UNTIL=20261231T225959Z", wantRule: "FREQ=DAILY;UNTIL=20261231T225959Z"},
		{in: "FREQ=HOURLY;INTERVAL=6", wantAfter: 6 * 3600},
		{in: "FREQ=MINUTELY", wantAfter: 60},
		{in: "FREQ=SECONDLY;INTERVAL=435", wantAfter: 435},
		{in: "", wantRule: ""},
		{in: "FREQ=HOURLY;BYHOUR=9", wantErr: true},
		{in: "FREQ=HOURLY;INTERVAL=999999999", wantErr: true},
		{in: "FREQ=MONTHLY;BYSETPOS=-1;BYDAY=MO,TU,WE,TH,FR", wantErr: true},
		{in: "nonsense", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			rule, after, err := parseVTODORrule(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRule, rule)
			assert.Equal(t, tc.wantAfter, after)
		})
	}
}

func TestSameRepeat(t *testing.T) {
	due := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	legacyDaily := &models.Task{RepeatAfter: 86400, DueDate: due}
	assert.True(t, SameRepeat(legacyDaily, &models.Task{RepeatRule: "FREQ=DAILY"}), "echoed legacy export")
	assert.False(t, SameRepeat(legacyDaily, &models.Task{RepeatRule: "FREQ=DAILY;INTERVAL=2"}))

	legacyMonthly := &models.Task{RepeatMode: models.TaskRepeatModeMonth, DueDate: due}
	assert.True(t, SameRepeat(legacyMonthly, &models.Task{RepeatRule: "FREQ=MONTHLY;BYMONTHDAY=5"}))

	legacySeconds := &models.Task{RepeatAfter: 435}
	assert.True(t, SameRepeat(legacySeconds, &models.Task{RepeatAfter: 435}))

	afterCompletion := &models.Task{RepeatRule: "FREQ=WEEKLY", RepeatMode: models.TaskRepeatModeFromCurrentDate}
	assert.True(t, SameRepeat(afterCompletion, &models.Task{RepeatRule: "FREQ=WEEKLY"}))

	assert.True(t, SameRepeat(&models.Task{}, &models.Task{}))
	assert.False(t, SameRepeat(&models.Task{}, &models.Task{RepeatRule: "FREQ=DAILY"}))
}

func TestParseTaskFromVTODORrule(t *testing.T) {
	vtodo := func(extra string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:repeat-test\r\nSUMMARY:Repeats\r\n" +
			"DUE:20261005T090000Z\r\n" + extra + "END:VTODO\r\nEND:VCALENDAR\r\n"
	}

	task, props, err := ParseTaskFromVTODO(vtodo("RRULE:FREQ=WEEKLY;BYDAY=FR,MO;UNTIL=20261231T000000Z\r\n"))
	require.NoError(t, err)
	assert.True(t, props.Repeat)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,FR;UNTIL=20261231T000000Z", task.RepeatRule)
	assert.Zero(t, task.RepeatAfter)

	task, props, err = ParseTaskFromVTODO(vtodo("RRULE:FREQ=HOURLY;INTERVAL=2\r\n"))
	require.NoError(t, err)
	assert.True(t, props.Repeat)
	assert.Empty(t, task.RepeatRule)
	assert.Equal(t, int64(7200), task.RepeatAfter)

	task, props, err = ParseTaskFromVTODO(vtodo("RRULE:FREQ=MONTHLY;BYSETPOS=1;BYDAY=MO\r\n"))
	require.NoError(t, err, "an unsupported rule must not fail the sync")
	assert.False(t, props.Repeat)
	assert.Empty(t, task.RepeatRule)

	_, props, err = ParseTaskFromVTODO(vtodo(""))
	require.NoError(t, err)
	assert.False(t, props.Repeat, "no RRULE leaves the stored repeat alone")
}

func TestParseTodosExportsRepeatRule(t *testing.T) {
	out := ParseTodos(&Config{Name: "test", ProdID: "test"}, []*Todo{{
		UID:        "uid",
		Summary:    "Repeats",
		Timestamp:  time.Unix(1543626724, 0),
		DueDate:    time.Unix(1543626724, 0),
		RepeatRule: "FREQ=MONTHLY;BYDAY=2TU;COUNT=3",
	}})
	assert.Contains(t, out, "\nRRULE:FREQ=MONTHLY;BYDAY=2TU;COUNT=3\n")
}
