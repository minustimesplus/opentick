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
	"code.vikunja.io/api/pkg/user"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterUserStatisticsRoutes wires the achievement statistics onto the Huma API.
func RegisterUserStatisticsRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "user-statistics",
		Summary:     "Get task statistics",
		Description: "Returns the authenticated user's completion statistics: completions per day, week and month (in the user's time zone and with weeks starting on their week start day, unless `timezone` / `week_start` override them), the on-time completion rate, completions per project and an achievement score with its level.\n\n" +
			"Only tasks the user created or is assigned to are counted, across every project they can access, archived ones included.\n\n" +
			"Score: +10 per completed task, +5 more if it was completed on or before its due date, -5 if it was completed after its due date, -2 per open task past its due date; never below 0. Reaching level n takes 50·n·(n−1)/2 points (level 2 at 50, level 3 at 150, level 4 at 300, …).\n\n" +
			"Not available to link shares.",
		Method: http.MethodGet,
		Path:   "/user/statistics",
		Tags:   []string{"user"},
	}, userStatistics)
}

func init() { AddRouteRegistrar(RegisterUserStatisticsRoutes) }

func userStatistics(ctx context.Context, in *struct {
	Days      int    `query:"days" minimum:"0" maximum:"366" doc:"Days in the daily series, ending today. Defaults to 30."`
	Weeks     int    `query:"weeks" minimum:"0" maximum:"104" doc:"Weeks in the weekly series, ending this week. Defaults to 12."`
	Months    int    `query:"months" minimum:"0" maximum:"60" doc:"Months in the monthly series, ending this month. Defaults to 12."`
	Timezone  string `query:"timezone" maxLength:"64" doc:"IANA time zone to compute the periods in, e.g. Europe/Berlin. Defaults to the user's time zone setting."`
	WeekStart int    `query:"week_start" minimum:"-1" maximum:"6" default:"-1" doc:"Day weeks start on, 0 = Sunday to 6 = Saturday. Defaults to the user's week start setting."`
}) (*singleBody[models.UserStatistics], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	doer, err := user.GetFromAuth(a)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	opts := models.UserStatisticsOptions{
		Days:   in.Days,
		Weeks:  in.Weeks,
		Months: in.Months,
	}
	if in.Timezone != "" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			return nil, huma.Error422UnprocessableEntity("unknown time zone " + in.Timezone)
		}
		opts.Timezone = in.Timezone
	}
	if in.WeekStart >= 0 {
		weekStart := time.Weekday(in.WeekStart)
		opts.WeekStart = &weekStart
	}

	stats, err := models.GetUserStatistics(s, doer, time.Now(), opts)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.UserStatistics]{Body: stats}, nil
}
