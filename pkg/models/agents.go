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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	vmail "code.vikunja.io/api/pkg/mail"
	"code.vikunja.io/api/pkg/richtext"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// Agent names a bot a task can be handed to.
const (
	AgentClaude   = "claude"
	AgentInstinct = "instinct"
)

// Statuses of an AgentDispatch.
const (
	AgentDispatchSent   = "sent"
	AgentDispatchFailed = "failed"
)

// DefaultAgentSystemPrompt is sent ahead of every task unless the user saved their own.
const DefaultAgentSystemPrompt = `You are an assistant working on a task handed to you from OpenTick, the user's to-do app.

- Do the task described below as completely as you can on your own.
- Follow the list context: it says how work for this list should be done (for example which language or tone to use).
- If you can reach OpenTick (through its MCP connector, or by replying to this email when it explains how), post a short summary of what you did as a comment on the task, using the task ID below. Mark the task as done only when it is fully finished.
- If something is unclear or needs the user's decision, ask in a task comment (or in your reply) instead of guessing.
- Never take irreversible or outward-facing actions, such as sending messages, making payments or deleting data, without the user's explicit approval.`

func validAgent(agent string) bool {
	return agent == AgentClaude || agent == AgentInstinct
}

// AgentConfig holds how one user's tasks reach their AI agents.
type AgentConfig struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	UserID int64 `xorm:"bigint not null unique" json:"-"`

	ClaudeFireURL string `xorm:"varchar(500) null" json:"claude_fire_url" maxLength:"500" doc:"The fire URL of a Claude Code scheduled task with an API trigger (https://api.anthropic.com/v1/claude_code/routines/<id>/fire). Clearing it also forgets the token."`
	ClaudeToken   string `xorm:"text null" json:"claude_token,omitempty" writeOnly:"true" doc:"The bearer token of that API trigger. Write-only: never returned. Leave it empty on an update to keep the stored one."`
	// Whether a Claude token is stored.
	ClaudeTokenSet bool `xorm:"-" json:"claude_token_set" readOnly:"true" doc:"Whether a Claude API trigger token is stored."`

	InstinctEmail string `xorm:"varchar(250) null" json:"instinct_email" maxLength:"250" doc:"The address tasks sent to Instinct are emailed to."`

	SystemPrompt        string `xorm:"longtext null" json:"system_prompt" maxLength:"20000" doc:"The instructions sent ahead of every task. Empty means the default in default_system_prompt."`
	DefaultSystemPrompt string `xorm:"-" json:"default_system_prompt" readOnly:"true" doc:"The built-in system prompt used while system_prompt is empty."`

	MailerEnabled bool `xorm:"-" json:"mailer_enabled" readOnly:"true" doc:"Whether this server can send email, which sending to Instinct needs."`

	// The private part of the user's inbound address. Whoever knows it can change the user's tasks by email.
	InboundSecret    string `xorm:"varchar(32) null INDEX 'inbound_secret'" json:"-"`
	InboundAddress   string `xorm:"-" json:"inbound_address" readOnly:"true" doc:"The private address agents (and you) email commands to, to create, change and delete your tasks. Empty until the server reads a mailbox and an address was created."`
	InboundEnabled   bool   `xorm:"-" json:"inbound_enabled" readOnly:"true" doc:"Whether this server reads a mailbox, which the inbound address needs."`
	InboundSenders   string `xorm:"text null 'inbound_senders'" json:"inbound_senders" maxLength:"2000" doc:"Comma-separated addresses allowed to email the inbound address. Your account email and the Instinct address are always allowed. A domain like @example.com allows every address of it."`
	InboundProjectID int64  `xorm:"bigint not null default 0 'inbound_project_id'" json:"inbound_project_id" doc:"The list tasks emailed without a list go to. 0 means your default list."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the config was first saved."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"When the config was last saved."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for agent configs.
func (*AgentConfig) TableName() string {
	return "agent_configs"
}

func getAgentConfig(s *xorm.Session, userID int64) (*AgentConfig, error) {
	c := &AgentConfig{}
	has, err := s.Where("user_id = ?", userID).Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		c = &AgentConfig{UserID: userID}
	}
	return c, nil
}

func (c *AgentConfig) fillComputed() {
	c.ClaudeTokenSet = c.ClaudeToken != ""
	c.ClaudeToken = ""
	c.DefaultSystemPrompt = DefaultAgentSystemPrompt
	c.MailerEnabled = config.MailerEnabled.GetBool()
	c.InboundEnabled = InboundMailEnabled()
	c.InboundAddress = ""
	if c.InboundEnabled && c.InboundSecret != "" {
		c.InboundAddress = InboundAddressFor(c.InboundSecret, 0)
	}
}

func (c *AgentConfig) systemPrompt() string {
	if strings.TrimSpace(c.SystemPrompt) == "" {
		return DefaultAgentSystemPrompt
	}
	return c.SystemPrompt
}

// ready reports why the given agent can't be used yet, or nil when it can.
func (c *AgentConfig) ready(agent string) error {
	switch agent {
	case AgentClaude:
		if c.ClaudeFireURL == "" || c.ClaudeToken == "" {
			return ErrAgentNotConfigured{Agent: agent}
		}
	case AgentInstinct:
		if c.InstinctEmail == "" {
			return ErrAgentNotConfigured{Agent: agent}
		}
		if !config.MailerEnabled.GetBool() {
			return ErrAgentMailerDisabled{}
		}
	default:
		return ErrInvalidAgent{Agent: agent}
	}
	return nil
}

// validateClaudeFireURL only accepts Anthropic's routine fire endpoint, so the
// server can't be pointed at arbitrary hosts.
func validateClaudeFireURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "api.anthropic.com" ||
		!strings.HasPrefix(u.Path, "/v1/claude_code/routines/") || !strings.HasSuffix(u.Path, "/fire") ||
		u.RawQuery != "" || u.User != nil {
		return ErrInvalidClaudeFireURL{}
	}
	return nil
}

// ReadOne returns the authenticated user's agent config.
func (c *AgentConfig) ReadOne(s *xorm.Session, _ web.Auth) error {
	stored, err := getAgentConfig(s, c.UserID)
	if err != nil {
		return err
	}
	*c = *stored
	c.fillComputed()
	return nil
}

// Update saves the authenticated user's agent config, creating it on first save.
func (c *AgentConfig) Update(s *xorm.Session, a web.Auth) error {
	stored, err := getAgentConfig(s, c.UserID)
	if err != nil {
		return err
	}

	c.ClaudeFireURL = strings.TrimSpace(c.ClaudeFireURL)
	c.InstinctEmail = strings.TrimSpace(c.InstinctEmail)
	if c.ClaudeFireURL != "" {
		if err := validateClaudeFireURL(c.ClaudeFireURL); err != nil {
			return err
		}
	}
	if c.InstinctEmail != "" {
		if _, err := mail.ParseAddress(c.InstinctEmail); err != nil {
			return ErrInvalidAgentEmail{}
		}
	}
	token := strings.TrimSpace(c.ClaudeToken)
	switch {
	case c.ClaudeFireURL == "":
		token = ""
	case token == "":
		token = stored.ClaudeToken
	}
	if strings.TrimSpace(c.SystemPrompt) == strings.TrimSpace(DefaultAgentSystemPrompt) {
		c.SystemPrompt = ""
	}
	senders, err := normalizeInboundSenders(c.InboundSenders)
	if err != nil {
		return err
	}
	if c.InboundProjectID < 0 {
		c.InboundProjectID = 0
	}
	if c.InboundProjectID != stored.InboundProjectID && c.InboundProjectID > 0 {
		can, err := (&Project{ID: c.InboundProjectID}).CanWrite(s, a)
		if err != nil {
			return err
		}
		if !can {
			return ErrGenericForbidden{}
		}
	}
	if stored.InboundSecret == "" {
		if stored.InboundSecret, err = newInboundSecret(); err != nil {
			return err
		}
	}

	stored.ClaudeFireURL = c.ClaudeFireURL
	stored.ClaudeToken = token
	stored.InstinctEmail = c.InstinctEmail
	stored.SystemPrompt = c.SystemPrompt
	stored.InboundSenders = senders
	stored.InboundProjectID = c.InboundProjectID
	if stored.ID == 0 {
		_, err = s.Insert(stored)
	} else {
		_, err = s.ID(stored.ID).Cols("claude_fire_url", "claude_token", "instinct_email", "system_prompt",
			"inbound_secret", "inbound_senders", "inbound_project_id").Update(stored)
	}
	if err != nil {
		return err
	}

	*c = *stored
	c.fillComputed()
	return nil
}

// ProjectAgentSettings holds the AI settings of one list: context for the bots
// and, optionally, an agent every new task in the list goes to.
type ProjectAgentSettings struct {
	ID        int64  `xorm:"bigint autoincr not null unique pk" json:"-"`
	ProjectID int64  `xorm:"bigint not null unique" json:"project_id" readOnly:"true" doc:"The list these settings belong to. Set from the URL."`
	AIContext string `xorm:"longtext null 'ai_context'" json:"ai_context" maxLength:"10000" doc:"Context sent to AI bots with every task of this list, such as the language or tone to use."`
	Agent     string `xorm:"varchar(20) not null default ''" json:"agent" enum:",claude,instinct" doc:"When set, every task created in this list is handed to this agent automatically. Empty for none."`
	// The user whose agent config runs the automatic dispatches: whoever last turned the agent on.
	AgentUserID int64 `xorm:"bigint not null default 0" json:"-"`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the settings were first saved."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"When the settings were last saved."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for per-list agent settings.
func (*ProjectAgentSettings) TableName() string {
	return "project_agent_settings"
}

func getProjectAgentSettings(s *xorm.Session, projectID int64) (*ProjectAgentSettings, error) {
	p := &ProjectAgentSettings{}
	has, err := s.Where("project_id = ?", projectID).Get(p)
	if err != nil {
		return nil, err
	}
	if !has {
		p = &ProjectAgentSettings{ProjectID: projectID}
	}
	return p, nil
}

// ReadOne returns the agent settings of a list; a list without any returns empty settings.
func (p *ProjectAgentSettings) ReadOne(s *xorm.Session, _ web.Auth) error {
	stored, err := getProjectAgentSettings(s, p.ProjectID)
	if err != nil {
		return err
	}
	*p = *stored
	return nil
}

// ReadAll returns the agent settings of every list the user can access that has any.
func (p *ProjectAgentSettings) ReadAll(s *xorm.Session, a web.Auth, _ string, _ int, _ int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, 0, 0, err
	}
	projects, _, err := getAllProjectsForUser(s, u.ID, &projectOptions{user: u, getArchived: true})
	if err != nil {
		return nil, 0, 0, err
	}
	ids := make([]int64, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	all := []*ProjectAgentSettings{}
	if len(ids) > 0 {
		if err := s.In("project_id", ids).OrderBy("project_id").Find(&all); err != nil {
			return nil, 0, 0, err
		}
	}
	return all, len(all), int64(len(all)), nil
}

// Update saves the agent settings of a list.
func (p *ProjectAgentSettings) Update(s *xorm.Session, a web.Auth) error {
	if p.Agent != "" && !validAgent(p.Agent) {
		return ErrInvalidAgent{Agent: p.Agent}
	}
	stored, err := getProjectAgentSettings(s, p.ProjectID)
	if err != nil {
		return err
	}

	switch {
	case p.Agent == "":
		stored.AgentUserID = 0
	case p.Agent != stored.Agent || stored.AgentUserID == 0:
		u, err := user.GetFromAuth(a)
		if err != nil {
			return err
		}
		stored.AgentUserID = u.ID
	}
	stored.Agent = p.Agent
	stored.AIContext = strings.TrimSpace(p.AIContext)

	if stored.ID == 0 {
		_, err = s.Insert(stored)
	} else {
		_, err = s.ID(stored.ID).Cols("ai_context", "agent", "agent_user_id").Update(stored)
	}
	if err != nil {
		return err
	}
	*p = *stored
	return nil
}

// AgentDispatch records one hand-off of a task to an AI agent.
type AgentDispatch struct {
	ID          int64  `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The id of this dispatch."`
	TaskID      int64  `xorm:"bigint not null INDEX" json:"task_id" readOnly:"true" doc:"The task that was handed off. Set from the URL."`
	Agent       string `xorm:"varchar(20) not null" json:"agent" enum:"claude,instinct" doc:"The agent the task goes to."`
	Status      string `xorm:"varchar(20) not null" json:"status" readOnly:"true" enum:"sent,failed" doc:"sent once the agent accepted the task, failed when delivery failed (see error)."`
	ExtraPrompt string `xorm:"text null" json:"extra_prompt" maxLength:"10000" doc:"An extra request sent along with this task only."`
	SessionURL  string `xorm:"varchar(500) null" json:"session_url" readOnly:"true" doc:"For Claude: the URL of the cloud session working on the task."`
	Error       string `xorm:"text null" json:"error" readOnly:"true" doc:"Why delivery failed."`
	Automatic   bool   `xorm:"bool not null default false" json:"automatic" readOnly:"true" doc:"Whether the task was sent automatically because its list belongs to the agent."`
	CreatedByID int64  `xorm:"bigint not null INDEX" json:"-"`

	// The full text sent to the agent. Only returned right after a dispatch.
	Prompt string `xorm:"-" json:"prompt,omitempty" readOnly:"true" doc:"The full text sent to the agent. Only returned by the create call."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the task was handed off."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for agent dispatches.
func (*AgentDispatch) TableName() string {
	return "agent_dispatches"
}

// ReadAll returns the dispatches of one task (TaskID set) or, without a task,
// the latest dispatch of every task the user handed off, newest first.
func (d *AgentDispatch) ReadAll(s *xorm.Session, a web.Auth, _ string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	all := []*AgentDispatch{}
	if d.TaskID != 0 {
		task := &Task{ID: d.TaskID}
		can, _, err := task.CanRead(s, a)
		if err != nil {
			return nil, 0, 0, err
		}
		if !can {
			return nil, 0, 0, ErrGenericForbidden{}
		}
		if err := s.Where("task_id = ?", d.TaskID).OrderBy("id DESC").Find(&all); err != nil {
			return nil, 0, 0, err
		}
		return all, len(all), int64(len(all)), nil
	}

	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, 0, 0, err
	}
	latest := builder.Select("MAX(id)").From("agent_dispatches").Where(builder.Eq{"created_by_id": u.ID}).GroupBy("task_id")
	query := s.Where("created_by_id = ?", u.ID).In("id", latest).OrderBy("id DESC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	total, err := query.FindAndCount(&all)
	if err != nil {
		return nil, 0, 0, err
	}
	return all, len(all), total, nil
}

// Create hands the task to the agent and records the outcome. A failed
// delivery is stored too (status failed) instead of returned as an error, so
// the task shows what went wrong.
func (d *AgentDispatch) Create(s *xorm.Session, a web.Auth) error {
	if !validAgent(d.Agent) {
		return ErrInvalidAgent{Agent: d.Agent}
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}
	cfg, err := getAgentConfig(s, u.ID)
	if err != nil {
		return err
	}
	if err := cfg.ready(d.Agent); err != nil {
		return err
	}
	task, err := GetTaskByIDSimple(s, d.TaskID)
	if err != nil {
		return err
	}
	d.ExtraPrompt = strings.TrimSpace(d.ExtraPrompt)
	return dispatchToAgent(s, cfg, &task, d)
}

func dispatchToAgent(s *xorm.Session, cfg *AgentConfig, task *Task, d *AgentDispatch) error {
	prompt, err := BuildAgentPrompt(s, cfg, task, d.ExtraPrompt)
	if err != nil {
		return err
	}
	d.ID = 0
	d.TaskID = task.ID
	d.CreatedByID = cfg.UserID
	d.Prompt = prompt

	var sendErr error
	switch d.Agent {
	case AgentClaude:
		d.SessionURL, sendErr = FireClaudeRoutine(context.Background(), cfg.ClaudeFireURL, cfg.ClaudeToken, prompt)
	case AgentInstinct:
		opts := &vmail.Opts{
			To:          cfg.InstinctEmail,
			Subject:     "OpenTick task: " + task.Title,
			Message:     prompt,
			ContentType: vmail.ContentTypePlain,
		}
		// With a mailbox to read, Instinct can answer by replying: the reply lands on this task.
		if InboundMailEnabled() && cfg.InboundSecret != "" {
			opts.ReplyTo = InboundAddressFor(cfg.InboundSecret, task.ID)
			opts.Message += "\n\n" + InboundReplyInstructions(task.ID)
		}
		sendErr = SendAgentMail(opts)
	}
	d.Status = AgentDispatchSent
	d.Error = ""
	if sendErr != nil {
		log.Errorf("Could not hand task %d to %s: %s", task.ID, d.Agent, sendErr)
		d.Status = AgentDispatchFailed
		d.Error = sendErr.Error()
	}
	_, err = s.Insert(d)
	return err
}

// AgentPromptPreview is the text a task would be sent to an agent with.
type AgentPromptPreview struct {
	Prompt string `json:"prompt" doc:"The full text the agent would receive."`
}

// PreviewAgentPrompt returns the text the task would be sent with, using the user's agent config.
// The caller checks read access to the task.
func PreviewAgentPrompt(s *xorm.Session, userID, taskID int64, extra string) (string, error) {
	cfg, err := getAgentConfig(s, userID)
	if err != nil {
		return "", err
	}
	task, err := GetTaskByIDSimple(s, taskID)
	if err != nil {
		return "", err
	}
	return BuildAgentPrompt(s, cfg, &task, extra)
}

// BuildAgentPrompt assembles the text an agent gets for a task: the system
// prompt, today's date, the task and list, the list's AI context and the extra request.
func BuildAgentPrompt(s *xorm.Session, cfg *AgentConfig, task *Task, extra string) (string, error) {
	project, err := GetProjectSimpleByID(s, task.ProjectID)
	if err != nil {
		return "", err
	}
	settings, err := getProjectAgentSettings(s, task.ProjectID)
	if err != nil {
		return "", err
	}
	loc := config.GetTimeZone()
	if u, err := user.GetUserByID(s, cfg.UserID); err == nil && u.Timezone != "" {
		if l, err := time.LoadLocation(u.Timezone); err == nil {
			loc = l
		}
	}
	now := time.Now().In(loc)

	var b strings.Builder
	b.WriteString(strings.TrimSpace(cfg.systemPrompt()))
	b.WriteString("\n\n---\n\n")
	fmt.Fprintf(&b, "Today: %s (%s)\n", now.Format("Monday, 2 January 2006"), now.Format("2006-01-02"))
	fmt.Fprintf(&b, "List: %s\n", project.Title)
	fmt.Fprintf(&b, "Task: %s\n", task.Title)
	fmt.Fprintf(&b, "Task ID: %d\n", task.ID)
	if base := strings.TrimRight(config.ServicePublicURL.GetString(), "/"); base != "" {
		fmt.Fprintf(&b, "Task link: %s/tasks/p%d/%d\n", base, task.ProjectID, task.ID)
	}
	if !task.DueDate.IsZero() {
		fmt.Fprintf(&b, "Due: %s\n", task.DueDate.In(loc).Format("Monday, 2 January 2006 15:04"))
	}
	if settings.AIContext != "" {
		fmt.Fprintf(&b, "\nList context:\n%s\n", settings.AIContext)
	}
	if notes := agentTaskNotes(task.Description); notes != "" {
		fmt.Fprintf(&b, "\nTask notes:\n%s\n", notes)
	}
	if extra = strings.TrimSpace(extra); extra != "" {
		fmt.Fprintf(&b, "\nAdditional request:\n%s\n", extra)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// agentTaskNotes turns the stored (HTML) task description into plain markdown.
func agentTaskNotes(description string) string {
	description = strings.TrimSpace(description)
	if description == "" || richtext.HTMLIsEmpty(description) {
		return ""
	}
	if strings.HasPrefix(description, "<") {
		if md, err := richtext.HTMLToMarkdown(description); err == nil {
			return strings.TrimSpace(md)
		}
	}
	return description
}

// SendAgentMail delivers a task to Instinct. Swapped out in tests.
var SendAgentMail = vmail.SendTestMail

// FireClaudeRoutine starts a Claude Code cloud session through a scheduled
// task's API trigger and returns the session URL. Swapped out in tests.
var FireClaudeRoutine = func(ctx context.Context, fireURL, token, text string) (string, error) {
	if err := validateClaudeFireURL(fireURL); err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fireURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "experimental-cc-routine-2026-04-01")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var problem struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		msg := strings.TrimSpace(string(raw))
		if json.Unmarshal(raw, &problem) == nil && problem.Error.Message != "" {
			msg = problem.Error.Message
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", fmt.Errorf("claude returned %s: %s", strconv.Itoa(resp.StatusCode), msg)
	}
	var fired struct {
		SessionURL string `json:"claude_code_session_url"`
	}
	if err := json.Unmarshal(raw, &fired); err != nil {
		return "", fmt.Errorf("unexpected response from claude: %w", err)
	}
	return fired.SessionURL, nil
}

// DispatchTasksToListAgent hands every new task in an agent list to that list's agent.
type DispatchTasksToListAgent struct{}

// Name defines the name for the DispatchTasksToListAgent listener
func (*DispatchTasksToListAgent) Name() string {
	return "task.created.agent.dispatch"
}

// Handle is executed when a task was created.
func (*DispatchTasksToListAgent) Handle(msg *message.Message) (err error) {
	event := &TaskCreatedEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}
	if event.Task == nil {
		return nil
	}

	s := db.NewSession()
	defer s.Close()

	settings, err := getProjectAgentSettings(s, event.Task.ProjectID)
	if err != nil {
		return err
	}
	if !validAgent(settings.Agent) || settings.AgentUserID == 0 {
		return nil
	}
	cfg, err := getAgentConfig(s, settings.AgentUserID)
	if err != nil {
		return err
	}
	task, err := GetTaskByIDSimple(s, event.Task.ID)
	if err != nil {
		if IsErrTaskDoesNotExist(err) {
			return nil
		}
		return err
	}

	d := &AgentDispatch{Agent: settings.Agent, Automatic: true}
	if err := cfg.ready(settings.Agent); err != nil {
		// Record why nothing was sent, so the task shows it instead of silently staying put.
		d.TaskID = task.ID
		d.CreatedByID = cfg.UserID
		d.Status = AgentDispatchFailed
		d.Error = err.Error()
		var friendly web.HTTPErrorProcessor
		if errors.As(err, &friendly) {
			d.Error = friendly.HTTPError().Message
		}
		if _, err := s.Insert(d); err != nil {
			_ = s.Rollback()
			return err
		}
		return s.Commit()
	}
	if err := dispatchToAgent(s, cfg, &task, d); err != nil {
		_ = s.Rollback()
		return err
	}
	return s.Commit()
}
