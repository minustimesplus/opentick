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
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ContactGroup is a named set of contacts, like "Sciences Po" or "Family".
type ContactGroup struct {
	ID          int64  `xorm:"bigint autoincr not null unique pk" json:"id" param:"contactgroup" readOnly:"true" doc:"The unique, numeric id of this group."`
	UserID      int64  `xorm:"bigint not null index" json:"-"`
	Name        string `xorm:"varchar(250) not null" json:"name" minLength:"1" maxLength:"250"`
	HexColor    string `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The group's color, in hex format."`
	Description string `xorm:"longtext null" json:"description"`

	Members int64 `xorm:"-" json:"members" readOnly:"true" doc:"How many contacts are in the group. Set by the server."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true"`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for contact groups.
func (*ContactGroup) TableName() string {
	return "contact_groups"
}

// ContactGroupMember puts a contact into a group.
type ContactGroupMember struct {
	ID        int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	ContactID int64 `xorm:"bigint not null unique(contact_group)" json:"contact_id"`
	GroupID   int64 `xorm:"bigint not null unique(contact_group) index" json:"group_id"`
}

// TableName returns the table name for contact group members.
func (*ContactGroupMember) TableName() string {
	return "contact_group_members"
}

// ContactTag is a label on contacts.
type ContactTag struct {
	ID       int64  `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true"`
	UserID   int64  `xorm:"bigint not null index" json:"-"`
	Name     string `xorm:"varchar(250) not null" json:"name" minLength:"1" maxLength:"250"`
	HexColor string `xorm:"varchar(6) null" json:"hex_color" maxLength:"7"`

	Contacts int64 `xorm:"-" json:"contacts" readOnly:"true" doc:"How many contacts have the tag. Set by the server."`
}

// TableName returns the table name for contact tags.
func (*ContactTag) TableName() string {
	return "contact_tags"
}

// ContactTagLink puts a tag on a contact.
type ContactTagLink struct {
	ID        int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	ContactID int64 `xorm:"bigint not null unique(contact_tag)" json:"contact_id"`
	TagID     int64 `xorm:"bigint not null unique(contact_tag) index" json:"tag_id"`
}

// TableName returns the table name for contact tag links.
func (*ContactTagLink) TableName() string {
	return "contact_tag_links"
}

func (g *ContactGroup) normalize() error {
	g.Name = strings.TrimSpace(g.Name)
	g.Description = strings.TrimSpace(g.Description)
	g.HexColor = utils.NormalizeHex(g.HexColor)
	if g.Name == "" {
		return ErrContactGroupNameEmpty{}
	}
	return nil
}

func listContactGroups(s *xorm.Session, userID int64) ([]*ContactGroup, error) {
	groups := []*ContactGroup{}
	if err := s.Where("user_id = ?", userID).OrderBy("name ASC, id ASC").Find(&groups); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return groups, nil
	}
	members := []*ContactGroupMember{}
	err := s.
		Where(builder.In("group_id", builder.Select("id").From("contact_groups").Where(builder.Eq{"user_id": userID}))).
		Cols("group_id").
		Find(&members)
	if err != nil {
		return nil, err
	}
	counts := map[int64]int64{}
	for _, m := range members {
		counts[m.GroupID]++
	}
	for _, g := range groups {
		g.Members = counts[g.ID]
	}
	return groups, nil
}

// ListContactTags returns the user's tags by name, with how many contacts have each.
func ListContactTags(s *xorm.Session, a web.Auth) ([]*ContactTag, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	return listContactTags(s, a.GetID())
}

func listContactTags(s *xorm.Session, userID int64) ([]*ContactTag, error) {
	tags := []*ContactTag{}
	if err := s.Where("user_id = ?", userID).OrderBy("name ASC, id ASC").Find(&tags); err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return tags, nil
	}
	links := []*ContactTagLink{}
	err := s.
		Where(builder.In("tag_id", builder.Select("id").From("contact_tags").Where(builder.Eq{"user_id": userID}))).
		Cols("tag_id").
		Find(&links)
	if err != nil {
		return nil, err
	}
	counts := map[int64]int64{}
	for _, l := range links {
		counts[l.TagID]++
	}
	for _, t := range tags {
		t.Contacts = counts[t.ID]
	}
	return tags, nil
}

