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
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
	"xorm.io/xorm"
)

type contactListBody struct {
	Body Paginated[*models.Contact]
}

// contactsTx runs a custom contact action in one transaction. The actions
// have no handler.Do* wrapper, so fn checks permissions itself, through the
// model.
func contactsTx[T any](ctx context.Context, fn func(s *xorm.Session, a web.Auth) (T, error)) (out T, err error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return out, err
	}
	s := db.NewSession()
	defer s.Close()
	out, err = fn(s, a)
	if err != nil {
		_ = s.Rollback()
		return out, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return out, translateDomainError(err)
	}
	return out, nil
}

// contactEntryTx changes the history or notes of a contact the caller may update.
func contactEntryTx(ctx context.Context, id int64, fn func(s *xorm.Session, c *models.Contact) error) error {
	_, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (bool, error) {
		c, err := updatableContact(s, a, id)
		if err != nil {
			return false, err
		}
		return true, fn(s, c)
	})
	return err
}

// updatableContact loads a contact the caller may change.
func updatableContact(s *xorm.Session, a web.Auth, id int64) (*models.Contact, error) {
	c := &models.Contact{ID: id}
	can, err := c.CanUpdate(s, a)
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, models.ErrContactDoesNotExist{ID: id}
	}
	return c, nil
}

const contactEntryLockDoc = " Locks the list against LinkedIn re-imports, like any hand edit."

