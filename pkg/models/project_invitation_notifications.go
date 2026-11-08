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
	"code.vikunja.io/api/pkg/i18n"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"
)

func init() {
	notifications.Register(func() notifications.PersistedNotification { return &ProjectInvitationNotification{} })
	notifications.Register(func() notifications.PersistedNotification { return &ProjectSharedNotification{} })
	notifications.Register(func() notifications.PersistedNotification { return &ProjectInvitationAcceptedNotification{} })
	notifications.Register(func() notifications.PersistedNotification { return &SharedTaskDoneNotification{} })
}

func newInvitedProject(p *Project) *invitedProject {
	return &invitedProject{ID: p.ID, Title: p.Title}
}

// invitedProject is the part of a project an invitee may see before joining.
type invitedProject struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// ProjectInvitationNotification tells a user they were invited to a project.
type ProjectInvitationNotification struct {
	Doer         *user.User      `json:"doer"`
	Project      *invitedProject `json:"project"`
	InvitationID int64           `json:"invitation_id"`
	Permission   Permission      `json:"permission"`
	// Holds the invitation token for email invitations; mail only.
	actionURL string
}

// ProjectID implements notifications.ProjectID. The invitee can't read the
// project yet, so the notification belongs to their account.
func (n *ProjectInvitationNotification) ProjectID() int64 { return 0 }

func (n *ProjectInvitationNotification) projectTitle() string {
	if n.Project == nil {
		return ""
	}
	return n.Project.Title
}

// ToTitle returns the translated one-line title.
func (n *ProjectInvitationNotification) ToTitle(lang string) string {
	return i18n.T(lang, "notifications.project.invitation.subject", n.Doer.GetName(), n.projectTitle())
}

// ToMail returns the mail notification.
func (n *ProjectInvitationNotification) ToMail(lang string) *notifications.Mail {
	return notifications.NewMail().
		From(n.Doer.GetNameAndFromEmail()).
		Line(i18n.T(lang, "notifications.project.invitation.message", notifications.EscapeMarkdown(n.Doer.GetName()), notifications.EscapeMarkdown(n.projectTitle()))).
		Action(i18n.T(lang, "notifications.project.invitation.action"), n.actionURL)
}

// ToDB returns the notification as stored in the database. The action URL,
// which may carry the invitation token, is not stored.
func (n *ProjectInvitationNotification) ToDB() interface{} {
	return n
}

// Name returns the name of the notification.
func (n *ProjectInvitationNotification) Name() string {
	return "project.invitation"
}

// ProjectSharedNotification tells a user a project was shared with them.
type ProjectSharedNotification struct {
	Doer    *user.User `json:"doer"`
	Project *Project   `json:"project"`
}

// ProjectID implements notifications.ProjectID.
func (n *ProjectSharedNotification) ProjectID() int64 { return notificationProjectID(nil, n.Project) }

// ToTitle returns the translated one-line title.
func (n *ProjectSharedNotification) ToTitle(lang string) string {
	return i18n.T(lang, "notifications.project.shared.subject", n.Doer.GetName(), n.Project.Title)
}

// ToMail returns the mail notification.
func (n *ProjectSharedNotification) ToMail(lang string) *notifications.Mail {
	return notifications.NewMail().
		From(n.Doer.GetNameAndFromEmail()).
		Line(i18n.T(lang, "notifications.project.shared.message", notifications.EscapeMarkdown(n.Doer.GetName()), notifications.EscapeMarkdown(n.Project.Title))).
		Action(i18n.T(lang, "notifications.common.actions.open_project"), projectFrontendURL(n.Project.ID))
}

// ToDB returns the notification as stored in the database.
func (n *ProjectSharedNotification) ToDB() interface{} {
	return n
}

// Name returns the name of the notification.
func (n *ProjectSharedNotification) Name() string {
	return "project.shared"
}

// ProjectInvitationAcceptedNotification tells the inviter someone joined.
type ProjectInvitationAcceptedNotification struct {
	Member  *user.User `json:"member"`
	Project *Project   `json:"project"`
}

// ProjectID implements notifications.ProjectID.
func (n *ProjectInvitationAcceptedNotification) ProjectID() int64 {
	return notificationProjectID(nil, n.Project)
}

// ToTitle returns the translated one-line title.
func (n *ProjectInvitationAcceptedNotification) ToTitle(lang string) string {
	return i18n.T(lang, "notifications.project.invitation.accepted", n.Member.GetName(), n.Project.Title)
}

// ToMail returns the mail notification.
func (n *ProjectInvitationAcceptedNotification) ToMail(lang string) *notifications.Mail {
	return notifications.NewMail().
		Line(i18n.T(lang, "notifications.project.invitation.accepted", notifications.EscapeMarkdown(n.Member.GetName()), notifications.EscapeMarkdown(n.Project.Title))).
		Action(i18n.T(lang, "notifications.common.actions.open_project"), projectFrontendURL(n.Project.ID))
}

// ToDB returns the notification as stored in the database.
func (n *ProjectInvitationAcceptedNotification) ToDB() interface{} {
	return n
}

// Name returns the name of the notification.
func (n *ProjectInvitationAcceptedNotification) Name() string {
	return "project.invitation.accepted"
}

// SharedTaskDoneNotification tells a project member someone else completed a task.
type SharedTaskDoneNotification struct {
	Doer    *user.User `json:"doer"`
	Task    *Task      `json:"task"`
	Project *Project   `json:"project"`
}

// ProjectID implements notifications.ProjectID.
func (n *SharedTaskDoneNotification) ProjectID() int64 {
	return notificationProjectID(n.Task, n.Project)
}

// ToTitle returns the translated one-line title.
func (n *SharedTaskDoneNotification) ToTitle(lang string) string {
	return i18n.T(lang, "notifications.task.done.subject", n.Task.Title, n.Task.GetFullIdentifier())
}

// ToMail returns the mail notification.
func (n *SharedTaskDoneNotification) ToMail(lang string) *notifications.Mail {
	return notifications.NewMail().
		From(n.Doer.GetNameAndFromEmail()).
		Line(i18n.T(lang, "notifications.task.done.message", notifications.EscapeMarkdown(n.Doer.GetName()), notifications.EscapeMarkdown(n.Task.Title), notifications.EscapeMarkdown(n.Task.GetFullIdentifier()))).
		Action(i18n.T(lang, "notifications.common.actions.open_task"), n.Task.GetFrontendURL()).
		IncludeLinkToSettings(lang)
}

// ToDB returns the notification as stored in the database.
func (n *SharedTaskDoneNotification) ToDB() interface{} {
	return n
}

// Name returns the name of the notification.
func (n *SharedTaskDoneNotification) Name() string {
	return "task.done"
}

// ThreadID returns the thread ID for email threading.
func (n *SharedTaskDoneNotification) ThreadID() string {
	return getThreadID(n.Task.ID)
}
