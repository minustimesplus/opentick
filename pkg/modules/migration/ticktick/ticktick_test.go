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

package ticktick

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/modules/migration"

	"code.vikunja.io/api/pkg/models"
	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertTicktickTasksToVikunja(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	at := func(s string) tickTickTime {
		parsed, err := time.Parse("2006-01-02T15:04:05-0700", s)
		require.NoError(t, err)
		return tickTickTime{Time: parsed}
	}

	tickTickTasks := []*tickTickTask{
		{
			TaskID:      1,
			ProjectName: "Project 1",
			Title:       "Timed with duration",
			Tags:        []string{"label1", "label2"},
			Content:     "line one\rline two\r\r",
			StartDate:   at("2026-11-12T13:30:00+0000"),
			DueDate:     at("2026-11-12T15:30:00+0000"),
			Reminders:   []time.Duration{-10 * time.Minute, 0},
			Repeat:      "FREQ=WEEKLY;INTERVAL=1;BYDAY=WE",
			Priority:    5,
			Status:      "0",
			IsAllDay:    "false",
			Order:       -1099511627776,
		},
		{
			TaskID:        2,
			ParentID:      1,
			ProjectName:   "Project 1",
			Title:         "Subtask",
			Status:        "2",
			CompletedTime: at("2026-10-16T11:51:08+0000"),
			Priority:      3,
		},
		{
			TaskID:      3,
			ProjectName: "Project 1",
			Title:       "All-day",
			StartDate:   at("2026-10-08T22:00:00+0000"),
			DueDate:     at("2026-10-08T22:00:00+0000"),
			Reminders:   []time.Duration{9 * time.Hour},
			Timezone:    "Europe/Paris",
			IsAllDay:    "true",
			Priority:    1,
		},
		{
			TaskID:      4,
			FolderName:  "Folder",
			ProjectName: "Project 2",
			Title:       "Multi-day all-day",
			StartDate:   at("2025-07-06T23:00:00+0000"),
			DueDate:     at("2025-07-11T23:00:00+0000"),
			Timezone:    "Europe/London",
			IsAllDay:    "true",
			ColumnName:  "Doing",
			ColumnOrder: 5,
		},
	}

	projects := convertTickTickToVikunja(tickTickTasks, berlin)

	require.Len(t, projects, 3)
	project1, folder, project2 := projects[0], projects[1], projects[2]
	assert.Equal(t, "Project 1", project1.Title)
	assert.Nil(t, project1.ParentProjectID, "lists outside a folder are top-level")
	assert.Equal(t, "Folder", folder.Title)
	assert.Equal(t, "Project 2", project2.Title)
	require.NotNil(t, project2.ParentProjectID)
	assert.Equal(t, folder.ID, *project2.ParentProjectID)
	require.Len(t, project1.Tasks, 3)

	timed := project1.Tasks[0]
	assert.Equal(t, "line one\nline two", timed.Description)
	assert.Equal(t, tickTickTasks[0].StartDate.Time, timed.DueDate)
	assert.Equal(t, tickTickTasks[0].StartDate.Time, timed.StartDate)
	assert.Equal(t, tickTickTasks[0].DueDate.Time, timed.EndDate)
	assert.Equal(t, []*models.Label{{Title: "label1"}, {Title: "label2"}}, timed.Labels)
	require.Len(t, timed.Reminders, 2)
	assert.Equal(t, models.ReminderRelationDueDate, timed.Reminders[0].RelativeTo)
	assert.Equal(t, int64(-600), timed.Reminders[0].RelativePeriod)
	assert.Equal(t, int64(0), timed.Reminders[1].RelativePeriod)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=WE", timed.RepeatRule)
	assert.Equal(t, int64(3), timed.Priority, "TickTick high (5) is high")
	assert.Equal(t, tickTickTasks[0].Order, timed.Position)
	assert.False(t, timed.Done)

	sub := project1.Tasks[1]
	assert.True(t, sub.Done)
	assert.Equal(t, tickTickTasks[1].CompletedTime.Time, sub.DoneAt)
	assert.Equal(t, int64(2), sub.Priority, "TickTick medium (3) is medium")
	assert.Equal(t, models.RelatedTaskMap{
		models.RelationKindParenttask: []*models.Task{{ID: 1}},
	}, sub.RelatedTasks)

	allDay := project1.Tasks[2]
	assert.Equal(t, time.Date(2026, 10, 9, 23, 59, 59, 0, berlin), allDay.DueDate)
	assert.True(t, allDay.StartDate.IsZero())
	assert.True(t, allDay.EndDate.IsZero())
	require.Len(t, allDay.Reminders, 1)
	// 09:00 on the day, relative to the 23:59:59 due date.
	assert.Equal(t, int64(-(14*3600 + 59*60 + 59)), allDay.Reminders[0].RelativePeriod)
	assert.Equal(t, int64(1), allDay.Priority)

	require.Len(t, project2.Tasks, 1)
	multi := project2.Tasks[0]
	assert.Equal(t, time.Date(2025, 7, 7, 0, 0, 0, 0, berlin), multi.StartDate)
	assert.Equal(t, time.Date(2025, 7, 11, 23, 59, 59, 0, berlin), multi.EndDate)
	assert.Equal(t, time.Date(2025, 7, 7, 23, 59, 59, 0, berlin), multi.DueDate)
	require.Len(t, project2.Buckets, 1)
	assert.Equal(t, "Doing", project2.Buckets[0].Title)
	assert.Equal(t, project2.Buckets[0].ID, multi.BucketID)
}

