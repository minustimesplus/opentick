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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	// ContactSourceManual marks contacts created by hand.
	ContactSourceManual = "manual"
	// ContactSourceCSV marks contacts created from a LinkedIn Connections.csv export.
	ContactSourceCSV = "csv"
	// ContactSourceLinkedin marks contacts last written by the browser extension.
	ContactSourceLinkedin = "linkedin"
	// ContactSourceBrightData marks contacts last written by a Bright Data run.
	ContactSourceBrightData = "brightdata"

	// ContactLockExperiences is the lock protecting a hand-edited experience list.
	ContactLockExperiences = "experiences"
	// ContactLockEducations is the lock protecting a hand-edited education list.
	ContactLockEducations = "educations"
	// ContactLockPhoto is the lock protecting a hand-picked photo.
	ContactLockPhoto = "photo"
)

// contactEditableFields are the scalar fields a user can edit by hand. Each
// one doubles as its lock name.
var contactEditableFields = []string{
	"full_name",
	"headline",
	"company",
	"title",
	"location",
	"email",
	"phone",
	"linkedin_url",
	"birthday",
	"about",
}

// Contact is a person in the user's personal CRM.
type Contact struct {
	ID          int64  `xorm:"bigint autoincr not null unique pk" json:"id" param:"contact" readOnly:"true" doc:"The unique, numeric id of this contact."`
	UserID      int64  `xorm:"bigint not null index" json:"-"`
	FirstName   string `xorm:"varchar(250) null" json:"first_name" readOnly:"true" doc:"Derived from full_name. Set by the server."`
	LastName    string `xorm:"varchar(250) null" json:"last_name" readOnly:"true" doc:"Derived from full_name. Set by the server."`
	FullName    string `xorm:"varchar(250) null" json:"full_name" maxLength:"250" doc:"The contact's full name."`
	Headline    string `xorm:"varchar(500) null" json:"headline" maxLength:"500" doc:"A one-line description, like a LinkedIn headline."`
	Company     string `xorm:"varchar(250) null" json:"company" maxLength:"250" doc:"The current employer."`
	Title       string `xorm:"varchar(250) null" json:"title" maxLength:"250" doc:"The current job title."`
	Location    string `xorm:"varchar(250) null" json:"location" maxLength:"250" doc:"Where the contact lives, as free text (e.g. \"Reims, Grand Est, France\")."`
	Email       string `xorm:"varchar(250) null index" json:"email" maxLength:"250"`
	Phone       string `xorm:"varchar(100) null" json:"phone" maxLength:"100"`
	LinkedinURL string `xorm:"varchar(250) null index" json:"linkedin_url" maxLength:"250" doc:"The LinkedIn profile URL. Normalized by the server to https://www.linkedin.com/in/<handle>."`
	Birthday    string `xorm:"varchar(10) null" json:"birthday" maxLength:"10" doc:"YYYY-MM-DD, or MM-DD when the year is unknown."`
	About       string `xorm:"longtext null" json:"about" doc:"Free-form text about the contact."`
	Source      string `xorm:"varchar(30) not null default 'manual'" json:"source" readOnly:"true" doc:"What last wrote this contact: manual, csv, linkedin or brightdata."`
	RawData     string `xorm:"longtext null" json:"-"`
	// LockedFields are never overwritten by imports or enrichment.
	LockedFields []string `xorm:"json null" json:"locked_fields" doc:"Fields edited by hand. Imports and enrichment never overwrite them. Editing a field locks it; send this list without a name to unlock that field again. Leave it out to keep the current locks."`
	ImageFileID  int64    `xorm:"bigint null" json:"-"`
	// ImportID is the id this contact had in the system it was imported from.
	ImportID string `xorm:"varchar(64) null index" json:"-"`

	Name        string               `xorm:"-" json:"name" readOnly:"true" doc:"The name to display: full_name, else first and last name."`
	HasPhoto    bool                 `xorm:"-" json:"has_photo" readOnly:"true" doc:"Whether the contact has a photo, served at /contacts/{id}/photo."`
	Experiences []*ContactExperience `xorm:"-" json:"experiences" readOnly:"true" doc:"Work history, newest first. Only set when reading a single contact."`
	Educations  []*ContactEducation  `xorm:"-" json:"educations" readOnly:"true" doc:"Education history. Only set when reading a single contact."`
	Notes       []*ContactNote       `xorm:"-" json:"notes" readOnly:"true" doc:"Notes, newest first. Only set when reading a single contact."`
	TagIDs      []int64              `xorm:"-" json:"tag_ids" readOnly:"true" doc:"Ids of the contact's tags."`
	GroupIDs    []int64              `xorm:"-" json:"group_ids" readOnly:"true" doc:"Ids of the groups the contact is in."`
	Tags        []string             `xorm:"-" json:"tags" readOnly:"true" doc:"Names of the contact's tags."`
	Groups      []string             `xorm:"-" json:"groups" readOnly:"true" doc:"Names of the groups the contact is in."`

	// AllowDuplicate lets Create add a contact whose LinkedIn URL or email is already taken.
	AllowDuplicate bool `xorm:"-" json:"-"`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this contact was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this contact was last updated. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for contacts.
func (*Contact) TableName() string {
	return "contacts"
}

// ContactExperience is one job in a contact's work history.
type ContactExperience struct {
	ID          int64  `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true"`
	ContactID   int64  `xorm:"bigint not null index" json:"contact_id" readOnly:"true"`
	Company     string `xorm:"varchar(250) null" json:"company" maxLength:"250"`
	Title       string `xorm:"varchar(250) null" json:"title" maxLength:"250"`
	Location    string `xorm:"varchar(250) null" json:"location" maxLength:"250"`
	StartDate   string `xorm:"varchar(50) null" json:"start_date" maxLength:"50" doc:"When the job started, as free text (e.g. 2021-09, 2021 or Sep 2021)."`
	EndDate     string `xorm:"varchar(50) null" json:"end_date" maxLength:"50" doc:"When the job ended. Empty or Present for a current job."`
	Description string `xorm:"longtext null" json:"description"`
	SortOrder   int    `xorm:"int not null default 0" json:"sort_order" readOnly:"true"`
}

// TableName returns the table name for contact experiences.
func (*ContactExperience) TableName() string {
	return "contact_experiences"
}

// ContactEducation is one school in a contact's education history.
type ContactEducation struct {
	ID           int64  `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true"`
	ContactID    int64  `xorm:"bigint not null index" json:"contact_id" readOnly:"true"`
	School       string `xorm:"varchar(250) null" json:"school" maxLength:"250"`
	Degree       string `xorm:"varchar(250) null" json:"degree" maxLength:"250"`
	FieldOfStudy string `xorm:"varchar(250) null" json:"field_of_study" maxLength:"250"`
	StartYear    string `xorm:"varchar(20) null" json:"start_year" maxLength:"20"`
	EndYear      string `xorm:"varchar(20) null" json:"end_year" maxLength:"20"`
	SortOrder    int    `xorm:"int not null default 0" json:"sort_order" readOnly:"true"`
}

// TableName returns the table name for contact educations.
func (*ContactEducation) TableName() string {
	return "contact_educations"
}

// ContactNote is a note about a contact.
type ContactNote struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true"`
	ContactID int64     `xorm:"bigint not null index" json:"contact_id" readOnly:"true"`
	Body      string    `xorm:"longtext not null" json:"body" minLength:"1"`
	Created   time.Time `xorm:"created not null" json:"created" readOnly:"true"`
	Updated   time.Time `xorm:"updated not null" json:"updated" readOnly:"true"`
}

