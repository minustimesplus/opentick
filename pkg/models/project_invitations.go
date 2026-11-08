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
	"net/mail"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Kinds of project invitations.
const (
	// ProjectInvitationKindUser invites an existing user by username. It waits
	// for them in their invitation list.
	ProjectInvitationKindUser = "user"
	// ProjectInvitationKindEmail invites an email address. Whoever opens the
	// mailed link joins, and an account with that address also sees it in its
	// invitation list. Used once.
	ProjectInvitationKindEmail = "email"
	// ProjectInvitationKindLink is a join link anyone signed in can use until
	// a project admin revokes it.
	ProjectInvitationKindLink = "link"
)

// ProjectInvitationStatusPending and ProjectInvitationStatusAccepted are the
// values of ProjectInvitation.Status.
const (
	ProjectInvitationStatusPending  = "pending"
	ProjectInvitationStatusAccepted = "accepted"
)

const projectInvitationEmailValidity = 30 * 24 * time.Hour

const projectInvitationTokenLength = 48

// ProjectInvitation invites someone to collaborate on a project. Accepting it
// creates a regular project <-> user share with the invitation's permission.
type ProjectInvitation struct {
	ID         int64      `xorm:"bigint autoincr not null unique pk" json:"id" param:"invitation" readOnly:"true" doc:"The unique, numeric id of this invitation. 0 when an invitation by username was accepted right away."`
	ProjectID  int64      `xorm:"bigint not null INDEX" json:"project_id" param:"project" readOnly:"true" doc:"The project the invitation is for."`
	InviterID  int64      `xorm:"bigint not null INDEX" json:"-"`
	InviteeID  int64      `xorm:"bigint not null default 0 INDEX" json:"-"`
	Email      string     `xorm:"varchar(250) null INDEX" json:"email,omitempty" maxLength:"250" doc:"Invite this email address. On responses: the invited address, for invitations by email."`
	Kind       string     `xorm:"varchar(10) not null" json:"kind" readOnly:"true" enum:"user,email,link" doc:"user: an existing user invited by username. email: an email address, joined through the mailed link. link: a join link anyone signed in can use until it is revoked."`
	Permission Permission `xorm:"bigint not null default 0" json:"permission" maximum:"2" default:"0" doc:"The permission the invitee gets on the project. 0 = Read only, 1 = Read & Write, 2 = Admin."`
	TokenHash  string     `xorm:"varchar(64) null INDEX" json:"-"`
	// Join links stay copyable for project admins, so their token is kept.
	LinkToken string     `xorm:"varchar(64) null" json:"-"`
	Uses      int64      `xorm:"bigint not null default 0" json:"uses" readOnly:"true" doc:"How many people joined through this join link. Always 0 for other kinds."`
	ExpiresAt *time.Time `xorm:"datetime null" json:"expires_at" readOnly:"true" doc:"When the invitation stops working. Null means it does not expire."`
	Created   time.Time  `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this invitation was created."`
	Updated   time.Time  `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this invitation was last updated."`

	// Request only.
	Username string `xorm:"-" json:"username,omitempty" doc:"Invite the user with this username. Set exactly one of username, email and link."`
	Link     bool   `xorm:"-" json:"link,omitempty" doc:"Create a join link instead of inviting someone in particular."`

	// Response only.
	Status       string     `xorm:"-" json:"status" readOnly:"true" enum:"pending,accepted" doc:"pending: the invitation waits to be used. accepted: the invited user joined right away, because they don't confirm invitations."`
	Invitee      *user.User `xorm:"-" json:"invitee,omitempty" readOnly:"true" doc:"The invited user, for invitations by username."`
	Inviter      *user.User `xorm:"-" json:"inviter,omitempty" readOnly:"true" doc:"The user who sent the invitation."`
	ProjectTitle string     `xorm:"-" json:"project_title,omitempty" readOnly:"true" doc:"The title of the project. Set on the invitations you received."`
	InviteURL    string     `xorm:"-" json:"invite_url,omitempty" readOnly:"true" doc:"The link that lets someone join. Always set on join links; set on email invitations only in the response that created them."`

	// Set by Create for an email invitation without a matching account; sent
	// by SendInvitationMail once the transaction committed.
	pendingMail *notifications.Mail

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName is the table name for ProjectInvitation.
func (*ProjectInvitation) TableName() string {
	return "project_invitations"
}

// ProjectInvitationPreview is what someone holding an invitation token may see
// before they join.
type ProjectInvitationPreview struct {
	ProjectTitle string     `json:"project_title" doc:"The title of the project."`
	InviterName  string     `json:"inviter_name" doc:"The display name of the user who sent the invitation."`
	Permission   Permission `json:"permission" doc:"The permission the invitee gets. 0 = Read only, 1 = Read & Write, 2 = Admin."`
	Kind         string     `json:"kind" enum:"email,link" doc:"email: an invitation mailed to one address. link: a join link."`
}

func frontendURL(path string) string {
	return strings.TrimSuffix(config.ServicePublicURL.GetString(), "/") + "/" + path
}

func projectInvitationURL(token string) string {
	return frontendURL("invite/" + token)
}

func projectFrontendURL(projectID int64) string {
	return frontendURL("tasks/p" + strconv.FormatInt(projectID, 10))
}

func newProjectInvitationToken() (token, hash string, err error) {
	token, err = utils.CryptoRandomString(projectInvitationTokenLength)
	if err != nil {
		return "", "", err
	}
	return token, utils.Sha256Hex(token), nil
}

func notExpiredCond() builder.Cond {
	return builder.Or(builder.IsNull{"expires_at"}, builder.Gt{"expires_at": time.Now()})
}

func normalizeInvitationEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 250 {
		return "", InvalidFieldErrorWithMessage([]string{"email"}, "This is not a valid email address.")
	}
	return email, nil
}