// findContactGroup finds a group by id, exact name or folded name.
func findContactGroup(groups []*ContactGroup, key string) *ContactGroup {
	key = strings.TrimSpace(key)
	if id, err := strconv.ParseInt(key, 10, 64); err == nil {
		for _, g := range groups {
			if g.ID == id {
				return g
			}
		}
	}
	folded := FoldContactText(key)
	for _, g := range groups {
		if strings.EqualFold(g.Name, key) || FoldContactText(g.Name) == folded {
			return g
		}
	}
	return nil
}

func findContactTag(tags []*ContactTag, key string) *ContactTag {
	key = strings.TrimSpace(key)
	if id, err := strconv.ParseInt(key, 10, 64); err == nil {
		for _, t := range tags {
			if t.ID == id {
				return t
			}
		}
	}
	folded := FoldContactText(key)
	for _, t := range tags {
		if strings.EqualFold(t.Name, key) || FoldContactText(t.Name) == folded {
			return t
		}
	}
	return nil
}

// Create adds a group. A group with the same name (ignoring case, accents
// and punctuation) is reused instead, and updated with the given
// description and color.
func (g *ContactGroup) Create(s *xorm.Session, a web.Auth) error {
	if err := g.normalize(); err != nil {
		return err
	}
	groups, err := listContactGroups(s, a.GetID())
	if err != nil {
		return err
	}
	if existing := findContactGroup(groups, g.Name); existing != nil && FoldContactText(existing.Name) == FoldContactText(g.Name) {
		cols := []string{}
		if g.Description != "" {
			existing.Description = g.Description
			cols = append(cols, "description")
		}
		if g.HexColor != "" {
			existing.HexColor = g.HexColor
			cols = append(cols, "hex_color")
		}
		if len(cols) > 0 {
			if _, err := s.ID(existing.ID).Cols(cols...).Update(existing); err != nil {
				return err
			}
		}
		*g = *existing
		return nil
	}
	g.ID = 0
	g.UserID = a.GetID()
	g.Members = 0
	_, err = s.Insert(g)
	return err
}

// ReadOne returns a group with its member count. CanRead already loaded it.
func (g *ContactGroup) ReadOne(s *xorm.Session, _ web.Auth) (err error) {
	g.Members, err = s.Where("group_id = ?", g.ID).Count(&ContactGroupMember{})
	return err
}

// ReadAll returns the authenticated user's groups by name, with member counts.
func (g *ContactGroup) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}
	all, err := listContactGroups(s, a.GetID())
	if err != nil {
		return nil, 0, 0, err
	}
	groups := all
	if search != "" {
		groups = []*ContactGroup{}
		needle := FoldContactText(search)
		for _, group := range all {
			if strings.Contains(FoldContactText(group.Name), needle) {
				groups = append(groups, group)
			}
		}
	}
	total := int64(len(groups))
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		start = min(start, len(groups))
		groups = groups[start:min(start+limit, len(groups))]
	}
	return groups, len(groups), total, nil
}

// Update renames a group or changes its description or color.
func (g *ContactGroup) Update(s *xorm.Session, _ web.Auth) error {
	if err := g.normalize(); err != nil {
		return err
	}
	if _, err := s.ID(g.ID).Cols("name", "hex_color", "description").Update(g); err != nil {
		return err
	}
	stored := &ContactGroup{}
	if _, err := s.ID(g.ID).Get(stored); err != nil {
		return err
	}
	*g = *stored
	return g.ReadOne(s, nil)
}

// Delete removes a group. Its contacts stay.
func (g *ContactGroup) Delete(s *xorm.Session, _ web.Auth) error {
	if _, err := s.Where("group_id = ?", g.ID).Delete(&ContactGroupMember{}); err != nil {
		return err
	}
	_, err := s.ID(g.ID).Delete(&ContactGroup{})
	return err
}

