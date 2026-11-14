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
	"encoding/csv"
	"encoding/json"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// ContactImportResult counts what an import did.
type ContactImportResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped" doc:"Rows without a name, LinkedIn URL or email."`
}

type contactIngestItem struct {
	FirstName   string
	LastName    string
	FullName    string
	Company     string
	Title       string
	Email       string
	LinkedinURL string
	Raw         map[string]any
}

// parseLinkedinConnectionsCSV parses a LinkedIn "Connections.csv" export.
// The preamble lines before the header row are skipped.
func parseLinkedinConnectionsCSV(text string) []*contactIngestItem {
	text = strings.TrimPrefix(text, "\uFEFF")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "first name") && strings.Contains(lower, "last name") {
			lines = lines[i:]
			break
		}
	}
	r := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return nil
	}
	col := func(names ...string) int {
		for _, n := range names {
			for i, h := range records[0] {
				if strings.ToLower(strings.TrimSpace(h)) == n {
					return i
				}
			}
		}
		return -1
	}
	first, last := col("first name"), col("last name")
	url, email := col("url", "profile url"), col("email address", "email")
	company, position, connected := col("company"), col("position", "title"), col("connected on")

	items := []*contactIngestItem{}
	for _, rec := range records[1:] {
		get := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		item := &contactIngestItem{
			FirstName:   get(first),
			LastName:    get(last),
			Company:     get(company),
			Title:       get(position),
			Email:       get(email),
			LinkedinURL: get(url),
			Raw: map[string]any{
				"connectedOn": get(connected),
				"source":      "linkedin-csv",
			},
		}
		item.FullName = strings.TrimSpace(item.FirstName + " " + item.LastName)
		if item.FullName != "" || item.LinkedinURL != "" || item.Email != "" {
			items = append(items, item)
		}
	}
	return items
}

// ImportLinkedinConnectionsCSV merges a LinkedIn "Connections.csv" export
// into the user's contacts: incoming values fill empty fields and refresh
// unlocked ones, never overwrite hand-edited (locked) fields, and never
// clear data. Rows are matched by LinkedIn URL, then email.
func ImportLinkedinConnectionsCSV(s *xorm.Session, a web.Auth, text string) (*ContactImportResult, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	items := parseLinkedinConnectionsCSV(text)
	if len(items) == 0 {
		return nil, ErrContactImportEmpty{}
	}
	userID := a.GetID()

	existing := []*Contact{}
	if err := s.Where("user_id = ?", userID).Find(&existing); err != nil {
		return nil, err
	}
	byLinkedin := map[string]*Contact{}
	byEmail := map[string]*Contact{}
	for _, c := range existing {
		if u := NormalizeLinkedinURL(c.LinkedinURL); u != "" {
			byLinkedin[u] = c
		}
		if e := strings.ToLower(strings.TrimSpace(c.Email)); e != "" {
			byEmail[e] = c
		}
	}

	res := &ContactImportResult{}
	for _, item := range items {
		item.LinkedinURL = NormalizeLinkedinURL(item.LinkedinURL)
		emailKey := strings.ToLower(item.Email)
		if item.FullName == "" && item.LinkedinURL == "" && emailKey == "" {
			res.Skipped++
			continue
		}
		c := byLinkedin[item.LinkedinURL]
		if c == nil {
			c = byEmail[emailKey]
		}
		if c == nil {
			c = &Contact{
				UserID:       userID,
				FirstName:    item.FirstName,
				LastName:     item.LastName,
				FullName:     item.FullName,
				Company:      item.Company,
				Title:        item.Title,
				Email:        item.Email,
				LinkedinURL:  item.LinkedinURL,
				Source:       ContactSourceCSV,
				RawData:      mergeRawData("", item.Raw),
				LockedFields: []string{},
			}
			if _, err := s.Insert(c); err != nil {
				return nil, err
			}
			res.Created++
		} else {
			cols := []string{"raw_data", "source", "updated"}
			set := func(lock, col string, field *string, value string) {
				if value == "" || c.isLocked(lock) {
					return
				}
				*field = value
				cols = append(cols, col)
			}
			set("full_name", "first_name", &c.FirstName, item.FirstName)
			set("full_name", "last_name", &c.LastName, item.LastName)
			set("full_name", "full_name", &c.FullName, item.FullName)
			set("company", "company", &c.Company, item.Company)
			set("title", "title", &c.Title, item.Title)
			set("email", "email", &c.Email, item.Email)
			set("linkedin_url", "linkedin_url", &c.LinkedinURL, item.LinkedinURL)
			c.RawData = mergeRawData(c.RawData, item.Raw)
			c.Source = ContactSourceCSV
			c.Updated = time.Now()
			if _, err := s.ID(c.ID).Cols(cols...).Update(c); err != nil {
				return nil, err
			}
			res.Updated++
		}
		if item.LinkedinURL != "" {
			byLinkedin[item.LinkedinURL] = c
		}
		if emailKey != "" {
			byEmail[emailKey] = c
		}
	}
	return res, nil
}