// TableName returns the table name for contact notes.
func (*ContactNote) TableName() string {
	return "contact_notes"
}

var whitespaceRun = regexp.MustCompile(`\s+`)

func cleanContactText(v string) string {
	return strings.TrimSpace(v)
}

// ContactDisplayName returns the name to show for a contact.
func ContactDisplayName(full, first, last string) string {
	if f := strings.TrimSpace(full); f != "" {
		return f
	}
	joined := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
	if joined == "" {
		return "Unnamed contact"
	}
	return joined
}

func splitContactName(full string) (first, last string) {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return "", ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// NormalizeLinkedinURL returns the lowercased https://www.linkedin.com/<path>
// form of a profile URL, without query or trailing slash. Empty if url is.
func NormalizeLinkedinURL(url string) string {
	c := strings.TrimSpace(url)
	if c == "" {
		return ""
	}
	withoutScheme := strings.TrimPrefix(strings.TrimPrefix(c, "https://"), "http://")
	i := strings.Index(withoutScheme, "/")
	if i < 0 {
		if !strings.Contains(withoutScheme, ".") {
			return strings.ToLower(c)
		}
		return "https://www.linkedin.com"
	}
	path := withoutScheme[i:]
	if j := strings.IndexAny(path, "?#"); j >= 0 {
		path = path[:j]
	}
	path = strings.TrimRight(path, "/")
	return strings.ToLower("https://www.linkedin.com" + path)
}

var digitRun = regexp.MustCompile(`\d+`)

// NormalizeBirthday accepts YYYY-MM-DD, DD.MM.YYYY and MM-DD and returns
// YYYY-MM-DD or MM-DD. It never invents a year. Empty when value is no date.
func NormalizeBirthday(value string) string {
	c := strings.TrimSpace(value)
	if c == "" {
		return ""
	}
	nums := digitRun.FindAllString(c, -1)
	var year, month, day int
	hasYear := false
	switch {
	case len(nums) >= 3 && len(nums[0]) == 4:
		year, _ = strconv.Atoi(nums[0])
		month, _ = strconv.Atoi(nums[1])
		day, _ = strconv.Atoi(nums[2])
		hasYear = true
	case len(nums) == 3 && len(nums[2]) == 4 && strings.Contains(c, "."):
		day, _ = strconv.Atoi(nums[0])
		month, _ = strconv.Atoi(nums[1])
		year, _ = strconv.Atoi(nums[2])
		hasYear = true
	case len(nums) == 2 && strings.ContainsAny(c, "-/"):
		month, _ = strconv.Atoi(nums[0])
		day, _ = strconv.Atoi(nums[1])
	default:
		return ""
	}
	checkYear := 2000
	if hasYear {
		checkYear = year
	}
	d := time.Date(checkYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if month < 1 || month > 12 || d.Day() != day || int(d.Month()) != month {
		return ""
	}
	if hasYear {
		return d.Format("2006-01-02")
	}
	return d.Format("01-02")
}

func (c *Contact) editableField(name string) *string {
	switch name {
	case "full_name":
		return &c.FullName
	case "headline":
		return &c.Headline
	case "company":
		return &c.Company
	case "title":
		return &c.Title
	case "location":
		return &c.Location
	case "email":
		return &c.Email
	case "phone":
		return &c.Phone
	case "linkedin_url":
		return &c.LinkedinURL
	case "birthday":
		return &c.Birthday
	case "about":
		return &c.About
	}
	return nil
}

// normalize trims the editable fields and brings the LinkedIn URL and
// birthday into their stored form.
func (c *Contact) normalize() error {
	for _, name := range contactEditableFields {
		f := c.editableField(name)
		*f = cleanContactText(*f)
	}
	c.FullName = whitespaceRun.ReplaceAllString(c.FullName, " ")
	c.LinkedinURL = NormalizeLinkedinURL(c.LinkedinURL)
	if c.Birthday != "" {
		b := NormalizeBirthday(c.Birthday)
		if b == "" {
			return ErrContactInvalidBirthday{Birthday: c.Birthday}
		}
		c.Birthday = b
	}
	return nil
}

func (c *Contact) isLocked(name string) bool {
	for _, l := range c.LockedFields {
		if l == name {
			return true
		}
	}
	return false
}

func (c *Contact) lock(name string) {
	if !c.isLocked(name) {
		c.LockedFields = append(c.LockedFields, name)
	}
}

func validContactLock(name string) bool {
	if name == ContactLockExperiences || name == ContactLockEducations || name == ContactLockPhoto {
		return true
	}
	for _, f := range contactEditableFields {
		if f == name {
			return true
		}
	}
	return false
}

func (c *Contact) present() {
	c.Name = ContactDisplayName(c.FullName, c.FirstName, c.LastName)
	c.HasPhoto = c.ImageFileID != 0
	if c.LockedFields == nil {
		c.LockedFields = []string{}
	}
}

// findContactDuplicate returns a contact of the user with the same LinkedIn
// URL or email, or nil.
func findContactDuplicate(s *xorm.Session, userID int64, linkedinURL, email string, exceptID int64) (*Contact, error) {
	conds := []builder.Cond{}
	if linkedinURL != "" {
		conds = append(conds, builder.Eq{"linkedin_url": linkedinURL})
	}
	if email != "" {
		conds = append(conds, builder.Eq{"LOWER(email)": strings.ToLower(email)})
	}
	if len(conds) == 0 {
		return nil, nil
	}
	dup := &Contact{}
	has, err := s.
		Where(builder.And(builder.Eq{"user_id": userID}, builder.Neq{"id": exceptID}, builder.Or(conds...))).
		Get(dup)
	if err != nil || !has {
		return nil, err
	}
	return dup, nil
}

// Create adds a contact for the authenticated user. Fields given on create
// are not locked: only later hand edits are.
func (c *Contact) Create(s *xorm.Session, a web.Auth) error {
	if err := c.normalize(); err != nil {
		return err
	}
	if c.FullName == "" && c.LinkedinURL == "" && c.Email == "" {
		return ErrContactNeedsIdentity{}
	}
	if !c.AllowDuplicate {
		dup, err := findContactDuplicate(s, a.GetID(), c.LinkedinURL, c.Email, 0)
		if err != nil {
			return err
		}
		if dup != nil {
			return ErrContactAlreadyExists{
				ID:   dup.ID,
				Name: ContactDisplayName(dup.FullName, dup.FirstName, dup.LastName),
			}
		}
	}
	c.ID = 0
	c.UserID = a.GetID()
	c.FirstName, c.LastName = splitContactName(c.FullName)
	c.Source = ContactSourceManual
	c.LockedFields = []string{}
	c.ImageFileID = 0
	c.RawData = ""
	c.ImportID = ""
	if _, err := s.Insert(c); err != nil {
		return err
	}
	return c.loadRelations(s)
}

// ReadOne returns a contact with its history, notes, tags and groups.
// CanRead already loaded the contact itself.
func (c *Contact) ReadOne(s *xorm.Session, _ web.Auth) error {
	return c.loadRelations(s)
}

// ReadAll returns the authenticated user's contacts by name, without their
// history and notes.
func (c *Contact) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": a.GetID()})
	if search != "" {
		cond = cond.And(builder.Or(
			db.ILIKE("full_name", search),
			db.ILIKE("company", search),
			db.ILIKE("title", search),
			db.ILIKE("email", search),
			db.ILIKE("location", search),
			db.ILIKE("headline", search),
		))
	}

	contacts := []*Contact{}
	query := s.Where(cond).OrderBy("full_name ASC, id ASC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&contacts); err != nil {
		return nil, 0, 0, err
	}
	total, err := s.Where(cond).Count(&Contact{})
	if err != nil {
		return nil, 0, 0, err
	}
	if err := addContactLinks(s, a.GetID(), contacts); err != nil {
		return nil, 0, 0, err
	}
	return contacts, len(contacts), total, nil
}

// Update changes a contact's fields. Every field whose value changes is
// locked against imports and enrichment, like any hand edit. CanUpdate
// loaded the stored contact into c.stored.
func (c *Contact) Update(s *xorm.Session, _ web.Auth) error {
	if err := c.normalize(); err != nil {
		return err
	}
	stored := &Contact{ID: c.ID}
	if err := stored.load(s); err != nil {
		return err
	}

	locks := stored.LockedFields
	if c.LockedFields != nil {
		locks = []string{}
		for _, l := range c.LockedFields {
			if validContactLock(l) {
				locks = append(locks, l)
			}
		}
	}
	incoming := *c
	*c = *stored
	c.LockedFields = locks
	for _, name := range contactEditableFields {
		next := *incoming.editableField(name)
		field := c.editableField(name)
		if *field == next {
			continue
		}
		*field = next
		c.lock(name)
	}
	c.FirstName, c.LastName = splitContactName(c.FullName)

	cols := append([]string{"first_name", "last_name", "locked_fields"}, contactEditableFields...)
	if _, err := s.ID(c.ID).Cols(cols...).Update(c); err != nil {
		return err
	}
	if err := c.load(s); err != nil {
		return err
	}
	return c.loadRelations(s)
}

// Delete removes a contact with its history, notes, photo and memberships.
func (c *Contact) Delete(s *xorm.Session, _ web.Auth) error {
	return deleteContacts(s, []*Contact{c})
}

func deleteContacts(s *xorm.Session, contacts []*Contact) error {
	if len(contacts) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(contacts))
	for _, c := range contacts {
		ids = append(ids, c.ID)
		if c.ImageFileID != 0 {
			if err := deleteContactPhotoFile(s, c.ImageFileID); err != nil {
				return err
			}
		}
	}
	for _, bean := range []any{
		&ContactExperience{},
		&ContactEducation{},
		&ContactNote{},
		&ContactTagLink{},
		&ContactGroupMember{},
		&ContactLinkedinStatus{},
	} {
		if _, err := s.In("contact_id", ids).Delete(bean); err != nil {
			return err
		}
	}
	_, err := s.In("id", ids).Delete(&Contact{})
	return err
}

