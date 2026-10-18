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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/user"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func userBackgroundRequest(t *testing.T, e *echo.Echo, method, token string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, "/api/v2/user/settings/background", body)
		req.Header.Set("Content-Type", contentType)
	} else {
		req = httptest.NewRequest(method, "/api/v2/user/settings/background", nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func userBackgroundFileID(t *testing.T, id int64) int64 {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	u, err := user.GetUserByID(s, id)
	require.NoError(t, err)
	return u.BackgroundFileID
}

func TestHumaUserBackground(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("No background yet is 404", func(t *testing.T) {
		rec := userBackgroundRequest(t, e, http.MethodGet, token, nil, "")
		assert.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("Upload, download, replace and remove", func(t *testing.T) {
		body, contentType := multipartFileBody(t, "background", "bg.png", pngBytes(t))
		rec := userBackgroundRequest(t, e, http.MethodPut, token, body, contentType)
		require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
		first := userBackgroundFileID(t, testuser1.ID)
		require.NotZero(t, first)

		rec = userBackgroundRequest(t, e, http.MethodGet, token, nil, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		assert.NotEmpty(t, rec.Body.Bytes())
		assert.NotEmpty(t, rec.Header().Get("Last-Modified"))

		// Another user does not see it.
		rec = userBackgroundRequest(t, e, http.MethodGet, humaTokenFor(t, &testuser2), nil, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)

		body, contentType = multipartFileBody(t, "background", "bg2.png", pngBytes(t))
		rec = userBackgroundRequest(t, e, http.MethodPut, token, body, contentType)
		require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
		second := userBackgroundFileID(t, testuser1.ID)
		require.NotZero(t, second)
		assert.NotEqual(t, first, second)
		s := db.NewSession()
		old := &files.File{ID: first}
		assert.True(t, files.IsErrFileDoesNotExist(old.LoadFileMetaByID(s)), "replacing must delete the previous file")
		s.Close()

		rec = userBackgroundRequest(t, e, http.MethodDelete, token, nil, "")
		require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
		assert.Zero(t, userBackgroundFileID(t, testuser1.ID))

		// Removing again is harmless.
		rec = userBackgroundRequest(t, e, http.MethodDelete, token, nil, "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("Non-image rejected with 400", func(t *testing.T) {
		body, contentType := multipartFileBody(t, "background", "x.txt", []byte("this is plain text, not an image"))
		rec := userBackgroundRequest(t, e, http.MethodPut, token, body, contentType)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		rec := userBackgroundRequest(t, e, http.MethodGet, "", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
