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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type calendarSubscriptionListBody struct {
	Body Paginated[*models.CalendarSubscription]
}

// RegisterCalendarSubscriptionRoutes wires calendar subscription CRUD and
// event expansion onto the Huma API.
func RegisterCalendarSubscriptionRoutes(api huma.API) {
	tags := []string{"calendar-subscriptions"}

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-list",
		Summary:     "List calendar subscriptions",
		Description: "Returns the external calendars the authenticated user subscribed to. Subscriptions are private: only their owner sees them.",
		Method:      http.MethodGet,
		Path:        "/calendar-subscriptions",
		Tags:        tags,
	}, calendarSubscriptionsList)

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-read",
		Summary:     "Get a calendar subscription",
		Description: "Returns a single calendar subscription. Only the owner may read it. Sends an ETag; pass it as If-None-Match on a later read to get a 304 Not Modified.",
		Method:      http.MethodGet,
		Path:        "/calendar-subscriptions/{id}",
		Tags:        tags,
	}, calendarSubscriptionsRead)

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-create",
		Summary:     "Subscribe to a calendar",
		Description: "Subscribes the authenticated user to an external iCalendar (ICS) feed. webcal:// urls are stored as https://. The feed is not fetched until its events are requested.",
		Method:      http.MethodPost,
		Path:        "/calendar-subscriptions",
		Tags:        tags,
	}, calendarSubscriptionsCreate)

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-update",
		Summary:     "Update a calendar subscription",
		Description: "Replaces all of a calendar subscription's fields — only the owner may update it. Use PATCH for a partial update.",
		Method:      http.MethodPut,
		Path:        "/calendar-subscriptions/{id}",
		Tags:        tags,
	}, calendarSubscriptionsUpdate)

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-delete",
		Summary:     "Unsubscribe from a calendar",
		Description: "Deletes a calendar subscription. Only the owner may delete it.",
		Method:      http.MethodDelete,
		Path:        "/calendar-subscriptions/{id}",
		Tags:        tags,
	}, calendarSubscriptionsDelete)

	Register(api, huma.Operation{
		OperationID: "calendar-subscriptions-events",
		Summary:     "Get the events of a subscribed calendar",
		Description: "Fetches the subscribed feed (cached server-side for 15 minutes) and returns the event occurrences overlapping [from, to), " +
			"with recurring events expanded. The range may span at most 366 days. All-day and floating events are placed in `timezone`, " +
			"which defaults to the user's timezone setting. Responds 502 when the feed cannot be fetched or parsed.",
		Method: http.MethodGet,
		Path:   "/calendar-subscriptions/{id}/events",
		Tags:   tags,
	}, calendarSubscriptionsEvents)
}

func init() { AddRouteRegistrar(RegisterCalendarSubscriptionRoutes) }

func calendarSubscriptionsList(ctx context.Context, in *ListParams) (*calendarSubscriptionListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.CalendarSubscription{}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.CalendarSubscription)
	if !ok {
		return nil, fmt.Errorf("calendar subscriptions ReadAll returned unexpected type %T (expected []*models.CalendarSubscription)", result)
	}
	return &calendarSubscriptionListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type calendarSubscriptionReadBody struct {
	models.CalendarSubscription
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this subscription. Always 2 (admin), as only the owner can read it."`
}

func calendarSubscriptionsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[calendarSubscriptionReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	cs := &models.CalendarSubscription{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, cs, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &calendarSubscriptionReadBody{CalendarSubscription: *cs, MaxPermission: models.Permission(maxPermission)}
	return conditionalReadResponse(&in.Params, body, cs.Updated, maxPermission)
}

func calendarSubscriptionsCreate(ctx context.Context, in *struct {
	Body models.CalendarSubscription
}) (*singleBody[models.CalendarSubscription], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.CalendarSubscription]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func calendarSubscriptionsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body calendarSubscriptionReadBody
}) (*singleBody[models.CalendarSubscription], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	cs := &in.Body.CalendarSubscription
	cs.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, cs, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.CalendarSubscription]{Body: cs}, nil
}

func calendarSubscriptionsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.CalendarSubscription{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type calendarSubscriptionEventsBody struct {
	Body *models.CalendarSubscriptionEvents
}

func calendarSubscriptionsEvents(ctx context.Context, in *struct {
	ID       int64     `path:"id"`
	From     time.Time `query:"from" required:"true" doc:"Start of the range (inclusive), RFC 3339."`
	To       time.Time `query:"to" required:"true" doc:"End of the range (exclusive), RFC 3339."`
	Timezone string    `query:"timezone" doc:"IANA timezone (e.g. Europe/Berlin) to place all-day and floating events in. Defaults to the user's timezone setting."`
}) (*calendarSubscriptionEventsBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// The feed download can take seconds; don't hold a db session through it.
	cs, loc, err := loadCalendarSubscriptionForEvents(in.ID, in.Timezone, a)
	if err != nil {
		return nil, err
	}

	events, err := cs.GetEvents(ctx, in.From, in.To, loc)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &calendarSubscriptionEventsBody{Body: events}, nil
}

func loadCalendarSubscriptionForEvents(id int64, timezone string, a web.Auth) (*models.CalendarSubscription, *time.Location, error) {
	var loc *time.Location
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return nil, nil, huma.Error400BadRequest(fmt.Sprintf("unknown timezone %q", timezone))
		}
	}

	s := db.NewSession()
	defer s.Close()

	cs := &models.CalendarSubscription{ID: id}
	can, _, err := cs.CanRead(s, a)
	if err != nil {
		return nil, nil, translateDomainError(err)
	}
	if !can {
		return nil, nil, huma.Error403Forbidden("forbidden")
	}

	if loc == nil {
		loc = time.UTC
		u, err := user.GetUserByID(s, a.GetID())
		if err != nil {
			return nil, nil, translateDomainError(err)
		}
		if userLoc, err := time.LoadLocation(u.Timezone); err == nil && u.Timezone != "" {
			loc = userLoc
		}
	}
	return cs, loc, nil
}
