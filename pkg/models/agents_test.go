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
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	vmail "code.vikunja.io/api/pkg/mail"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures: agent_configs.yml (user1 has Claude and Instinct set up),
// project_agent_settings.yml (project 1 has AI context), agent_dispatches.yml.
// user1 owns project 1 (task 1), only reads project 3 (task 32); user2 can't see project 1.

type firedClaude struct {
	url, token, text string
}

// fakeAgents swaps out the network calls and records what would have been sent.
func fakeAgents(t *testing.T, claudeErr error) (fired *[]firedClaude, mails *[]*vmail.Opts) {
	fired = &[]firedClaude{}
	mails = &[]*vmail.Opts{}
	origFire, origMail := FireClaudeRoutine, SendAgentMail
	FireClaudeRoutine = func(_ context.Context, url, token, text string) (string, error) {
		*fired = append(*fired, firedClaude{url, token, text})
		if claudeErr != nil {
			return "", claudeErr
		}
		return "https://claude.ai/code/session_test", nil
	}
	SendAgentMail = func(opts *vmail.Opts) error {
		*mails = append(*mails, opts)
		return nil
	}
	mailer := config.MailerEnabled.GetBool()
	config.MailerEnabled.Set(true)
	t.Cleanup(func() {
		FireClaudeRoutine, SendAgentMail = origFire, origMail
		config.MailerEnabled.Set(mailer)
	})
	return fired, mails
}

func TestAgentPermissions(t *testing.T) {
	u1 := &user.User{ID: 1}
	u2 := &user.User{ID: 2}
	share := &LinkSharing{ID: 1, ProjectID: 1, Permission: PermissionAdmin}

	t.Run("config belongs to the user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		c := &AgentConfig{UserID: 99}
		can, _, err := c.CanRead(s, u2)
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int64(2), c.UserID, "the auth decides whose config is read")
	})
	t.Run("link share has no config", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, _, err := (&AgentConfig{}).CanRead(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&AgentConfig{}).CanUpdate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("list settings follow list access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, _, err := (&ProjectAgentSettings{ProjectID: 1}).CanRead(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
		can, err = (&ProjectAgentSettings{ProjectID: 1}).CanUpdate(s, u1)
		require.NoError(t, err)
		assert.True(t, can)

		can, _, err = (&ProjectAgentSettings{ProjectID: 1}).CanRead(s, u2)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&ProjectAgentSettings{ProjectID: 3}).CanUpdate(s, u1)
		require.NoError(t, err)
		assert.False(t, can, "read only access can't change the AI context")
		can, err = (&ProjectAgentSettings{ProjectID: 1}).CanUpdate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("dispatch needs write access to the task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&AgentDispatch{TaskID: 1}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
		can, err = (&AgentDispatch{TaskID: 32}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&AgentDispatch{TaskID: 1}).CanCreate(s, u2)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&AgentDispatch{TaskID: 1}).CanCreate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("task dispatches need read access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, _, _, err := (&AgentDispatch{TaskID: 1}).ReadAll(s, u2, "", 0, 0)
		require.Error(t, err)
		result, _, _, err := (&AgentDispatch{TaskID: 1}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestAgentConfig_Update(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("keeps the token when none is sent and never returns it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		c := &AgentConfig{UserID: 1, ClaudeFireURL: "https://api.anthropic.com/v1/claude_code/routines/trig_test/fire", InstinctEmail: "bot@example.com"}
		require.NoError(t, c.Update(s, u1))
		assert.Empty(t, c.ClaudeToken)
		assert.True(t, c.ClaudeTokenSet)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "agent_configs", map[string]any{"user_id": 1, "claude_token": "sk-ant-oat01-test", "instinct_email": "bot@example.com"}, false)
	})
	t.Run("clearing the URL forgets the token", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		c := &AgentConfig{UserID: 1}
		require.NoError(t, c.Update(s, u1))
		assert.False(t, c.ClaudeTokenSet)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "agent_configs", map[string]any{"user_id": 1, "claude_token": ""}, false)
	})
	t.Run("creates a config on first save", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		c := &AgentConfig{UserID: 2, ClaudeFireURL: "https://api.anthropic.com/v1/claude_code/routines/trig_2/fire", ClaudeToken: "tok", SystemPrompt: "Be brief."}
		require.NoError(t, c.Update(s, &user.User{ID: 2}))
		assert.True(t, c.ClaudeTokenSet)
		assert.Equal(t, DefaultAgentSystemPrompt, c.DefaultSystemPrompt)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "agent_configs", map[string]any{"user_id": 2, "claude_token": "tok", "system_prompt": "Be brief."}, false)
	})
	t.Run("rejects fire URLs on other hosts", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		for _, bad := range []string{
			"http://api.anthropic.com/v1/claude_code/routines/x/fire",
			"https://evil.example.com/v1/claude_code/routines/x/fire",
			"https://api.anthropic.com/v1/messages",
			"https://user@api.anthropic.com/v1/claude_code/routines/x/fire",
		} {
			err := (&AgentConfig{UserID: 1, ClaudeFireURL: bad}).Update(s, u1)
			assert.True(t, IsErrInvalidClaudeFireURL(err), bad)
		}
	})
	t.Run("rejects an invalid email", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&AgentConfig{UserID: 1, InstinctEmail: "not an email"}).Update(s, u1)
		assert.True(t, IsErrInvalidAgentEmail(err))
	})
}