func hasDirectProjectAccess(s *xorm.Session, project *Project, userID int64) (bool, error) {
	if project.OwnerID == userID {
		return true, nil
	}
	return s.Where("project_id = ? AND user_id = ?", project.ID, userID).Exist(&ProjectUser{})
}

// addUserToProject shares the project with u, unless they already are its
// owner or have a direct share. Access through a team does not count, the
// same as for ProjectUser.Create.
func addUserToProject(s *xorm.Session, project *Project, u *user.User, permission Permission, doer *user.User) (added bool, err error) {
	has, err := hasDirectProjectAccess(s, project, u.ID)
	if err != nil || has {
		return false, err
	}
	if _, err := s.Insert(&ProjectUser{UserID: u.ID, ProjectID: project.ID, Permission: permission}); err != nil {
		return false, err
	}
	events.DispatchOnCommit(s, &ProjectSharedWithUserEvent{Project: project, User: u, Doer: doer})
	return true, updateProjectLastUpdated(s, project)
}

// Create invites a user, an email address, or creates a join link.
func (pi *ProjectInvitation) Create(s *xorm.Session, a web.Auth) error {
	if err := pi.Permission.isValid(); err != nil {
		return err
	}
	doer, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}
	doer, err = user.GetUserByID(s, doer.ID)
	if err != nil {
		return err
	}
	project, err := GetProjectSimpleByID(s, pi.ProjectID)
	if err != nil {
		return err
	}

	set := 0
	for _, isSet := range []bool{pi.Username != "", strings.TrimSpace(pi.Email) != "", pi.Link} {
		if isSet {
			set++
		}
	}
	if set != 1 {
		return InvalidFieldErrorWithMessage([]string{"username", "email", "link"}, "Set exactly one of username, email and link.")
	}

	pi.ID = 0
	pi.InviterID = doer.ID
	inviter := *doer
	inviter.Email = ""
	pi.Inviter = &inviter
	pi.Uses = 0
	pi.Status = ProjectInvitationStatusPending

	switch {
	case pi.Link:
		return pi.createLink(s)
	case pi.Username != "":
		return pi.createForUser(s, project, doer)
	default:
		return pi.createForEmail(s, project, doer)
	}
}

func (pi *ProjectInvitation) createLink(s *xorm.Session) error {
	token, hash, err := newProjectInvitationToken()
	if err != nil {
		return err
	}
	pi.Kind = ProjectInvitationKindLink
	pi.Email = ""
	pi.TokenHash = hash
	pi.LinkToken = token
	pi.ExpiresAt = nil
	if _, err := s.Insert(pi); err != nil {
		return err
	}
	pi.InviteURL = projectInvitationURL(token)
	return nil
}

