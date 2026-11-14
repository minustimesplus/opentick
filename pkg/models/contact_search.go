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
	"sort"
	"strings"
	"time"
	"unicode"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

var contactFoldReplacements = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ą': "a",
	'ç': "c", 'ć': "c", 'č': "c",
	'ď': "d", 'đ': "d",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ę': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i",
	'ł': "l",
	'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ő': "o",
	'ř': "r",
	'ś': "s", 'š': "s", 'ş': "s",
	'ť': "t", 'ţ': "t",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u", 'ű': "u",
	'ý': "y", 'ÿ': "y",
	'ź': "z", 'ż': "z", 'ž': "z",
	'ß': "ss",
	'æ': "ae",
	'œ': "oe",
}

// FoldContactText lowercases text, strips accents and keeps only letters and
// digits, so that "Sciences Po", "SciencesPo" and "sciences-po" compare equal.
func FoldContactText(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for _, ch := range strings.ToLower(text) {
		if rep, ok := contactFoldReplacements[ch]; ok {
			out.WriteString(rep)
			continue
		}
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			out.WriteRune(ch)
		}
	}
	return out.String()
}

// loadFullContacts returns all contacts of a user (or only those in ids, if
// not nil) with their history, notes, tags and groups, ordered by name.
func loadFullContacts(s *xorm.Session, userID int64, ids []int64) ([]*Contact, error) {
	contacts := []*Contact{}
	q := s.Where("user_id = ?", userID)
	if ids != nil {
		q = q.In("id", ids)
	}
	if err := q.Find(&contacts); err != nil {
		return nil, err
	}
	if len(contacts) == 0 {
		return contacts, nil
	}
	byID := make(map[int64]*Contact, len(contacts))
	contactIDs := make([]int64, 0, len(contacts))
	for _, c := range contacts {
		c.Experiences = []*ContactExperience{}
		c.Educations = []*ContactEducation{}
		c.Notes = []*ContactNote{}
		byID[c.ID] = c
		contactIDs = append(contactIDs, c.ID)
	}
	if err := addContactLinks(s, userID, contacts); err != nil {
		return nil, err
	}

	exps := []*ContactExperience{}
	if err := contactScope(s, userID, contactIDs).OrderBy("sort_order ASC, id ASC").Find(&exps); err != nil {
		return nil, err
	}
	for _, e := range exps {
		if c, ok := byID[e.ContactID]; ok {
			c.Experiences = append(c.Experiences, e)
		}
	}
	edus := []*ContactEducation{}
	if err := contactScope(s, userID, contactIDs).OrderBy("sort_order ASC, id ASC").Find(&edus); err != nil {
		return nil, err
	}
	for _, e := range edus {
		if c, ok := byID[e.ContactID]; ok {
			c.Educations = append(c.Educations, e)
		}
	}
	notes := []*ContactNote{}
	if err := contactScope(s, userID, contactIDs).OrderBy("created DESC, id DESC").Find(&notes); err != nil {
		return nil, err
	}
	for _, n := range notes {
		if c, ok := byID[n.ContactID]; ok {
			c.Notes = append(c.Notes, n)
		}
	}
	sort.SliceStable(contacts, func(i, j int) bool {
		return strings.ToLower(contacts[i].Name) < strings.ToLower(contacts[j].Name)
	})
	return contacts, nil
}

