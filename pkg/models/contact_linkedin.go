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
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// LinkedIn profiles come from two sources:
//   - Bright Data "LinkedIn people profiles" rows (LinkedinProfileFromBrightData).
//     They come from LinkedIn's logged-out public view, so experience is
//     usually missing and education often lacks degree and field.
//   - The browser extension (LinkedinProfileFromBrowser). It reads the
//     logged-in view and is treated as complete. It sends structured entries,
//     raw text lines per list item, or both.
//
// Merge rule: scalar fields fill and refresh, never overwrite locked fields
// and never clear. Experience and education lists are replaced only when the
// incoming list is complete or strictly richer than what is stored, so a thin
// public scrape never replaces a full history.

const (
	// LinkedinMethodBrightData is the status method of Bright Data runs.
	LinkedinMethodBrightData = "brightdata"
	// LinkedinMethodBrowser is the status method of the browser extension.
	LinkedinMethodBrowser = "browser"

	linkedinStatusOK          = "ok"
	linkedinStatusPartial     = "partial"
	linkedinStatusEmpty       = "empty"
	linkedinStatusError       = "error"
	linkedinStatusUnavailable = "unavailable"

	linkedinMaxAttempts = 3
)

// ContactLinkedinStatus is the last outcome of an import method for a
// contact, so retries and the extension's queue know what was already tried.
type ContactLinkedinStatus struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	ContactID int64     `xorm:"bigint not null unique(contact_method)" json:"contact_id"`
	Method    string    `xorm:"varchar(20) not null unique(contact_method)" json:"method"`
	Status    string    `xorm:"varchar(20) not null" json:"status"`
	Detail    string    `xorm:"text null" json:"detail"`
	Attempts  int       `xorm:"int not null default 0" json:"attempts"`
	Updated   time.Time `xorm:"updated not null" json:"updated"`
}

// TableName returns the table name for LinkedIn import statuses.
func (*ContactLinkedinStatus) TableName() string {
	return "contact_linkedin_status"
}

// LinkedinProfile is a profile from any source, ready to merge into a contact.
type LinkedinProfile struct {
	LinkedinURL string
	FirstName   string
	LastName    string
	FullName    string
	Headline    string
	Company     string
	Title       string
	Location    string
	About       string
	Birthday    string
	AvatarURL   string
	Experiences []*ContactExperience
	Educations  []*ContactEducation
	// Complete is true when the lists come from the full logged-in view.
	Complete bool
	// Raw is merged into the contact's stored raw payload, key by key.
	Raw map[string]any
}

/* ---------------- small helpers ---------------- */

func collapseWhitespace(t string) string {
	return strings.Join(strings.Fields(t), " ")
}

// jsonText returns a trimmed non-empty string, ignoring placeholders like "-".
func jsonText(v any) string {
	var t string
	switch x := v.(type) {
	case string:
		t = x
	case float64:
		t = strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		t = x.String()
	default:
		return ""
	}
	t = collapseWhitespace(t)
	switch t {
	case "-", "–", "—", "null", "N/A":
		return ""
	}
	return t
}

func jsonObject(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func jsonArray(v any) []any {
	a, _ := v.([]any)
	return a
}

func jsonFirst(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if t := jsonText(m[k]); t != "" {
			return t
		}
	}
	return ""
}

// jsonBlock is jsonFirst for multi-line text: line breaks are kept.
func jsonBlock(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if t, ok := m[k].(string); ok {
			if b := cleanTextBlock(t); b != "" {
				return b
			}
		}
	}
	return ""
}

