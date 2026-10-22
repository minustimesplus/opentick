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
	"net/url"
	"testing"

	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHumaTask_WontDo covers the won't-do round trip a client does over v2:
// PATCH the flag on, find the task through a filter, then re-open it.
func TestHumaTask_WontDo(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	patch := func(body string) *models.Task {
		t.Helper()
		rec := humaRequest(t, e, http.MethodPatch, "/api/v2/tasks/1", body, token, "application/merge-patch+json")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		task := &models.Task{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), task))
		return task
	}
	listIDs := func(filter string) []int64 {
		t.Helper()
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects/1/tasks?filter="+url.QueryEscape(filter), "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var page struct {
			Items []*models.Task `json:"items"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		ids := []int64{}
		for _, task := range page.Items {
			ids = append(ids, task.ID)
		}
		return ids
	}

	task := patch(`{"wont_do":true}`)
	assert.True(t, task.WontDo)
	assert.True(t, task.Done, "won't do closes the task")

	assert.Equal(t, []int64{1}, listIDs("wont_do = true"))
	assert.NotContains(t, listIDs("done = false"), int64(1))
	assert.NotContains(t, listIDs("done = true && wont_do = false"), int64(1))

	task = patch(`{"done":false}`)
	assert.False(t, task.Done)
	assert.False(t, task.WontDo, "re-opening clears won't do")
}