func TestConvertTicktickTasksChildBeforeParent(t *testing.T) {
	// Child appears BEFORE parent in the input — this is the order that
	// causes the import to fail in create_from_structure.go because the
	// placeholder for the not-yet-created parent has no title.
	tickTickTasks := []*tickTickTask{
		{
			TaskID:      2,
			ParentID:    1,
			ProjectName: "Project 1",
			Title:       "Child task",
			Status:      "0",
			Order:       -1099511626,
		},
		{
			TaskID:      1,
			ParentID:    0,
			ProjectName: "Project 1",
			Title:       "Parent task",
			Status:      "0",
			Order:       -1099511627776,
		},
	}

	vikunjaTasks := convertTickTickToVikunja(tickTickTasks, time.UTC)

	// Find the project with tasks
	var projectTasks []*models.TaskWithComments
	for _, p := range vikunjaTasks {
		if len(p.Tasks) > 0 {
			projectTasks = p.Tasks
			break
		}
	}

	require.Len(t, projectTasks, 2)

	// The parent (TaskID=1) must come before the child (TaskID=2) in the
	// output so that create_from_structure.go processes it first.
	assert.Equal(t, "Parent task", projectTasks[0].Title)
	assert.Equal(t, "Child task", projectTasks[1].Title)

	// The child still has the correct parent relation
	assert.Equal(t, models.RelatedTaskMap{
		models.RelationKindParenttask: []*models.Task{
			{ID: 1},
		},
	}, projectTasks[1].RelatedTasks)
}

func TestConvertTicktickWontDoTask(t *testing.T) {
	tickTickTasks := []*tickTickTask{
		{TaskID: 1, ProjectName: "Project 1", Title: "Abandoned", Status: "-1"},
		{TaskID: 2, ProjectName: "Project 1", Title: "Completed", Status: "1"},
	}

	vikunjaTasks := convertTickTickToVikunja(tickTickTasks, time.UTC)

	var projectTasks []*models.TaskWithComments
	for _, p := range vikunjaTasks {
		if len(p.Tasks) > 0 {
			projectTasks = p.Tasks
			break
		}
	}
	require.Len(t, projectTasks, 2)

	for _, task := range projectTasks {
		switch task.Title {
		case "Abandoned":
			assert.True(t, task.Done, "won't do tasks are closed")
			assert.True(t, task.WontDo)
		case "Completed":
			assert.True(t, task.Done)
			assert.False(t, task.WontDo)
		}
	}
}

func TestConvertTicktickTasksDeeplyNested(t *testing.T) {
	// Grandchild -> child -> parent, all in reverse order
	tickTickTasks := []*tickTickTask{
		{
			TaskID:      3,
			ParentID:    2,
			ProjectName: "Project 1",
			Title:       "Grandchild",
			Status:      "0",
		},
		{
			TaskID:      2,
			ParentID:    1,
			ProjectName: "Project 1",
			Title:       "Child",
			Status:      "0",
		},
		{
			TaskID:      1,
			ParentID:    0,
			ProjectName: "Project 1",
			Title:       "Root",
			Status:      "0",
		},
	}

	vikunjaTasks := convertTickTickToVikunja(tickTickTasks, time.UTC)

	var projectTasks []*models.TaskWithComments
	for _, p := range vikunjaTasks {
		if len(p.Tasks) > 0 {
			projectTasks = p.Tasks
			break
		}
	}

	require.Len(t, projectTasks, 3)
	assert.Equal(t, "Root", projectTasks[0].Title)
	assert.Equal(t, "Child", projectTasks[1].Title)
	assert.Equal(t, "Grandchild", projectTasks[2].Title)
}