func cleanTextBlock(t string) string {
	lines := strings.Split(t, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func stripHTML(t string) string {
	var out strings.Builder
	inTag := false
	for _, ch := range t {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>' && inTag:
			inTag = false
			out.WriteRune(' ')
		case !inTag:
			out.WriteRune(ch)
		}
	}
	return strings.NewReplacer("&amp;", "&", "&nbsp;", " ", "&#39;", "'", "&quot;", "\"").Replace(out.String())
}

// linkedinYears returns the four-digit years between 1900 and 2100 in t.
func linkedinYears(t string) []string {
	out := []string{}
	for _, p := range digitRun.FindAllString(t, -1) {
		if len(p) != 4 {
			continue
		}
		if y, _ := strconv.Atoi(p); y >= 1900 && y <= 2100 {
			out = append(out, p)
		}
	}
	return out
}

var linkedinPresentWords = []string{
	"present", "heute", "aujourd", "actuel", "presente", "actualidad", "oggi", "hoy", "huidig", "heden", "now",
}

var linkedinEmploymentTypes = []string{
	"full-time", "part-time", "self-employed", "freelance", "contract", "internship", "apprenticeship",
	"seasonal", "temporary", "volunteer", "trainee", "vollzeit", "teilzeit", "praktikum", "selbstständig",
	"werkstudent", "temps plein", "temps particle", "stage", "alternance", "cdi", "cdd",
}

func isEmploymentType(t string) bool {
	l := strings.ToLower(strings.TrimSpace(t))
	for _, e := range linkedinEmploymentTypes {
		if l == e {
			return true
		}
	}
	return false
}

func containsPresentWord(lower string) bool {
	for _, w := range linkedinPresentWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func beforeDot(t string) string {
	return strings.TrimSpace(strings.SplitN(t, "·", 2)[0])
}

// ParseLinkedinDateRange parses a LinkedIn date range line such as
// "Jan 2020 - Present · 4 yrs", "2018 – 2022" or "Sep 2019 - Jun 2021".
// It returns start and end as shown, without the duration suffix; ok is
// false if the line is not a date range.
func ParseLinkedinDateRange(line string) (start, end string, ok bool) {
	main := beforeDot(line)
	if main == "" || utf8.RuneCountInString(main) > 60 {
		return "", "", false
	}
	if len(linkedinYears(main)) == 0 && !containsPresentWord(strings.ToLower(main)) {
		return "", "", false
	}
	// Too many words to be a date, e.g. a description that mentions a year.
	if len(strings.Fields(main)) > 8 {
		return "", "", false
	}
	var parts []string
	for _, sep := range []string{" - ", " – ", " — ", "–", "—", " bis ", " à ", " to "} {
		if a, b, found := strings.Cut(main, sep); found {
			parts = []string{a, b}
			break
		}
	}
	if parts == nil {
		ys := linkedinYears(main)
		// "2018-2022" with a bare hyphen between two years.
		if len(ys) == 2 && strings.Contains(main, "-") {
			return ys[0], ys[1], true
		}
		if len(ys) == 1 && len(strings.Fields(main)) <= 3 {
			return collapseWhitespace(main), "", true
		}
		return "", "", false
	}
	if s := collapseWhitespace(parts[0]); len(linkedinYears(s)) > 0 {
		start = s
	}
	endRaw := strings.TrimSpace(parts[1])
	if containsPresentWord(strings.ToLower(endRaw)) {
		end = "Present"
	} else if e := collapseWhitespace(endRaw); len(linkedinYears(e)) > 0 {
		end = e
	}
	if start == "" && end == "" {
		return "", "", false
	}
	return start, end, true
}

func isDateRange(line string) bool {
	_, _, ok := ParseLinkedinDateRange(line)
	return ok
}

// isDurationLine matches durations like "4 yrs 10 mos" or "Full-time · 5 yrs".
func isDurationLine(t string) bool {
	l := strings.ToLower(t)
	if len(linkedinYears(l)) > 0 || utf8.RuneCountInString(l) >= 40 || !strings.ContainsAny(l, "0123456789") {
		return false
	}
	words := strings.FieldsFunc(l, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r < utf8.RuneSelf
	})
	for _, w := range words {
		for _, u := range []string{"yr", "mo", "jahr", "monat", "an", "mois", "año", "mes"} {
			if strings.HasPrefix(w, u) {
				return true
			}
		}
	}
	return false
}

func firstLinkedinYear(t string) string {
	if ys := linkedinYears(t); len(ys) > 0 {
		return ys[0]
	}
	return ""
}

/* ---------------- text lines (the extension's DOM fallback) ---------------- */

func dropLinkedinNoise(lines []string) []string {
	out := []string{}
	for _, l := range lines {
		t := collapseWhitespace(l)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "show all") || strings.HasPrefix(lower, "see more") || lower == "…see more" || strings.HasSuffix(lower, " logo") {
			continue
		}
		// LinkedIn renders visible and screen-reader copies; drop exact repeats.
		if len(out) > 0 && out[len(out)-1] == t {
			continue
		}
		out = append(out, t)
	}
	return out
}

// experienceFromLines reads one position from its text lines. company is set
// for grouped items, where the company is the group header.
func experienceFromLines(rawLines []string, company string) *ContactExperience {
	lines := dropLinkedinNoise(rawLines)
	if len(lines) == 0 {
		return nil
	}
	dateIdx := -1
	for i, l := range lines {
		if isDateRange(l) {
			dateIdx = i
			break
		}
	}
	exp := &ContactExperience{
		Title:   lines[0],
		Company: company,
	}
	headEnd := min(len(lines), 2)
	if dateIdx >= 0 {
		exp.StartDate, exp.EndDate, _ = ParseLinkedinDateRange(lines[dateIdx])
		headEnd = dateIdx
	}
	// Lines between title and date: "Company · Full-time" or just an employment type.
	for i := 1; i < headEnd; i++ {
		name := beforeDot(lines[i])
		if exp.Company == "" && name != "" && !isEmploymentType(name) && !isDurationLine(lines[i]) {
			exp.Company = name
		}
	}
	if dateIdx >= 0 {
		rest := lines[dateIdx+1:]
		if len(rest) > 0 {
			loc := rest[0]
			locMain := beforeDot(loc)
			looksLikeLocation := utf8.RuneCountInString(loc) <= 80 &&
				!strings.HasSuffix(loc, ".") &&
				!strings.HasPrefix(strings.ToLower(loc), "skills") &&
				len(strings.Fields(locMain)) <= 8
			if looksLikeLocation {
				exp.Location = collapseWhitespace(locMain)
				rest = rest[1:]
			}
		}
		desc := []string{}
		for _, l := range rest {
			if !strings.HasPrefix(strings.ToLower(l), "skills:") {
				desc = append(desc, l)
			}
		}
		exp.Description = cleanTextBlock(strings.Join(desc, "\n"))
	}
	return exp
}

