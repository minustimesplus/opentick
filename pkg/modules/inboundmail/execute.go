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

package inboundmail

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/richtext"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

const maxFindResults = 25

// executor runs the commands of one mail as one user.
type executor struct {
	user *user.User
	cfg  *models.AgentConfig
	loc  *time.Location
	now  time.Time
	// contextTaskID is the task a reply was addressed to, used when a command names none.
	contextTaskID int64
}

// outcome is the result of one command, for the reply.
type outcome struct {
	command string
	ok      bool
	text    string
}

// errUser is an error whose message is meant for the sender as is.
type errUser struct{ msg string }

func (e errUser) Error() string { return e.msg }

func userErr(format string, args ...any) error {
	return errUser{msg: fmt.Sprintf(format, args...)}
}

// run executes every command in its own transaction, so one failing doesn't undo the others.
func (e *executor) run(cmds []*command) []outcome {
	out := make([]outcome, 0, len(cmds))
	for _, c := range cmds {
		label := c.Verb
		if c.Arg != "" {
			label += ": " + c.Arg
		}
		text, err := e.runOne(c)
		if err != nil {
			out = append(out, outcome{command: label, text: friendlyError(err)})
			continue
		}
		out = append(out, outcome{command: label, ok: true, text: text})
	}
	return out
}

func (e *executor) runOne(c *command) (string, error) {
	if len(c.Unknown) > 0 {
		return "", userErr("I didn't understand %q. Use \"field: value\" lines; send \"help\" for the list of fields.", c.Unknown[0])
	}
	s := db.NewSession()
	defer s.Close()

	var (
		text string
		err  error
	)
	switch c.Verb {
	case "add":
		text, err = e.add(s, c)
	case "update":
		text, err = e.update(s, c)
	case "done":
		text, err = e.setDone(s, c, true)
	case "undone":
		text, err = e.setDone(s, c, false)
	case "delete":
		text, err = e.remove(s, c, strings.ToLower(strings.TrimSpace(c.Fields["scope"])))
	case "skip":
		text, err = e.remove(s, c, "this")
	case "comment":
		text, err = e.comment(s, c)
	case "find":
		text, err = e.find(s, c)
	case "lists":
		text, err = e.lists(s)
	case "help":
		text = models.InboundCommandReference
	default:
		err = userErr("unknown command %q", c.Verb)
	}
	if err != nil {
		_ = s.Rollback()
		return "", err
	}
	if err := s.Commit(); err != nil {
		return "", err
	}
	return text, nil
}

// friendlyError turns an error into a sentence for the sender, hiding internal details.
func friendlyError(err error) string {
	var ue errUser
	if errors.As(err, &ue) {
		return ue.msg
	}
	var httpErr web.HTTPErrorProcessor
	if errors.As(err, &httpErr) {
		return httpErr.HTTPError().Message
	}
	log.Errorf("[inbound mail] command failed: %s", err)
	return "Something went wrong on the server. Try again later."
}

var taskIDRe = regexp.MustCompile(`^(?:task\s*)?#?(\d+)\b\s*(.*)$`)

// taskRef reads a task ID from a command argument, falling back to the task replied to.
// rest is whatever followed the ID.
func (e *executor) taskRef(arg string) (id int64, rest string, err error) {
	arg = strings.TrimSpace(arg)
	if m := taskIDRe.FindStringSubmatch(strings.ToLower(arg)); m != nil {
		id, _ = strconv.ParseInt(m[1], 10, 64)
		return id, strings.TrimSpace(arg[len(arg)-len(m[2]):]), nil
	}
	if e.contextTaskID > 0 {
		return e.contextTaskID, arg, nil
	}
	return 0, "", userErr("Which task? Give its ID, like \"done: 123\". Use \"find: <words>\" to look IDs up.")
}

// loadTask reads a task the user may change, with everything an update writes back.
func (e *executor) loadTask(s *xorm.Session, id int64) (*models.Task, error) {
	check := &models.Task{ID: id}
	can, err := check.CanUpdate(s, e.user)
	if err != nil {
		if models.IsErrTaskDoesNotExist(err) {
			return nil, userErr("Task %d doesn't exist (or is in the trash).", id)
		}
		return nil, err
	}
	if !can {
		return nil, userErr("You can't change task %d.", id)
	}
	t := &models.Task{ID: id}
	if err := t.ReadOne(s, e.user); err != nil {
		return nil, err
	}
	return t, nil
}

