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
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/migration"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"

	"github.com/gocarina/gocsv"
)

var timeFormats = []string{
	"2006-01-02T15:04:05-0700",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
}

type Migrator struct {
}

type tickTickTask struct {
	FolderName        string           `csv:"Folder Name"`
	ProjectName       string           `csv:"List Name"`
	Title             string           `csv:"Title"`
	Kind              string           `csv:"Kind"`
	TagsList          string           `csv:"Tags"`
	Tags              []string         `csv:"-"`
	Content           string           `csv:"Content"`
	IsChecklistString string           `csv:"Is Check list"`
	IsChecklist       bool             `csv:"-"`
	StartDate         tickTickTime     `csv:"Start Date"`
	DueDate           tickTickTime     `csv:"Due Date"`
	ReminderDuration  string           `csv:"Reminder"`
	Reminders         []time.Duration  `csv:"-"`
	Repeat            string           `csv:"Repeat"`
	Priority          tickTickPriority `csv:"Priority"`
	Status            string           `csv:"Status"`
	CreatedTime       tickTickTime     `csv:"Created Time"`
	CompletedTime     tickTickTime     `csv:"Completed Time"`
	Order             float64          `csv:"Order"`
	Timezone          string           `csv:"Timezone"`
	IsAllDay          string           `csv:"Is All Day"`
	ColumnName        string           `csv:"Column Name"`
	ColumnOrder       tickTickNumber   `csv:"Column Order"`
	TaskID            tickTickNumber   `csv:"taskId"`
	ParentID          tickTickNumber   `csv:"parentId"`
}

type tickTickTime struct {
	time.Time
}

// tickTickNumber is an int64 that tolerates non-numeric or malformed values in
// the numeric ID columns (taskId, parentId) of TickTick exports. Such values can
// occur through column misalignment caused by unescaped delimiters, which would
// otherwise make gocsv fail the entire import with an internal server error
// (go-vikunja/vikunja#2822). Rather than aborting the import, we fall back to 0
// for any value we cannot parse.
type tickTickNumber int64

func (n *tickTickNumber) UnmarshalCSV(csv string) error {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		*n = 0
		return nil
	}
	parsed, err := strconv.ParseInt(csv, 10, 64)
	if err != nil {
		// Deliberately ignore the parse error and fall back to 0 so a single
		// malformed value does not abort the whole import.
		*n = 0
		return nil //nolint:nilerr
	}
	*n = tickTickNumber(parsed)
	return nil
}

// tickTickPriority parses the TickTick "Priority" column. TickTick exports the
// priority either as a plain number (0, 1, 3, 5) or, in some exports, prefixed
// with "p" (p1, p2, p3). We accept both forms and fall back to 0 (no priority)
// for anything we cannot parse, so a stray value never fails the whole import
// (go-vikunja/vikunja#2822). Vikunja's task priority is a free-form sortable
// integer, so the parsed value is carried over as-is.
type tickTickPriority int64

func (p *tickTickPriority) UnmarshalCSV(csv string) error {
	csv = strings.TrimSpace(csv)
	csv = strings.TrimPrefix(csv, "p")
	csv = strings.TrimPrefix(csv, "P")
	if csv == "" {
		*p = 0
		return nil
	}
	parsed, err := strconv.ParseInt(csv, 10, 64)
	if err != nil {
		// Deliberately ignore the parse error and fall back to 0 so a single
		// malformed value does not abort the whole import.
		*p = 0
		return nil //nolint:nilerr
	}
	*p = tickTickPriority(parsed)
	return nil
}

func (date *tickTickTime) UnmarshalCSV(csv string) (err error) {
	date.Time = time.Time{}
	if csv == "" {
		return nil
	}
	for _, format := range timeFormats {
		date.Time, err = time.Parse(format, csv)
		if err == nil {
			return nil
		}
	}
	return err
}

type placeState int

const (
	unvisited placeState = iota
	visiting
	donePlacing
)