// ExperiencesFromLinkedinItem reads a list item of the experience page. sub
// holds nested positions when several roles at one company are grouped under
// a company header.
func ExperiencesFromLinkedinItem(lines []string, sub [][]string) []*ContactExperience {
	head := dropLinkedinNoise(lines)
	subs := [][]string{}
	for _, s := range sub {
		if len(dropLinkedinNoise(s)) > 0 {
			subs = append(subs, s)
		}
	}
	out := []*ContactExperience{}
	if len(subs) == 0 {
		if e := experienceFromLines(head, ""); e != nil {
			out = append(out, e)
		}
		return out
	}
	company := ""
	if len(head) > 0 {
		company = beforeDot(head[0])
	}
	location := ""
	for _, l := range head[min(1, len(head)):] {
		if !isDurationLine(l) && !isEmploymentType(beforeDot(l)) && !isDateRange(l) {
			location = l
			break
		}
	}
	for _, s := range subs {
		e := experienceFromLines(s, company)
		if e == nil {
			continue
		}
		if e.Location == "" {
			e.Location = location
		}
		out = append(out, e)
	}
	return out
}

// EducationFromLinkedinLines reads one school from its text lines.
func EducationFromLinkedinLines(rawLines []string) *ContactEducation {
	lines := dropLinkedinNoise(rawLines)
	if len(lines) == 0 {
		return nil
	}
	edu := &ContactEducation{School: lines[0]}
	dateIdx := -1
	for i, l := range lines {
		if isDateRange(l) {
			dateIdx = i
			break
		}
	}
	if dateIdx >= 0 {
		ys := linkedinYears(beforeDot(lines[dateIdx]))
		if len(ys) > 0 {
			edu.StartYear = ys[0]
		}
		if len(ys) > 1 {
			edu.EndYear = ys[1]
		}
	}
	if len(lines) > 1 && dateIdx != 1 {
		d := lines[1]
		lower := strings.ToLower(d)
		if !strings.HasPrefix(lower, "grade") && !strings.HasPrefix(lower, "activities") && !strings.HasPrefix(lower, "note") {
			if deg, field, found := strings.Cut(d, ", "); found {
				edu.Degree = collapseWhitespace(deg)
				edu.FieldOfStudy = collapseWhitespace(field)
			} else {
				edu.Degree = collapseWhitespace(d)
			}
		}
	}
	return edu
}

/* ---------------- Bright Data rows ---------------- */

// BrightDataRow is one classified Bright Data result row: a profile, or a
// failed input.
type BrightDataRow struct {
	Profile *LinkedinProfile
	// URL is the input URL the row answers, normalized.
	URL string
	// Error is set for failed inputs. Retry is false for permanent failures (a dead page).
	Error string
	Retry bool
}

// BrightDataInputURL returns the URL Bright Data was asked for, which is
// what results are matched on.
func BrightDataInputURL(raw map[string]any) string {
	u := ""
	switch input := raw["input"].(type) {
	case map[string]any:
		u = jsonFirst(input, "url", "profile_url")
	case string:
		u = jsonText(input)
	}
	if u == "" {
		u = jsonFirst(raw, "input_url", "url", "linkedin_url")
	}
	return NormalizeLinkedinURL(u)
}