// ContactBundle is an export of another personal CRM: whole contacts with
// their history, notes, tags and groups, keyed by the ids of the source.
type ContactBundle struct {
	Contacts []*ContactBundleContact `json:"contacts"`
	Groups   []*ContactBundleGroup   `json:"groups,omitempty" doc:"Colors and descriptions of the groups the contacts name."`
	Tags     []*ContactBundleTag     `json:"tags,omitempty" doc:"Colors of the tags the contacts name."`
}

// ContactBundleGroup describes a group named by bundle contacts.
type ContactBundleGroup struct {
	Name        string `json:"name"`
	HexColor    string `json:"hex_color,omitempty"`
	Description string `json:"description,omitempty"`
}

// ContactBundleTag describes a tag named by bundle contacts.
type ContactBundleTag struct {
	Name     string `json:"name"`
	HexColor string `json:"hex_color,omitempty"`
}

// ContactBundleContact is one contact of a bundle.
type ContactBundleContact struct {
	ImportID     string               `json:"import_id" minLength:"1" maxLength:"64" doc:"The contact's id in the source. Importing the same id again updates that contact instead of adding a second one."`
	FirstName    string               `json:"first_name,omitempty"`
	LastName     string               `json:"last_name,omitempty"`
	FullName     string               `json:"full_name,omitempty"`
	Headline     string               `json:"headline,omitempty"`
	Company      string               `json:"company,omitempty"`
	Title        string               `json:"title,omitempty"`
	Location     string               `json:"location,omitempty"`
	Email        string               `json:"email,omitempty"`
	Phone        string               `json:"phone,omitempty"`
	LinkedinURL  string               `json:"linkedin_url,omitempty"`
	Birthday     string               `json:"birthday,omitempty"`
	About        string               `json:"about,omitempty"`
	Source       string               `json:"source,omitempty"`
	RawData      string               `json:"raw_data,omitempty" doc:"The raw import payload, as a JSON object in a string."`
	LockedFields []string             `json:"locked_fields,omitempty"`
	Created      time.Time            `json:"created,omitempty"`
	Updated      time.Time            `json:"updated,omitempty"`
	Experiences  []*ContactExperience `json:"experiences,omitempty"`
	Educations   []*ContactEducation  `json:"educations,omitempty"`
	Notes        []*ContactBundleNote `json:"notes,omitempty"`
	Tags         []string             `json:"tags,omitempty"`
	Groups       []string             `json:"groups,omitempty"`
	Statuses     []*ContactBundleStat `json:"linkedin_statuses,omitempty"`
}

// ContactBundleNote is a note of a bundle contact.
type ContactBundleNote struct {
	Body    string    `json:"body"`
	Created time.Time `json:"created,omitempty"`
	Updated time.Time `json:"updated,omitempty"`
}