// sortParentsBeforeChildren reorders tasks so that every parent task
// appears before any of its children. Tasks without a parent come first.
// The relative order of siblings / unrelated tasks is preserved.
func sortParentsBeforeChildren(tasks []*tickTickTask) []*tickTickTask {
	tasksByID := make(map[tickTickNumber]*tickTickTask, len(tasks))
	for _, t := range tasks {
		tasksByID[t.TaskID] = t
	}

	// state is keyed by the task itself rather than by TaskID: malformed
	// exports can collapse several taskIds to 0 (see tickTickNumber), and
	// keying by ID would treat every zero-ID task after the first as already
	// placed and silently drop it.
	state := make(map[*tickTickTask]placeState, len(tasks))
	result := make([]*tickTickTask, 0, len(tasks))

	var place func(t *tickTickTask)
	place = func(t *tickTickTask) {
		if state[t] != unvisited {
			return
		}
		// Must be marked before recursing: a parentId cycle would otherwise
		// recurse until the stack overflows, which kills the whole process.
		state[t] = visiting
		// If this task has a parent that we know about, place the parent first.
		if t.ParentID != 0 {
			if parent, ok := tasksByID[t.ParentID]; ok {
				place(parent)
			}
		}
		state[t] = donePlacing
		result = append(result, t)
	}

	for _, t := range tasks {
		place(t)
	}

	return result
}

// tickTickPriorityToVikunja maps TickTick's priorities (0 none, 1 low, 3 medium,
// 5 high) onto the levels the frontend shows (0 none, 1 low, 2 medium, 3 high).
func tickTickPriorityToVikunja(p tickTickPriority) int64 {
	switch {
	case p <= 0:
		return 0
	case p < 3:
		return 1
	case p < 5:
		return 2
	default:
		return 3
	}
}

// tickTickDay returns the calendar day an all-day TickTick date stands for.
// TickTick stores all-day dates as local midnight in the task's time zone;
// rounding to the nearest midnight tolerates a zone column that is off by a
// few hours (DST, a moved device).
func tickTickDay(t time.Time, tz *time.Location) time.Time {
	local := t.In(tz).Add(12 * time.Hour)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func onDay(day time.Time, hour, minute, sec int, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, sec, 0, loc)
}

// applyTickTickSchedule converts TickTick's start/due dates into the frontend's
// scheduling model: an all-day task is due at 23:59:59 local time, a multi-day
// all-day task spans start 00:00 to end 23:59:59, and a timed task with a
// duration has due = start. Reminders are relative to TickTick's anchor (the
// start, or local midnight for all-day tasks) and are re-anchored on due_date.
func applyTickTickSchedule(task *models.Task, t *tickTickTask, userLoc *time.Location) {
	start, due := t.StartDate.Time, t.DueDate.Time
	if start.IsZero() {
		start = due
	}
	if due.IsZero() {
		due = start
	}
	if start.IsZero() {
		return
	}

	// TickTick reminders are measured from this point.
	var anchor time.Time
	if t.IsAllDay == "true" {
		taskLoc := userLoc
		if l, err := time.LoadLocation(t.Timezone); err == nil && t.Timezone != "" {
			taskLoc = l
		}
		first := tickTickDay(start, taskLoc)
		// The due date of a multi-day all-day task is the day after its last one.
		last := tickTickDay(due, taskLoc).AddDate(0, 0, -1)
		task.DueDate = onDay(first, 23, 59, 59, userLoc)
		if last.After(first) {
			task.StartDate = onDay(first, 0, 0, 0, userLoc)
			task.EndDate = onDay(last, 23, 59, 59, userLoc)
		}
		anchor = onDay(first, 0, 0, 0, userLoc)
	} else {
		task.DueDate = start
		if due.After(start) {
			task.StartDate = start
			task.EndDate = due
		}
		anchor = start
	}

	for _, offset := range t.Reminders {
		task.Reminders = append(task.Reminders, &models.TaskReminder{
			RelativeTo:     models.ReminderRelationDueDate,
			RelativePeriod: int64(anchor.Add(offset).Sub(task.DueDate).Seconds()),
		})
	}

	if rule, err := models.NormalizeTaskRepeatRule(t.Repeat); err == nil {
		task.RepeatRule = rule
	}
}