// RegisterContactRoutes wires the personal CRM's contacts onto the Huma API.
func RegisterContactRoutes(api huma.API) {
	tags := []string{"contacts"}

	Register(api, huma.Operation{
		OperationID: "contacts-list",
		Summary:     "List contacts",
		Description: "Returns the authenticated user's contacts by name, paginated, without their history and notes. q matches name, company, title, email, location and headline. Contacts are private: only their owner sees them. For structured filters use the search operation.",
		Method:      http.MethodGet,
		Path:        "/contacts",
		Tags:        tags,
	}, contactsList)

	Register(api, huma.Operation{
		OperationID: "contacts-overview",
		Summary:     "Get all contacts at once",
		Description: "Returns every contact as a compact list row, plus all tags and groups, in one response. This is what the People view loads; it is not paginated.",
		Method:      http.MethodGet,
		Path:        "/contacts/overview",
		Tags:        tags,
	}, contactsOverview)

	Register(api, huma.Operation{
		OperationID: "contacts-read",
		Summary:     "Get a contact",
		Description: "Everything about one contact: fields, experience, education, notes, tags and groups. Only the owner may read it.",
		Method:      http.MethodGet,
		Path:        "/contacts/{id}",
		Tags:        tags,
	}, contactsRead)

	Register(api, huma.Operation{
		OperationID: "contacts-create",
		Summary:     "Create a contact",
		Description: "Creates a contact. Needs a name, LinkedIn URL or email. Refuses with 409 if a contact with the same LinkedIn URL or email already exists (the error names it) unless allow_duplicate is set.",
		Method:      http.MethodPost,
		Path:        "/contacts",
		Tags:        tags,
	}, contactsCreate)

	Register(api, huma.Operation{
		OperationID: "contacts-update",
		Summary:     "Update a contact",
		Description: "Sets a contact's fields: full_name, headline, company, title, location, email, phone, linkedin_url, birthday (YYYY-MM-DD or MM-DD) and about. An empty string clears a field. " +
			"Every field whose value changes is locked, so LinkedIn imports and enrichment won't overwrite it. PUT replaces all fields; use PATCH to change only some.",
		Method: http.MethodPut,
		Path:   "/contacts/{id}",
		Tags:   tags,
	}, contactsUpdate)

	Register(api, huma.Operation{
		OperationID: "contacts-delete",
		Summary:     "Delete a contact",
		Description: "Deletes a contact with its history, notes and photo. This cannot be undone.",
		Method:      http.MethodDelete,
		Path:        "/contacts/{id}",
		Tags:        tags,
	}, contactsDelete)

	Register(api, huma.Operation{
		OperationID: "contacts-search",
		Summary:     "Search contacts",
		Description: "Finds contacts by structured filters and/or free text. Each result says why it matched (e.g. \"school: Sciences Po (Master, 2019–2021)\"), so edge cases can be checked before acting. " +
			"Tip: call the distinct values operation first to discover spelling variants of a school, company or location.",
		Method:        http.MethodPost,
		Path:          "/contacts/search",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsSearch)

	Register(api, huma.Operation{
		OperationID: "contacts-distinct-values",
		Summary:     "List the distinct values of a contact field",
		Description: "Lists the distinct values of a field across all contacts, with how many contacts have each, most common first. Use it to find every spelling of a school, company or city before searching.",
		Method:      http.MethodGet,
		Path:        "/contacts/distinct-values",
		Tags:        tags,
	}, contactsDistinctValues)

	Register(api, huma.Operation{
		OperationID:   "contacts-add-to-group",
		Summary:       "Add contacts to a group",
		Description:   "Adds contacts to a group. Reports how many were added, how many already were members, and unknown ids. Use dry_run to preview.",
		Method:        http.MethodPost,
		Path:          "/contacts/bulk/add-to-group",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsAddToGroup)

	Register(api, huma.Operation{
		OperationID:   "contacts-remove-from-group",
		Summary:       "Remove contacts from a group",
		Description:   "Removes contacts from a group. The contacts themselves stay. Use dry_run to preview.",
		Method:        http.MethodPost,
		Path:          "/contacts/bulk/remove-from-group",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsRemoveFromGroup)

	Register(api, huma.Operation{
		OperationID:   "contacts-tag",
		Summary:       "Tag contacts",
		Description:   "Adds a tag to contacts. The tag is created if needed. Use dry_run to preview.",
		Method:        http.MethodPost,
		Path:          "/contacts/bulk/tag",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsTag)

	Register(api, huma.Operation{
		OperationID:   "contacts-untag",
		Summary:       "Untag contacts",
		Description:   "Removes a tag from contacts. Use dry_run to preview.",
		Method:        http.MethodPost,
		Path:          "/contacts/bulk/untag",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsUntag)

	Register(api, huma.Operation{
		OperationID:   "contacts-bulk-delete",
		Summary:       "Delete several contacts",
		Description:   "Deletes the given contacts with their history, notes and photos. Ids that are not the caller's contacts are ignored. This cannot be undone.",
		Method:        http.MethodPost,
		Path:          "/contacts/bulk/delete",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactsBulkDelete)

	Register(api, huma.Operation{
		OperationID: "contacts-experiences-create",
		Summary:     "Add a job to a contact",
		Description: "Adds a job to a contact's experience. Needs a company or a title. A current job (no end date) goes to the top of the list." + contactEntryLockDoc,
		Method:      http.MethodPost,
		Path:        "/contacts/{id}/experiences",
		Tags:        tags,
	}, contactsExperiencesCreate)

	Register(api, huma.Operation{
		OperationID: "contacts-experiences-update",
		Summary:     "Update a job of a contact",
		Description: "Replaces the fields of one job." + contactEntryLockDoc,
		Method:      http.MethodPut,
		Path:        "/contacts/{id}/experiences/{entry}",
		Tags:        tags,
	}, contactsExperiencesUpdate)

	Register(api, huma.Operation{
		OperationID: "contacts-experiences-delete",
		Summary:     "Remove a job from a contact",
		Description: "Removes one job from a contact's experience." + contactEntryLockDoc,
		Method:      http.MethodDelete,
		Path:        "/contacts/{id}/experiences/{entry}",
		Tags:        tags,
	}, contactsExperiencesDelete)

	Register(api, huma.Operation{
		OperationID: "contacts-educations-create",
		Summary:     "Add a school to a contact",
		Description: "Adds a school to a contact's education. Needs the school's name." + contactEntryLockDoc,
		Method:      http.MethodPost,
		Path:        "/contacts/{id}/educations",
		Tags:        tags,
	}, contactsEducationsCreate)

	Register(api, huma.Operation{
		OperationID: "contacts-educations-update",
		Summary:     "Update a school of a contact",
		Description: "Replaces the fields of one school." + contactEntryLockDoc,
		Method:      http.MethodPut,
		Path:        "/contacts/{id}/educations/{entry}",
		Tags:        tags,
	}, contactsEducationsUpdate)

	Register(api, huma.Operation{
		OperationID: "contacts-educations-delete",
		Summary:     "Remove a school from a contact",
		Description: "Removes one school from a contact's education." + contactEntryLockDoc,
		Method:      http.MethodDelete,
		Path:        "/contacts/{id}/educations/{entry}",
		Tags:        tags,
	}, contactsEducationsDelete)

	Register(api, huma.Operation{
		OperationID: "contacts-notes-create",
		Summary:     "Add a note to a contact",
		Description: "Adds a note to a contact. Notes are listed newest first when reading the contact.",
		Method:      http.MethodPost,
		Path:        "/contacts/{id}/notes",
		Tags:        tags,
	}, contactsNotesCreate)

	Register(api, huma.Operation{
		OperationID: "contacts-notes-update",
		Summary:     "Update a note of a contact",
		Description: "Replaces the text of one note.",
		Method:      http.MethodPut,
		Path:        "/contacts/{id}/notes/{entry}",
		Tags:        tags,
	}, contactsNotesUpdate)

	Register(api, huma.Operation{
		OperationID: "contacts-notes-delete",
		Summary:     "Delete a note of a contact",
		Description: "Deletes one note.",
		Method:      http.MethodDelete,
		Path:        "/contacts/{id}/notes/{entry}",
		Tags:        tags,
	}, contactsNotesDelete)

	Register(api, huma.Operation{
		OperationID: "contacts-photo-get",
		Summary:     "Get a contact's photo",
		Description: "Returns the contact's photo as JPEG bytes, at most 512 pixels on its longer edge. 404 if the contact has none (has_photo is false).",
		Method:      http.MethodGet,
		Path:        "/contacts/{id}/photo",
		Tags:        tags,
		// Spell out the binary response; a bare []byte Body would otherwise be
		// modeled as a base64 JSON string instead of binary image data.
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The photo, as image/jpeg.",
				Content: map[string]*huma.MediaType{
					"image/jpeg": {
						Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"},
					},
				},
			},
		},
	}, contactsPhotoGet)

	// POST, not PUT: a PUT next to the binary GET would make AutoPatch synthesise a PATCH for the photo.
	Register(api, withUploadLimits(huma.Operation{
		OperationID:   "contacts-photo-upload",
		Summary:       "Set a contact's photo",
		Description:   "Uploads an image as the contact's photo via multipart/form-data under the \"photo\" field, replacing the previous one. It is scaled down and stored as JPEG. Locks the photo, so enrichment won't replace it.",
		Method:        http.MethodPost,
		Path:          "/contacts/{id}/photo",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}), contactsPhotoUpload)

	Register(api, huma.Operation{
		OperationID: "contacts-photo-delete",
		Summary:     "Remove a contact's photo",
		Description: "Removes the contact's photo and locks it, so enrichment won't add one again. Succeeds even if there was none.",
		Method:      http.MethodDelete,
		Path:        "/contacts/{id}/photo",
		Tags:        tags,
	}, contactsPhotoDelete)
}

func init() { AddRouteRegistrar(RegisterContactRoutes) }

func contactsList(ctx context.Context, in *struct {
	ListParams
}) (*contactListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.Contact{}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Contact)
	if !ok {
		return nil, fmt.Errorf("contacts ReadAll returned unexpected type %T (expected []*models.Contact)", result)
	}
	return &contactListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type contactOverviewBody struct {
	Body *models.ContactOverview
}

func contactsOverview(ctx context.Context, _ *struct{}) (*contactOverviewBody, error) {
	overview, err := contactsTx(ctx, models.GetContactOverview)
	if err != nil {
		return nil, err
	}
	return &contactOverviewBody{Body: overview}, nil
}

type contactReadBody struct {
	models.Contact
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this contact. Always 2 (admin), as only the owner can read it."`
}

func contactsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[contactReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &models.Contact{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, c, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &contactReadBody{Contact: *c, MaxPermission: models.Permission(maxPermission)}
	return conditionalReadResponse(&in.Params, body, c.Updated, maxPermission)
}

func contactsCreate(ctx context.Context, in *struct {
	AllowDuplicate bool `query:"allow_duplicate" doc:"Create the contact even if one with the same LinkedIn URL or email exists."`
	Body           models.Contact
}) (*singleBody[models.Contact], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.AllowDuplicate = in.AllowDuplicate
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Contact]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func contactsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body contactReadBody
}) (*singleBody[models.Contact], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &in.Body.Contact
	c.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, c, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Contact]{Body: c}, nil
}

func contactsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Contact{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type contactSearchBody struct {
	Body *models.ContactSearchResult
}

func contactsSearch(ctx context.Context, in *struct {
	Body models.ContactSearch
}) (*contactSearchBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.ContactSearchResult, error) {
		return models.SearchContacts(s, a, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &contactSearchBody{Body: res}, nil
}

type contactDistinctValuesBody struct {
	Body *models.ContactDistinctValues
}

func contactsDistinctValues(ctx context.Context, in *struct {
	Field    string `query:"field" required:"true" enum:"school,company,location,title,degree,field_of_study,tag,group" doc:"The field to list the values of. company and title include past jobs."`
	Contains string `query:"contains" doc:"Only values containing this (ignores case, accents, spaces and punctuation)."`
	Limit    int    `query:"limit" default:"100" minimum:"1" doc:"Maximum values returned. total_values always gives the full count."`
}) (*contactDistinctValuesBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.ContactDistinctValues, error) {
		return models.GetContactDistinctValues(s, a, in.Field, in.Contains, in.Limit)
	})
	if err != nil {
		return nil, err
	}
	return &contactDistinctValuesBody{Body: res}, nil
}

type contactMembershipBody struct {
	Body *models.ContactMembershipResult
}

type contactGroupMembershipInput struct {
	Body struct {
		Group      string  `json:"group" minLength:"1" doc:"Group name or id."`
		ContactIDs []int64 `json:"contact_ids" minItems:"1" doc:"The contacts (ids from a search)."`
		Create     bool    `json:"create,omitempty" doc:"When adding: create the group if it does not exist."`
		DryRun     bool    `json:"dry_run,omitempty" doc:"Report what would change without writing."`
	}
}

type contactTagMembershipInput struct {
	Body struct {
		Tag        string  `json:"tag" minLength:"1" doc:"Tag name or id."`
		ContactIDs []int64 `json:"contact_ids" minItems:"1" doc:"The contacts (ids from a search)."`
		DryRun     bool    `json:"dry_run,omitempty" doc:"Report what would change without writing."`
	}
}

func changeContactMembership(ctx context.Context, change *models.ContactMembershipChange) (*contactMembershipBody, error) {
	res, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.ContactMembershipResult, error) {
		return models.ChangeContactMembership(s, a, change)
	})
	if err != nil {
		return nil, err
	}
	return &contactMembershipBody{Body: res}, nil
}