// TestSortParentsBeforeChildrenWithCycle guards against a regression where a
// parentId cycle made place() recurse forever. That is a stack overflow, which
// Go raises as a fatal error the recover middleware cannot catch, so it took
// down the whole process instead of failing the one import request.
func TestSortParentsBeforeChildrenWithCycle(t *testing.T) {
	tests := []struct {
		name           string
		tasks          []*tickTickTask
		expectedTitles []string
	}{
		{
			name: "two tasks pointing at each other",
			tasks: []*tickTickTask{
				{TaskID: 1, ParentID: 2, ProjectName: "Project 1", Title: "Task A"},
				{TaskID: 2, ParentID: 1, ProjectName: "Project 1", Title: "Task B"},
			},
			expectedTitles: []string{"Task A", "Task B"},
		},
		{
			name: "task is its own parent",
			tasks: []*tickTickTask{
				{TaskID: 1, ParentID: 1, ProjectName: "Project 1", Title: "Own parent"},
			},
			expectedTitles: []string{"Own parent"},
		},
		{
			name: "longer cycle",
			tasks: []*tickTickTask{
				{TaskID: 1, ParentID: 3, ProjectName: "Project 1", Title: "Task A"},
				{TaskID: 2, ParentID: 1, ProjectName: "Project 1", Title: "Task B"},
				{TaskID: 3, ParentID: 2, ProjectName: "Project 1", Title: "Task C"},
			},
			expectedTitles: []string{"Task A", "Task B", "Task C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := sortParentsBeforeChildren(tt.tasks)

			titles := make([]string, 0, len(sorted))
			for _, task := range sorted {
				titles = append(titles, task.Title)
			}
			assert.ElementsMatch(t, tt.expectedTitles, titles, "every input task must be returned exactly once")

			vikunjaTasks := convertTickTickToVikunja(tt.tasks, time.UTC)
			convertedTitles := []string{}
			for _, project := range vikunjaTasks {
				for _, task := range project.Tasks {
					convertedTitles = append(convertedTitles, task.Title)
				}
			}
			assert.ElementsMatch(t, tt.expectedTitles, convertedTitles)
		})
	}
}

// TestSortParentsBeforeChildrenCycleDoesNotAffectOtherTasks makes sure breaking
// a cycle still leaves the ordering of unrelated, acyclic tasks intact.
func TestSortParentsBeforeChildrenCycleDoesNotAffectOtherTasks(t *testing.T) {
	tasks := []*tickTickTask{
		{TaskID: 1, ParentID: 2, ProjectName: "Project 1", Title: "Cycle A"},
		{TaskID: 3, ParentID: 4, ProjectName: "Project 1", Title: "Child"},
		{TaskID: 2, ParentID: 1, ProjectName: "Project 1", Title: "Cycle B"},
		{TaskID: 4, ParentID: 0, ProjectName: "Project 1", Title: "Parent"},
	}

	sorted := sortParentsBeforeChildren(tasks)
	require.Len(t, sorted, 4)

	positions := make(map[string]int, len(sorted))
	for i, task := range sorted {
		positions[task.Title] = i
	}
	assert.Less(t, positions["Parent"], positions["Child"])
}

func TestLinesToSkipBeforeHeader(t *testing.T) {
	csvContent := "Date: 2024-01-01+0000\nVersion: 7.1\n" +
		"\"Folder Name\",\"List Name\",\"Title\",\"Kind\",\"Tags\",\"Content\",\"Is Check list\",\"Start Date\",\"Due Date\",\"Reminder\",\"Repeat\",\"Priority\",\"Status\",\"Created Time\",\"Completed Time\",\"Order\",\"Timezone\",\"Is All Day\",\"Is Floating\",\"Column Name\",\"Column Order\",\"View Mode\",\"taskId\",\"parentId\"\n" +
		",\"list\",\"task1\",\"TEXT\",\"\",\"\",\"N\",\"\",\"\",\"\",\"\",\"0\",\"0\",\"2022-10-09T15:09:48+0000\",\"\",\"-1099511627776\",\"\",\"true\",\"false\",,,\"list\",\"1\",\"\"\n"

	r := bytes.NewReader([]byte(csvContent))
	lines, err := linesToSkipBeforeHeader(r, int64(len(csvContent)))
	require.NoError(t, err)
	assert.Equal(t, 2, lines)

	r2 := bytes.NewReader([]byte(csvContent))
	dec, err := newLineSkipDecoder(r2, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "task1", tasks[0].Title)
}