// normalizeTickTickContent turns TickTick's carriage-return line breaks into newlines.
func normalizeTickTickContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

// convertTickTickToVikunja maps TickTick lists to projects, folders to parent
// projects and kanban columns to buckets. userLoc is the zone all-day dates are
// placed in, so they land on the same calendar day in the user's app.
func convertTickTickToVikunja(tasks []*tickTickTask, userLoc *time.Location) (result []*models.ProjectWithTasksAndBuckets) {
	// Sort tasks so that parent tasks always come before their children.
	// Without this, create_from_structure.go would try to create a
	// placeholder for a not-yet-seen parent, which fails because the
	// placeholder has no title.  (go-vikunja/vikunja#2487)
	tasks = sortParentsBeforeChildren(tasks)

	var nextID int64
	newID := func() int64 {
		nextID++
		return nextID
	}

	folders := make(map[string]*models.ProjectWithTasksAndBuckets)
	projects := make(map[string]*models.ProjectWithTasksAndBuckets)
	buckets := make(map[*models.ProjectWithTasksAndBuckets]map[string]*models.Bucket)
	for _, t := range tasks {
		key := t.FolderName + "\x00" + t.ProjectName
		project, has := projects[key]
		if !has {
			project = &models.ProjectWithTasksAndBuckets{
				Project: models.Project{
					ID:    newID(),
					Title: t.ProjectName,
				},
			}
			if t.FolderName != "" {
				folder, has := folders[t.FolderName]
				if !has {
					folder = &models.ProjectWithTasksAndBuckets{
						Project: models.Project{
							ID:    newID(),
							Title: t.FolderName,
						},
					}
					folders[t.FolderName] = folder
					result = append(result, folder)
				}
				project.ParentProjectID = models.Ptr(folder.ID)
			}
			projects[key] = project
			buckets[project] = make(map[string]*models.Bucket)
			result = append(result, project)
		}

		labels := make([]*models.Label, 0, len(t.Tags))
		for _, tag := range t.Tags {
			// Only create labels for non-empty tags after trimming whitespace
			trimmedTag := strings.TrimSpace(tag)
			if trimmedTag != "" {
				labels = append(labels, &models.Label{
					Title: trimmedTag,
				})
			}
		}

		task := &models.TaskWithComments{
			Task: models.Task{
				ID:          int64(t.TaskID),
				Title:       t.Title,
				Description: normalizeTickTickContent(t.Content),
				// TickTick statuses: 0 open, 1 completed, 2 archived (completed), -1 won't do.
				Done:     t.Status == "1" || t.Status == "2" || t.Status == "-1",
				WontDo:   t.Status == "-1",
				DoneAt:   t.CompletedTime.Time,
				Position: t.Order,
				Priority: tickTickPriorityToVikunja(t.Priority),
				Labels:   labels,
			},
		}
		applyTickTickSchedule(&task.Task, t, userLoc)

		if t.ColumnName != "" {
			bucket, has := buckets[project][t.ColumnName]
			if !has {
				bucket = &models.Bucket{
					ID:       int64(len(project.Buckets) + 1),
					Title:    t.ColumnName,
					Position: float64(t.ColumnOrder),
				}
				buckets[project][t.ColumnName] = bucket
				project.Buckets = append(project.Buckets, bucket)
			}
			task.BucketID = bucket.ID
		}

		if t.ParentID != 0 {
			task.RelatedTasks = map[models.RelationKind][]*models.Task{
				models.RelationKindParenttask: {{ID: int64(t.ParentID)}},
			}
		}

		project.Tasks = append(project.Tasks, task)
	}

	return
}