func contactsAddToGroup(ctx context.Context, in *contactGroupMembershipInput) (*contactMembershipBody, error) {
	return changeContactMembership(ctx, &models.ContactMembershipChange{
		Kind:       models.ContactMembershipGroup,
		Target:     in.Body.Group,
		ContactIDs: in.Body.ContactIDs,
		On:         true,
		Create:     in.Body.Create,
		DryRun:     in.Body.DryRun,
	})
}

func contactsRemoveFromGroup(ctx context.Context, in *contactGroupMembershipInput) (*contactMembershipBody, error) {
	return changeContactMembership(ctx, &models.ContactMembershipChange{
		Kind:       models.ContactMembershipGroup,
		Target:     in.Body.Group,
		ContactIDs: in.Body.ContactIDs,
		DryRun:     in.Body.DryRun,
	})
}

func contactsTag(ctx context.Context, in *contactTagMembershipInput) (*contactMembershipBody, error) {
	return changeContactMembership(ctx, &models.ContactMembershipChange{
		Kind:       models.ContactMembershipTag,
		Target:     in.Body.Tag,
		ContactIDs: in.Body.ContactIDs,
		On:         true,
		DryRun:     in.Body.DryRun,
	})
}

func contactsUntag(ctx context.Context, in *contactTagMembershipInput) (*contactMembershipBody, error) {
	return changeContactMembership(ctx, &models.ContactMembershipChange{
		Kind:       models.ContactMembershipTag,
		Target:     in.Body.Tag,
		ContactIDs: in.Body.ContactIDs,
		DryRun:     in.Body.DryRun,
	})
}