func TestProjectAgentSettings(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("reads empty settings for a list without any", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		p := &ProjectAgentSettings{ProjectID: 2}
		require.NoError(t, p.ReadOne(s, u1))
		assert.Empty(t, p.AIContext)
		assert.Empty(t, p.Agent)
	})
	t.Run("turning the agent on remembers who did it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		p := &ProjectAgentSettings{ProjectID: 1, Agent: AgentClaude, AIContext: " Write in French. "}
		require.NoError(t, p.Update(s, u1))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "project_agent_settings", map[string]any{"project_id": 1, "agent": "claude", "agent_user_id": 1, "ai_context": "Write in French."}, false)

		s2 := db.NewSession()
		defer s2.Close()
		p = &ProjectAgentSettings{ProjectID: 1}
		require.NoError(t, p.Update(s2, u1))
		require.NoError(t, s2.Commit())
		db.AssertExists(t, "project_agent_settings", map[string]any{"project_id": 1, "agent": "", "agent_user_id": 0}, false)
	})
	t.Run("rejects unknown agents", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectAgentSettings{ProjectID: 1, Agent: "gpt"}).Update(s, u1)
		assert.True(t, IsErrInvalidAgent(err))
	})
	t.Run("lists only accessible lists", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		result, _, _, err := (&ProjectAgentSettings{}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		result, _, _, err = (&ProjectAgentSettings{}).ReadAll(s, &user.User{ID: 2}, "", 0, 0)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestBuildAgentPrompt(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	publicURL := config.ServicePublicURL.GetString()
	config.ServicePublicURL.Set("https://opentick.test/")
	t.Cleanup(func() { config.ServicePublicURL.Set(publicURL) })
	s := db.NewSession()
	defer s.Close()

	prompt, err := PreviewAgentPrompt(s, 1, 1, "Keep it short.")
	require.NoError(t, err)
	assert.Contains(t, prompt, DefaultAgentSystemPrompt)
	assert.Contains(t, prompt, "Today: ")
	assert.Contains(t, prompt, time.Now().Format("2006"))
	assert.Contains(t, prompt, "List: Test1")
	assert.Contains(t, prompt, "Task: task #1")
	assert.Contains(t, prompt, "Task ID: 1")
	assert.Contains(t, prompt, "Task link: https://opentick.test/tasks/p1/1")
	assert.Contains(t, prompt, "List context:\nAlways write in French.")
	assert.Contains(t, prompt, "Task notes:\nLorem Ipsum")
	assert.Contains(t, prompt, "Additional request:\nKeep it short.")

	_, err = s.Exec("UPDATE agent_configs SET system_prompt = ? WHERE user_id = 1", "My own instructions.")
	require.NoError(t, err)
	prompt, err = PreviewAgentPrompt(s, 1, 1, "")
	require.NoError(t, err)
	assert.True(t, len(prompt) > 0 && prompt[:20] == "My own instructions.")
	assert.NotContains(t, prompt, "Additional request")
}

func TestAgentDispatch_Create(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("claude", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fired, _ := fakeAgents(t, nil)
		s := db.NewSession()
		defer s.Close()
		d := &AgentDispatch{TaskID: 1, Agent: AgentClaude, ExtraPrompt: "Draft it."}
		require.NoError(t, d.Create(s, u1))
		require.Len(t, *fired, 1)
		assert.Equal(t, "sk-ant-oat01-test", (*fired)[0].token)
		assert.Contains(t, (*fired)[0].text, "Additional request:\nDraft it.")
		assert.Equal(t, AgentDispatchSent, d.Status)
		assert.Equal(t, "https://claude.ai/code/session_test", d.SessionURL)
		assert.NotEmpty(t, d.Prompt)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "agent_dispatches", map[string]any{"task_id": 1, "agent": "claude", "status": "sent", "created_by_id": 1, "automatic": false}, false)
	})
	t.Run("instinct", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		_, mails := fakeAgents(t, nil)
		s := db.NewSession()
		defer s.Close()
		d := &AgentDispatch{TaskID: 1, Agent: AgentInstinct}
		require.NoError(t, d.Create(s, u1))
		require.Len(t, *mails, 1)
		assert.Equal(t, "instinct@example.com", (*mails)[0].To)
		assert.Equal(t, "OpenTick task: task #1", (*mails)[0].Subject)
		assert.Contains(t, (*mails)[0].Message, "Task: task #1")
		assert.Equal(t, AgentDispatchSent, d.Status)
	})
	t.Run("a failed delivery is recorded", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeAgents(t, errors.New("claude returned 401: bad token"))
		s := db.NewSession()
		defer s.Close()
		d := &AgentDispatch{TaskID: 1, Agent: AgentClaude}
		require.NoError(t, d.Create(s, u1))
		assert.Equal(t, AgentDispatchFailed, d.Status)
		assert.Contains(t, d.Error, "401")
	})
	t.Run("an agent that isn't set up is refused", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeAgents(t, nil)
		s := db.NewSession()
		defer s.Close()
		err := (&AgentDispatch{TaskID: 1, Agent: AgentClaude}).Create(s, &user.User{ID: 2})
		assert.True(t, IsErrAgentNotConfigured(err))
	})
	t.Run("instinct needs a mailer", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeAgents(t, nil)
		config.MailerEnabled.Set(false)
		s := db.NewSession()
		defer s.Close()
		err := (&AgentDispatch{TaskID: 1, Agent: AgentInstinct}).Create(s, u1)
		assert.True(t, IsErrAgentMailerDisabled(err))
	})
}

