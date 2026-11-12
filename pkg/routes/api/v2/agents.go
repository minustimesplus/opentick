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

package apiv2

import (
	"context"
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type projectAgentSettingsListBody struct {
	Body Paginated[*models.ProjectAgentSettings]
}

type agentDispatchListBody struct {
	Body Paginated[*models.AgentDispatch]
}

// RegisterAgentRoutes wires the AI agent hand-off (Claude, Instinct) onto the Huma API.
func RegisterAgentRoutes(api huma.API) {
	tags := []string{"agents"}

	Register(api, huma.Operation{
		OperationID: "agents-config-read",
		Summary:     "Get your agent setup",
		Description: "Returns how the authenticated user's tasks reach Claude and Instinct. The Claude token is never returned; claude_token_set says whether one is stored. Not available to link shares.",
		Method:      http.MethodGet,
		Path:        "/agents/config",
		Tags:        tags,
	}, agentsConfigRead)

	Register(api, huma.Operation{
		OperationID: "agents-config-update",
		Summary:     "Save your agent setup",
		Description: "Replaces the authenticated user's agent setup. An empty claude_token keeps the stored token; clearing claude_fire_url forgets it. The fire URL must be an api.anthropic.com routine fire endpoint. An empty system_prompt means the default.",
		Method:      http.MethodPut,
		Path:        "/agents/config",
		Tags:        tags,
	}, agentsConfigUpdate)

	Register(api, huma.Operation{
		OperationID: "agents-inbound-address-rotate",
		Summary:     "Get a new inbound address",
		Description: "Gives the authenticated user a new private address for emailing task commands (and creates the first one). The previous address stops working right away. Returns the updated agent setup; inbound_address stays empty while the server reads no mailbox.",
		Method:      http.MethodPost,
		Path:        "/agents/inbound-address",
		Tags:        tags,
	}, agentsInboundAddressRotate)

	Register(api, huma.Operation{
		OperationID: "agents-projects-list",
		Summary:     "List agent settings of your lists",
		Description: "Returns the saved AI settings (context for AI bots, agent list) of every list the user can access. Lists without saved settings are left out.",
		Method:      http.MethodGet,
		Path:        "/agents/projects",
		Tags:        tags,
	}, agentsProjectsList)

	Register(api, huma.Operation{
		OperationID: "agents-dispatches-list",
		Summary:     "List your latest hand-offs",
		Description: "Returns the latest dispatch of every task the authenticated user handed to an agent (by hand or through an agent list), newest first.",
		Method:      http.MethodGet,
		Path:        "/agents/dispatches",
		Tags:        tags,
	}, agentsDispatchesList)

	Register(api, huma.Operation{
		OperationID: "projects-agent-settings-read",
		Summary:     "Get a list's agent settings",
		Description: "Returns the context for AI bots and the agent of a list. A list without any returns empty settings. Needs read access to the list.",
		Method:      http.MethodGet,
		Path:        "/projects/{projectid}/agent-settings",
		Tags:        []string{"projects"},
	}, projectAgentSettingsRead)

	Register(api, huma.Operation{
		OperationID: "projects-agent-settings-update",
		Summary:     "Save a list's agent settings",
		Description: "Replaces the context for AI bots and the agent of a list. Needs write access. With an agent set, every task later created in the list is sent to it automatically, using the agent setup of whoever turned the agent on.",
		Method:      http.MethodPut,
		Path:        "/projects/{projectid}/agent-settings",
		Tags:        []string{"projects"},
	}, projectAgentSettingsUpdate)

	Register(api, huma.Operation{
		OperationID: "tasks-agent-dispatches-create",
		Summary:     "Send a task to an agent",
		Description: "Hands the task to Claude (starts a Claude Code cloud session through the configured scheduled task) or Instinct (emails it). Needs write access to the task. A delivery failure is still recorded and returned with status failed; a missing setup is a 400.",
		Method:      http.MethodPost,
		Path:        "/tasks/{taskid}/agent-dispatches",
		Tags:        []string{"tasks"},
	}, taskAgentDispatchCreate)

	Register(api, huma.Operation{
		OperationID: "tasks-agent-dispatches-list",
		Summary:     "List a task's hand-offs",
		Description: "Returns every time the task was handed to an agent, newest first. Needs read access to the task.",
		Method:      http.MethodGet,
		Path:        "/tasks/{taskid}/agent-dispatches",
		Tags:        []string{"tasks"},
	}, taskAgentDispatchesList)

	Register(api, huma.Operation{
		OperationID: "tasks-agent-prompt",
		Summary:     "Preview what an agent gets",
		Description: "Returns the full text a task would be sent to an agent with: the user's system prompt, today's date, task and list name, the list's context for AI bots, the task notes and the extra request. Needs read access to the task.",
		Method:      http.MethodGet,
		Path:        "/tasks/{taskid}/agent-prompt",
		Tags:        []string{"tasks"},
	}, taskAgentPrompt)
}

func init() { AddRouteRegistrar(RegisterAgentRoutes) }

func agentsConfigRead(ctx context.Context, in *struct {
	conditional.Params
}) (*singleReadBody[models.AgentConfig], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	cfg := &models.AgentConfig{}
	maxPermission, err := handler.DoReadOne(ctx, cfg, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	// mailer_enabled comes from the server config, not the row: without it in
	// the ETag, a client that cached the config while the mailer was off keeps
	// getting 304s and says Instinct can't be reached after the mailer is set up.
	return conditionalReadResponseWithState(&in.Params, cfg, cfg.Updated, maxPermission,
		fmt.Sprintf("m%t-i%t-%s", cfg.MailerEnabled, cfg.InboundEnabled, cfg.InboundAddress))
}

func agentsInboundAddressRotate(ctx context.Context, _ *struct{}) (*singleBody[models.AgentConfig], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()
	cfg, err := models.RotateInboundAddress(s, a)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.AgentConfig]{Body: cfg}, nil
}

func agentsConfigUpdate(ctx context.Context, in *struct {
	Body models.AgentConfig
}) (*singleBody[models.AgentConfig], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoUpdate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.AgentConfig]{Body: &in.Body}, nil
}