func brightDataExperiences(item map[string]any) []*ContactExperience {
	company := jsonFirst(item, "company", "company_name")
	location := jsonFirst(item, "location")
	one := func(p map[string]any, company, inheritedLocation string) *ContactExperience {
		start := jsonFirst(p, "start_date", "startDate")
		end := jsonFirst(p, "end_date", "endDate")
		if start == "" {
			if a, b, ok := ParseLinkedinDateRange(jsonFirst(p, "duration", "meta", "date_range")); ok {
				start = a
				if end == "" {
					end = b
				}
			}
		}
		description := jsonBlock(p, "description")
		if description == "" {
			description = cleanTextBlock(stripHTML(jsonFirst(p, "description_html")))
		}
		title := jsonFirst(p, "title", "position", "subtitle")
		if isEmploymentType(title) {
			title = ""
		}
		loc := jsonFirst(p, "location")
		if loc == "" {
			loc = inheritedLocation
		}
		return &ContactExperience{
			Company:     company,
			Title:       title,
			Location:    loc,
			StartDate:   start,
			EndDate:     end,
			Description: description,
		}
	}
	positions := jsonArray(item["positions"])
	if len(positions) > 0 {
		// Grouped roles: the group's title is the company when company is missing.
		if company == "" {
			company = jsonFirst(item, "title")
		}
		out := make([]*ContactExperience, 0, len(positions))
		for _, p := range positions {
			out = append(out, one(jsonObject(p), company, location))
		}
		return out
	}
	e := one(item, company, "")
	if e.Company == "" {
		e.Company = beforeDot(jsonFirst(item, "subtitle"))
	}
	return []*ContactExperience{e}
}

func brightDataEducation(item map[string]any) *ContactEducation {
	degree := jsonFirst(item, "degree", "degree_name")
	field := jsonFirst(item, "field", "field_of_study", "fieldOfStudy")
	// Some rows put "Degree, Field" into degree.
	if field == "" {
		if d, f, found := strings.Cut(degree, ", "); found {
			degree = collapseWhitespace(d)
			field = collapseWhitespace(f)
		}
	}
	start := jsonFirst(item, "start_year", "startYear")
	if start == "" {
		start = firstLinkedinYear(jsonFirst(item, "start_date"))
	}
	end := jsonFirst(item, "end_year", "endYear")
	if end == "" {
		end = firstLinkedinYear(jsonFirst(item, "end_date"))
	}
	if start == "" && end == "" {
		ys := linkedinYears(jsonFirst(item, "meta", "duration", "date_range"))
		if len(ys) > 0 {
			start = ys[0]
		}
		if len(ys) > 1 {
			end = ys[1]
		}
	}
	return &ContactEducation{
		School:       jsonFirst(item, "title", "school", "institute", "school_name", "name"),
		Degree:       degree,
		FieldOfStudy: field,
		StartYear:    start,
		EndYear:      end,
	}
}

// LinkedinProfileFromBrightData classifies and maps one Bright Data result row.
func LinkedinProfileFromBrightData(raw map[string]any) *BrightDataRow {
	row := &BrightDataRow{URL: BrightDataInputURL(raw)}
	errText := jsonFirst(raw, "error", "error_message")
	errorCode := strings.ToLower(jsonFirst(raw, "error_code", "warning_code"))
	dead := strings.Contains(errorCode, "dead_page") || strings.Contains(errorCode, "not_found") || strings.Contains(errorCode, "private")
	name := jsonFirst(raw, "name", "full_name")
	if errText != "" || (dead && name == "") {
		row.Error = errText
		if row.Error == "" {
			row.Error = jsonFirst(raw, "warning")
		}
		if row.Error == "" {
			row.Error = errorCode
		}
		row.Retry = !dead
		return row
	}
	if name == "" && jsonFirst(raw, "first_name", "last_name") == "" {
		row.Error = "empty result"
		row.Retry = true
		return row
	}

	p := &LinkedinProfile{
		LinkedinURL: row.URL,
		FullName:    name,
		FirstName:   jsonFirst(raw, "first_name"),
		LastName:    jsonFirst(raw, "last_name"),
		Headline:    jsonFirst(raw, "position", "headline"),
		Location:    jsonFirst(raw, "city", "location"),
		About:       jsonBlock(raw, "about", "summary"),
		Birthday:    NormalizeBirthday(jsonFirst(raw, "birthday", "birth_date", "date_of_birth")),
		Experiences: []*ContactExperience{},
		Educations:  []*ContactEducation{},
		Raw:         raw,
	}
	if p.LinkedinURL == "" {
		p.LinkedinURL = NormalizeLinkedinURL(jsonFirst(raw, "url"))
	}
	for _, item := range jsonArray(raw["experience"]) {
		for _, e := range brightDataExperiences(jsonObject(item)) {
			if e.Title != "" || e.Company != "" {
				p.Experiences = append(p.Experiences, e)
			}
		}
	}
	for _, item := range jsonArray(raw["education"]) {
		if e := brightDataEducation(jsonObject(item)); e.School != "" || e.Degree != "" {
			p.Educations = append(p.Educations, e)
		}
	}
	if len(p.Educations) == 0 {
		if school := jsonFirst(raw, "educations_details"); school != "" {
			p.Educations = append(p.Educations, &ContactEducation{School: school})
		}
	}
	if isDefault, _ := raw["default_avatar"].(bool); !isDefault {
		p.AvatarURL = jsonFirst(raw, "avatar", "profile_pic_url", "profile_image", "image_url")
	}
	current := jsonObject(raw["current_company"])
	p.Company = jsonFirst(current, "name")
	if p.Company == "" {
		p.Company = jsonFirst(raw, "current_company_name", "company")
	}
	p.Title = jsonFirst(current, "title")
	if p.Title == "" && len(p.Experiences) > 0 {
		if first := p.Experiences[0]; first.EndDate == "" || first.EndDate == "Present" {
			p.Title = first.Title
		}
	}
	row.Profile = p
	return row
}