// deleteContactsOfUser removes everything the personal CRM stores for a user.
func deleteContactsOfUser(s *xorm.Session, userID int64) error {
	contacts := []*Contact{}
	if err := s.Where("user_id = ?", userID).Find(&contacts); err != nil {
		return err
	}
	if err := deleteContacts(s, contacts); err != nil {
		return err
	}
	for _, bean := range []any{
		&ContactTag{},
		&ContactGroup{},
	} {
		if _, err := s.Where("user_id = ?", userID).Delete(bean); err != nil {
			return err
		}
	}
	return nil
}

// load replaces c with the stored contact.
func (c *Contact) load(s *xorm.Session) error {
	// A fresh bean, as Get would also filter by c's other non-zero fields.
	stored := &Contact{}
	exists, err := s.ID(c.ID).Get(stored)
	if err != nil {
		return err
	}
	if !exists {
		return ErrContactDoesNotExist{ID: c.ID}
	}
	stored.AllowDuplicate = c.AllowDuplicate
	*c = *stored
	c.present()
	return nil
}

// touch bumps the contact's updated timestamp, so its ETag changes with its relations.
func (c *Contact) touch(s *xorm.Session) error {
	_, err := s.ID(c.ID).Cols("updated").Update(&Contact{Updated: time.Now()})
	return err
}