// Name is used to get the name of the ticktick migration - we're using the docs here to annotate the status route.
// @Summary Get migration status
// @Description Returns if the current user already did the migation or not. This is useful to show a confirmation message in the frontend if the user is trying to do the same migration again.
// @tags migration
// @Produce json
// @Security JWTKeyAuth
// @Success 200 {object} migration.Status "The migration status"
// @Failure 500 {object} models.Message "Internal server error"
// @Router /migration/ticktick/status [get]
func (m *Migrator) Name() string {
	return "ticktick"
}

// stripBOM removes the UTF-8 BOM from the beginning of a reader
func stripBOM(r io.Reader) io.Reader {
	// Read the first few bytes to check for BOM
	buf := make([]byte, 3)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		// If we read some bytes before the error, preserve them
		if n > 0 {
			return io.MultiReader(bytes.NewReader(buf[:n]), r)
		}
		return r
	}

	// Check if it starts with UTF-8 BOM (0xEF, 0xBB, 0xBF)
	// We need exactly 3 bytes and they must match the BOM sequence
	if n == 3 && len(buf) >= 3 && buf[0] == 0xEF && buf[1] == 0xBB && buf[2] == 0xBF {
		// BOM found, return reader without BOM
		return io.MultiReader(bytes.NewReader(buf[3:n]), r)
	}

	// No BOM found, return reader with the bytes we read back
	return io.MultiReader(bytes.NewReader(buf[:n]), r)
}

func newLineSkipDecoder(r io.Reader, linesToSkip int) (gocsv.SimpleDecoder, error) {
	// Strip BOM if present - this must be done consistently with linesToSkipBeforeHeader
	r = stripBOM(r)

	// Metadata may contain quoted newlines, so skip raw lines without buffering the file.
	br := bufio.NewReader(r)
	linesFound := 0
	for linesFound < linesToSkip {
		if _, err := br.ReadBytes('\n'); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		linesFound++
	}

	// The CSV reader will properly handle any multiline quoted fields in the actual data
	reader := csv.NewReader(br)

	// Allow variable field counts and be lenient with parsing
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	return boundedSimpleDecoder{inner: gocsv.NewSimpleDecoderFromCSVReader(reader)}, nil
}

// boundedSimpleDecoder replaces gocsv's unbounded ReadAll path (GHSA-pqf9-h8g4-8gmh).
type boundedSimpleDecoder struct {
	inner   gocsv.SimpleDecoder
	maxRows int64
}

func (d boundedSimpleDecoder) limit() int64 {
	if d.maxRows > 0 {
		return d.maxRows
	}
	if maxRows := config.MigrationMaxCSVRows.GetInt64(); maxRows > 0 {
		return maxRows
	}
	return 100000
}

func (d boundedSimpleDecoder) GetCSVRows() ([][]string, error) {
	// GetCSVRows includes the header row, which does not count against the limit.
	rows := make([][]string, 0)
	for int64(len(rows)) < d.limit()+1 {
		record, err := d.inner.GetCSVRow()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return rows, nil
			}
			return nil, err
		}
		rows = append(rows, record)
	}
	if _, err := d.inner.GetCSVRow(); err == nil {
		return nil, &migration.ErrImportRowLimitExceeded{MaxRows: d.limit()}
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}
	return rows, nil
}

func (d boundedSimpleDecoder) GetCSVRow() ([]string, error) {
	record, err := d.inner.GetCSVRow()
	if err != nil {
		return nil, err
	}
	return record, nil
}

func linesToSkipBeforeHeader(file io.ReaderAt, size int64) (int, error) {
	sr := io.NewSectionReader(file, 0, size)
	// Strip BOM before scanning for header
	r := stripBOM(sr)
	scanner := bufio.NewScanner(r)
	lines := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Folder Name") &&
			strings.Contains(line, "List Name") &&
			strings.Contains(line, "Title") {
			break
		}
		lines++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return lines, nil
}

// defaultProjectID loads the user's default project; the user passed to a
// migration comes from a serialized event, which leaves it out.
func defaultProjectID(userID int64) (int64, error) {
	s := db.NewSession()
	defer s.Close()
	u, err := user.GetUserByID(s, userID)
	if err != nil {
		return 0, err
	}
	return u.DefaultProjectID, nil
}