func (pi *ProjectInvitation) createForUser(s *xorm.Session, project *Project, doer *user.User) error {
	invitee, err := user.GetUserByUsername(s, pi.Username)
	if err != nil {
		return err
	}
	has, err := hasDirectProjectAccess(s, project, invitee.ID)
	if err != nil {
		return err
	}
	if has {
		return ErrUserAlreadyHasAccess{UserID: invitee.ID, ProjectID: project.ID}
	}
	pending, err := s.Where("project_id = ? AND invitee_id = ?", project.ID, invitee.ID).And(notExpiredCond()).Exist(&ProjectInvitation{})
	if err != nil {
		return err
	}
	if pending {
		return ErrProjectInvitationExists{ProjectID: project.ID}
	}

	pi.Kind = ProjectInvitationKindUser
	pi.Email = ""
	pi.InviteeID = invitee.ID
	pi.ExpiresAt = nil
	invitee.Email = ""
	pi.Invitee = invitee

	if !invitee.ConfirmProjectInvites {
		if _, err := addUserToProject(s, project, invitee, pi.Permission, doer); err != nil {
			return err
		}
		pi.Status = ProjectInvitationStatusAccepted
		return notifications.Notify(invitee, &ProjectSharedNotification{Doer: doer, Project: project}, s)
	}

	if _, err := s.Insert(pi); err != nil {
		return err
	}
	return notifications.Notify(invitee, &ProjectInvitationNotification{
		Doer:         doer,
		Project:      newInvitedProject(project),
		InvitationID: pi.ID,
		Permission:   pi.Permission,
		actionURL:    frontendURL("invitations"),
	}, s)
}

func (pi *ProjectInvitation) createForEmail(s *xorm.Session, project *Project, doer *user.User) error {
	email, err := normalizeInvitationEmail(pi.Email)
	if err != nil {
		return err
	}
	pending, err := s.Where("project_id = ? AND kind = ? AND email = ?", project.ID, ProjectInvitationKindEmail, email).And(notExpiredCond()).Exist(&ProjectInvitation{})
	if err != nil {
		return err
	}
	if pending {
		return ErrProjectInvitationExists{ProjectID: project.ID}
	}

	token, hash, err := newProjectInvitationToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(projectInvitationEmailValidity)
	pi.Kind = ProjectInvitationKindEmail
	pi.Email = email
	pi.TokenHash = hash
	pi.LinkToken = ""
	pi.ExpiresAt = &expires

	// The response looks the same whether or not the address has an account,
	// so inviting by email does not reveal who is registered.
	invitee, err := findActiveUserByEmail(s, email)
	if err != nil {
		return err
	}
	if invitee != nil {
		pi.InviteeID = invitee.ID
	}
	if _, err := s.Insert(pi); err != nil {
		return err
	}
	pi.InviteURL = projectInvitationURL(token)

	n := &ProjectInvitationNotification{
		Doer:         doer,
		Project:      newInvitedProject(project),
		InvitationID: pi.ID,
		Permission:   pi.Permission,
		actionURL:    pi.InviteURL,
	}
	if invitee != nil {
		return notifications.Notify(invitee, n, s)
	}
	lang := doer.Language
	pi.pendingMail = n.ToMail(lang).Subject(n.ToTitle(lang)).To(email)
	return nil
}

// SendInvitationMail mails an email invitation to an address without an
// account. Call it after the transaction that created the invitation
// committed; it does nothing for other invitations or without a mailer.
func (pi *ProjectInvitation) SendInvitationMail() error {
	if pi.pendingMail == nil || !config.MailerEnabled.GetBool() {
		return nil
	}
	m := pi.pendingMail
	pi.pendingMail = nil
	lang := ""
	if pi.Inviter != nil {
		lang = pi.Inviter.Language
	}
	return notifications.SendMail(m, lang)
}

// findActiveUserByEmail returns the one active, non-bot user with this email,
// or nil when there is none or the address is ambiguous.
func findActiveUserByEmail(s *xorm.Session, email string) (*user.User, error) {
	users := []*user.User{}
	err := s.Where("LOWER(email) = ?", email).And("bot_owner_id = 0 OR bot_owner_id IS NULL").And("status = ?", user.StatusActive).Limit(2).Find(&users)
	if err != nil {
		return nil, err
	}
	if len(users) != 1 || !invitationEmailTrusted(users[0]) {
		return nil, nil
	}
	return users[0], nil
}