func agentsProjectsList(ctx context.Context, in *ListParams) (*projectAgentSettingsListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := user.GetFromAuth(a); err != nil {
		return nil, translateDomainError(err)
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.ProjectAgentSettings{}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.ProjectAgentSettings)
	if !ok {
		return nil, fmt.Errorf("projectAgentSettings.ReadAll returned unexpected type %T", result)
	}
	return &projectAgentSettingsListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func agentsDispatchesList(ctx context.Context, in *ListParams) (*agentDispatchListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := user.GetFromAuth(a); err != nil {
		return nil, translateDomainError(err)
	}
	return listDispatches(ctx, a, &models.AgentDispatch{}, in)
}

func listDispatches(ctx context.Context, a web.Auth, d *models.AgentDispatch, in *ListParams) (*agentDispatchListBody, error) {
	result, _, total, err := handler.DoReadAll(ctx, d, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.AgentDispatch)
	if !ok {
		return nil, fmt.Errorf("agentDispatch.ReadAll returned unexpected type %T", result)
	}
	return &agentDispatchListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func projectAgentSettingsRead(ctx context.Context, in *struct {
	ProjectID int64 `path:"projectid" doc:"The numeric id of the list."`
	conditional.Params
}) (*singleReadBody[models.ProjectAgentSettings], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	settings := &models.ProjectAgentSettings{ProjectID: in.ProjectID}
	maxPermission, err := handler.DoReadOne(ctx, settings, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return conditionalReadResponse(&in.Params, settings, settings.Updated, maxPermission)
}

func projectAgentSettingsUpdate(ctx context.Context, in *struct {
	ProjectID int64 `path:"projectid" doc:"The numeric id of the list."`
	Body      models.ProjectAgentSettings
}) (*singleBody[models.ProjectAgentSettings], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	settings := &in.Body
	settings.ProjectID = in.ProjectID // URL wins over body
	if err := handler.DoUpdate(ctx, settings, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.ProjectAgentSettings]{Body: settings}, nil
}

func taskAgentDispatchCreate(ctx context.Context, in *struct {
	TaskID int64 `path:"taskid" doc:"The numeric id of the task to send."`
	Body   models.AgentDispatch
}) (*singleBody[models.AgentDispatch], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	d := &in.Body
	d.TaskID = in.TaskID
	if err := handler.DoCreate(ctx, d, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.AgentDispatch]{Body: d}, nil
}

func taskAgentDispatchesList(ctx context.Context, in *struct {
	TaskID int64 `path:"taskid" doc:"The numeric id of the task."`
	ListParams
}) (*agentDispatchListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	return listDispatches(ctx, a, &models.AgentDispatch{TaskID: in.TaskID}, &in.ListParams)
}

func taskAgentPrompt(ctx context.Context, in *struct {
	TaskID int64  `path:"taskid" doc:"The numeric id of the task."`
	Extra  string `query:"extra" maxLength:"10000" doc:"The extra request to include, as typed in the send dialog."`
}) (*struct{ Body models.AgentPromptPreview }, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	task := &models.Task{ID: in.TaskID}
	can, _, err := task.CanRead(s, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	if !can {
		return nil, huma.Error403Forbidden("forbidden")
	}
	prompt, err := models.PreviewAgentPrompt(s, u.ID, in.TaskID, in.Extra)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &struct{ Body models.AgentPromptPreview }{Body: models.AgentPromptPreview{Prompt: prompt}}, nil
}