func TestLinesToSkipBeforeHeaderWithRealCSV(t *testing.T) {
	// This is the actual format from a real TickTick export with BOM and multi-line status
	csvContent := "\uFEFF\"Date: 2025-11-25+0000\"\n" +
		"\"Version: 7.1\"\n" +
		"\"Status: \n" +
		"0 Normal\n" +
		"1 Completed\n" +
		"2 Archived\"\n" +
		"\"Folder Name\",\"List Name\",\"Title\",\"Kind\",\"Tags\",\"Content\",\"Is Check list\",\"Start Date\",\"Due Date\",\"Reminder\",\"Repeat\",\"Priority\",\"Status\",\"Created Time\",\"Completed Time\",\"Order\",\"Timezone\",\"Is All Day\",\"Is Floating\",\"Column Name\",\"Column Order\",\"View Mode\",\"taskId\",\"parentId\"\n" +
		"\"dsx\",\"x\",\"this task repeats\",\"TEXT\",\"\",\"\",\"N\",\"\",\"\",\"\",\"\",\"0\",\"0\",\"2022-10-09T15:09:48+0000\",\"\",\"-1099511627776\",\"Europe/Berlin\",,\"false\",,,\"list\",\"2\",\"\"\n"

	t.Logf("CSV content length: %d", len(csvContent))
	t.Logf("CSV content first 100 chars: %q", csvContent[:100])

	r := bytes.NewReader([]byte(csvContent))
	lines, err := linesToSkipBeforeHeader(r, int64(len(csvContent)))
	require.NoError(t, err)
	t.Logf("Lines to skip: %d", lines)
	assert.Equal(t, 6, lines) // Should skip 6 lines to get to the header

	r2 := bytes.NewReader([]byte(csvContent))
	dec, err := newLineSkipDecoder(r2, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "this task repeats", tasks[0].Title)
	assert.Equal(t, "dsx", tasks[0].FolderName)
	assert.Equal(t, "x", tasks[0].ProjectName)
}

func TestLinesToSkipBeforeHeaderWithCleanTestFile(t *testing.T) {
	// Test with the cleaned-up test CSV file
	file, err := os.Open("testdata_ticktick_export.csv")
	require.NoError(t, err)
	defer file.Close()

	stat, err := file.Stat()
	require.NoError(t, err)

	lines, err := linesToSkipBeforeHeader(file, stat.Size())
	require.NoError(t, err)
	t.Logf("Lines to skip in test file: %d", lines)
	assert.Equal(t, 6, lines) // Should skip 6 lines to get to the header

	// Reset file position
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	// Let's manually check what the header line looks like after skipping
	r := stripBOM(file)
	scanner := bufio.NewScanner(r)
	for i := 0; i <= lines; i++ {
		if !scanner.Scan() {
			break
		}
		if i == lines {
			t.Logf("Header line after skipping %d lines: %q", lines, scanner.Text())
		}
	}

	// Reset file position again
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	dec, err := newLineSkipDecoder(file, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err)
	require.Greater(t, len(tasks), 0)

	// Verify that the first task has actual data
	assert.Equal(t, "Work", tasks[0].FolderName)
	assert.Equal(t, "Project Alpha", tasks[0].ProjectName)
	assert.Equal(t, "Task with repeating schedule", tasks[0].Title)
}

func TestBOMStripping(t *testing.T) {
	// Test BOM stripping specifically
	csvWithBOM := "\uFEFF\"Folder Name\",\"List Name\",\"Title\"\n\"test\",\"list\",\"task\"\n"

	r := stripBOM(bytes.NewReader([]byte(csvWithBOM)))
	scanner := bufio.NewScanner(r)

	// Read first line (header)
	require.True(t, scanner.Scan())
	header := scanner.Text()
	t.Logf("Header after BOM stripping: %q", header)

	// Read second line (data)
	require.True(t, scanner.Scan())
	data := scanner.Text()
	t.Logf("Data line: %q", data)

	// Test CSV parsing
	r2 := stripBOM(bytes.NewReader([]byte(csvWithBOM)))
	reader := csv.NewReader(r2)
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	t.Logf("CSV records: %+v", records)
}

func TestEmptyLabelHandling(t *testing.T) {
	t.Run("Normal tags", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "work, personal, urgent",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{"work", "personal", "urgent"}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Tags with extra spaces", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "work,  personal  , urgent",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{"work", "personal", "urgent"}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Empty tags mixed with valid ones", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "work, , urgent, ",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{"work", "urgent"}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Only whitespace tags", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    " ,  ,   ",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Empty string", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Single valid tag", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "important",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{"important"}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Single empty tag", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    " ",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})

	t.Run("Tags with leading/trailing spaces", func(t *testing.T) {
		task := &tickTickTask{
			Title:       "Test Task",
			ProjectName: "Test Project",
			TagsList:    "  work  , personal,   urgent   ",
		}
		task.Tags = strings.Split(task.TagsList, ", ")

		vikunjaTasks := convertTickTickToVikunja([]*tickTickTask{task}, time.UTC)
		projectWithTasks := findProjectWithTasks(t, vikunjaTasks)
		vikunjaTask := projectWithTasks.Tasks[0]

		expectedTags := []string{"work", "personal", "urgent"}
		assertLabelsMatch(t, vikunjaTask, expectedTags)
	})
}