// DeleteContactTag removes one of the user's tags from every contact.
func DeleteContactTag(s *xorm.Session, a web.Auth, id int64) error {
	tag := &ContactTag{}
	has, err := s.ID(id).Get(tag)
	if err != nil {
		return err
	}
	if _, isShareAuth := a.(*LinkSharing); !has || isShareAuth || tag.UserID != a.GetID() {
		return ErrContactTagDoesNotExist{Tag: strconv.FormatInt(id, 10)}
	}
	if _, err := s.Where("tag_id = ?", id).Delete(&ContactTagLink{}); err != nil {
		return err
	}
	_, err = s.ID(id).Delete(&ContactTag{})
	return err
}

// ContactMembershipKind is what a bulk membership change targets.
type ContactMembershipKind int

const (
	// ContactMembershipGroup adds contacts to or removes them from a group.
	ContactMembershipGroup ContactMembershipKind = iota
	// ContactMembershipTag adds a tag to or removes it from contacts.
	ContactMembershipTag
)

// ContactMembershipChange describes a bulk change of group members or tags.
type ContactMembershipChange struct {
	Kind ContactMembershipKind
	// Target is the name or id of the group or tag.
	Target     string
	ContactIDs []int64
	// On adds the contacts; off removes them.
	On bool
	// Create makes a missing group. Missing tags are always created.
	Create bool
	DryRun bool
}

// ContactRef names a contact.
type ContactRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ContactMembershipTarget is the group or tag a bulk change applied to.
type ContactMembershipTarget struct {
	ID      int64  `json:"id" doc:"0 while a dry run would still have to create it."`
	Name    string `json:"name"`
	Created bool   `json:"created" doc:"Whether this call created it."`
}

// ContactMembershipResult reports what a bulk membership change did.
type ContactMembershipResult struct {
	DryRun     bool                    `json:"dry_run"`
	Target     ContactMembershipTarget `json:"target"`
	Changed    int                     `json:"changed" doc:"How many contacts were added or removed (or would be, on a dry run)."`
	Unchanged  int                     `json:"unchanged" doc:"How many contacts already were in the wanted state."`
	Contacts   []ContactRef            `json:"contacts" doc:"The contacts that changed."`
	UnknownIDs []int64                 `json:"unknown_ids" doc:"Ids that are not contacts of the user."`
	Note       string                  `json:"note,omitempty"`
}

// resolveMembershipTarget finds the group or tag a change names. create is
// true when it does not exist yet and the change may make it.
func resolveMembershipTarget(s *xorm.Session, userID int64, change *ContactMembershipChange, name string) (target ContactMembershipTarget, create bool, err error) {
	if change.Kind == ContactMembershipGroup {
		groups, err := listContactGroups(s, userID)
		if err != nil {
			return target, false, err
		}
		if g := findContactGroup(groups, name); g != nil {
			return ContactMembershipTarget{ID: g.ID, Name: g.Name}, false, nil
		}
		if change.On && change.Create {
			return ContactMembershipTarget{Name: name}, true, nil
		}
		names := make([]string, 0, len(groups))
		for _, g := range groups {
			names = append(names, g.Name)
		}
		return target, false, ErrContactGroupNotFound{
			Group:  name,
			Groups: names,
		}
	}
	tags, err := listContactTags(s, userID)
	if err != nil {
		return target, false, err
	}
	if t := findContactTag(tags, name); t != nil {
		return ContactMembershipTarget{ID: t.ID, Name: t.Name}, false, nil
	}
	if change.On {
		return ContactMembershipTarget{Name: name}, true, nil
	}
	return target, false, ErrContactTagDoesNotExist{Tag: name}
}

// membershipOf returns the ids of the contacts in a group or with a tag.
func membershipOf(s *xorm.Session, kind ContactMembershipKind, id int64) (map[int64]bool, error) {
	member := map[int64]bool{}
	if id == 0 {
		return member, nil
	}
	if kind == ContactMembershipGroup {
		rows := []*ContactGroupMember{}
		if err := s.Where("group_id = ?", id).Find(&rows); err != nil {
			return nil, err
		}
		for _, r := range rows {
			member[r.ContactID] = true
		}
		return member, nil
	}
	rows := []*ContactTagLink{}
	if err := s.Where("tag_id = ?", id).Find(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		member[r.ContactID] = true
	}
	return member, nil
}

