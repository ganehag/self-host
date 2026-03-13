// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	ie "github.com/self-host/self-host/internal/errors"
)

// Tests can run in any order, so we need to run everything (Timeseries related) in one function
// as we are required to do things in a certain order since we are not mocking the PostgreSQL data-store.
func TestTimeseriesAll(t *testing.T) {
	svc := NewTimeseriesService(db)

	params := &NewTimeseriesParams{
		Name:      "MyTimeseries",
		CreatedBy: uuid.MustParse("00000000-0000-1000-8000-000000000000"), // UUID for Root user
		Tags:      []string{},
	}

	timeseries, err := svc.AddTimeseries(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}

	tsUUID, err := uuid.Parse(timeseries.Uuid)
	if err != nil {
		log.Fatal(err)
	}

	if timeseries.Name != "MyTimeseries" {
		log.Fatal("Name does not match expected")
	}

	if tsUUID == uuid.MustParse("00000000-0000-0000-0000-000000000000") {
		log.Fatal("UUID of new time series is nil")
	}

	count, err := svc.DeleteTimeseries(context.Background(), tsUUID)
	if err != nil {
		log.Fatal(err)
	} else if count == 0 {
		log.Fatal("Timeseries was not deleted")
	}
}

type RangeRowT struct {
	V   float32
	Le  *float32
	Ge  *float32
	Res bool
}

func Float32P(v float32) *float32 {
	return &v
}

func TestInValidRange(t *testing.T) {
	// value, le, ge
	checks := []RangeRowT{
		{
			V:   10,
			Le:  Float32P(10),
			Ge:  Float32P(10),
			Res: true,
		},
		{
			V:   10,
			Le:  Float32P(100),
			Ge:  Float32P(-100),
			Res: true,
		},
		{
			V:   10,
			Le:  Float32P(-100),
			Ge:  Float32P(100),
			Res: false,
		},
		{
			V:   -150,
			Le:  Float32P(-100),
			Ge:  Float32P(100),
			Res: true,
		},
		{
			V:   150,
			Le:  Float32P(-100),
			Ge:  Float32P(100),
			Res: true,
		},
		{
			V:   -100,
			Le:  Float32P(-100),
			Res: true,
		},
		{
			V:   100,
			Ge:  Float32P(100),
			Res: true,
		},
		{
			V:   101,
			Le:  Float32P(100),
			Res: false,
		},
		{
			V:   99,
			Ge:  Float32P(100),
			Res: false,
		},
	}

	for _, row := range checks {
		if inValidRange(row.V, row.Le, row.Ge) != row.Res {
			log.Fatal("Check failed")
		}
	}
}

func TestSplitDailyRollupRangesUsesUTCBoundaries(t *testing.T) {
	start := time.Date(2024, 1, 1, 23, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	end := time.Date(2024, 1, 4, 2, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))

	rollupRange, rawRanges := splitDailyRollupRanges(start, end)
	if rollupRange == nil {
		t.Fatal("expected daily rollup range")
	}

	if !rollupRange.Start.Equal(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected rollup start: %s", rollupRange.Start)
	}
	if !rollupRange.Stop.Equal(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected rollup stop: %s", rollupRange.Stop)
	}
	if len(rawRanges) != 2 {
		t.Fatalf("expected 2 raw ranges, got %d", len(rawRanges))
	}
	if !rawRanges[0].Stop.Equal(time.Date(2024, 1, 1, 23, 59, 59, int(time.Second-time.Microsecond), time.UTC)) {
		t.Fatalf("unexpected left raw stop: %s", rawRanges[0].Stop)
	}
	if !rawRanges[1].Start.Equal(time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected right raw start: %s", rawRanges[1].Start)
	}
}

func TestQueryMultiSourceDataEnforcesPerSeriesLimitAfterValueFiltering(t *testing.T) {
	ctx := context.Background()
	svc := NewTimeseriesService(db)
	createdBy := uuid.MustParse(rootUserUUID)

	ts1, err := svc.AddTimeseries(ctx, &NewTimeseriesParams{
		Name:      "QueryLimitSeriesOne",
		CreatedBy: createdBy,
		Tags:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts1ID := uuid.MustParse(ts1.Uuid)

	ts2, err := svc.AddTimeseries(ctx, &NewTimeseriesParams{
		Name:      "QueryLimitSeriesTwo",
		CreatedBy: createdBy,
		Tags:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts2ID := uuid.MustParse(ts2.Uuid)

	base := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.AddDataToTimeseries(ctx, AddDataToTimeseriesParams{
		Uuid:      ts1ID,
		CreatedBy: createdBy,
		Points: []DataPoint{
			{Timestamp: base, Value: 1},
			{Timestamp: base.Add(time.Minute), Value: 2},
			{Timestamp: base.Add(2 * time.Minute), Value: 3},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.AddDataToTimeseries(ctx, AddDataToTimeseriesParams{
		Uuid:      ts2ID,
		CreatedBy: createdBy,
		Points: []DataPoint{
			{Timestamp: base, Value: 10},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ge := float32(2)
	results, err := svc.QueryMultiSourceData(ctx, QueryMultiSourceDataParams{
		Uuids:              []uuid.UUID{ts1ID, ts2ID},
		Start:              base.Add(-time.Minute),
		End:                base.Add(3 * time.Minute),
		GreaterOrEq:        &ge,
		Timezone:           "UTC",
		MaxPointsPerSeries: 2,
		MaxTotalPoints:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 series in result, got %d", len(results))
	}
	if len(results[0].Data) != 2 {
		t.Fatalf("expected 2 filtered points for first series, got %d", len(results[0].Data))
	}
}

func TestQueryMultiSourceDataReturns413WhenSeriesExceedsLimit(t *testing.T) {
	ctx := context.Background()
	svc := NewTimeseriesService(db)
	createdBy := uuid.MustParse(rootUserUUID)

	ts, err := svc.AddTimeseries(ctx, &NewTimeseriesParams{
		Name:      "QueryLimitExceededSeries",
		CreatedBy: createdBy,
		Tags:      []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	tsID := uuid.MustParse(ts.Uuid)

	base := time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)
	if _, err := svc.AddDataToTimeseries(ctx, AddDataToTimeseriesParams{
		Uuid:      tsID,
		CreatedBy: createdBy,
		Points: []DataPoint{
			{Timestamp: base, Value: 1},
			{Timestamp: base.Add(time.Minute), Value: 2},
			{Timestamp: base.Add(2 * time.Minute), Value: 3},
		},
	}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.QueryMultiSourceData(ctx, QueryMultiSourceDataParams{
		Uuids:              []uuid.UUID{tsID},
		Start:              base.Add(-time.Minute),
		End:                base.Add(3 * time.Minute),
		Timezone:           "UTC",
		MaxPointsPerSeries: 2,
		MaxTotalPoints:     10,
	})
	if err == nil {
		t.Fatal("expected point limit error")
	}

	httpErr, ok := err.(*ie.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", httpErr.Code)
	}
}