// invitationEmailTrusted reports whether u's email address is known to be
// theirs, so an invitation sent to it may show up in their account. Local
// accounts only confirm their address when the mailer is enabled.
func invitationEmailTrusted(u *user.User) bool {
	return u.Issuer != user.IssuerLocal || config.MailerEnabled.GetBool()
}

// ReadAll lists the open invitations and join links of a project. Only
// project admins may see them.
func (pi *ProjectInvitation) ReadAll(s *xorm.Session, a web.Auth, _ string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	can, err := pi.canManage(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !can {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.And(builder.Eq{"project_id": pi.ProjectID}, notExpiredCond())
	limit, start := getLimitFromPageIndex(page, perPage)
	query := s.Where(cond).OrderBy("id ASC")
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	all := []*ProjectInvitation{}
	if err := query.Find(&all); err != nil {
		return nil, 0, 0, err
	}
	total, err := s.Where(cond).Count(&ProjectInvitation{})
	if err != nil {
		return nil, 0, 0, err
	}
	if err := addProjectInvitationDetails(s, all, false); err != nil {
		return nil, 0, 0, err
	}
	return all, len(all), total, nil
}

// addProjectInvitationDetails fills the response only fields.
func addProjectInvitationDetails(s *xorm.Session, all []*ProjectInvitation, received bool) error {
	if len(all) == 0 {
		return nil
	}
	userIDs := []int64{}
	projectIDs := []int64{}
	for _, inv := range all {
		userIDs = append(userIDs, inv.InviterID)
		if inv.InviteeID != 0 {
			userIDs = append(userIDs, inv.InviteeID)
		}
		projectIDs = append(projectIDs, inv.ProjectID)
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return err
	}
	projects := map[int64]*Project{}
	if received {
		if err := s.In("id", projectIDs).Find(&projects); err != nil {
			return err
		}
	}
	for _, inv := range all {
		inv.Status = ProjectInvitationStatusPending
		if u, ok := users[inv.InviterID]; ok {
			inviter := *u
			inviter.Email = ""
			inv.Inviter = &inviter
		}
		// Email invitations only show the address, so admins can't learn
		// which addresses belong to an account.
		if inv.Kind == ProjectInvitationKindUser {
			if u, ok := users[inv.InviteeID]; ok {
				invitee := *u
				invitee.Email = ""
				inv.Invitee = &invitee
			}
		}
		if inv.Kind == ProjectInvitationKindLink && inv.LinkToken != "" {
			inv.InviteURL = projectInvitationURL(inv.LinkToken)
		}
		if p, ok := projects[inv.ProjectID]; ok {
			inv.ProjectTitle = p.Title
		}
	}
	return nil
}

// Delete revokes an invitation or join link.
func (pi *ProjectInvitation) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.Where("id = ? AND project_id = ?", pi.ID, pi.ProjectID).Delete(&ProjectInvitation{})
	return err
}

// ListReceivedProjectInvitations returns the invitations waiting for u to
// accept or decline them. Email invitations sent to u's address before they
// had an account are attached to the account here.
func ListReceivedProjectInvitations(s *xorm.Session, u *user.User) ([]*ProjectInvitation, error) {
	full, err := user.GetUserWithEmail(s, &user.User{ID: u.ID})
	if err != nil {
		return nil, err
	}
	if email := strings.ToLower(strings.TrimSpace(full.Email)); email != "" && invitationEmailTrusted(full) {
		_, err = s.Where(builder.Eq{"kind": ProjectInvitationKindEmail, "invitee_id": 0, "email": email}).
			And(notExpiredCond()).
			Cols("invitee_id").
			Update(&ProjectInvitation{InviteeID: u.ID})
		if err != nil {
			return nil, err
		}
	}

	all := []*ProjectInvitation{}
	err = s.Where(builder.Eq{"invitee_id": u.ID}).
		And(builder.In("kind", ProjectInvitationKindUser, ProjectInvitationKindEmail)).
		And(notExpiredCond()).
		OrderBy("id DESC").
		Find(&all)
	if err != nil {
		return nil, err
	}
	if err := addProjectInvitationDetails(s, all, true); err != nil {
		return nil, err
	}
	for _, inv := range all {
		// The invitee knows their own address; nothing else to hide.
		inv.Invitee = nil
	}
	return all, nil
}

func getReceivedProjectInvitation(s *xorm.Session, u *user.User, id int64) (*ProjectInvitation, error) {
	inv := &ProjectInvitation{}
	has, err := s.Where(builder.Eq{"id": id, "invitee_id": u.ID}).
		And(builder.In("kind", ProjectInvitationKindUser, ProjectInvitationKindEmail)).
		And(notExpiredCond()).
		Get(inv)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrProjectInvitationDoesNotExist{}
	}
	return inv, nil
}

