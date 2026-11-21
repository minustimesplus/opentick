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
	"encoding/json"
	"net/http"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"xorm.io/xorm"
)

// The browser extension reads LinkedIn profiles from the user's logged-in
// session and sends them here. These routes are scoped so an API token can
// be granted exactly them (contacts: linkedin_profiles, linkedin_failures,
// linkedin_queue, linkedin_lookup).

// RegisterContactLinkedinRoutes wires the browser extension's routes onto the Huma API.
func RegisterContactLinkedinRoutes(api huma.API) {
	tags := []string{"contacts"}

	Register(api, huma.Operation{
		OperationID: "contacts-linkedin-profile-save",
		Summary:     "Save a LinkedIn profile from the browser extension",
		Description: "Merges a profile read from LinkedIn's logged-in view into the contact with that profile URL. Scalar fields fill and refresh unlocked fields and are never cleared; " +
			"experience and education replace the stored lists when the capture is complete or richer, unless they were edited by hand. " +
			"With create, a person who is no contact yet is added; without it, 404 means nobody in the contacts has that URL. " +
			"When the contact has no photo, the profile photo is downloaded from LinkedIn's image CDN. Records the outcome so the queue skips the contact.",
		Method:        http.MethodPost,
		Path:          "/contacts/linkedin/profiles",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
		MaxBodyBytes:  4 * 1024 * 1024,
	}, contactsLinkedinSaveProfile)

	Register(api, huma.Operation{
		OperationID: "contacts-linkedin-failure-record",
		Summary:     "Record that a LinkedIn profile could not be read",
		Description: "Notes a failed read for the contact with that profile URL. unavailable marks a profile that will never load (deleted, private, not a profile) and removes it from the queue for good; " +
			"other failures are retried until three attempts have failed. 404 if nobody in the contacts has that URL.",
		Method:        http.MethodPost,
		Path:          "/contacts/linkedin/failures",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsLinkedinRecordFailure)

	Register(api, huma.Operation{
		OperationID: "contacts-linkedin-queue",
		Summary:     "List contacts the browser extension should visit",
		Description: "Returns the caller's contacts that have a LinkedIn URL but lack experience or education, most recently updated first, " +
			"leaving out those already captured, marked unavailable, or failed three times. total counts the whole queue; coverage counts history across all contacts.",
		Method: http.MethodGet,
		Path:   "/contacts/linkedin/queue",
		Tags:   tags,
	}, contactsLinkedinQueue)

	Register(api, huma.Operation{
		OperationID: "contacts-linkedin-lookup",
		Summary:     "Find the contact behind a LinkedIn profile URL",
		Description: "Returns the caller's contact with that profile URL (normalized, so tracking parameters and case don't matter), or 404 when there is none.",
		Method:      http.MethodGet,
		Path:        "/contacts/linkedin/lookup",
		Tags:        tags,
	}, contactsLinkedinLookup)
}

func init() { AddRouteRegistrar(RegisterContactLinkedinRoutes) }

// linkedinBrowserProfile is the extension's payload. Item lists stay loosely
// typed: each item is either an array of text lines or {lines, sub}.
type linkedinBrowserProfile struct {
	_                struct{}               `additionalProperties:"true"`
	LinkedinURL      string                 `json:"linkedin_url" minLength:"1" doc:"The profile URL, e.g. https://www.linkedin.com/in/jane-doe."`
	Create           bool                   `json:"create,omitempty" doc:"Add the person as a new contact when nobody has this URL yet."`
	Complete         *bool                  `json:"complete,omitempty" doc:"Whether the lists are the full logged-in view. Defaults to true; send false for partial reads so they only replace poorer lists."`
	Method           string                 `json:"method,omitempty" doc:"How the extension read the profile, e.g. voyager-dash, embedded-json or dom. Stored with the outcome."`
	ExtensionVersion string                 `json:"extension_version,omitempty"`
	Profile          *linkedinProfileBasics `json:"profile,omitempty"`
	Experiences      []linkedinExperience   `json:"experiences,omitempty" doc:"Structured jobs, newest first."`
	Educations       []linkedinEducation    `json:"educations,omitempty" doc:"Structured schools, newest first."`
	ExperienceItems  []any                  `json:"experience_items,omitempty" doc:"Text lines of each item on /details/experience/, used only when experiences is empty. An item is an array of lines or {lines, sub} with grouped roles in sub."`
	EducationItems   []any                  `json:"education_items,omitempty" doc:"Text lines of each item on /details/education/, used only when educations is empty."`
}

type linkedinProfileBasics struct {
	_         struct{} `additionalProperties:"true"`
	FirstName string   `json:"first_name,omitempty"`
	LastName  string   `json:"last_name,omitempty"`
	FullName  string   `json:"full_name,omitempty"`
	Headline  string   `json:"headline,omitempty"`
	Location  string   `json:"location,omitempty"`
	About     string   `json:"about,omitempty"`
	AvatarURL string   `json:"avatar_url,omitempty" doc:"Profile photo on LinkedIn's image CDN (licdn.com); other hosts are ignored."`
}

type linkedinExperience struct {
	_           struct{} `additionalProperties:"true"`
	Title       string   `json:"title,omitempty"`
	Company     string   `json:"company,omitempty"`
	Location    string   `json:"location,omitempty"`
	StartDate   string   `json:"start_date,omitempty" doc:"As LinkedIn shows it, e.g. Jan 2022 or 2019."`
	EndDate     string   `json:"end_date,omitempty" doc:"Present for a current job."`
	Description string   `json:"description,omitempty"`
}

type linkedinEducation struct {
	_            struct{} `additionalProperties:"true"`
	School       string   `json:"school,omitempty"`
	Degree       string   `json:"degree,omitempty"`
	FieldOfStudy string   `json:"field_of_study,omitempty"`
	StartYear    string   `json:"start_year,omitempty"`
	EndYear      string   `json:"end_year,omitempty"`
}

// toMap hands the payload to models.LinkedinProfileFromBrowser, which reads
// the same JSON the old desktop bridge received.
func (p *linkedinBrowserProfile) toMap() (map[string]any, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type linkedinSaveResult struct {
	models.LinkedinApplyOutcome
	PhotoSaved bool `json:"photo_saved" doc:"A profile photo was downloaded and stored for a contact that had none."`
}

type linkedinSaveBody struct {
	Body *linkedinSaveResult
}

func contactsLinkedinSaveProfile(ctx context.Context, in *struct {
	Body linkedinBrowserProfile
}) (*linkedinSaveBody, error) {
	payload, err := in.Body.toMap()
	if err != nil {
		return nil, err
	}
	out, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.LinkedinApplyOutcome, error) {
		return models.SaveLinkedinBrowserProfile(s, a, payload, in.Body.Create)
	})
	if err != nil {
		return nil, err
	}
	res := &linkedinSaveResult{LinkedinApplyOutcome: *out}
	if out.WantsPhoto != "" {
		res.PhotoSaved = saveLinkedinPhoto(ctx, out.ContactID, out.WantsPhoto)
	}
	return &linkedinSaveBody{Body: res}, nil
}

