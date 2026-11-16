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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterWeightRoutes wires the body weight log onto the Huma API.
func RegisterWeightRoutes(api huma.API) {
	tags := []string{"weights"}

	Register(api, huma.Operation{
		OperationID: "weights-list",
		Summary:     "List weights",
		Description: "Returns the authenticated user's logged body weights between from and to, both inclusive, oldest first. " +
			"Without from and to, returns about the last year. The range may span at most 3660 days. Weights are private to their user.",
		Method: http.MethodGet,
		Path:   "/weights",
		Tags:   tags,
	}, weightsList)

	Register(api, huma.Operation{
		OperationID: "weights-set",
		Summary:     "Log a weight",
		Description: "Sets the body weight of a day in kilograms, replacing that day's earlier value; it is rounded to 0.1 kg. " +
			"Also checks off the user's active weigh-in habits (tracks_weight) for that day. Days more than one day ahead of UTC are refused.",
		Method: http.MethodPut,
		Path:   "/weights/{date}",
		Tags:   tags,
	}, weightsSet)

	Register(api, huma.Operation{
		OperationID: "weights-delete",
		Summary:     "Remove a weight",
		Description: "Removes the weight of a day. Succeeds even if there was none. Habit check-ins are kept.",
		Method:      http.MethodDelete,
		Path:        "/weights/{date}",
		Tags:        tags,
	}, weightsDelete)
}

func init() { AddRouteRegistrar(RegisterWeightRoutes) }

type weightListBody struct {
	Body []*models.Weight
}

func weightsList(ctx context.Context, in *struct {
	From string `query:"from" doc:"First day of the range, YYYY-MM-DD. Defaults to 364 days before to."`
	To   string `query:"to" doc:"Last day of the range (inclusive), YYYY-MM-DD. Defaults to tomorrow in UTC, which covers today in every timezone."`
}) (*weightListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	s := db.NewSession()
	defer s.Close()

	var to time.Time
	if in.To != "" {
		if to, err = models.ParseHabitDate(in.To); err != nil {
			return nil, translateDomainError(err)
		}
	} else {
		now := time.Now().UTC()
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	}
	from := to.AddDate(0, 0, -364)
	if in.From != "" {
		if from, err = models.ParseHabitDate(in.From); err != nil {
			return nil, translateDomainError(err)
		}
	}

	weights, err := models.GetWeights(s, a, from, to)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &weightListBody{Body: weights}, nil
}

type weightInput struct {
	Weight float64 `json:"weight" minimum:"1" maximum:"1000" doc:"The body weight in kilograms."`
}

func weightsSet(ctx context.Context, in *struct {
	Date  string `path:"date" doc:"The day of the weigh-in, YYYY-MM-DD."`
	Today string `query:"today" doc:"The day the progress of checked-off weigh-in habits is relative to, YYYY-MM-DD — normally the client's local date. Defaults to today in the user's timezone setting."`
	Body  weightInput
}) (*singleBody[models.Weight], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	day, err := models.ParseHabitDate(in.Date)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	w, err := models.SetWeight(s, a, day, in.Body.Weight, in.Today)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Weight]{Body: w}, nil
}

func weightsDelete(ctx context.Context, in *struct {
	Date string `path:"date" doc:"The day to remove the weight of, YYYY-MM-DD."`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	day, err := models.ParseHabitDate(in.Date)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	if err := models.DeleteWeight(s, a, day); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
