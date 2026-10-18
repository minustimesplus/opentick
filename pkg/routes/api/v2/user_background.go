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
	"net/http"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	backgroundHandler "code.vikunja.io/api/pkg/modules/background/handler"
	"code.vikunja.io/api/pkg/user"
	webfiles "code.vikunja.io/api/pkg/web/files"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"xorm.io/xorm"
)

type userBackgroundUploadInput struct {
	// Same allow-list as project background uploads; the byte-level image check is the real gate.
	RawBody huma.MultipartFormFiles[struct {
		Background huma.FormFile `form:"background" contentType:"image/jpeg,image/png,image/gif,image/bmp,image/tiff,image/webp,application/octet-stream" required:"true" doc:"The background image to upload. Must be a decodable raster image (JPEG, PNG, GIF, BMP, TIFF or WebP); it is resized server-side and re-encoded as JPEG."`
	}]
}

// RegisterUserBackgroundRoutes wires the user's app-wide background image,
// gated by the same static config as project backgrounds.
func RegisterUserBackgroundRoutes(api huma.API) {
	if !config.BackgroundsEnabled.GetBool() {
		return
	}

	tags := []string{"user"}

	Register(api, huma.Operation{
		OperationID: "user-background-get",
		Summary:     "Get your background",
		Description: "Streams the authenticated user's app-wide background image. Always served as image/jpeg with a revalidation Last-Modified header, so a conditional If-Modified-Since request gets a 304. Returns 404 when no background is set.",
		Method:      http.MethodGet,
		Path:        "/user/settings/background",
		Tags:        tags,
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The user background as a jpeg image.",
				Content: map[string]*huma.MediaType{
					"image/jpeg": {
						Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"},
					},
				},
			},
		},
	}, userBackgroundGet)

	if config.BackgroundsUploadEnabled.GetBool() {
		Register(api, withUploadLimits(huma.Operation{
			OperationID:   "user-background-upload",
			Summary:       "Upload your background",
			Description:   "Uploads an image via multipart/form-data under the \"background\" field as the authenticated user's app-wide background. The image is resized server-side and stored as JPEG; it replaces and deletes any previous background (idempotent replace, hence PUT).",
			Method:        http.MethodPut,
			Path:          "/user/settings/background",
			DefaultStatus: http.StatusNoContent,
			Tags:          tags,
		}), userBackgroundUpload)
	}

	Register(api, huma.Operation{
		OperationID: "user-background-delete",
		Summary:     "Remove your background",
		Description: "Removes the authenticated user's background and deletes the stored image. Succeeds even when no background is set.",
		Method:      http.MethodDelete,
		Path:        "/user/settings/background",
		Tags:        tags,
	}, userBackgroundDelete)
}

func init() { AddRouteRegistrar(RegisterUserBackgroundRoutes) }

// backgroundUser re-fetches the full user behind the request: the auth user
// from the JWT claims is partial, and link shares have no user settings.
func backgroundUser(ctx context.Context, s *xorm.Session) (*user.User, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, is := a.(*user.User)
	if !is {
		return nil, huma.Error403Forbidden("only users have a background")
	}
	u, err := user.GetUserByID(s, authUser.ID)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return u, nil
}

func userBackgroundGet(ctx context.Context, _ *struct{}) (*huma.StreamResponse, error) {
	s := db.NewSession()
	defer s.Close()

	u, err := backgroundUser(ctx, s)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}
	bgFile, stat, err := backgroundHandler.LoadUserBackgroundForDownload(u)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		_ = bgFile.File.Close()
		return nil, translateDomainError(err)
	}

	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer func() { _ = bgFile.File.Close() }()
		c := humaecho.Unwrap(hctx)
		webfiles.WriteProjectBackground((*c).Response(), (*c).Request(), bgFile, stat)
	}}, nil
}

func userBackgroundUpload(ctx context.Context, in *userBackgroundUploadInput) (*emptyBody, error) {
	s := db.NewSession()
	defer s.Close()

	u, err := backgroundUser(ctx, s)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}

	file := in.RawBody.Data().Background
	defer func() { _ = file.Close() }()

	if err := backgroundHandler.ValidateAndSaveUserBackground(s, u, file, file.Filename); err != nil {
		_ = s.Rollback()
		if backgroundHandler.IsErrFileIsNoImage(err) || backgroundHandler.IsErrFileUnsupportedImageFormat(err) {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

func userBackgroundDelete(ctx context.Context, _ *struct{}) (*emptyBody, error) {
	s := db.NewSession()
	defer s.Close()

	u, err := backgroundUser(ctx, s)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}
	if err := backgroundHandler.RemoveUserBackground(s, u); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