func (c *Contact) saveLocks(s *xorm.Session) error {
	_, err := s.ID(c.ID).Cols("locked_fields").Update(c)
	return err
}

func (c *Contact) loadRelations(s *xorm.Session) error {
	c.present()
	c.Experiences = []*ContactExperience{}
	if err := s.Where("contact_id = ?", c.ID).OrderBy("sort_order ASC, id ASC").Find(&c.Experiences); err != nil {
		return err
	}
	c.Educations = []*ContactEducation{}
	if err := s.Where("contact_id = ?", c.ID).OrderBy("sort_order ASC, id ASC").Find(&c.Educations); err != nil {
		return err
	}
	c.Notes = []*ContactNote{}
	if err := s.Where("contact_id = ?", c.ID).OrderBy("created DESC, id DESC").Find(&c.Notes); err != nil {
		return err
	}
	return addContactLinks(s, c.UserID, []*Contact{c})
}

// addContactLinks sets the tag and group ids and names of contacts, which
// all belong to userID.
func addContactLinks(s *xorm.Session, userID int64, contacts []*Contact) error {
	byID := make(map[int64]*Contact, len(contacts))
	ids := make([]int64, 0, len(contacts))
	for _, c := range contacts {
		c.present()
		c.TagIDs = []int64{}
		c.GroupIDs = []int64{}
		c.Tags = []string{}
		c.Groups = []string{}
		byID[c.ID] = c
		ids = append(ids, c.ID)
	}
	if len(contacts) == 0 {
		return nil
	}

	tags := []*ContactTag{}
	if err := s.Where("user_id = ?", userID).Find(&tags); err != nil {
		return err
	}
	tagNames := make(map[int64]string, len(tags))
	for _, t := range tags {
		tagNames[t.ID] = t.Name
	}
	groups := []*ContactGroup{}
	if err := s.Where("user_id = ?", userID).Find(&groups); err != nil {
		return err
	}
	groupNames := make(map[int64]string, len(groups))
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	tagLinks := []*ContactTagLink{}
	if err := contactScope(s, userID, ids).OrderBy("id ASC").Find(&tagLinks); err != nil {
		return err
	}
	for _, l := range tagLinks {
		c, ok := byID[l.ContactID]
		name, known := tagNames[l.TagID]
		if !ok || !known {
			continue
		}
		c.TagIDs = append(c.TagIDs, l.TagID)
		c.Tags = append(c.Tags, name)
	}
	members := []*ContactGroupMember{}
	if err := contactScope(s, userID, ids).OrderBy("id ASC").Find(&members); err != nil {
		return err
	}
	for _, m := range members {
		c, ok := byID[m.ContactID]
		name, known := groupNames[m.GroupID]
		if !ok || !known {
			continue
		}
		c.GroupIDs = append(c.GroupIDs, m.GroupID)
		c.Groups = append(c.Groups, name)
	}
	for _, c := range contacts {
		sort.Strings(c.Tags)
		sort.Strings(c.Groups)
	}
	return nil
}

// contactScope limits a query on a table with a contact_id column to the
// given contacts, or to all contacts of the user when there are too many to
// list in one statement.
func contactScope(s *xorm.Session, userID int64, ids []int64) *xorm.Session {
	if len(ids) <= 500 {
		return s.In("contact_id", ids)
	}
	return s.Where(builder.In("contact_id", builder.Select("id").From("contacts").Where(builder.Eq{"user_id": userID})))
}