// saveLinkedinPhoto stores the profile photo after the profile itself is
// committed. It is best effort: a failed download never fails the save.
func saveLinkedinPhoto(ctx context.Context, contactID int64, url string) bool {
	a, err := authFromCtx(ctx)
	if err != nil {
		return false
	}
	s := db.NewSession()
	defer s.Close()
	saved, err := models.SaveLinkedinPhoto(ctx, s, a, contactID, url)
	if err != nil {
		_ = s.Rollback()
		log.Debugf("Could not store the LinkedIn photo of contact %d: %s", contactID, err)
		return false
	}
	if err := s.Commit(); err != nil {
		log.Debugf("Could not store the LinkedIn photo of contact %d: %s", contactID, err)
		return false
	}
	return saved
}

type linkedinFailureResult struct {
	Recorded bool `json:"recorded"`
}

type linkedinFailureBody struct {
	Body *linkedinFailureResult
}

func contactsLinkedinRecordFailure(ctx context.Context, in *struct {
	Body struct {
		LinkedinURL string `json:"linkedin_url" minLength:"1" doc:"The profile URL that could not be read."`
		Unavailable bool   `json:"unavailable,omitempty" doc:"The profile will never load (not found, private, not a profile URL). Takes it off the queue for good."`
		Detail      string `json:"detail,omitempty" maxLength:"2000" doc:"What went wrong, for the record."`
	}
}) (*linkedinFailureBody, error) {
	_, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (bool, error) {
		return true, models.RecordLinkedinBrowserFailure(s, a, in.Body.LinkedinURL, in.Body.Unavailable, in.Body.Detail)
	})
	if err != nil {
		return nil, err
	}
	return &linkedinFailureBody{Body: &linkedinFailureResult{Recorded: true}}, nil
}

type linkedinQueueResult struct {
	Items    []*models.LinkedinQueueItem `json:"items"`
	Total    int                         `json:"total" doc:"How many contacts are left in the queue, beyond the limit too."`
	Coverage *models.LinkedinCoverage    `json:"coverage"`
}

type linkedinQueueBody struct {
	Body *linkedinQueueResult
}

func contactsLinkedinQueue(ctx context.Context, in *struct {
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"500" doc:"How many contacts to return."`
}) (*linkedinQueueBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*linkedinQueueResult, error) {
		items, err := models.GetLinkedinBrowserQueue(s, a, in.Limit)
		if err != nil {
			return nil, err
		}
		coverage, err := models.GetLinkedinCoverage(s, a)
		if err != nil {
			return nil, err
		}
		return &linkedinQueueResult{Items: items, Total: coverage.BrowserQueue, Coverage: coverage}, nil
	})
	if err != nil {
		return nil, err
	}
	return &linkedinQueueBody{Body: res}, nil
}

type linkedinLookupBody struct {
	Body *models.LinkedinContactMatch
}

func contactsLinkedinLookup(ctx context.Context, in *struct {
	URL string `query:"url" required:"true" minLength:"1" doc:"A LinkedIn profile URL."`
}) (*linkedinLookupBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.LinkedinContactMatch, error) {
		return models.FindLinkedinContact(s, a, in.URL)
	})
	if err != nil {
		return nil, err
	}
	return &linkedinLookupBody{Body: res}, nil
}
