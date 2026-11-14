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

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"xorm.io/xorm"
)

// Imports carry whole address books; the default 1 MiB body limit is too small for them.
const contactImportMaxBody = 48 * 1024 * 1024

// RegisterContactImportRoutes wires contact imports onto the Huma API.
func RegisterContactImportRoutes(api huma.API) {
	tags := []string{"contacts"}

	Register(api, huma.Operation{
		OperationID: "contacts-import-linkedin-csv",
		Summary:     "Import a LinkedIn Connections.csv",
		Description: "Merges the Connections.csv of a LinkedIn data export into the contacts. Rows are matched by LinkedIn URL, then email. Incoming values fill empty fields and refresh unlocked ones; " +
			"hand-edited (locked) fields are never overwritten and nothing is cleared.",
		Method:        http.MethodPost,
		Path:          "/contacts/import/linkedin-csv",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
		MaxBodyBytes:  contactImportMaxBody,
	}, contactsImportLinkedinCSV)

	Register(api, huma.Operation{
		OperationID: "contacts-import-bundle",
		Summary:     "Import contacts from another personal CRM",
		Description: "Stores whole contacts with their history, notes, tags, groups and locked fields, keyed by their id in the source (import_id). Importing an id again replaces that contact, so an import can be repeated or sent in several requests. " +
			"Photos are uploaded afterwards, to the contact ids this returns. Used by the import-personalcrm command.",
		Method:        http.MethodPost,
		Path:          "/contacts/import/bundle",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
		MaxBodyBytes:  contactImportMaxBody,
	}, contactsImportBundle)
}

func init() { AddRouteRegistrar(RegisterContactImportRoutes) }

type contactImportBody struct {
	Body *models.ContactImportResult
}

func contactsImportLinkedinCSV(ctx context.Context, in *struct {
	Body struct {
		CSV string `json:"csv" minLength:"1" doc:"The text of Connections.csv, preamble included."`
	}
}) (*contactImportBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.ContactImportResult, error) {
		return models.ImportLinkedinConnectionsCSV(s, a, in.Body.CSV)
	})
	if err != nil {
		return nil, err
	}
	return &contactImportBody{Body: res}, nil
}

type contactBundleBody struct {
	Body *models.ContactBundleResult
}

func contactsImportBundle(ctx context.Context, in *struct {
	Body models.ContactBundle
}) (*contactBundleBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.ContactBundleResult, error) {
		return models.ImportContactBundle(s, a, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &contactBundleBody{Body: res}, nil
}