func (e *executor) add(s *xorm.Session, c *command) (string, error) {
	title := strings.TrimSpace(c.Arg)
	if v, ok := c.Fields["title"]; ok && strings.TrimSpace(v) != "" {
		title = strings.TrimSpace(v)
	}
	if title == "" {
		return "", userErr("A new task needs a title: \"add: Buy milk\".")
	}
	if c.has("scope") {
		return "", userErr("scope only goes with delete.")
	}

	var projectID int64
	var err error
	if v, ok := c.Fields["list"]; ok && !isNone(v) {
		projectID, err = e.resolveList(s, v)
	} else {
		projectID, err = e.defaultList(s, c)
	}
	if err != nil {
		return "", err
	}

	t := &models.Task{Title: title, ProjectID: projectID}
	if err := e.apply(t, c); err != nil {
		return "", err
	}
	can, err := t.CanCreate(s, e.user)
	if err != nil {
		return "", err
	}
	if !can {
		return "", userErr("You can't add tasks to that list.")
	}
	if err := t.Create(s, e.user); err != nil {
		return "", err
	}
	if v, ok := c.Fields["labels"]; ok {
		if err := e.setLabels(s, t, v); err != nil {
			return "", err
		}
	}
	return "Created " + e.describe(s, t), nil
}

func (e *executor) update(s *xorm.Session, c *command) (string, error) {
	id, rest, err := e.taskRef(c.Arg)
	if err != nil {
		return "", err
	}
	if rest != "" && !c.has("title") {
		return "", userErr("Put the new title on its own line: \"title: %s\".", rest)
	}
	if c.has("scope") {
		return "", userErr("scope only goes with delete.")
	}
	t, err := e.loadTask(s, id)
	if err != nil {
		return "", err
	}
	if v, ok := c.Fields["title"]; ok {
		if strings.TrimSpace(v) == "" {
			return "", userErr("The title can't be empty.")
		}
		t.Title = strings.TrimSpace(v)
	}
	if v, ok := c.Fields["list"]; ok {
		projectID, err := e.resolveList(s, v)
		if err != nil {
			return "", err
		}
		if projectID != t.ProjectID {
			can, err := (&models.Task{ID: t.ID, ProjectID: projectID}).CanUpdate(s, e.user)
			if err != nil {
				return "", err
			}
			if !can {
				return "", userErr("You can't move tasks into that list.")
			}
			t.ProjectID = projectID
		}
	}
	if err := e.apply(t, c); err != nil {
		return "", err
	}
	cols := []string{"title", "description", "due_date", "start_date", "end_date",
		"repeat_rule", "repeat_after", "repeat_mode", "priority", "project_id"}
	if err := t.UpdateColumns(s, e.user, cols); err != nil {
		return "", err
	}
	if v, ok := c.Fields["labels"]; ok {
		if err := e.setLabels(s, t, v); err != nil {
			return "", err
		}
	}
	return "Updated " + e.describe(s, t), nil
}

func (e *executor) setDone(s *xorm.Session, c *command, done bool) (string, error) {
	id, _, err := e.taskRef(c.Arg)
	if err != nil {
		return "", err
	}
	t, err := e.loadTask(s, id)
	if err != nil {
		return "", err
	}
	if t.Done == done {
		if done {
			return "Task " + strconv.FormatInt(id, 10) + " was already done.", nil
		}
		return "Task " + strconv.FormatInt(id, 10) + " was already open.", nil
	}
	repeating := t.IsRepeating()
	t.Done = done
	if err := t.UpdateColumns(s, e.user, []string{"done"}); err != nil {
		return "", err
	}
	switch {
	case done && repeating && !t.Done:
		return "Completed this occurrence; next: " + e.describe(s, t), nil
	case done:
		return "Completed " + e.describe(s, t), nil
	}
	return "Reopened " + e.describe(s, t), nil
}