// Helper function to find the project that contains tasks
func findProjectWithTasks(t *testing.T, vikunjaTasks []*models.ProjectWithTasksAndBuckets) *models.ProjectWithTasksAndBuckets {
	t.Helper()

	// Lists outside a folder become top-level projects.
	require.Len(t, vikunjaTasks, 1)

	// Find the project with tasks (should be the child project)
	for _, project := range vikunjaTasks {
		if len(project.Tasks) > 0 {
			require.Len(t, project.Tasks, 1)
			return project
		}
	}

	t.Fatal("Should find a project with tasks")
	return nil
}

// Helper function to assert that labels match expected tags
func assertLabelsMatch(t *testing.T, vikunjaTask *models.TaskWithComments, expectedTags []string) {
	t.Helper()

	// Check that only non-empty labels were created
	assert.Len(t, vikunjaTask.Labels, len(expectedTags), "Number of labels should match expected")

	// Check that the label titles match expected tags
	actualTags := make([]string, len(vikunjaTask.Labels))
	for i, label := range vikunjaTask.Labels {
		actualTags[i] = label.Title
	}

	assert.ElementsMatch(t, expectedTags, actualTags, "Label titles should match expected tags")

	// Ensure no empty labels were created
	for _, label := range vikunjaTask.Labels {
		assert.NotEmpty(t, strings.TrimSpace(label.Title), "No label should be empty or whitespace-only")
	}
}

func TestUnmarshalCSVTimeFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "ISO format with timezone offset",
			input:    "2022-10-09T15:09:48+0000",
			expected: time.Date(2022, 10, 9, 15, 9, 48, 0, time.UTC),
		},
		{
			name:     "Space-separated without timezone",
			input:    "2026-02-27 14:59:52",
			expected: time.Date(2026, 2, 27, 14, 59, 52, 0, time.UTC),
		},
		{
			name:     "ISO format with Z suffix",
			input:    "2022-10-09T15:09:48Z",
			expected: time.Date(2022, 10, 9, 15, 9, 48, 0, time.UTC),
		},
		{
			name:     "ISO format with positive offset",
			input:    "2018-12-11T23:00:00+0100",
			expected: time.Date(2018, 12, 11, 23, 0, 0, 0, time.FixedZone("", 3600)),
		},
		{
			name:     "empty string",
			input:    "",
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ttt tickTickTime
			err := ttt.UnmarshalCSV(tt.input)
			require.NoError(t, err)
			assert.True(t, tt.expected.Equal(ttt.Time), "expected %v, got %v", tt.expected, ttt.Time)
		})
	}

	t.Run("invalid format returns error", func(t *testing.T) {
		var ttt tickTickTime
		err := ttt.UnmarshalCSV("not-a-date")
		require.Error(t, err)
	})
}

func TestSpaceSeparatedDatesCSV(t *testing.T) {
	file, err := os.Open("testdata_ticktick_space_dates.csv")
	require.NoError(t, err)
	defer file.Close()

	stat, err := file.Stat()
	require.NoError(t, err)

	lines, err := linesToSkipBeforeHeader(file, stat.Size())
	require.NoError(t, err)
	assert.Equal(t, 6, lines)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	dec, err := newLineSkipDecoder(file, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	// First task: has start date, due date, and created time in space-separated format
	assert.Equal(t, "Task with space dates", tasks[0].Title)
	assert.Equal(t, time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC), tasks[0].StartDate.Time)
	assert.Equal(t, time.Date(2026, 2, 27, 14, 59, 52, 0, time.UTC), tasks[0].DueDate.Time)
	assert.Equal(t, time.Date(2026, 2, 15, 9, 30, 0, 0, time.UTC), tasks[0].CreatedTime.Time)
	assert.True(t, tasks[0].CompletedTime.IsZero())

	// Second task: completed, has created time and completed time
	assert.Equal(t, "Completed task with space dates", tasks[1].Title)
	assert.Equal(t, time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC), tasks[1].CreatedTime.Time)
	assert.Equal(t, time.Date(2026, 2, 25, 16, 45, 30, 0, time.UTC), tasks[1].CompletedTime.Time)

	// Verify the tasks convert to Vikunja format without error
	for _, task := range tasks {
		task.Tags = strings.Split(task.TagsList, ", ")
	}
	vikunjaTasks := convertTickTickToVikunja(tasks, time.UTC)
	require.Greater(t, len(vikunjaTasks), 0)
}