func getProjectInvitationByToken(s *xorm.Session, token string) (*ProjectInvitation, error) {
	if token == "" {
		return nil, ErrProjectInvitationDoesNotExist{}
	}
	inv := &ProjectInvitation{}
	has, err := s.Where(builder.Eq{"token_hash": utils.Sha256Hex(token)}).
		And(builder.In("kind", ProjectInvitationKindEmail, ProjectInvitationKindLink)).
		And(notExpiredCond()).
		Get(inv)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrProjectInvitationDoesNotExist{}
	}
	return inv, nil
}

// useProjectInvitation lets u join through inv. Single use invitations are
// removed, join links count the use.
func useProjectInvitation(s *xorm.Session, inv *ProjectInvitation, u *user.User) (*Project, error) {
	project, err := GetProjectSimpleByID(s, inv.ProjectID)
	if err != nil {
		return nil, err
	}
	inviter, err := user.GetUserByID(s, inv.InviterID)
	if err != nil && !user.IsErrUserDoesNotExist(err) && !user.IsErrUserStatusError(err) {
		return nil, err
	}
	if inviter == nil || inviter.ID == 0 {
		inviter = nil
	}

	added, err := addUserToProject(s, project, u, inv.Permission, inviter)
	if err != nil {
		return nil, err
	}

	if inv.Kind == ProjectInvitationKindLink {
		if added {
			if _, err := s.ID(inv.ID).Incr("uses").Update(&ProjectInvitation{}); err != nil {
				return nil, err
			}
		}
	} else if _, err := s.ID(inv.ID).Delete(&ProjectInvitation{}); err != nil {
		return nil, err
	}

	if added && inviter != nil && inviter.ID != u.ID {
		member := *u
		member.Email = ""
		if err := notifications.Notify(inviter, &ProjectInvitationAcceptedNotification{Member: &member, Project: project}, s); err != nil {
			return nil, err
		}
	}
	return project, nil
}

// AcceptProjectInvitation lets u join the project of an invitation they received.
func AcceptProjectInvitation(s *xorm.Session, u *user.User, id int64) (*Project, error) {
	inv, err := getReceivedProjectInvitation(s, u, id)
	if err != nil {
		return nil, err
	}
	return useProjectInvitation(s, inv, u)
}

// DeclineProjectInvitation removes an invitation u received.
func DeclineProjectInvitation(s *xorm.Session, u *user.User, id int64) error {
	inv, err := getReceivedProjectInvitation(s, u, id)
	if err != nil {
		return err
	}
	_, err = s.ID(inv.ID).Delete(&ProjectInvitation{})
	return err
}

// ClaimProjectInvitation lets u join through an email invitation link or a
// join link.
func ClaimProjectInvitation(s *xorm.Session, u *user.User, token string) (*Project, error) {
	inv, err := getProjectInvitationByToken(s, token)
	if err != nil {
		return nil, err
	}
	return useProjectInvitation(s, inv, u)
}

// PreviewProjectInvitation shows what an invitation link leads to. It needs
// no account, so the invitee knows what they sign up for.
func PreviewProjectInvitation(s *xorm.Session, token string) (*ProjectInvitationPreview, error) {
	inv, err := getProjectInvitationByToken(s, token)
	if err != nil {
		return nil, err
	}
	project, err := GetProjectSimpleByID(s, inv.ProjectID)
	if err != nil {
		return nil, err
	}
	preview := &ProjectInvitationPreview{ProjectTitle: project.Title, Permission: inv.Permission, Kind: inv.Kind}
	if inviter, err := user.GetUserByID(s, inv.InviterID); err == nil {
		preview.InviterName = inviter.GetName()
	}
	return preview, nil
}