/* ---------------- browser extension payload ---------------- */

func jsonLines(v any) []string {
	out := []string{}
	for _, l := range jsonArray(v) {
		if s, ok := l.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// itemLines returns the text lines of a list item, which is either an object
// with a lines array or the array itself.
func itemLines(item any) []string {
	if m := jsonObject(item); m != nil {
		return jsonLines(m["lines"])
	}
	return jsonLines(item)
}

// LinkedinProfileFromBrowser maps the extension's JSON. Structured
// experiences and educations win; the raw experience_items and
// education_items (text lines per list item) are parsed only when the
// structured lists are absent.
func LinkedinProfileFromBrowser(v map[string]any) (*LinkedinProfile, error) {
	url := NormalizeLinkedinURL(jsonFirst(v, "linkedin_url", "url"))
	if url == "" {
		return nil, ErrContactInvalidProfile{Message: "missing linkedin_url"}
	}
	if !strings.Contains(url, "/in/") {
		return nil, ErrContactInvalidProfile{Message: "not a profile URL: " + url}
	}
	profile := jsonObject(v["profile"])

	p := &LinkedinProfile{
		LinkedinURL: url,
		FirstName:   jsonFirst(profile, "first_name"),
		LastName:    jsonFirst(profile, "last_name"),
		FullName:    jsonFirst(profile, "full_name", "name"),
		Headline:    jsonFirst(profile, "headline"),
		Location:    jsonFirst(profile, "location"),
		About:       jsonBlock(profile, "about"),
		AvatarURL:   jsonFirst(profile, "avatar_url"),
		Experiences: []*ContactExperience{},
		Educations:  []*ContactEducation{},
		// The extension says whether it saw the full lists (true unless a section failed to load).
		Complete: true,
		Raw: map[string]any{
			"linkedinCapture": map[string]any{
				"method":           jsonFirst(v, "method"),
				"capturedAt":       time.Now().UTC().Format(time.RFC3339),
				"extensionVersion": jsonFirst(v, "extension_version"),
			},
		},
	}
	if complete, ok := v["complete"].(bool); ok {
		p.Complete = complete
	}
	if p.FullName == "" {
		p.FullName = collapseWhitespace(p.FirstName + " " + p.LastName)
	}

	for _, item := range jsonArray(v["experiences"]) {
		e := jsonObject(item)
		exp := &ContactExperience{
			Company:     jsonFirst(e, "company"),
			Title:       jsonFirst(e, "title"),
			Location:    jsonFirst(e, "location"),
			StartDate:   jsonFirst(e, "start_date", "start"),
			EndDate:     jsonFirst(e, "end_date", "end"),
			Description: jsonBlock(e, "description"),
		}
		if exp.Title != "" || exp.Company != "" {
			p.Experiences = append(p.Experiences, exp)
		}
	}
	if len(p.Experiences) == 0 {
		for _, item := range jsonArray(v["experience_items"]) {
			sub := [][]string{}
			if m := jsonObject(item); m != nil {
				for _, s := range jsonArray(m["sub"]) {
					sub = append(sub, jsonLines(s))
				}
			}
			p.Experiences = append(p.Experiences, ExperiencesFromLinkedinItem(itemLines(item), sub)...)
		}
	}

	for _, item := range jsonArray(v["educations"]) {
		e := jsonObject(item)
		edu := &ContactEducation{
			School:       jsonFirst(e, "school"),
			Degree:       jsonFirst(e, "degree"),
			FieldOfStudy: jsonFirst(e, "field_of_study", "field"),
			StartYear:    jsonFirst(e, "start_year"),
			EndYear:      jsonFirst(e, "end_year"),
		}
		if edu.StartYear == "" {
			edu.StartYear = firstLinkedinYear(jsonFirst(e, "start"))
		}
		if edu.EndYear == "" {
			edu.EndYear = firstLinkedinYear(jsonFirst(e, "end"))
		}
		if edu.School != "" {
			p.Educations = append(p.Educations, edu)
		}
	}
	if len(p.Educations) == 0 {
		for _, item := range jsonArray(v["education_items"]) {
			if e := EducationFromLinkedinLines(itemLines(item)); e != nil {
				p.Educations = append(p.Educations, e)
			}
		}
	}

	for _, e := range p.Experiences {
		if e.EndDate == "Present" || (e.EndDate == "" && e.StartDate != "") {
			p.Company = e.Company
			p.Title = e.Title
			break
		}
	}
	return p, nil
}

/* ---------------- merging into the database ---------------- */

func filled(values ...string) int {
	n := 0
	for _, v := range values {
		if v != "" {
			n++
		}
	}
	return n
}

func experienceScore(list []*ContactExperience) int {
	score := 0
	for _, e := range list {
		score += 1 + filled(e.Title, e.Company, e.StartDate, e.Description)
	}
	return score
}

func educationScore(list []*ContactEducation) int {
	score := 0
	for _, e := range list {
		score += 1 + filled(e.School, e.Degree, e.FieldOfStudy, e.StartYear)
	}
	return score
}

// shouldReplaceList decides whether an incoming list replaces the stored one.
func shouldReplaceList(incoming, existing int, complete, locked bool) bool {
	return !locked && incoming > 0 && (complete || incoming > existing)
}

// LinkedinApplyOutcome reports what applying a profile did.
type LinkedinApplyOutcome struct {
	ContactID          int64  `json:"contact_id"`
	Created            bool   `json:"created"`
	Status             string `json:"status" doc:"ok when the profile has experience and education, partial with only one, empty with neither."`
	Experiences        int    `json:"experiences" doc:"How many jobs the contact has now."`
	Educations         int    `json:"educations" doc:"How many schools the contact has now."`
	ExperiencesWritten bool   `json:"experiences_written"`
	EducationsWritten  bool   `json:"educations_written"`
	// WantsPhoto is set when the contact has no photo yet and the profile offers one.
	WantsPhoto string `json:"-"`
}

// FindContactByLinkedinURL returns the user's contact with that profile URL, or nil.
func FindContactByLinkedinURL(s *xorm.Session, userID int64, url string) (*Contact, error) {
	norm := NormalizeLinkedinURL(url)
	if norm == "" {
		return nil, nil
	}
	c := &Contact{}
	has, err := s.Where(builder.Eq{"user_id": userID, "linkedin_url": norm}).Get(c)
	if err != nil {
		return nil, err
	}
	if has {
		c.present()
		return c, nil
	}
	return nil, nil
}

func mergeRawData(stored string, incoming map[string]any) string {
	merged := map[string]any{}
	if stored != "" {
		_ = json.Unmarshal([]byte(stored), &merged)
	}
	for k, v := range incoming {
		merged[k] = v
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return stored
	}
	return string(out)
}

func linkedinStatusFor(p *LinkedinProfile) string {
	switch {
	case len(p.Experiences) > 0 && len(p.Educations) > 0:
		return linkedinStatusOK
	case len(p.Experiences) == 0 && len(p.Educations) == 0:
		return linkedinStatusEmpty
	}
	return linkedinStatusPartial
}

// ApplyLinkedinProfile writes a profile into the user's matching contact:
// scalar fields with the lock rule, lists when richer, raw payload merged.
// It creates the contact when create is set and none matches, and returns
// nil when there is no contact to write to.
func ApplyLinkedinProfile(s *xorm.Session, userID int64, p *LinkedinProfile, source string, create bool) (*LinkedinApplyOutcome, error) {
	if p.LinkedinURL == "" {
		return nil, nil
	}
	out := &LinkedinApplyOutcome{Status: linkedinStatusFor(p)}
	c, err := FindContactByLinkedinURL(s, userID, p.LinkedinURL)
	if err != nil {
		return nil, err
	}
	if c == nil {
		if !create {
			return nil, nil
		}
		c = &Contact{
			UserID:       userID,
			LinkedinURL:  NormalizeLinkedinURL(p.LinkedinURL),
			Source:       source,
			LockedFields: []string{},
		}
		if _, err := s.Insert(c); err != nil {
			return nil, err
		}
		out.Created = true
	}
	out.ContactID = c.ID

	// Scalars: fill and refresh unlocked fields, never clear.
	cols := []string{"raw_data", "source", "updated"}
	set := func(lock, col string, field *string, value string) {
		if value == "" || c.isLocked(lock) {
			return
		}
		*field = value
		cols = append(cols, col)
	}
	nameLocked := c.isLocked("full_name")
	if !nameLocked {
		set("full_name", "first_name", &c.FirstName, p.FirstName)
		set("full_name", "last_name", &c.LastName, p.LastName)
		set("full_name", "full_name", &c.FullName, p.FullName)
		if p.FullName != "" && p.FirstName == "" && p.LastName == "" {
			c.FirstName, c.LastName = splitContactName(p.FullName)
			cols = append(cols, "first_name", "last_name")
		}
	}
	set("headline", "headline", &c.Headline, p.Headline)
	set("company", "company", &c.Company, p.Company)
	set("title", "title", &c.Title, p.Title)
	set("location", "location", &c.Location, p.Location)
	set("about", "about", &c.About, p.About)
	set("birthday", "birthday", &c.Birthday, p.Birthday)
	c.RawData = mergeRawData(c.RawData, p.Raw)
	c.Source = source
	c.Updated = time.Now()
	if _, err := s.ID(c.ID).Cols(cols...).Update(c); err != nil {
		return nil, err
	}

	oldExps := []*ContactExperience{}
	if err := s.Where("contact_id = ?", c.ID).Find(&oldExps); err != nil {
		return nil, err
	}
	out.Experiences = len(oldExps)
	if shouldReplaceList(experienceScore(p.Experiences), experienceScore(oldExps), p.Complete, c.isLocked(ContactLockExperiences)) {
		if _, err := s.Where("contact_id = ?", c.ID).Delete(&ContactExperience{}); err != nil {
			return nil, err
		}
		for i, e := range p.Experiences {
			e.ID = 0
			e.ContactID = c.ID
			e.SortOrder = i
			if _, err := s.Insert(e); err != nil {
				return nil, err
			}
		}
		out.ExperiencesWritten = true
		out.Experiences = len(p.Experiences)
	}
	oldEdus := []*ContactEducation{}
	if err := s.Where("contact_id = ?", c.ID).Find(&oldEdus); err != nil {
		return nil, err
	}
	out.Educations = len(oldEdus)
	if shouldReplaceList(educationScore(p.Educations), educationScore(oldEdus), p.Complete, c.isLocked(ContactLockEducations)) {
		if _, err := s.Where("contact_id = ?", c.ID).Delete(&ContactEducation{}); err != nil {
			return nil, err
		}
		for i, e := range p.Educations {
			e.ID = 0
			e.ContactID = c.ID
			e.SortOrder = i
			if _, err := s.Insert(e); err != nil {
				return nil, err
			}
		}
		out.EducationsWritten = true
		out.Educations = len(p.Educations)
	}

	if c.ImageFileID == 0 && !c.isLocked(ContactLockPhoto) {
		out.WantsPhoto = p.AvatarURL
	}
	return out, nil
}

// RecordLinkedinStatus records the last outcome of a method for a contact.
func RecordLinkedinStatus(s *xorm.Session, contactID int64, method, status, detail string) error {
	existing := &ContactLinkedinStatus{}
	has, err := s.Where(builder.Eq{"contact_id": contactID, "method": method}).Get(existing)
	if err != nil {
		return err
	}
	if !has {
		_, err = s.Insert(&ContactLinkedinStatus{
			ContactID: contactID,
			Method:    method,
			Status:    status,
			Detail:    detail,
			Attempts:  1,
		})
		return err
	}
	existing.Attempts++
	if status == linkedinStatusOK {
		existing.Attempts = 0
	}
	existing.Status = status
	existing.Detail = detail
	_, err = s.ID(existing.ID).Cols("status", "detail", "attempts", "updated").Update(existing)
	return err
}

// LinkedinCoverage counts how much LinkedIn history the user's contacts have.
type LinkedinCoverage struct {
	Total          int `json:"total"`
	WithURL        int `json:"with_url"`
	WithExperience int `json:"with_experience"`
	WithEducation  int `json:"with_education"`
	MissingAny     int `json:"missing_any" doc:"Contacts with a LinkedIn URL that lack experience or education."`
	BrowserQueue   int `json:"browser_queue" doc:"Of those, how many the browser extension would still visit."`
}

// LinkedinQueueItem is a contact the browser extension should visit.
type LinkedinQueueItem struct {
	ContactID   int64  `json:"contact_id"`
	Name        string `json:"name"`
	LinkedinURL string `json:"linkedin_url"`
}

type linkedinCandidates struct {
	coverage LinkedinCoverage
	// missing lack a list; queue is the part of missing the extension still visits.
	missing []*Contact
	queue   []*Contact
}

// loadLinkedinCandidates sorts the user's contacts into the ones missing
// history and the ones the extension should still visit: missing a list, not
// already captured from the logged-in view, and not given up on.
func loadLinkedinCandidates(s *xorm.Session, userID int64) (*linkedinCandidates, error) {
	contacts := []*Contact{}
	err := s.
		Where("user_id = ?", userID).
		Cols("id", "first_name", "last_name", "full_name", "linkedin_url", "updated").
		OrderBy("updated DESC").
		Find(&contacts)
	if err != nil {
		return nil, err
	}
	scope := builder.In("contact_id", builder.Select("id").From("contacts").Where(builder.Eq{"user_id": userID}))
	hasExp := map[int64]bool{}
	exps := []*ContactExperience{}
	if err := s.Where(scope).Cols("contact_id").Find(&exps); err != nil {
		return nil, err
	}
	for _, e := range exps {
		hasExp[e.ContactID] = true
	}
	hasEdu := map[int64]bool{}
	edus := []*ContactEducation{}
	if err := s.Where(scope).Cols("contact_id").Find(&edus); err != nil {
		return nil, err
	}
	for _, e := range edus {
		hasEdu[e.ContactID] = true
	}
	statuses := []*ContactLinkedinStatus{}
	if err := s.Where(scope).And("method = ?", LinkedinMethodBrowser).Find(&statuses); err != nil {
		return nil, err
	}
	browser := map[int64]*ContactLinkedinStatus{}
	for _, st := range statuses {
		browser[st.ContactID] = st
	}

	out := &linkedinCandidates{
		coverage: LinkedinCoverage{
			Total:          len(contacts),
			WithExperience: len(hasExp),
			WithEducation:  len(hasEdu),
		},
	}
	for _, c := range contacts {
		if c.LinkedinURL == "" {
			continue
		}
		out.coverage.WithURL++
		if hasExp[c.ID] && hasEdu[c.ID] {
			continue
		}
		out.missing = append(out.missing, c)
		if st := browser[c.ID]; st != nil {
			done := st.Status != linkedinStatusError
			if done || st.Attempts >= linkedinMaxAttempts {
				continue
			}
		}
		out.queue = append(out.queue, c)
	}
	out.coverage.MissingAny = len(out.missing)
	out.coverage.BrowserQueue = len(out.queue)
	return out, nil
}

// GetLinkedinCoverage counts how much LinkedIn history the user's contacts have.
func GetLinkedinCoverage(s *xorm.Session, a web.Auth) (*LinkedinCoverage, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	cand, err := loadLinkedinCandidates(s, a.GetID())
	if err != nil {
		return nil, err
	}
	return &cand.coverage, nil
}

// GetLinkedinBrowserQueue returns the contacts the browser extension should visit next.
func GetLinkedinBrowserQueue(s *xorm.Session, a web.Auth, limit int) ([]*LinkedinQueueItem, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	cand, err := loadLinkedinCandidates(s, a.GetID())
	if err != nil {
		return nil, err
	}
	items := []*LinkedinQueueItem{}
	for _, c := range cand.queue {
		if len(items) >= limit {
			break
		}
		items = append(items, &LinkedinQueueItem{
			ContactID:   c.ID,
			Name:        ContactDisplayName(c.FullName, c.FirstName, c.LastName),
			LinkedinURL: c.LinkedinURL,
		})
	}
	return items, nil
}

// GetBrightDataTargets returns the LinkedIn profile URLs for a Bright Data
// run: every contact with one, or only those missing a list.
func GetBrightDataTargets(s *xorm.Session, userID int64, onlyMissing bool) ([]string, error) {
	var contacts []*Contact
	if onlyMissing {
		cand, err := loadLinkedinCandidates(s, userID)
		if err != nil {
			return nil, err
		}
		contacts = cand.missing
	} else if err := s.Where("user_id = ?", userID).Cols("linkedin_url").Find(&contacts); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	urls := []string{}
	for _, c := range contacts {
		u := NormalizeLinkedinURL(c.LinkedinURL)
		if u == "" || !strings.Contains(u, "/in/") || seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	return urls, nil
}

// SaveLinkedinBrowserProfile merges a profile sent by the browser extension
// and records the outcome. create adds the person when they are no contact yet.
func SaveLinkedinBrowserProfile(s *xorm.Session, a web.Auth, payload map[string]any, create bool) (*LinkedinApplyOutcome, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	profile, err := LinkedinProfileFromBrowser(payload)
	if err != nil {
		return nil, err
	}
	out, err := ApplyLinkedinProfile(s, a.GetID(), profile, ContactSourceLinkedin, create)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, ErrContactNotInContacts{URL: profile.LinkedinURL}
	}
	if err := RecordLinkedinStatus(s, out.ContactID, LinkedinMethodBrowser, out.Status, jsonFirst(payload, "method")); err != nil {
		return nil, err
	}
	return out, nil
}

// RecordLinkedinBrowserFailure notes that the extension could not read a
// contact's profile. unavailable marks profiles that will never load.
func RecordLinkedinBrowserFailure(s *xorm.Session, a web.Auth, url string, unavailable bool, detail string) error {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return ErrGenericForbidden{}
	}
	c, err := FindContactByLinkedinURL(s, a.GetID(), url)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrContactNotInContacts{URL: url}
	}
	status := linkedinStatusError
	if unavailable {
		status = linkedinStatusUnavailable
	}
	return RecordLinkedinStatus(s, c.ID, LinkedinMethodBrowser, status, detail)
}

// RecordLinkedinBrightDataFailure notes that Bright Data could not scrape a
// contact's profile. retry is false for profiles that will never load.
func RecordLinkedinBrightDataFailure(s *xorm.Session, contactID int64, retry bool, detail string) error {
	status := linkedinStatusUnavailable
	if retry {
		status = linkedinStatusError
	}
	return RecordLinkedinStatus(s, contactID, LinkedinMethodBrightData, status, detail)
}