type contactBulkDeleteBody struct {
	Body struct {
		Deleted int `json:"deleted"`
	}
}

func contactsBulkDelete(ctx context.Context, in *struct {
	Body struct {
		ContactIDs []int64 `json:"contact_ids" minItems:"1"`
	}
}) (*contactBulkDeleteBody, error) {
	deleted, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (int, error) {
		return models.DeleteContactsByID(s, a, in.Body.ContactIDs)
	})
	if err != nil {
		return nil, err
	}
	out := &contactBulkDeleteBody{}
	out.Body.Deleted = deleted
	return out, nil
}

func contactsExperiencesCreate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body models.ContactExperience
}) (*singleBody[models.ContactExperience], error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.AddExperience(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactExperience]{Body: &in.Body}, nil
}

func contactsExperiencesUpdate(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the job."`
	Body  models.ContactExperience
}) (*singleBody[models.ContactExperience], error) {
	in.Body.ID = in.Entry
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.UpdateExperience(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactExperience]{Body: &in.Body}, nil
}

func contactsExperiencesDelete(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the job."`
}) (*emptyBody, error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.DeleteExperience(s, in.Entry)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}

func contactsEducationsCreate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body models.ContactEducation
}) (*singleBody[models.ContactEducation], error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.AddEducation(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactEducation]{Body: &in.Body}, nil
}