// parseTickTickCSV reads the tasks of a TickTick backup, skipping the metadata
// lines TickTick writes before the header.
func parseTickTickCSV(file io.ReaderAt, size int64) ([]*tickTickTask, error) {
	fr := io.NewSectionReader(file, 0, size)

	// Check if the file is a valid CSV
	buf := make([]byte, 1024)
	n, err := fr.Read(buf)
	if errors.Is(err, io.EOF) || n == 0 {
		return nil, &migration.ErrFileIsEmpty{}
	}
	if err != nil {
		return nil, err
	}

	// Reset the reader position to start
	if _, err = fr.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if !isValidCSV(string(buf[:n])) {
		return nil, &migration.ErrNotACSVFile{}
	}

	skip, err := linesToSkipBeforeHeader(file, size)
	if err != nil {
		return nil, err
	}
	decode, err := newLineSkipDecoder(fr, skip)
	if err != nil {
		return nil, err
	}

	allTasks := []*tickTickTask{}
	if err := gocsv.UnmarshalDecoder(decode, &allTasks); err != nil {
		return nil, err
	}

	// Also check if no tasks were found after decoding
	if len(allTasks) == 0 {
		return nil, &migration.ErrFileIsEmpty{}
	}

	for _, task := range allTasks {
		if task.IsChecklistString == "Y" {
			task.IsChecklist = true
		}

		// Several reminders come newline-separated, e.g. "PT0S\n-PT1440M".
		for _, r := range strings.Fields(task.ReminderDuration) {
			if strings.Contains(r, "P") {
				task.Reminders = append(task.Reminders, utils.ParseISO8601Duration(r))
			}
		}

		task.Tags = strings.Split(task.TagsList, ", ")
	}

	return allTasks, nil
}

// Migrate takes a ticktick export, parses it and imports everything in it into Vikunja.
// @Summary Import all projects, tasks etc. from a TickTick backup export
// @Description Imports all projects, tasks, notes, reminders, subtasks and files from a TickTick backup export into Vikunja.
// @tags migration
// @Accept x-www-form-urlencoded
// @Produce json
// @Security JWTKeyAuth
// @Param import formData string true "The TickTick backup csv file."
// @Success 200 {object} models.Message "A message telling you everything was migrated successfully."
// @Failure 500 {object} models.Message "Internal server error"
// @Router /migration/ticktick/migrate [put]
func (m *Migrator) Migrate(user *user.User, file io.ReaderAt, size int64) error {
	if size == 0 {
		return &migration.ErrFileIsEmpty{}
	}

	allTasks, err := parseTickTickCSV(file, size)
	if err != nil {
		return err
	}

	userLoc := config.GetTimeZone()
	if l, err := time.LoadLocation(user.Timezone); err == nil && user.Timezone != "" {
		userLoc = l
	}
	vikunjaTasks := convertTickTickToVikunja(allTasks, userLoc)

	inboxID, err := defaultProjectID(user.ID)
	if err != nil {
		return err
	}
	for _, p := range vikunjaTasks {
		// TickTick's Inbox goes into the user's own Inbox instead of a second one.
		if p.Title == "Inbox" && p.ParentProjectID == nil {
			p.ImportIntoProjectID = inboxID
		}
	}

	return migration.InsertFromStructure(vikunjaTasks, user)
}

// isValidCSV performs a basic check to determine if the content looks like a CSV file
func isValidCSV(content string) bool {
	// Check for common CSV headers from TickTick export
	if !strings.Contains(content, "Folder Name") ||
		!strings.Contains(content, "List Name") ||
		!strings.Contains(content, "Title") {
		return false
	}

	// Check if the file has commas as separators and multiple lines
	hasCommas := strings.Contains(content, ",")
	hasNewlines := strings.Contains(content, "\n")

	return hasCommas && hasNewlines
}