func TestDispatchTasksToListAgent(t *testing.T) {
	t.Run("sends new tasks of an agent list", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fired, _ := fakeAgents(t, nil)
		s := db.NewSession()
		_, err := s.Exec("UPDATE project_agent_settings SET agent = 'claude', agent_user_id = 1 WHERE project_id = 1")
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		s.Close()

		events.TestListener(t, &TaskCreatedEvent{Task: &Task{ID: 1, ProjectID: 1}, Doer: &user.User{ID: 1}}, &DispatchTasksToListAgent{})
		require.Len(t, *fired, 1)
		db.AssertExists(t, "agent_dispatches", map[string]any{"task_id": 1, "status": "sent", "automatic": true}, false)
	})
	t.Run("ignores lists without an agent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fired, _ := fakeAgents(t, nil)
		events.TestListener(t, &TaskCreatedEvent{Task: &Task{ID: 1, ProjectID: 1}, Doer: &user.User{ID: 1}}, &DispatchTasksToListAgent{})
		assert.Empty(t, *fired)
		db.AssertMissing(t, "agent_dispatches", map[string]any{"automatic": true})
	})
	t.Run("records a missing setup instead of sending", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fired, _ := fakeAgents(t, nil)
		s := db.NewSession()
		_, err := s.Exec("UPDATE project_agent_settings SET agent = 'claude', agent_user_id = 2 WHERE project_id = 1")
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		s.Close()

		events.TestListener(t, &TaskCreatedEvent{Task: &Task{ID: 1, ProjectID: 1}, Doer: &user.User{ID: 1}}, &DispatchTasksToListAgent{})
		assert.Empty(t, *fired)
		db.AssertExists(t, "agent_dispatches", map[string]any{"task_id": 1, "status": "failed", "automatic": true, "created_by_id": 2}, false)
	})
}