func (e *executor) remove(s *xorm.Session, c *command, scope string) (string, error) {
	id, _, err := e.taskRef(c.Arg)
	if err != nil {
		return "", err
	}
	t, err := e.loadTask(s, id)
	if err != nil {
		return "", err
	}
	can, err := (&models.Task{ID: id}).CanDelete(s, e.user)
	if err != nil {
		return "", err
	}
	if !can {
		return "", userErr("You can't delete task %d.", id)
	}

	if !t.IsRepeating() {
		if scope == "this" {
			return "", userErr("Task %d doesn't repeat, so there's no single occurrence to skip. Use \"delete: %d\" to delete it.", id, id)
		}
		if err := t.Delete(s, e.user); err != nil {
			return "", err
		}
		return fmt.Sprintf("Moved task %d %q to the trash.", id, t.Title), nil
	}

	switch scope {
	case "this", "occurrence", "this occurrence", "instance", "this one", "once", "skip":
		next, deleted, err := models.SkipTaskOccurrence(s, e.user, id)
		if err != nil {
			return "", err
		}
		if deleted {
			return fmt.Sprintf("That was the last occurrence of task %d %q, so it went to the trash.", id, t.Title), nil
		}
		return "Skipped this occurrence; next: " + e.describe(s, next), nil
	case "future", "all", "series", "all future", "this and future", "following", "all occurrences":
		if err := t.Delete(s, e.user); err != nil {
			return "", err
		}
		return fmt.Sprintf("Moved repeating task %d %q to the trash, with all its future occurrences.", id, t.Title), nil
	case "":
		return "", userErr("Task %d repeats. Add \"scope: this\" to skip only this occurrence, or \"scope: future\" to delete it and all later ones.", id)
	}
	return "", userErr("%q is not a scope: use this or future.", scope)
}

func (e *executor) comment(s *xorm.Session, c *command) (string, error) {
	id, rest, err := e.taskRef(c.Arg)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(c.Fields["text"])
	if text == "" {
		text = rest
	}
	if text == "" {
		return "", userErr("The comment is empty. Put it in a \"text:\" line.")
	}
	return e.addComment(s, id, text)
}

func (e *executor) addComment(s *xorm.Session, taskID int64, text string) (string, error) {
	html, err := richtext.MarkdownToHTML(text)
	if err != nil {
		return "", err
	}
	tc := &models.TaskComment{TaskID: taskID, Comment: html}
	can, err := tc.CanCreate(s, e.user)
	if err != nil {
		if models.IsErrTaskDoesNotExist(err) {
			return "", userErr("Task %d doesn't exist (or is in the trash).", taskID)
		}
		return "", err
	}
	if !can {
		return "", userErr("You can't comment on task %d.", taskID)
	}
	if err := tc.Create(s, e.user); err != nil {
		return "", err
	}
	return fmt.Sprintf("Commented on task %d.", taskID), nil
}

func (e *executor) find(s *xorm.Session, c *command) (string, error) {
	query := strings.TrimSpace(c.Arg)
	tc := &models.TaskCollection{Filter: "done = false"}
	if v, ok := c.Fields["list"]; ok && !isNone(v) {
		projectID, err := e.resolveList(s, v)
		if err != nil {
			return "", err
		}
		tc.ProjectID = projectID
	}
	if query == "" && tc.ProjectID == 0 {
		return "", userErr("What should I look for? \"find: <words>\", optionally with a \"list:\" line.")
	}
	result, _, total, err := tc.ReadAll(s, e.user, query, 1, maxFindResults)
	if err != nil {
		return "", err
	}
	tasks, ok := result.([]*models.Task)
	if !ok {
		return "", fmt.Errorf("task collection returned unexpected type %T", result)
	}
	if len(tasks) == 0 {
		return "No open tasks match.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d open task(s) match", total)
	if int(total) > len(tasks) {
		fmt.Fprintf(&b, ", showing the first %d", len(tasks))
	}
	b.WriteString(":")
	for _, t := range tasks {
		b.WriteString("\n  " + e.describe(s, t))
	}
	return b.String(), nil
}

func (e *executor) lists(s *xorm.Session) (string, error) {
	projects, err := e.projects(s)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "You have no lists.", nil
	}
	byID := projectIndex(projects)
	paths := make([]string, 0, len(projects))
	for _, p := range projects {
		paths = append(paths, fmt.Sprintf("%s (ID %d)", projectPath(byID, p), p.ID))
	}
	sort.Strings(paths)
	return "Your lists:\n  " + strings.Join(paths, "\n  "), nil
}

// projects returns the user's lists, without archived ones and pseudo lists.
func (e *executor) projects(s *xorm.Session) ([]*models.Project, error) {
	result, _, _, err := (&models.Project{}).ReadAll(s, e.user, "", -1, 0)
	if err != nil {
		return nil, err
	}
	all, ok := result.([]*models.Project)
	if !ok {
		return nil, fmt.Errorf("project list returned unexpected type %T", result)
	}
	out := make([]*models.Project, 0, len(all))
	for _, p := range all {
		if p.ID > 0 && !p.IsArchived {
			out = append(out, p)
		}
	}
	return out, nil
}