func TestMultilineDescriptions(t *testing.T) {
	// Test with a CSV fixture that contains actual multiline content in quoted fields
	file, err := os.Open("testdata_ticktick_multiline.csv")
	require.NoError(t, err, "Failed to open test fixture")
	defer file.Close()

	stat, err := file.Stat()
	require.NoError(t, err)

	lines, err := linesToSkipBeforeHeader(file, stat.Size())
	require.NoError(t, err)
	t.Logf("Lines to skip: %d", lines)
	assert.Equal(t, 6, lines, "Should skip 6 metadata lines")

	// Reset file position
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	dec, err := newLineSkipDecoder(file, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err, "Failed to parse CSV with multiline descriptions")

	// We expect 2 tasks in this fixture
	require.Len(t, tasks, 2, "Should parse exactly 2 tasks")

	// First task has multiline content in both Title and Content fields
	task1 := tasks[0]
	assert.Equal(t, "Work", task1.FolderName)
	assert.Equal(t, "Project Alpha", task1.ProjectName)

	// The title contains a newline
	assert.Contains(t, task1.Title, "Task with multiline")
	assert.Contains(t, task1.Title, "description")
	assert.Contains(t, task1.Title, "\n", "Title should contain actual newline character")

	// The content contains multiple newlines and paragraphs
	assert.Contains(t, task1.Content, "This is a task description")
	assert.Contains(t, task1.Content, "that spans multiple lines")
	assert.Contains(t, task1.Content, "It has paragraphs and everything!")
	assert.Contains(t, task1.Content, "Including special characters: #, *, @")

	// Count newlines in content - should have at least 3 (between the 4 lines)
	newlineCount := strings.Count(task1.Content, "\n")
	assert.GreaterOrEqual(t, newlineCount, 3, "Content should have multiple newlines")

	// Second task is a regular task without multiline content
	task2 := tasks[1]
	assert.Equal(t, "Regular task", task2.Title)
	assert.Equal(t, "Simple description", task2.Content)
	assert.NotContains(t, task2.Title, "\n", "Regular task title should not have newlines")

	t.Logf("Successfully parsed tasks with multiline content:")
	t.Logf("  Task 1 title: %q", task1.Title)
	t.Logf("  Task 1 content: %q", task1.Content)
	t.Logf("  Task 2 title: %q", task2.Title)
}

func TestEmptyLabelHandlingWithRealCSV(t *testing.T) {
	t.Run("Parse CSV file", func(t *testing.T) {
		file, err := os.Open("testdata_ticktick_export.csv")
		require.NoError(t, err)
		defer file.Close()

		stat, err := file.Stat()
		require.NoError(t, err)

		lines, err := linesToSkipBeforeHeader(file, stat.Size())
		require.NoError(t, err)

		// Reset file position
		_, err = file.Seek(0, io.SeekStart)
		require.NoError(t, err)

		dec, err := newLineSkipDecoder(file, lines)
		require.NoError(t, err)
		tasks := []*tickTickTask{}
		err = gocsv.UnmarshalDecoder(dec, &tasks)
		require.NoError(t, err)
		require.Greater(t, len(tasks), 0)

		t.Logf("Successfully parsed %d tasks from CSV file", len(tasks))
	})

	t.Run("Process tags and check for empty labels", func(t *testing.T) {
		file, err := os.Open("testdata_ticktick_export.csv")
		require.NoError(t, err)
		defer file.Close()

		stat, err := file.Stat()
		require.NoError(t, err)

		lines, err := linesToSkipBeforeHeader(file, stat.Size())
		require.NoError(t, err)

		// Reset file position
		_, err = file.Seek(0, io.SeekStart)
		require.NoError(t, err)

		dec, err := newLineSkipDecoder(file, lines)
		require.NoError(t, err)
		tasks := []*tickTickTask{}
		err = gocsv.UnmarshalDecoder(dec, &tasks)
		require.NoError(t, err)

		// Process tags as the migration code does
		for _, task := range tasks {
			task.Tags = strings.Split(task.TagsList, ", ")
		}

		// Convert to Vikunja format
		vikunjaTasks := convertTickTickToVikunja(tasks, time.UTC)

		// Check all tasks for empty labels
		totalLabels := 0
		for _, project := range vikunjaTasks {
			for _, task := range project.Tasks {
				totalLabels += len(task.Labels)
				for _, label := range task.Labels {
					assert.NotEmpty(t, strings.TrimSpace(label.Title),
						"No label should be empty or whitespace-only. Found empty label in task: %s", task.Title)
				}
			}
		}

		t.Logf("Successfully processed %d tasks with %d total labels, no empty labels created", len(tasks), totalLabels)
	})
}