// ContactBundleStat is a LinkedIn import status of a bundle contact.
type ContactBundleStat struct {
	Method   string `json:"method"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Attempts int    `json:"attempts,omitempty"`
}

// ContactBundleResult reports a bundle import.
type ContactBundleResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	// IDs maps each import id to the contact it became.
	IDs map[string]int64 `json:"ids" doc:"The contact id each import_id was stored under, e.g. to upload photos to."`
}

// contactLockAliases maps the lock names of the Personal CRM desktop app to ours.
var contactLockAliases = map[string]string{
	"fullName":     "full_name",
	"firstName":    "full_name",
	"lastName":     "full_name",
	"linkedinUrl":  "linkedin_url",
	"profileImage": ContactLockPhoto,
}

func importedContactLocks(names []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, n := range names {
		if alias, ok := contactLockAliases[n]; ok {
			n = alias
		}
		if validContactLock(n) && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func importedContactSource(source string) string {
	switch source {
	case ContactSourceCSV, ContactSourceLinkedin, ContactSourceBrightData:
		return source
	case "scraper":
		return ContactSourceLinkedin
	}
	return ContactSourceManual
}

// bundleImporter resolves the tags and groups bundle contacts name,
// creating the ones the user does not have yet.
type bundleImporter struct {
	s         *xorm.Session
	userID    int64
	groups    []*ContactGroup
	tags      []*ContactTag
	groupMeta map[string]*ContactBundleGroup
	tagMeta   map[string]*ContactBundleTag
}

func bundleHexColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	return hex
}

func (b *bundleImporter) groupID(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if g := findContactGroup(b.groups, name); g != nil {
		return g.ID, nil
	}
	g := &ContactGroup{
		UserID: b.userID,
		Name:   name,
	}
	if meta := b.groupMeta[FoldContactText(name)]; meta != nil {
		g.HexColor = bundleHexColor(meta.HexColor)
		g.Description = meta.Description
	}
	if _, err := b.s.Insert(g); err != nil {
		return 0, err
	}
	b.groups = append(b.groups, g)
	return g.ID, nil
}

func (b *bundleImporter) tagID(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if t := findContactTag(b.tags, name); t != nil {
		return t.ID, nil
	}
	t := &ContactTag{
		UserID: b.userID,
		Name:   name,
	}
	if meta := b.tagMeta[FoldContactText(name)]; meta != nil {
		t.HexColor = bundleHexColor(meta.HexColor)
	}
	if _, err := b.s.Insert(t); err != nil {
		return 0, err
	}
	b.tags = append(b.tags, t)
	return t.ID, nil
}

func (bc *ContactBundleContact) toContact(userID int64) *Contact {
	raw := bc.RawData
	if raw != "" && !json.Valid([]byte(raw)) {
		raw = ""
	}
	c := &Contact{
		UserID:       userID,
		FirstName:    strings.TrimSpace(bc.FirstName),
		LastName:     strings.TrimSpace(bc.LastName),
		FullName:     strings.TrimSpace(bc.FullName),
		Headline:     strings.TrimSpace(bc.Headline),
		Company:      strings.TrimSpace(bc.Company),
		Title:        strings.TrimSpace(bc.Title),
		Location:     strings.TrimSpace(bc.Location),
		Email:        strings.TrimSpace(bc.Email),
		Phone:        strings.TrimSpace(bc.Phone),
		LinkedinURL:  NormalizeLinkedinURL(bc.LinkedinURL),
		Birthday:     NormalizeBirthday(bc.Birthday),
		About:        strings.TrimSpace(bc.About),
		Source:       importedContactSource(bc.Source),
		RawData:      raw,
		LockedFields: importedContactLocks(bc.LockedFields),
		ImportID:     bc.ImportID,
		Created:      bc.Created,
		Updated:      bc.Updated,
	}
	if c.FirstName == "" && c.LastName == "" {
		c.FirstName, c.LastName = splitContactName(c.FullName)
	}
	if c.Created.IsZero() {
		c.Created = time.Now()
	}
	if c.Updated.IsZero() {
		c.Updated = c.Created
	}
	return c
}

// writeHistory stores the history, notes and LinkedIn statuses of a bundle contact.
func (b *bundleImporter) writeHistory(contactID int64, bc *ContactBundleContact) error {
	for i, e := range bc.Experiences {
		e.clean()
		e.ID = 0
		e.ContactID = contactID
		e.SortOrder = i
		if _, err := b.s.Insert(e); err != nil {
			return err
		}
	}
	for i, e := range bc.Educations {
		e.clean()
		e.ID = 0
		e.ContactID = contactID
		e.SortOrder = i
		if _, err := b.s.Insert(e); err != nil {
			return err
		}
	}
	for _, n := range bc.Notes {
		if strings.TrimSpace(n.Body) == "" {
			continue
		}
		note := &ContactNote{
			ContactID: contactID,
			Body:      strings.TrimSpace(n.Body),
			Created:   n.Created,
			Updated:   n.Updated,
		}
		if note.Created.IsZero() {
			note.Created = time.Now()
		}
		if note.Updated.IsZero() {
			note.Updated = note.Created
		}
		if _, err := b.s.NoAutoTime().Insert(note); err != nil {
			return err
		}
	}
	methods := map[string]bool{}
	for _, st := range bc.Statuses {
		if (st.Method != LinkedinMethodBrightData && st.Method != LinkedinMethodBrowser) || methods[st.Method] {
			continue
		}
		methods[st.Method] = true
		_, err := b.s.Insert(&ContactLinkedinStatus{
			ContactID: contactID,
			Method:    st.Method,
			Status:    st.Status,
			Detail:    st.Detail,
			Attempts:  st.Attempts,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// writeLinks puts a bundle contact into its tags and groups.
func (b *bundleImporter) writeLinks(contactID int64, bc *ContactBundleContact) error {
	linked := map[int64]bool{}
	for _, name := range bc.Tags {
		if strings.TrimSpace(name) == "" {
			continue
		}
		id, err := b.tagID(name)
		if err != nil {
			return err
		}
		if linked[id] {
			continue
		}
		linked[id] = true
		if _, err := b.s.Insert(&ContactTagLink{ContactID: contactID, TagID: id}); err != nil {
			return err
		}
	}
	linked = map[int64]bool{}
	for _, name := range bc.Groups {
		if strings.TrimSpace(name) == "" {
			continue
		}
		id, err := b.groupID(name)
		if err != nil {
			return err
		}
		if linked[id] {
			continue
		}
		linked[id] = true
		if _, err := b.s.Insert(&ContactGroupMember{ContactID: contactID, GroupID: id}); err != nil {
			return err
		}
	}
	return nil
}

// ImportContactBundle stores a bundle as is: unlike the other imports it
// replaces the contact a bundle entry was imported to before, with its
// history, notes, tags and groups, because the bundle is the source of truth.
func ImportContactBundle(s *xorm.Session, a web.Auth, bundle *ContactBundle) (*ContactBundleResult, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	if len(bundle.Contacts) == 0 {
		return nil, ErrContactImportEmpty{}
	}
	b := &bundleImporter{
		s:         s,
		userID:    a.GetID(),
		groupMeta: map[string]*ContactBundleGroup{},
		tagMeta:   map[string]*ContactBundleTag{},
	}
	var err error
	if b.groups, err = listContactGroups(s, b.userID); err != nil {
		return nil, err
	}
	if b.tags, err = listContactTags(s, b.userID); err != nil {
		return nil, err
	}
	for _, g := range bundle.Groups {
		b.groupMeta[FoldContactText(g.Name)] = g
	}
	for _, t := range bundle.Tags {
		b.tagMeta[FoldContactText(t.Name)] = t
	}

	importIDs := make([]string, 0, len(bundle.Contacts))
	for _, bc := range bundle.Contacts {
		if strings.TrimSpace(bc.ImportID) == "" {
			return nil, ErrContactInvalidProfile{Message: "a bundle contact has no import_id"}
		}
		importIDs = append(importIDs, bc.ImportID)
	}
	previous := []*Contact{}
	if err := s.Where("user_id = ?", b.userID).In("import_id", importIDs).Find(&previous); err != nil {
		return nil, err
	}
	byImportID := make(map[string]*Contact, len(previous))
	for _, c := range previous {
		byImportID[c.ImportID] = c
	}

	res := &ContactBundleResult{IDs: map[string]int64{}}
	for _, bc := range bundle.Contacts {
		c := bc.toContact(b.userID)
		if old := byImportID[bc.ImportID]; old != nil {
			c.ID = old.ID
			c.ImageFileID = old.ImageFileID
			for _, bean := range []any{
				&ContactExperience{},
				&ContactEducation{},
				&ContactNote{},
				&ContactTagLink{},
				&ContactGroupMember{},
				&ContactLinkedinStatus{},
			} {
				if _, err := s.Where("contact_id = ?", c.ID).Delete(bean); err != nil {
					return nil, err
				}
			}
			if _, err := s.ID(c.ID).AllCols().NoAutoTime().Update(c); err != nil {
				return nil, err
			}
			res.Updated++
		} else {
			// Keeps the source's timestamps, which "contacts added per month" is built on.
			if _, err := s.NoAutoTime().Insert(c); err != nil {
				return nil, err
			}
			res.Created++
		}
		res.IDs[bc.ImportID] = c.ID
		if err := b.writeHistory(c.ID, bc); err != nil {
			return nil, err
		}
		if err := b.writeLinks(c.ID, bc); err != nil {
			return nil, err
		}
	}
	return res, nil
}