// writeMembership applies a change to the contacts in res, creating the
// group or tag first if asked to.
func writeMembership(s *xorm.Session, userID int64, change *ContactMembershipChange, res *ContactMembershipResult, create bool) (err error) {
	isGroup := change.Kind == ContactMembershipGroup
	if create {
		if isGroup {
			g := &ContactGroup{
				UserID: userID,
				Name:   res.Target.Name,
			}
			_, err = s.Insert(g)
			res.Target.ID = g.ID
		} else {
			t := &ContactTag{
				UserID: userID,
				Name:   res.Target.Name,
			}
			_, err = s.Insert(t)
			res.Target.ID = t.ID
		}
		if err != nil {
			return err
		}
		res.Target.Created = true
	}

	ids := make([]int64, 0, len(res.Contacts))
	for _, ref := range res.Contacts {
		ids = append(ids, ref.ID)
	}
	switch {
	case change.On && isGroup:
		for _, id := range ids {
			if _, err := s.Insert(&ContactGroupMember{ContactID: id, GroupID: res.Target.ID}); err != nil {
				return err
			}
		}
	case change.On:
		for _, id := range ids {
			if _, err := s.Insert(&ContactTagLink{ContactID: id, TagID: res.Target.ID}); err != nil {
				return err
			}
		}
	case isGroup:
		_, err = s.Where("group_id = ?", res.Target.ID).In("contact_id", ids).Delete(&ContactGroupMember{})
	default:
		_, err = s.Where("tag_id = ?", res.Target.ID).In("contact_id", ids).Delete(&ContactTagLink{})
	}
	if err != nil {
		return err
	}
	_, err = s.In("id", ids).Cols("updated").Update(&Contact{Updated: time.Now()})
	return err
}

// ChangeContactMembership adds contacts to a group or tag, or removes them.
func ChangeContactMembership(s *xorm.Session, a web.Auth, change *ContactMembershipChange) (*ContactMembershipResult, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	userID := a.GetID()
	if len(change.ContactIDs) == 0 {
		return nil, ErrContactIDsEmpty{}
	}
	name := strings.TrimSpace(change.Target)
	if name == "" {
		return nil, ErrContactGroupNameEmpty{}
	}

	res := &ContactMembershipResult{
		DryRun:     change.DryRun,
		Contacts:   []ContactRef{},
		UnknownIDs: []int64{},
	}
	var create bool
	var err error
	if res.Target, create, err = resolveMembershipTarget(s, userID, change, name); err != nil {
		return nil, err
	}

	contacts := []*Contact{}
	if err := s.Where("user_id = ?", userID).In("id", change.ContactIDs).Find(&contacts); err != nil {
		return nil, err
	}
	known := make(map[int64]*Contact, len(contacts))
	for _, c := range contacts {
		known[c.ID] = c
	}
	member, err := membershipOf(s, change.Kind, res.Target.ID)
	if err != nil {
		return nil, err
	}

	seen := map[int64]bool{}
	for _, id := range change.ContactIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		c, ok := known[id]
		switch {
		case !ok:
			res.UnknownIDs = append(res.UnknownIDs, id)
		case member[id] == change.On:
			res.Unchanged++
		default:
			res.Contacts = append(res.Contacts, ContactRef{
				ID:   id,
				Name: ContactDisplayName(c.FullName, c.FirstName, c.LastName),
			})
		}
	}
	res.Changed = len(res.Contacts)

	if change.DryRun || res.Changed == 0 {
		if create && change.DryRun {
			res.Note = "it does not exist yet and would be created"
		}
		return res, nil
	}
	return res, writeMembership(s, userID, change, res, create)
}

// DeleteContactsByID deletes the given contacts of the user and returns how
// many there were.
func DeleteContactsByID(s *xorm.Session, a web.Auth, ids []int64) (int, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return 0, ErrGenericForbidden{}
	}
	if len(ids) == 0 {
		return 0, ErrContactIDsEmpty{}
	}
	contacts := []*Contact{}
	if err := s.Where("user_id = ?", a.GetID()).In("id", ids).Find(&contacts); err != nil {
		return 0, err
	}
	return len(contacts), deleteContacts(s, contacts)
}
