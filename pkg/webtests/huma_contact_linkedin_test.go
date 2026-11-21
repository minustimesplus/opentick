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

package webtests

import (
	"encoding/json"
	"net/http"
	"testing"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A payload shaped like the browser extension's Voyager capture.
const linkedinAdaPayload = `{
	"linkedin_url": "https://www.linkedin.com/in/Ada/?trk=x",
	"extension_version": "2.0.0",
	"method": "voyager-dash",
	"complete": true,
	"profile": {"first_name": "Ada", "last_name": "Lovelace", "headline": "Countess", "avatar_url": "https://evil.example/ada.jpg"},
	"experiences": [
		{"title": "Analyst", "company": "Babbage & Co", "start_date": "Jan 1842", "end_date": "Present"},
		{"title": "Translator", "company": "Taylor's Scientific Memoirs", "start_date": "1842", "end_date": "1843"}
	],
	"educations": [{"school": "Home schooled", "degree": "Mathematics", "start_year": "1828", "end_year": "1835"}]
}`

// Fixtures: user1's Ada (#1) has a job but no school, Grace (#2) has both,
// Jean (#3) has no LinkedIn URL. user2's contact #4 is linkedin.com/in/else.
func TestHumaContactLinkedin(t *testing.T) {
	testHandler := webHandlerTestV2{user: &testuser1, t: t}

	t.Run("Queue", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/contacts/linkedin/queue", "")
		require.NoError(t, err)
		var body struct {
			Items []struct {
				ContactID        int64  `json:"contact_id"`
				MissingEducation bool   `json:"missing_education"`
				Status           string `json:"status"`
			} `json:"items"`
			Total    int `json:"total"`
			Coverage struct {
				WithURL int `json:"with_url"`
			} `json:"coverage"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 1)
		assert.EqualValues(t, 1, body.Items[0].ContactID)
		assert.True(t, body.Items[0].MissingEducation)
		assert.Empty(t, body.Items[0].Status)
		assert.Equal(t, 1, body.Total)
		assert.Equal(t, 2, body.Coverage.WithURL)
		assert.NotContains(t, rec.Body.String(), "Someone Else")
	})

	t.Run("Lookup", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/contacts/linkedin/lookup?url=https://linkedin.com/in/GRACE/", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"contact_id":2`)
		assert.Contains(t, rec.Body.String(), `"has_education":true`)

		_, err = testHandler.serve(http.MethodGet, "/api/v2/contacts/linkedin/lookup?url=https://www.linkedin.com/in/else", "")
		require.Error(t, err, "another user's contact")
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})

	t.Run("Save profile", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/profiles", linkedinAdaPayload)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"contact_id":1`)
		assert.Contains(t, rec.Body.String(), `"created":false`)
		assert.Contains(t, rec.Body.String(), `"status":"ok"`)
		assert.Contains(t, rec.Body.String(), `"experiences":2`)
		assert.Contains(t, rec.Body.String(), `"educations_written":true`)
		assert.Contains(t, rec.Body.String(), `"photo_saved":false`, "photos only come from LinkedIn's CDN")
		db.AssertExists(t, "contact_experiences", map[string]any{"contact_id": 1, "title": "Translator"}, false)
		db.AssertExists(t, "contact_educations", map[string]any{"contact_id": 1, "school": "Home schooled"}, false)
		db.AssertExists(t, "contact_linkedin_status", map[string]any{"contact_id": 1, "method": "browser", "status": "ok"}, false)
		// Company is locked on Ada; the current job doesn't overwrite it.
		db.AssertExists(t, "contacts", map[string]any{"id": 1, "company": "Analytical Engines", "headline": "Countess", "title": "Analyst"}, false)

		rec, err = testHandler.serve(http.MethodGet, "/api/v2/contacts/1", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"school":"Home schooled"`)

		rec, err = testHandler.serve(http.MethodGet, "/api/v2/contacts/linkedin/queue", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"total":0`, "a captured contact leaves the queue")
	})

	t.Run("Save profile of a stranger", func(t *testing.T) {
		stranger := `{"linkedin_url":"https://www.linkedin.com/in/else","profile":{"full_name":"New Person"},"experiences":[{"title":"CEO","company":"Acme"}]}`
		_, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/profiles", stranger)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err), "without create, user2's contact with that URL is not touched")
		db.AssertExists(t, "contacts", map[string]any{"id": 4, "full_name": "Someone Else"}, false)

		withCreate := `{"linkedin_url":"https://www.linkedin.com/in/else","create":true,"profile":{"full_name":"New Person"},"experiences":[{"title":"CEO","company":"Acme"}]}`
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/profiles", withCreate)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"created":true`)
		assert.Contains(t, rec.Body.String(), `"status":"partial"`)
		db.AssertExists(t, "contacts", map[string]any{"user_id": 1, "full_name": "New Person", "linkedin_url": "https://www.linkedin.com/in/else"}, false)
		db.AssertExists(t, "contacts", map[string]any{"id": 4, "full_name": "Someone Else"}, false)
	})

	t.Run("Save rejects a non-profile URL", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/profiles", `{"linkedin_url":"https://www.linkedin.com/company/acme"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
	})

	t.Run("Save from rendered page lines", func(t *testing.T) {
		dom := `{"linkedin_url":"https://www.linkedin.com/in/grace","method":"dom","complete":false,
			"experience_items":[{"lines":["Rear Admiral","US Navy","1943 - 1986 · 43 yrs"],"sub":[]}],
			"education_items":[["Vassar College","BA, Mathematics","1924 - 1928"]]}`
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/profiles", dom)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"contact_id":2`)
	})

	t.Run("Record failure", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/failures", `{"linkedin_url":"https://www.linkedin.com/in/grace","unavailable":true,"detail":"profile not found"}`)
		require.NoError(t, err)
		db.AssertExists(t, "contact_linkedin_status", map[string]any{"contact_id": 2, "method": "browser", "status": "unavailable"}, false)

		_, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/linkedin/failures", `{"linkedin_url":"https://www.linkedin.com/in/nobody"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})
}

// The extension authenticates with an API token granted only its four routes.
func TestHumaContactLinkedin_APIToken(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	jwt := humaTokenFor(t, &testuser1)

	rec := humaRequest(t, e, http.MethodPost, "/api/v2/tokens", `{
		"title": "LinkedIn extension",
		"expires_at": "2099-01-01T00:00:00Z",
		"permissions": {"contacts": ["linkedin_profiles", "linkedin_failures", "linkedin_queue", "linkedin_lookup"]}
	}`, jwt, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	var created struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Token)

	rec = humaRequest(t, e, http.MethodGet, "/api/v2/contacts/linkedin/queue", "", created.Token, "")
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	rec = humaRequest(t, e, http.MethodGet, "/api/v2/contacts/linkedin/lookup?url=linkedin.com/in/ada", "", created.Token, "")
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	rec = humaRequest(t, e, http.MethodPost, "/api/v2/contacts/linkedin/profiles", linkedinAdaPayload, created.Token, "")
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	rec = humaRequest(t, e, http.MethodPost, "/api/v2/contacts/linkedin/failures", `{"linkedin_url":"https://www.linkedin.com/in/grace"}`, created.Token, "")
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	// Nothing else: the token can't read or list contacts.
	rec = humaRequest(t, e, http.MethodGet, "/api/v2/contacts", "", created.Token, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
	rec = humaRequest(t, e, http.MethodGet, "/api/v2/contacts/1", "", created.Token, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}