func projectIndex(projects []*models.Project) map[int64]*models.Project {
	byID := make(map[int64]*models.Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	return byID
}

func projectPath(byID map[int64]*models.Project, p *models.Project) string {
	path := p.Title
	seen := map[int64]bool{p.ID: true}
	parentOf := func(p *models.Project) *models.Project {
		if p.ParentProjectID == nil {
			return nil
		}
		return byID[*p.ParentProjectID]
	}
	for parent := parentOf(p); parent != nil && !seen[parent.ID]; parent = parentOf(parent) {
		seen[parent.ID] = true
		path = parent.Title + "/" + path
	}
	return path
}

// resolveList finds a list by ID, name or Parent/Child path, ignoring case.
func (e *executor) resolveList(s *xorm.Session, raw string) (int64, error) {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "~"))
	if name == "" || isNone(name) {
		return 0, userErr("Which list? Send \"lists\" to see them.")
	}
	projects, err := e.projects(s)
	if err != nil {
		return 0, err
	}
	byID := projectIndex(projects)
	if id, err := strconv.ParseInt(strings.TrimPrefix(name, "#"), 10, 64); err == nil {
		if _, ok := byID[id]; ok {
			return id, nil
		}
	}
	var matches []*models.Project
	for _, p := range projects {
		if strings.EqualFold(p.Title, name) || strings.EqualFold(projectPath(byID, p), name) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		return 0, userErr("There's no list called %q. Send \"lists\" to see your lists.", name)
	}
	options := make([]string, 0, len(matches))
	for _, p := range matches {
		options = append(options, fmt.Sprintf("%s (ID %d)", projectPath(byID, p), p.ID))
	}
	return 0, userErr("More than one list is called %q: %s. Use the full path or the ID.", name, strings.Join(options, ", "))
}

// defaultList is where tasks without a list go: next to the task replied to,
// the list picked in the settings, or the user's default list.
func (e *executor) defaultList(s *xorm.Session, _ *command) (int64, error) {
	candidates := []int64{}
	if e.contextTaskID > 0 {
		if t, err := models.GetTaskByIDSimple(s, e.contextTaskID); err == nil {
			candidates = append(candidates, t.ProjectID)
		}
	}
	candidates = append(candidates, e.cfg.InboundProjectID, e.user.DefaultProjectID)
	for _, id := range candidates {
		if id <= 0 {
			continue
		}
		can, err := (&models.Project{ID: id}).CanWrite(s, e.user)
		if err != nil && !models.IsErrProjectDoesNotExist(err) {
			return 0, err
		}
		if can {
			return id, nil
		}
	}
	projects, err := e.projects(s)
	if err != nil {
		return 0, err
	}
	for _, p := range projects {
		if can, err := p.CanWrite(s, e.user); err == nil && can {
			return p.ID, nil
		}
	}
	return 0, userErr("You have no list to add tasks to.")
}

func (e *executor) setLabels(s *xorm.Session, t *models.Task, raw string) error {
	var labels []*models.Label
	if !isNone(raw) {
		seen := map[string]bool{}
		for _, name := range strings.Split(raw, ",") {
			name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "#"))
			key := strings.ToLower(name)
			if name == "" || seen[key] {
				continue
			}
			seen[key] = true
			l, err := e.findOrCreateLabel(s, name)
			if err != nil {
				return err
			}
			labels = append(labels, l)
		}
	}
	if err := t.UpdateTaskLabels(s, e.user, labels); err != nil {
		return err
	}
	t.Labels = labels
	return nil
}

func (e *executor) findOrCreateLabel(s *xorm.Session, name string) (*models.Label, error) {
	found, _, _, err := models.GetLabelsForUser(s, e.user, []string{name}, -1, 0)
	if err != nil {
		return nil, err
	}
	for _, l := range found {
		if strings.EqualFold(l.Title, name) {
			label := l.Label
			return &label, nil
		}
	}
	l := &models.Label{Title: name}
	if err := l.Create(s, e.user); err != nil {
		return nil, err
	}
	return l, nil
}