func TestTickTickPriorityParsing(t *testing.T) {
	cases := []struct {
		input    string
		expected int64
	}{
		{"", 0},
		{"0", 0},
		{"1", 1},
		{"3", 3},
		{"5", 5},
		{"p1", 1},
		{"P2", 2},
		{"p3", 3},
		{" p5 ", 5},
		{"garbage", 0},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			var p tickTickPriority
			require.NoError(t, p.UnmarshalCSV(c.input))
			assert.Equal(t, tickTickPriority(c.expected), p)
		})
	}
}

// TestNonNumericNumberColumns ensures that exports containing non-numeric values
// in columns Vikunja parses as integers (Priority, taskId, parentId) do not fail
// the whole import. See go-vikunja/vikunja#2822.
func TestNonNumericNumberColumns(t *testing.T) {
	file, err := os.Open("testdata_ticktick_invalid_numbers.csv")
	require.NoError(t, err)
	defer file.Close()

	stat, err := file.Stat()
	require.NoError(t, err)

	lines, err := linesToSkipBeforeHeader(file, stat.Size())
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	dec, err := newLineSkipDecoder(file, lines)
	require.NoError(t, err)
	tasks := []*tickTickTask{}
	err = gocsv.UnmarshalDecoder(dec, &tasks)
	require.NoError(t, err, "non-numeric values in numeric columns should not fail the import")
	require.Len(t, tasks, 2)

	// "p1" in the Priority column is parsed as priority 1 instead of crashing.
	assert.Equal(t, tickTickPriority(1), tasks[0].Priority)
	assert.Equal(t, tickTickNumber(1), tasks[0].TaskID)

	// Non-numeric taskId/parentId fall back to 0 as well.
	assert.Equal(t, tickTickNumber(0), tasks[1].TaskID)
	assert.Equal(t, tickTickNumber(0), tasks[1].ParentID)

	// And the tasks still convert to the Vikunja structure, carrying the parsed
	// priority over to the resulting task.
	for _, task := range tasks {
		task.Tags = strings.Split(task.TagsList, ", ")
	}
	vikunjaTasks := convertTickTickToVikunja(tasks, time.UTC)
	require.Greater(t, len(vikunjaTasks), 0)

	var priority int64
	for _, project := range vikunjaTasks {
		for _, task := range project.Tasks {
			if task.Title == "Task with non-numeric priority" {
				priority = task.Priority
			}
		}
	}
	assert.Equal(t, int64(1), priority)
}

// TestMultipleTasksWithMalformedIDsAreNotDropped guards against a regression
// where collapsing several unparseable taskIds to 0 caused all but the first
// zero-ID task to be silently dropped by sortParentsBeforeChildren.
func TestMultipleTasksWithMalformedIDsAreNotDropped(t *testing.T) {
	tasks := []*tickTickTask{
		{TaskID: 0, ProjectName: "Project 1", Title: "First malformed"},
		{TaskID: 0, ProjectName: "Project 1", Title: "Second malformed"},
		{TaskID: 0, ProjectName: "Project 1", Title: "Third malformed"},
	}

	sorted := sortParentsBeforeChildren(tasks)
	require.Len(t, sorted, 3, "no task with a zero ID should be dropped")

	vikunjaTasks := convertTickTickToVikunja(tasks, time.UTC)
	titles := []string{}
	for _, project := range vikunjaTasks {
		for _, task := range project.Tasks {
			titles = append(titles, task.Title)
		}
	}
	assert.ElementsMatch(t, []string{"First malformed", "Second malformed", "Third malformed"}, titles)
}

