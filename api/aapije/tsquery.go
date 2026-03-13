// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package aapije

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/internal/services"
	"github.com/self-host/self-host/pkg/util"
	"github.com/spf13/viper"
)

// FindTsdataByQuery query multiple time series for data
func (ra *RestApi) FindTsdataByQuery(w http.ResponseWriter, r *http.Request, p rest.FindTsdataByQueryParams) {
	timezone := "UTC"
	aggregate := ""
	precision := ""

	if time.Time(p.End).Sub(time.Time(p.Start)) > 31622401*time.Second {
		// FIXME: these errors should be declared in one location
		ie.SendHTTPError(w, ie.NewBadRequestError(fmt.Errorf("start to end range exceeds limit")))
		return
	}

	if p.Timezone != nil {
		timezone = string(*p.Timezone)
	}

	if p.Aggregate != nil {
		aggregate = string(*p.Aggregate)
	}

	if p.Precision != nil {
		precision = string(*p.Precision)
	}

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	svc := services.NewTimeseriesService(db)
	policySvc := services.NewPolicyCheckService(db)

	uuids, err := util.StringSliceToUuidSlice([]string(p.Uuids))
	if err != nil {
		ie.SendHTTPError(w, ie.NewBadRequestError(fmt.Errorf("uuids has invalid format")))
		return
	}
	uuids = uniqueUUIDs(uuids)

	maxSeries := viper.GetInt("timeseries_queries.max_series")
	if maxSeries > 0 && len(uuids) > maxSeries {
		ie.SendHTTPError(w, ie.NewBadRequestError(fmt.Errorf("requested %d timeseries, limit is %d", len(uuids), maxSeries)))
		return
	}

	// Generate check rules for access control
	resources := make([]string, 0)
	for _, id := range uuids {
		resources = append(resources, fmt.Sprintf("timeseries/%v/data", id.String()))
	}

	// Ensure that the User has access to all requested items
	ok, err = policySvc.UserHasManyAccessViaToken(r.Context(), []byte(domaintoken.Token), "read", resources)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	} else if ok == false {
		// Access denied to one or more requested resources
		ie.SendHTTPError(w, ie.ErrorForbidden)
		return
	}

	params := services.QueryMultiSourceDataParams{
		Uuids:              uuids,
		Start:              time.Time(p.Start),
		End:                time.Time(p.End),
		GreaterOrEq:        (*float32)(p.Ge),
		LessOrEq:           (*float32)(p.Le),
		Aggregate:          aggregate,
		Precision:          precision,
		Timezone:           timezone,
		UseRollups:         viper.GetBool("timeseries_rollups.enabled"),
		MaxPointsPerSeries: viper.GetInt("timeseries_queries.max_points_per_series"),
		MaxTotalPoints:     viper.GetInt("timeseries_queries.max_total_points"),
	}

	data, err := svc.QueryMultiSourceData(r.Context(), params)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	if len(data) < uniqueUUIDCount(uuids) {
		ok, err = svc.ExistAll(r.Context(), uuids)
		if err != nil {
			ie.SendHTTPError(w, ie.ParseDBError(err))
			return
		} else if ok == false {
			ie.SendHTTPError(w, ie.ErrorNotFound)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
	return
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func uniqueUUIDCount(ids []uuid.UUID) int {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return len(seen)
}