func contactsEducationsUpdate(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the school entry."`
	Body  models.ContactEducation
}) (*singleBody[models.ContactEducation], error) {
	in.Body.ID = in.Entry
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.UpdateEducation(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactEducation]{Body: &in.Body}, nil
}

func contactsEducationsDelete(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the school entry."`
}) (*emptyBody, error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.DeleteEducation(s, in.Entry)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}

func contactsNotesCreate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body models.ContactNote
}) (*singleBody[models.ContactNote], error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.AddNote(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactNote]{Body: &in.Body}, nil
}

func contactsNotesUpdate(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the note."`
	Body  models.ContactNote
}) (*singleBody[models.ContactNote], error) {
	in.Body.ID = in.Entry
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.UpdateNote(s, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.ContactNote]{Body: &in.Body}, nil
}

func contactsNotesDelete(ctx context.Context, in *struct {
	ID    int64 `path:"id"`
	Entry int64 `path:"entry" doc:"The id of the note."`
}) (*emptyBody, error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.DeleteNote(s, in.Entry)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}

type contactPhotoResponse struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	Body         []byte
}

func contactsPhotoGet(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*contactPhotoResponse, error) {
	photo, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) ([]byte, error) {
		c := &models.Contact{ID: in.ID}
		can, _, err := c.CanRead(s, a)
		if err != nil {
			return nil, err
		}
		if !can {
			return nil, models.ErrContactDoesNotExist{ID: in.ID}
		}
		return c.Photo()
	})
	if err != nil {
		return nil, err
	}
	return &contactPhotoResponse{
		ContentType:  "image/jpeg",
		CacheControl: "private, max-age=300",
		Body:         photo,
	}, nil
}

type contactPhotoUploadInput struct {
	ID      int64 `path:"id"`
	RawBody huma.MultipartFormFiles[struct {
		Photo huma.FormFile `form:"photo" contentType:"image/png,image/jpeg,image/gif,image/tiff,image/bmp,application/octet-stream" required:"true" doc:"The photo. Must be a decodable raster image (PNG, JPEG, GIF, TIFF or BMP) of at most 8 MB."`
	}]
}

func contactsPhotoUpload(ctx context.Context, in *contactPhotoUploadInput) (*singleBody[models.Contact], error) {
	src := in.RawBody.Data().Photo
	defer func() { _ = src.Close() }()
	c, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (*models.Contact, error) {
		c, err := updatableContact(s, a, in.ID)
		if err != nil {
			return nil, err
		}
		if err := c.SetPhoto(s, a, src, true); err != nil {
			return nil, err
		}
		stored, err := models.LoadOwnedContact(s, a, in.ID)
		if err != nil {
			return nil, err
		}
		return stored, stored.ReadOne(s, a)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.Contact]{Body: c}, nil
}

func contactsPhotoDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	err := contactEntryTx(ctx, in.ID, func(s *xorm.Session, c *models.Contact) error {
		return c.RemovePhoto(s)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}