// apply copies the date, duration, repeat, priority and notes fields of c onto t.
func (e *executor) apply(t *models.Task, c *command) error {
	if v, ok := c.Fields["priority"]; ok {
		p, err := parsePriority(v)
		if err != nil {
			return errUser{msg: err.Error()}
		}
		t.Priority = p
	}
	if v, ok := c.Fields["notes"]; ok {
		if isNone(v) || strings.TrimSpace(v) == "" {
			t.Description = ""
		} else {
			html, err := richtext.MarkdownToHTML(v)
			if err != nil {
				return err
			}
			t.Description = html
		}
	}
	if err := e.applySchedule(t, c); err != nil {
		return err
	}
	return e.applyRepeat(t, c)
}

// isAllDay mirrors the web app: all-day dates are stored at 23:59 local time.
func isAllDay(t time.Time) bool {
	return t.Hour() == 23 && t.Minute() == 59
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// applySchedule sets due, start and end date the way the web app stores a schedule:
// the date in due_date (all-day at 23:59), and a duration as start_date/end_date.
func (e *executor) applySchedule(t *models.Task, c *command) error {
	_, hasDate := c.Fields["date"]
	_, hasDuration := c.Fields["duration"]
	_, hasEnd := c.Fields["end"]
	if !hasDate && !hasDuration && !hasEnd {
		return nil
	}
	if hasDuration && hasEnd {
		return userErr("Give either duration or end, not both.")
	}

	cur, curLength, hasCur := e.currentSchedule(t)
	date := cur
	length := curLength
	if hasDate {
		v := c.Fields["date"]
		if isNone(v) {
			if hasDuration || hasEnd {
				return userErr("A duration needs a date.")
			}
			t.DueDate, t.StartDate, t.EndDate = time.Time{}, time.Time{}, time.Time{}
			return nil
		}
		w, err := parseWhen(v, e.now, e.loc)
		if err != nil {
			return errUser{msg: err.Error()}
		}
		// A new date keeps an existing block's length, as moving a block does in the app;
		// moving between timed and all-day drops it.
		if !hasCur || cur.timed != w.timed {
			length = 0
		}
		date = w
	} else if !hasCur {
		return userErr("A duration needs a date: add a \"date:\" line.")
	}

	length, err := e.blockLength(c, date, length)
	if err != nil {
		return err
	}

	if !date.timed {
		day := startOfDay(date.at)
		days := int(length / (24 * time.Hour))
		if length%(24*time.Hour) != 0 {
			return userErr("A %s block needs a time: give the date like %s 14:00. All-day tasks take whole days (2d).", formatLength(length), day.Format("2006-01-02"))
		}
		t.DueDate = endOfDay(day)
		t.StartDate, t.EndDate = time.Time{}, time.Time{}
		if days > 1 {
			t.StartDate = day
			t.EndDate = endOfDay(day.AddDate(0, 0, days-1))
		}
		return nil
	}
	t.DueDate = date.at
	t.StartDate, t.EndDate = time.Time{}, time.Time{}
	if length > 0 {
		t.StartDate = date.at
		t.EndDate = date.at.Add(length)
	}
	return nil
}

// currentSchedule reads a task's date and block length the way the web app does.
func (e *executor) currentSchedule(t *models.Task) (cur when, length time.Duration, ok bool) {
	start, end, due := t.StartDate.In(e.loc), t.EndDate.In(e.loc), t.DueDate.In(e.loc)
	switch {
	case !t.StartDate.IsZero() && !t.EndDate.IsZero() && end.After(start):
		allDay := start.Hour() == 0 && start.Minute() == 0 && isAllDay(end)
		return when{at: start, timed: !allDay}, end.Sub(start), true
	case !t.DueDate.IsZero():
		return when{at: due, timed: !isAllDay(due)}, 0, true
	case !t.StartDate.IsZero():
		return when{at: start, timed: !isAllDay(start)}, 0, true
	}
	return when{}, 0, false
}

// blockLength applies the duration or end field to the current block length.
func (e *executor) blockLength(c *command, date when, length time.Duration) (time.Duration, error) {
	_, hasDuration := c.Fields["duration"]
	_, hasEnd := c.Fields["end"]
	if hasDuration {
		v := c.Fields["duration"]
		if isNone(v) {
			length = 0
		} else {
			d, err := parseDuration(v)
			if err != nil {
				return 0, errUser{msg: err.Error()}
			}
			length = d
		}
	}
	if hasEnd {
		v := c.Fields["end"]
		if isNone(v) {
			length = 0
		} else {
			w, err := parseWhen(v, e.now, e.loc)
			if err != nil {
				return 0, errUser{msg: err.Error()}
			}
			switch {
			case date.timed && !w.timed:
				return 0, userErr("Give the end with a date and time, like %s 18:00.", date.at.Format("2006-01-02"))
			case date.timed:
				length = w.at.Sub(date.at)
			default:
				length = startOfDay(w.at).Sub(startOfDay(date.at)) + 24*time.Hour
			}
			if length <= 0 {
				return 0, userErr("The end must be after the start.")
			}
		}
	}
	return length, nil
}

func (e *executor) applyRepeat(t *models.Task, c *command) error {
	if v, ok := c.Fields["repeat"]; ok {
		if isNone(v) {
			t.RepeatRule, t.RepeatAfter, t.RepeatMode = "", 0, models.TaskRepeatModeDefault
		} else {
			rule, err := parseRepeat(v)
			if err != nil {
				return errUser{msg: err.Error()}
			}
			t.RepeatRule, t.RepeatAfter = rule, 0
			if t.RepeatMode == models.TaskRepeatModeMonth {
				t.RepeatMode = models.TaskRepeatModeDefault
			}
		}
	}
	if v, ok := c.Fields["repeat_from"]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "completion", "completed", "done", "completion date":
			t.RepeatMode = models.TaskRepeatModeFromCurrentDate
		case "due", "due date", "date", "schedule":
			t.RepeatMode = models.TaskRepeatModeDefault
		default:
			return userErr("%q is not a repeat from: use due or completion.", v)
		}
	}
	if t.IsRepeating() && t.DueDate.IsZero() && t.StartDate.IsZero() {
		return userErr("A repeating task needs a date: add a \"date:\" line.")
	}
	return nil
}