// TestTickTickRowLimit covers TickTick's bounded CSV decoder (GHSA-pqf9-h8g4-8gmh).
func TestTickTickRowLimit(t *testing.T) {
	config.MigrationMaxCSVRows.Set("3")
	defer config.MigrationMaxCSVRows.Set("100000")

	buildTickTickCSV := func(rows int) string {
		content := "Date: 2024-01-01+0000\nVersion: 7.1\n" +
			"\"Folder Name\",\"List Name\",\"Title\",\"Kind\",\"Tags\",\"Content\",\"Is Check list\",\"Start Date\",\"Due Date\",\"Reminder\",\"Repeat\",\"Priority\",\"Status\",\"Created Time\",\"Completed Time\",\"Order\",\"Timezone\",\"Is All Day\",\"Is Floating\",\"Column Name\",\"Column Order\",\"View Mode\",\"taskId\",\"parentId\"\n"
		for i := 0; i < rows; i++ {
			content += ",\"list\",\"task" + strconv.Itoa(i) + "\",\"TEXT\",\"\",\"\",\"N\",\"\",\"\",\"\",\"\",\"0\",\"0\",\"2022-10-09T15:09:48+0000\",\"\",\"-1099511627776\",\"\",\"true\",\"false\",,,\"list\",\"1\",\"\"\n"
		}
		return content
	}

	decode := func(t *testing.T, content string) ([]*tickTickTask, error) {
		t.Helper()
		lines, err := linesToSkipBeforeHeader(bytes.NewReader([]byte(content)), int64(len(content)))
		require.NoError(t, err)
		dec, err := newLineSkipDecoder(bytes.NewReader([]byte(content)), lines)
		require.NoError(t, err)
		tasks := []*tickTickTask{}
		err = gocsv.UnmarshalDecoder(dec, &tasks)
		return tasks, err
	}

	t.Run("exactly the limit passes", func(t *testing.T) {
		tasks, err := decode(t, buildTickTickCSV(3))
		require.NoError(t, err)
		require.Len(t, tasks, 3)
	})

	t.Run("limit+1 rows fail with the typed error", func(t *testing.T) {
		_, err := decode(t, buildTickTickCSV(4))
		require.Error(t, err)
		var limitErr *migration.ErrImportRowLimitExceeded
		require.True(t, errors.As(err, &limitErr), "expected ErrImportRowLimitExceeded, got %v", err)
	})

	t.Run("multiline metadata still skips correctly", func(t *testing.T) {
		content := "\uFEFF\"Date: 2025-11-25+0000\"\n" +
			"\"Version: 7.1\"\n" +
			"\"Status: \n" +
			"0 Normal\n" +
			"1 Completed\n" +
			"2 Archived\"\n" +
			"\"Folder Name\",\"List Name\",\"Title\",\"Kind\",\"Tags\",\"Content\",\"Is Check list\",\"Start Date\",\"Due Date\",\"Reminder\",\"Repeat\",\"Priority\",\"Status\",\"Created Time\",\"Completed Time\",\"Order\",\"Timezone\",\"Is All Day\",\"Is Floating\",\"Column Name\",\"Column Order\",\"View Mode\",\"taskId\",\"parentId\"\n" +
			"\"dsx\",\"x\",\"this task repeats\",\"TEXT\",\"\",\"\",\"N\",\"\",\"\",\"\",\"\",\"0\",\"0\",\"2022-10-09T15:09:48+0000\",\"\",\"-1099511627776\",\"Europe/Berlin\",,\"false\",,,\"list\",\"2\",\"\"\n"

		tasks, err := decode(t, content)
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, "this task repeats", tasks[0].Title)
	})
}

// TestParseTickTickV7Backup reads a backup in the format TickTick 7.2 writes:
// metadata lines before the header, carriage returns as line breaks inside the
// content, newline-separated reminders and unquoted empty kanban columns.
func TestParseTickTickV7Backup(t *testing.T) {
	file, err := os.Open("testdata_ticktick_v7.csv")
	require.NoError(t, err)
	defer file.Close()
	stat, err := file.Stat()
	require.NoError(t, err)

	tasks, err := parseTickTickCSV(file, stat.Size())
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	assert.Equal(t, "Plan offsite", tasks[0].Title)
	assert.Equal(t, []time.Duration{-time.Hour, -10 * time.Minute}, tasks[0].Reminders)
	assert.Equal(t, tickTickNumber(1), tasks[1].ParentID)
	assert.Equal(t, "Ideas", tasks[2].ColumnName)
	assert.Equal(t, []string{"travel", "fun"}, tasks[2].Tags)

	projects := convertTickTickToVikunja(tasks, time.UTC)
	require.Len(t, projects, 3)
	assert.Equal(t, "Work", projects[0].Title)
	assert.Equal(t, "Personal", projects[1].Title)
	assert.Equal(t, "Trips", projects[2].Title)

	offsite := projects[0].Tasks[0]
	assert.Equal(t, "agenda\n- [ ] venue\n- [ ] food", offsite.Description)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=TH", offsite.RepeatRule)
	assert.Equal(t, int64(3), offsite.Priority)

	holiday := projects[2].Tasks[0]
	assert.True(t, holiday.Done)
	assert.True(t, holiday.WontDo)
	assert.Equal(t, time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC), holiday.StartDate)
	assert.Equal(t, time.Date(2025, 7, 11, 23, 59, 59, 0, time.UTC), holiday.EndDate)
}