// ContactListItem is a contact as the People list shows it: the fields the
// list, its filters and the stats page need, without history and notes.
type ContactListItem struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Headline    string  `json:"headline"`
	Company     string  `json:"company"`
	Title       string  `json:"title"`
	Location    string  `json:"location"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	LinkedinURL string  `json:"linkedin_url"`
	Birthday    string  `json:"birthday"`
	Source      string  `json:"source"`
	HasPhoto    bool    `json:"has_photo"`
	TagIDs      []int64 `json:"tag_ids"`
	GroupIDs    []int64 `json:"group_ids"`
	// Distinct schools, in the order of the education history.
	Schools       []string `json:"schools"`
	NotesCount    int      `json:"notes_count"`
	HasExperience bool     `json:"has_experience"`
	HasEducation  bool     `json:"has_education"`
	// Text the list search matches besides the fields above.
	SearchText string    `json:"search_text" doc:"About text, degrees, fields of study and past employers and titles, for client-side search."`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
}

// ContactOverview is everything the People view shows at once.
type ContactOverview struct {
	Contacts []*ContactListItem `json:"contacts"`
	Tags     []*ContactTag      `json:"tags"`
	Groups   []*ContactGroup    `json:"groups"`
}

// GetContactOverview returns all contacts of the user as list items, with
// their tags and groups.
func GetContactOverview(s *xorm.Session, a web.Auth) (*ContactOverview, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	userID := a.GetID()
	contacts, err := loadFullContacts(s, userID, nil)
	if err != nil {
		return nil, err
	}
	out := &ContactOverview{
		Contacts: make([]*ContactListItem, 0, len(contacts)),
	}
	for _, c := range contacts {
		item := &ContactListItem{
			ID:            c.ID,
			Name:          c.Name,
			Headline:      c.Headline,
			Company:       c.Company,
			Title:         c.Title,
			Location:      c.Location,
			Email:         c.Email,
			Phone:         c.Phone,
			LinkedinURL:   c.LinkedinURL,
			Birthday:      c.Birthday,
			Source:        c.Source,
			HasPhoto:      c.HasPhoto,
			TagIDs:        c.TagIDs,
			GroupIDs:      c.GroupIDs,
			Schools:       []string{},
			NotesCount:    len(c.Notes),
			HasExperience: len(c.Experiences) > 0,
			HasEducation:  len(c.Educations) > 0,
			Created:       c.Created,
			Updated:       c.Updated,
		}
		extra := []string{c.About}
		seen := map[string]bool{}
		for _, e := range c.Educations {
			if e.School != "" && !seen[e.School] {
				seen[e.School] = true
				item.Schools = append(item.Schools, e.School)
			}
			extra = append(extra, e.Degree, e.FieldOfStudy)
		}
		for _, e := range c.Experiences {
			extra = append(extra, e.Company, e.Title)
		}
		parts := extra[:0]
		for _, p := range extra {
			if p != "" {
				parts = append(parts, p)
			}
		}
		item.SearchText = strings.Join(parts, " ")
		out.Contacts = append(out.Contacts, item)
	}
	if out.Tags, err = listContactTags(s, userID); err != nil {
		return nil, err
	}
	if out.Groups, err = listContactGroups(s, userID); err != nil {
		return nil, err
	}
	return out, nil
}

// ContactFillableFields are the names ContactSearch.Has and Missing accept.
var ContactFillableFields = []string{
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
	"photo",
	"notes",
	"experience",
	"education",
}

// ContactSearch are the filters of a contact search. In the term lists any
// term may match; matching ignores case, accents, spaces and punctuation.
type ContactSearch struct {
	Text        string   `json:"text,omitempty" doc:"Free text. Every word must appear somewhere on the contact (name, headline, about, company, title, location, email, schools, degrees, experience, tags, groups, notes)."`
	Name        []string `json:"name,omitempty" doc:"Contact name. Any of the terms may match; matching ignores case, accents, spaces and punctuation, so \"sciences po\" also finds \"SciencesPo\"."`
	Org         []string `json:"org,omitempty" doc:"Any affiliation: a school attended, a current or past employer, or the headline. Use this for \"has been at X\"."`
	School      []string `json:"school,omitempty" doc:"School in the education history."`
	Company     []string `json:"company,omitempty" doc:"Current company or any employer in the experience history."`
	Title       []string `json:"title,omitempty" doc:"Current title, headline or any past job title."`
	Location    []string `json:"location,omitempty" doc:"The contact's current location (e.g. \"Reims\")."`
	Tag         []string `json:"tag,omitempty" doc:"Has this tag (exact name)."`
	Group       []string `json:"group,omitempty" doc:"Is in this group (exact name)."`
	NotGroup    []string `json:"not_group,omitempty" doc:"Exclude contacts in this group (exact name)."`
	NotTag      []string `json:"not_tag,omitempty" doc:"Exclude contacts with this tag (exact name)."`
	Has         []string `json:"has,omitempty" enum:"full_name,headline,company,title,location,email,phone,linkedin_url,birthday,about,photo,notes,experience,education" doc:"Only contacts where all these fields are filled in."`
	Missing     []string `json:"missing,omitempty" enum:"full_name,headline,company,title,location,email,phone,linkedin_url,birthday,about,photo,notes,experience,education" doc:"Only contacts where all these fields are empty."`
	CurrentOnly bool     `json:"current_only,omitempty" doc:"For company, title and org: ignore past jobs (those with an end date)."`
	Match       string   `json:"match,omitempty" enum:"all,any" doc:"all (default): every given filter must match. any: at least one of them (e.g. school Sciences Po OR location Reims). Exclusions (not_group, not_tag, has, missing) always apply."`
	IDs         []int64  `json:"ids,omitempty" doc:"Only consider these contact ids."`
	IDsOnly     bool     `json:"ids_only,omitempty" doc:"Return just the matching ids (all of them, ignoring limit). Handy to pass into a bulk write."`
	Limit       int      `json:"limit,omitempty" minimum:"0" doc:"Maximum contacts returned (default 100). total always gives the full count."`
}

// ContactSearchHit is one search result with the reasons it matched.
type ContactSearchHit struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Company  string   `json:"company"`
	Title    string   `json:"title"`
	Location string   `json:"location"`
	Groups   []string `json:"groups"`
	Tags     []string `json:"tags"`
	Matched  []string `json:"matched,omitempty" doc:"Why the contact matched, e.g. \"school: Sciences Po (Master, 2019–2021)\"."`
}

// ContactSearchResult is the outcome of a contact search.
type ContactSearchResult struct {
	Total    int                 `json:"total" doc:"How many contacts match, whatever the limit."`
	Returned int                 `json:"returned"`
	Contacts []*ContactSearchHit `json:"contacts,omitempty"`
	IDs      []int64             `json:"ids,omitempty" doc:"Set instead of contacts when ids_only was requested."`
	Note     string              `json:"note,omitempty"`
}

func contactYears(a, b string) string {
	switch {
	case a != "" && b != "":
		return ", " + a + "–" + b
	case a != "":
		return ", " + a + "–"
	case b != "":
		return ", –" + b
	}
	return ""
}

func (e *ContactExperience) isCurrent() bool {
	end := strings.TrimSpace(e.EndDate)
	return end == "" || FoldContactText(end) == "present"
}

func (e *ContactEducation) label() string {
	extra := []string{}
	for _, v := range []string{e.Degree, e.FieldOfStudy} {
		if v != "" {
			extra = append(extra, v)
		}
	}
	label := e.School
	if len(extra) > 0 {
		label += " (" + strings.Join(extra, ", ") + ")"
	}
	return label + contactYears(e.StartYear, e.EndYear)
}

func (e *ContactExperience) label() string {
	label := e.Company
	switch {
	case e.Title != "" && e.Company != "":
		label = e.Title + " at " + e.Company
	case e.Title != "":
		label = e.Title
	}
	return label + contactYears(e.StartDate, e.EndDate)
}

func contactTermHits(field string, terms []string) bool {
	f := FoldContactText(field)
	if f == "" {
		return false
	}
	for _, t := range terms {
		if strings.Contains(f, t) {
			return true
		}
	}
	return false
}

func (c *Contact) fieldFilled(name string) bool {
	switch name {
	case "photo":
		return c.ImageFileID != 0
	case "notes":
		return len(c.Notes) > 0
	case "experience":
		return len(c.Experiences) > 0
	case "education":
		return len(c.Educations) > 0
	}
	f := c.editableField(name)
	return f != nil && strings.TrimSpace(*f) != ""
}

func (c *Contact) searchHaystack() string {
	parts := []string{
		c.FullName,
		c.FirstName,
		c.LastName,
		c.Headline,
		c.Company,
		c.Title,
		c.Location,
		c.Email,
		c.Phone,
		c.About,
	}
	for _, e := range c.Educations {
		parts = append(parts, e.School, e.Degree, e.FieldOfStudy)
	}
	for _, e := range c.Experiences {
		parts = append(parts, e.Company, e.Title, e.Location)
	}
	parts = append(parts, c.Tags...)
	parts = append(parts, c.Groups...)
	for _, n := range c.Notes {
		parts = append(parts, n.Body)
	}
	return FoldContactText(strings.Join(parts, " "))
}

func (c *Contact) titleReasons(terms []string, currentOnly bool) []string {
	r := []string{}
	if contactTermHits(c.Title, terms) {
		r = append(r, "title: "+c.Title)
	}
	if contactTermHits(c.Headline, terms) {
		r = append(r, "headline: "+c.Headline)
	}
	for _, e := range c.Experiences {
		if (!currentOnly || e.isCurrent()) && contactTermHits(e.Title, terms) {
			r = append(r, "experience: "+e.label())
		}
	}
	return r
}

// matchReasons returns why a contact matches one filter; empty means no match.
func (c *Contact) matchReasons(key string, terms []string, currentOnly bool) []string {
	r := []string{}
	school := func() {
		for _, e := range c.Educations {
			if contactTermHits(e.School, terms) {
				r = append(r, "school: "+e.label())
			}
		}
	}
	company := func() {
		if contactTermHits(c.Company, terms) {
			r = append(r, "company: "+c.Company+" (current)")
		}
		for _, e := range c.Experiences {
			if (!currentOnly || e.isCurrent()) && contactTermHits(e.Company, terms) {
				r = append(r, "experience: "+e.label())
			}
		}
	}
	switch key {
	case "name":
		if contactTermHits(c.FullName, terms) || contactTermHits(c.FirstName, terms) || contactTermHits(c.LastName, terms) {
			r = append(r, "name: "+c.Name)
		}
	case "school":
		school()
	case "company":
		company()
	case "org":
		if !currentOnly {
			school()
		}
		company()
		if contactTermHits(c.Headline, terms) {
			r = append(r, "headline: "+c.Headline)
		}
	case "title":
		r = c.titleReasons(terms, currentOnly)
	case "location":
		if contactTermHits(c.Location, terms) {
			r = append(r, "location: "+c.Location)
		}
	case "tag", "group":
		names := c.Tags
		if key == "group" {
			names = c.Groups
		}
		for _, n := range names {
			folded := FoldContactText(n)
			for _, t := range terms {
				if folded == t {
					r = append(r, key+": "+n)
					break
				}
			}
		}
	}
	return r
}

func foldContactTerms(terms []string) []string {
	out := []string{}
	for _, t := range terms {
		if f := FoldContactText(t); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func containsFolded(names []string, folded []string) bool {
	for _, n := range names {
		f := FoldContactText(n)
		for _, x := range folded {
			if f == x {
				return true
			}
		}
	}
	return false
}

// validate checks the match mode and field names, and reports whether any filter may match.
func (q *ContactSearch) validate() (matchAny bool, err error) {
	switch q.Match {
	case "", "all":
	case "any":
		matchAny = true
	default:
		return false, ErrContactInvalidSearch{Message: `match must be "all" or "any"`}
	}
	for _, f := range append(append([]string{}, q.Has...), q.Missing...) {
		known := false
		for _, name := range ContactFillableFields {
			known = known || name == f
		}
		if !known {
			return false, ErrContactInvalidSearch{Message: "unknown field " + f + "; use one of " + strings.Join(ContactFillableFields, ", ")}
		}
	}
	return matchAny, nil
}

// excluded reports whether the exclusions of q rule the contact out.
func (q *ContactSearch) excluded(c *Contact, notGroup, notTag []string) bool {
	if containsFolded(c.Groups, notGroup) || containsFolded(c.Tags, notTag) {
		return true
	}
	for _, h := range q.Has {
		if !c.fieldFilled(h) {
			return true
		}
	}
	for _, m := range q.Missing {
		if c.fieldFilled(m) {
			return true
		}
	}
	return false
}

// SearchContacts finds the user's contacts matching the filters.
func SearchContacts(s *xorm.Session, a web.Auth, q *ContactSearch) (*ContactSearchResult, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	matchAny, err := q.validate()
	if err != nil {
		return nil, err
	}

	type filter struct {
		key   string
		terms []string
	}
	filters := []filter{}
	for _, f := range []filter{
		{key: "name", terms: q.Name},
		{key: "org", terms: q.Org},
		{key: "school", terms: q.School},
		{key: "company", terms: q.Company},
		{key: "title", terms: q.Title},
		{key: "location", terms: q.Location},
		{key: "tag", terms: q.Tag},
		{key: "group", terms: q.Group},
	} {
		if terms := foldContactTerms(f.terms); len(terms) > 0 {
			filters = append(filters, filter{key: f.key, terms: terms})
		}
	}
	words := foldContactTerms(strings.Fields(q.Text))
	notGroup := foldContactTerms(q.NotGroup)
	notTag := foldContactTerms(q.NotTag)

	// A non-nil empty list means "none of them", not "all".
	ids := q.IDs
	if ids != nil && len(ids) == 0 {
		ids = []int64{}
	}
	contacts, err := loadFullContacts(s, a.GetID(), ids)
	if err != nil {
		return nil, err
	}

	hits := []*ContactSearchHit{}
	for _, c := range contacts {
		if q.excluded(c, notGroup, notTag) {
			continue
		}
		matched := []string{}
		ok, total := 0, 0
		for _, f := range filters {
			total++
			if r := c.matchReasons(f.key, f.terms, q.CurrentOnly); len(r) > 0 {
				ok++
				matched = append(matched, r...)
			}
		}
		if len(words) > 0 {
			total++
			h := c.searchHaystack()
			all := true
			for _, w := range words {
				all = all && strings.Contains(h, w)
			}
			if all {
				ok++
				matched = append(matched, "text: "+strings.TrimSpace(q.Text))
			}
		}
		if total != 0 && ((matchAny && ok == 0) || (!matchAny && ok != total)) {
			continue
		}
		hits = append(hits, &ContactSearchHit{
			ID:       c.ID,
			Name:     c.Name,
			Company:  c.Company,
			Title:    c.Title,
			Location: c.Location,
			Groups:   c.Groups,
			Tags:     c.Tags,
			Matched:  matched,
		})
	}

	res := &ContactSearchResult{Total: len(hits)}
	if q.IDsOnly {
		res.IDs = make([]int64, 0, len(hits))
		for _, h := range hits {
			res.IDs = append(res.IDs, h.ID)
		}
		res.Returned = len(res.IDs)
		return res, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if len(hits) > limit {
		res.Note = "more contacts match than shown; raise limit or use ids_only"
		hits = hits[:limit]
	}
	res.Contacts = hits
	res.Returned = len(hits)
	return res, nil
}

// ContactDistinctValue is one value of a field with how many contacts have it.
type ContactDistinctValue struct {
	Value    string `json:"value"`
	Contacts int    `json:"contacts"`
}

// ContactDistinctValues lists the distinct values of a field.
type ContactDistinctValues struct {
	Field       string                  `json:"field"`
	TotalValues int                     `json:"total_values"`
	Values      []*ContactDistinctValue `json:"values"`
}

// GetContactDistinctValues lists the distinct values of a field across the
// user's contacts, most common first.
func GetContactDistinctValues(s *xorm.Session, a web.Auth, field, contains string, limit int) (*ContactDistinctValues, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	contacts, err := loadFullContacts(s, a.GetID(), nil)
	if err != nil {
		return nil, err
	}
	people := map[string]map[int64]bool{}
	add := func(id int64, values ...string) {
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if people[v] == nil {
				people[v] = map[int64]bool{}
			}
			people[v][id] = true
		}
	}
	for _, c := range contacts {
		switch field {
		case "school":
			for _, e := range c.Educations {
				add(c.ID, e.School)
			}
		case "degree":
			for _, e := range c.Educations {
				add(c.ID, e.Degree)
			}
		case "field_of_study":
			for _, e := range c.Educations {
				add(c.ID, e.FieldOfStudy)
			}
		case "company":
			add(c.ID, c.Company)
			for _, e := range c.Experiences {
				add(c.ID, e.Company)
			}
		case "title":
			add(c.ID, c.Title)
			for _, e := range c.Experiences {
				add(c.ID, e.Title)
			}
		case "location":
			add(c.ID, c.Location)
		case "tag":
			add(c.ID, c.Tags...)
		case "group":
			add(c.ID, c.Groups...)
		default:
			return nil, ErrContactInvalidSearch{Message: "unknown field " + field}
		}
	}
	needle := FoldContactText(contains)
	values := []*ContactDistinctValue{}
	for v, ids := range people {
		if needle == "" || strings.Contains(FoldContactText(v), needle) {
			values = append(values, &ContactDistinctValue{
				Value:    v,
				Contacts: len(ids),
			})
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Contacts != values[j].Contacts {
			return values[i].Contacts > values[j].Contacts
		}
		return values[i].Value < values[j].Value
	})
	out := &ContactDistinctValues{
		Field:       field,
		TotalValues: len(values),
	}
	if limit <= 0 {
		limit = 100
	}
	out.Values = values[:min(limit, len(values))]
	return out, nil
}