func formatLength(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%d minute", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%02dm", h, m)
}

// describe sums a task up in one line for the reply.
func (e *executor) describe(s *xorm.Session, t *models.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d %q", t.ID, t.Title)
	if p, err := models.GetProjectSimpleByID(s, t.ProjectID); err == nil {
		fmt.Fprintf(&b, " in %s", p.Title)
	}
	if sched := e.scheduleText(t); sched != "" {
		b.WriteString(", " + sched)
	}
	if t.RepeatRule != "" {
		fmt.Fprintf(&b, ", repeats %s", t.RepeatRule)
		if t.RepeatMode == models.TaskRepeatModeFromCurrentDate {
			b.WriteString(" from completion")
		}
	} else if t.IsRepeating() {
		b.WriteString(", repeats")
	}
	switch {
	case t.Priority >= 3:
		b.WriteString(", priority high")
	case t.Priority == 2:
		b.WriteString(", priority medium")
	case t.Priority == 1:
		b.WriteString(", priority low")
	}
	if len(t.Labels) > 0 {
		names := make([]string, 0, len(t.Labels))
		for _, l := range t.Labels {
			names = append(names, l.Title)
		}
		b.WriteString(", labels " + strings.Join(names, ", "))
	}
	if t.Done {
		b.WriteString(", done")
	}
	if base := strings.TrimRight(config.ServicePublicURL.GetString(), "/"); base != "" {
		fmt.Fprintf(&b, " — %s/tasks/p%d/%d", base, t.ProjectID, t.ID)
	}
	return b.String()
}

func (e *executor) scheduleText(t *models.Task) string {
	const day = "Mon 2 Jan 2006"
	const clock = "15:04"
	start, end, due := t.StartDate.In(e.loc), t.EndDate.In(e.loc), t.DueDate.In(e.loc)
	switch {
	case !t.StartDate.IsZero() && !t.EndDate.IsZero() && end.After(start):
		if start.Hour() == 0 && start.Minute() == 0 && isAllDay(end) {
			return "from " + start.Format(day) + " to " + end.Format(day)
		}
		if start.YearDay() == end.YearDay() && start.Year() == end.Year() {
			return start.Format(day+" "+clock) + "–" + end.Format(clock) + " (" + formatLength(end.Sub(start)) + ")"
		}
		return start.Format(day+" "+clock) + " – " + end.Format(day+" "+clock)
	case !t.DueDate.IsZero():
		if isAllDay(due) {
			return "due " + due.Format(day)
		}
		return "due " + due.Format(day+" "+clock)
	}
	return ""
}
