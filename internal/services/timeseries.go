// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"

	units "github.com/ganehag/go-units"
)

const insertTimeseriesBatchSize = 256

// NewTimeseries defines model for NewTimeseries.
type NewTimeseriesParams struct {
	CreatedBy  uuid.UUID
	ThingUuid  uuid.UUID
	Name       string
	SiUnit     string
	Tags       []string
	LowerBound sql.NullFloat64
	UpperBound sql.NullFloat64
}

func inValidRange(v float32, leLimit, geLimit *float32) bool {
	// leLimit: less or equal to (<=) this value
	// geLimit: greater or equal to (>=) this value
	//
	// When geLimit is more than leLimit, we have a range outside of the window
	// ge > le: ge <= x OR x <= le
	//
	// When leLimit is more than geLimit, we have a range inside of the window
	// le >= ge: ge <= x <= le

	if leLimit != nil && geLimit != nil {
		if *leLimit >= *geLimit {
			// Inside of the window
			if v >= *geLimit && v <= *leLimit {
				return true
			}
		} else {
			// Outside of the window
			if v >= *geLimit || v <= *leLimit {
				return true
			}
		}
	} else if leLimit != nil && v <= *leLimit {
		return true
	} else if geLimit != nil && v >= *geLimit {
		return true
	} else if leLimit == nil && geLimit == nil {
		return true
	}

	return false
}

// User represents the repository used for interacting with User records.
type TimeseriesService struct {
	q  *postgres.Queries
	db *sql.DB
}

// NewUser instantiates the User repository.
func NewTimeseriesService(db *sql.DB) *TimeseriesService {
	if db == nil {
		return nil
	}

	return &TimeseriesService{
		q:  postgres.New(db),
		db: db,
	}
}

func (svc *TimeseriesService) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := svc.q.ExistsTimeseries(ctx, id)
	if err != nil {
		return false, err
	}

	return found > 0, nil
}

func (svc *TimeseriesService) ExistAll(ctx context.Context, ids []uuid.UUID) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}

	count, err := svc.q.CountExistingTimeseries(ctx, ids)
	if err != nil {
		return false, err
	}

	return count == int64(len(ids)), nil
}

func (svc *TimeseriesService) AddTimeseries(ctx context.Context, opt *NewTimeseriesParams) (*rest.Timeseries, error) {
	// Use a transaction for this action
	tx, err := svc.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}

	q := svc.q.WithTx(tx)

	tags := make([]string, 0)
	if opt.Tags != nil {
		for _, tag := range opt.Tags {
			tags = append(tags, tag)
		}
	}

	params := postgres.CreateTimeseriesParams{
		CreatedBy:  nullableUUIDValue(opt.CreatedBy),
		ThingUuid:  opt.ThingUuid,
		Name:       opt.Name,
		SiUnit:     opt.SiUnit,
		LowerBound: opt.LowerBound,
		UpperBound: opt.UpperBound,
		Tags:       tags,
	}

	timeseries, err := q.CreateTimeseries(ctx, params)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()

	var lb *float64
	var ub *float64

	if timeseries.LowerBound.Valid {
		lb = &timeseries.LowerBound.Float64
	}

	if timeseries.UpperBound.Valid {
		ub = &timeseries.UpperBound.Float64
	}

	t := &rest.Timeseries{
		Uuid:       timeseries.Uuid.String(),
		CreatedBy:  nullableUUIDString(timeseries.CreatedBy),
		Name:       timeseries.Name,
		SiUnit:     timeseries.SiUnit,
		LowerBound: lb,
		UpperBound: ub,
		Tags:       timeseries.Tags,
	}

	if timeseries.ThingUuid.Valid {
		v := timeseries.ThingUuid.UUID.String()
		t.ThingUuid = &v
	}

	return t, nil
}

type AddDataToTimeseriesParams struct {
	Uuid      uuid.UUID
	Points    []DataPoint
	CreatedBy uuid.UUID
	Unit      *string
}

func (svc *TimeseriesService) AddDataToTimeseries(ctx context.Context, p AddDataToTimeseriesParams) (int64, error) {
	series, err := svc.q.GetTimeseriesByUUID(ctx, p.Uuid)
	if err != nil {
		return 0, err
	}

	filteredPoints := make([]DataPoint, 0, len(p.Points))

	var fromUnit units.Unit
	var toUnit units.Unit

	if p.Unit != nil {
		tsUnit, err := svc.q.GetUnitFromTimeseries(ctx, p.Uuid)
		if err != nil {
			return 0, err
		}

		if tsUnit == *p.Unit {
			p.Unit = nil
		} else {

			fromUnit, err = units.Find(*p.Unit)
			if err != nil {
				return 0, ie.ErrorInvalidUnit
			}

			toUnit, err = units.Find(tsUnit)
			if err != nil {
				// This should never error out, as there should be no incompatible units in the DB
				return 0, ie.ErrorInvalidUnit
			}
		}
	}

	for _, item := range p.Points {
		// Do not use a pointer to the item variable as this is a known gotcha.
		pItem := item

		if p.Unit != nil {
			v := units.NewValue(pItem.Value, fromUnit)
			conv, err := v.Convert(toUnit)
			if err != nil {
				return 0, ie.ErrorInvalidUnitConversion
			}
			pItem.Value = float64(conv.Float())
		}

		if series.LowerBound.Valid {
			// Should we skip this value
			if pItem.Value < series.LowerBound.Float64 {
				continue
			}
		}
		if series.UpperBound.Valid {
			// Should we skip this value
			if pItem.Value > series.UpperBound.Float64 {
				continue
			}
		}

		filteredPoints = append(filteredPoints, pItem)
	}

	if len(filteredPoints) == 0 {
		return 0, nil
	}

	var totalCount int64
	for start := 0; start < len(filteredPoints); start += insertTimeseriesBatchSize {
		stop := start + insertTimeseriesBatchSize
		if stop > len(filteredPoints) {
			stop = len(filteredPoints)
		}

		count, err := insertTimeseriesBatch(ctx, svc.db, p.Uuid, p.CreatedBy, filteredPoints[start:stop])
		if err != nil {
			return 0, err
		}
		totalCount += count
	}

	return totalCount, nil
}

func (svc *TimeseriesService) FindByTags(ctx context.Context, p FindByTagsParams) ([]*rest.Timeseries, error) {
	timeseries := make([]*rest.Timeseries, 0)

	params := postgres.FindTimeseriesByTagsParams{
		Tags:  p.Tags,
		Token: p.Token,
	}
	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	tsList, err := svc.q.FindTimeseriesByTags(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, item := range tsList {
		var lBound *float64
		var uBound *float64

		if item.LowerBound.Valid {
			lBound = &item.LowerBound.Float64
		}
		if item.UpperBound.Valid {
			uBound = &item.UpperBound.Float64
		}

		t := &rest.Timeseries{
			CreatedBy:  nullableUUIDString(item.CreatedBy),
			LowerBound: lBound,
			Name:       item.Name,
			SiUnit:     item.SiUnit,
			Tags:       item.Tags,
			UpperBound: uBound,
			Uuid:       item.Uuid.String(),
		}

		if item.ThingUuid.Valid {
			v := item.ThingUuid.UUID.String()
			t.ThingUuid = &v
		}

		timeseries = append(timeseries, t)
	}

	return timeseries, nil
}

func (svc *TimeseriesService) FindByThing(ctx context.Context, thing uuid.UUID) ([]*rest.Timeseries, error) {
	timeseries := make([]*rest.Timeseries, 0)

	tsList, err := svc.q.FindTimeseriesByThing(ctx, nullableUUIDValue(thing))
	if err != nil {
		return nil, err
	}

	if len(tsList) == 0 {
		count, err := svc.q.ExistsThing(ctx, thing)
		if err != nil {
			return nil, err
		} else if count == 0 {
			return nil, ie.ErrorNotFound
		}
	}

	for _, item := range tsList {
		var lBound *float64
		var uBound *float64

		if item.LowerBound.Valid {
			lBound = &item.LowerBound.Float64
		}
		if item.UpperBound.Valid {
			uBound = &item.UpperBound.Float64
		}

		t := &rest.Timeseries{
			CreatedBy:  nullableUUIDString(item.CreatedBy),
			LowerBound: lBound,
			Name:       item.Name,
			SiUnit:     item.SiUnit,
			Tags:       item.Tags,
			UpperBound: uBound,
			Uuid:       item.Uuid.String(),
		}

		if item.ThingUuid.Valid {
			v := item.ThingUuid.UUID.String()
			t.ThingUuid = &v
		}

		timeseries = append(timeseries, t)
	}

	return timeseries, nil
}

func (svc *TimeseriesService) FindByUuid(ctx context.Context, id uuid.UUID) (*rest.Timeseries, error) {
	t, err := svc.q.FindTimeseriesByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	var lBound *float64
	var uBound *float64

	if t.LowerBound.Valid {
		lBound = &t.LowerBound.Float64
	}
	if t.UpperBound.Valid {
		uBound = &t.UpperBound.Float64
	}

	timeseries := &rest.Timeseries{
		Uuid:       t.Uuid.String(),
		Name:       t.Name,
		SiUnit:     t.SiUnit,
		Tags:       t.Tags,
		LowerBound: lBound,
		UpperBound: uBound,
		CreatedBy:  nullableUUIDString(t.CreatedBy),
	}

	if t.ThingUuid.Valid {
		v := t.ThingUuid.UUID.String()
		timeseries.ThingUuid = &v
	}

	return timeseries, nil
}

func (svc *TimeseriesService) FindAll(ctx context.Context, p FindAllParams) ([]*rest.Timeseries, error) {
	timeseries := make([]*rest.Timeseries, 0)

	params := postgres.FindTimeseriesParams{
		Token: p.Token,
	}
	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	tsList, err := svc.q.FindTimeseries(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, item := range tsList {
		var lBound *float64
		var uBound *float64

		if item.LowerBound.Valid {
			lBound = &item.LowerBound.Float64
		}
		if item.UpperBound.Valid {
			uBound = &item.UpperBound.Float64
		}

		t := &rest.Timeseries{
			Uuid:       item.Uuid.String(),
			Name:       item.Name,
			SiUnit:     item.SiUnit,
			UpperBound: uBound,
			LowerBound: lBound,
			Tags:       item.Tags,
			CreatedBy:  nullableUUIDString(item.CreatedBy),
		}

		if item.ThingUuid.Valid {
			v := item.ThingUuid.UUID.String()
			t.ThingUuid = &v
		}

		timeseries = append(timeseries, t)
	}

	return timeseries, nil
}

type QuerySingleSourceDataParams struct {
	Uuid        uuid.UUID
	Start       time.Time
	End         time.Time
	GreaterOrEq *float32
	LessOrEq    *float32
	Unit        *string
	Aggregate   string
	Precision   string
	Timezone    string
}

func (svc *TimeseriesService) QuerySingleSourceData(ctx context.Context, p QuerySingleSourceDataParams) ([]*rest.TsRow, error) {
	tsdata := make([]*rest.TsRow, 0)

	var fromUnit units.Unit
	var toUnit units.Unit

	if p.Unit != nil {
		tsUnit, err := svc.q.GetUnitFromTimeseries(ctx, p.Uuid)
		if err != nil {
			return nil, err
		}

		if tsUnit == *p.Unit {
			p.Unit = nil
		} else {

			toUnit, err = units.Find(*p.Unit)
			if err != nil {
				return nil, ie.ErrorInvalidUnit
			}

			fromUnit, err = units.Find(tsUnit)
			if err != nil {
				// This should never error out, as there should be no incompatible units in the DB
				return nil, ie.ErrorInvalidUnit
			}
		}
	}

	tzloc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return nil, err
	}

	if shouldAggregateData(p.Aggregate, p.Precision) {
		dataList, err := svc.q.GetTsDataRange(ctx, postgres.GetTsDataRangeParams{
			TsUuids: []uuid.UUID{p.Uuid},
			Start:   p.Start,
			Stop:    p.End,
		})
		if err != nil {
			return nil, err
		}
		result, err := aggregateSingleSourceRows(dataList, p, tzloc, fromUnit, toUnit)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			if _, err := svc.q.FindTimeseriesByUUID(ctx, p.Uuid); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	dataList, err := svc.q.GetTsDataRange(ctx, postgres.GetTsDataRangeParams{
		TsUuids: []uuid.UUID{p.Uuid},
		Start:   p.Start,
		Stop:    p.End,
	})
	if err != nil {
		return nil, err
	}

	for _, item := range dataList {
		var f float32
		if p.Unit != nil {
			v := units.NewValue(item.Value, fromUnit)
			conv, err := v.Convert(toUnit)
			if err != nil {
				return nil, ie.ErrorInvalidUnitConversion
			}
			f = float32(conv.Float())
		} else {
			f = float32(item.Value)
		}

		if inValidRange(f, p.LessOrEq, p.GreaterOrEq) == false {
			continue
		}

		d := rest.TsRow{
			V:  f,
			Ts: item.Ts.In(tzloc),
		}
		tsdata = append(tsdata, &d)
	}

	if len(tsdata) == 0 {
		if _, err := svc.q.FindTimeseriesByUUID(ctx, p.Uuid); err != nil {
			return nil, err
		}
	}

	return tsdata, nil
}

type QueryMultiSourceDataParams struct {
	Uuids       []uuid.UUID
	Start       time.Time
	End         time.Time
	GreaterOrEq *float32
	LessOrEq    *float32
	Aggregate   string
	Precision   string
	Timezone    string
}

func (svc *TimeseriesService) QueryMultiSourceData(ctx context.Context, p QueryMultiSourceDataParams) ([]*rest.TsResults, error) {
	tzloc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return nil, ie.NewInvalidRequestError(err)
	}

	dataList, err := svc.q.GetTsDataRange(ctx, postgres.GetTsDataRangeParams{
		TsUuids: p.Uuids,
		Start:   p.Start,
		Stop:    p.End,
	})
	if err != nil {
		return nil, err
	}

	mapping := make(map[uuid.UUID][]rest.TsRow, len(p.Uuids))
	if shouldAggregateData(p.Aggregate, p.Precision) {
		mapping, err = aggregateMultiSourceRows(dataList, p, tzloc)
		if err != nil {
			return nil, err
		}
	} else {
		appendRawMultiSourceRows(mapping, dataList, p, tzloc)
	}

	tsResult := make([]*rest.TsResults, 0)
	seen := make(map[uuid.UUID]struct{}, len(p.Uuids))
	for _, key := range p.Uuids {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		data, ok := mapping[key]
		if ok == false {
			continue
		}

		tsResult = append(tsResult, &rest.TsResults{
			Uuid: key.String(),
			Data: data,
		})
	}

	return tsResult, nil
}

func shouldAggregateData(aggregate, precision string) bool {
	return strings.TrimSpace(precision) != "" && strings.TrimSpace(aggregate) != ""
}

func insertTimeseriesBatch(ctx context.Context, db *sql.DB, tsUUID, createdBy uuid.UUID, points []DataPoint) (int64, error) {
	var query strings.Builder
	args := make([]any, 0, len(points)*4)

	query.WriteString("INSERT INTO tsdata(ts_uuid, value, ts, created_by) VALUES ")
	for i, point := range points {
		if i > 0 {
			query.WriteString(",")
		}

		base := i*4 + 1
		query.WriteString("(")
		query.WriteString("$")
		query.WriteString(strconv.Itoa(base))
		query.WriteString(",$")
		query.WriteString(strconv.Itoa(base + 1))
		query.WriteString(",$")
		query.WriteString(strconv.Itoa(base + 2))
		query.WriteString(",$")
		query.WriteString(strconv.Itoa(base + 3))
		query.WriteString(")")

		args = append(args, tsUUID, point.Value, point.Timestamp, createdBy)
	}

	result, err := db.ExecContext(ctx, query.String(), args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

type aggregateBucket struct {
	ts    time.Time
	count int64
	sum   float64
	min   float64
	max   float64
}

func aggregateSingleSourceRows(rows []postgres.GetTsDataRangeRow, p QuerySingleSourceDataParams, tzloc *time.Location, fromUnit, toUnit units.Unit) ([]*rest.TsRow, error) {
	buckets := make(map[time.Time]*aggregateBucket, len(rows))
	keys := make([]time.Time, 0, len(rows))

	for _, item := range rows {
		bucketTs, err := truncateTime(item.Ts, p.Precision, tzloc)
		if err != nil {
			return nil, err
		}

		bucket := buckets[bucketTs]
		if bucket == nil {
			bucket = &aggregateBucket{
				ts:  bucketTs,
				min: item.Value,
				max: item.Value,
			}
			buckets[bucketTs] = bucket
			keys = append(keys, bucketTs)
		}

		bucket.count++
		bucket.sum += item.Value
		if item.Value < bucket.min {
			bucket.min = item.Value
		}
		if item.Value > bucket.max {
			bucket.max = item.Value
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Before(keys[j])
	})

	result := make([]*rest.TsRow, 0, len(keys))
	for _, key := range keys {
		bucket := buckets[key]
		value, err := aggregateBucketValue(bucket, p.Aggregate)
		if err != nil {
			return nil, err
		}

		var f float32
		if p.Unit != nil {
			v := units.NewValue(value, fromUnit)
			conv, err := v.Convert(toUnit)
			if err != nil {
				return nil, ie.ErrorInvalidUnitConversion
			}
			f = float32(conv.Float())
		} else {
			f = float32(value)
		}

		if inValidRange(f, p.LessOrEq, p.GreaterOrEq) == false {
			continue
		}

		result = append(result, &rest.TsRow{
			V:  f,
			Ts: bucket.ts.In(tzloc),
		})
	}

	return result, nil
}

func appendRawMultiSourceRows(mapping map[uuid.UUID][]rest.TsRow, rows []postgres.GetTsDataRangeRow, p QueryMultiSourceDataParams, tzloc *time.Location) {
	for _, item := range rows {
		f := float32(item.Value)

		if inValidRange(f, p.LessOrEq, p.GreaterOrEq) == false {
			continue
		}

		mapping[item.TsUuid] = append(mapping[item.TsUuid], rest.TsRow{
			V:  f,
			Ts: item.Ts.In(tzloc),
		})
	}

	for key := range mapping {
		sort.Slice(mapping[key], func(i, j int) bool {
			return mapping[key][i].Ts.Before(mapping[key][j].Ts)
		})
	}
}

func aggregateMultiSourceRows(rows []postgres.GetTsDataRangeRow, p QueryMultiSourceDataParams, tzloc *time.Location) (map[uuid.UUID][]rest.TsRow, error) {
	mapping := make(map[uuid.UUID][]rest.TsRow, len(p.Uuids))
	if len(rows) == 0 {
		return mapping, nil
	}

	bucketsBySeries := make(map[uuid.UUID]map[time.Time]*aggregateBucket, len(p.Uuids))
	keysBySeries := make(map[uuid.UUID][]time.Time, len(p.Uuids))

	for _, item := range rows {
		bucketTs, err := truncateTime(item.Ts, p.Precision, tzloc)
		if err != nil {
			return nil, err
		}

		seriesBuckets := bucketsBySeries[item.TsUuid]
		if seriesBuckets == nil {
			seriesBuckets = make(map[time.Time]*aggregateBucket)
			bucketsBySeries[item.TsUuid] = seriesBuckets
		}

		bucket := seriesBuckets[bucketTs]
		if bucket == nil {
			bucket = &aggregateBucket{
				ts:  bucketTs,
				min: item.Value,
				max: item.Value,
			}
			seriesBuckets[bucketTs] = bucket
			keysBySeries[item.TsUuid] = append(keysBySeries[item.TsUuid], bucketTs)
		}

		bucket.count++
		bucket.sum += item.Value
		if item.Value < bucket.min {
			bucket.min = item.Value
		}
		if item.Value > bucket.max {
			bucket.max = item.Value
		}
	}

	for seriesID, keys := range keysBySeries {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Before(keys[j])
		})

		rows := make([]rest.TsRow, 0, len(keys))
		for _, key := range keys {
			value, err := aggregateBucketValue(bucketsBySeries[seriesID][key], p.Aggregate)
			if err != nil {
				return nil, err
			}

			f := float32(value)
			if inValidRange(f, p.LessOrEq, p.GreaterOrEq) == false {
				continue
			}

			rows = append(rows, rest.TsRow{
				V:  f,
				Ts: key.In(tzloc),
			})
		}

		if len(rows) > 0 {
			mapping[seriesID] = rows
		}
	}

	return mapping, nil
}

func aggregateBucketValue(bucket *aggregateBucket, aggregate string) (float64, error) {
	switch aggregate {
	case "avg":
		if bucket.count == 0 {
			return 0, nil
		}
		return bucket.sum / float64(bucket.count), nil
	case "min":
		return bucket.min, nil
	case "max":
		return bucket.max, nil
	case "count":
		return float64(bucket.count), nil
	case "sum":
		return bucket.sum, nil
	default:
		return 0, ie.NewInvalidRequestError(fmt.Errorf("unsupported aggregate %q", aggregate))
	}
}

func truncateTime(ts time.Time, precision string, loc *time.Location) (time.Time, error) {
	local := ts.In(loc)

	switch precision {
	case "microseconds":
		return local.Truncate(time.Microsecond), nil
	case "milliseconds":
		return local.Truncate(time.Millisecond), nil
	case "second":
		return local.Truncate(time.Second), nil
	case "minute":
		return local.Truncate(time.Minute), nil
	case "minute5":
		return truncateByMinuteStep(local, 5), nil
	case "minute10":
		return truncateByMinuteStep(local, 10), nil
	case "minute15":
		return truncateByMinuteStep(local, 15), nil
	case "minute20":
		return truncateByMinuteStep(local, 20), nil
	case "minute30":
		return truncateByMinuteStep(local, 30), nil
	case "hour":
		return local.Truncate(time.Hour), nil
	case "day":
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc), nil
	case "week":
		return truncateWeek(local), nil
	case "month":
		return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc), nil
	case "year":
		return time.Date(local.Year(), time.January, 1, 0, 0, 0, 0, loc), nil
	case "decade":
		return time.Date((local.Year()/10)*10, time.January, 1, 0, 0, 0, 0, loc), nil
	case "century":
		return time.Date(((local.Year()-1)/100)*100+1, time.January, 1, 0, 0, 0, 0, loc), nil
	case "millennia":
		return time.Date(((local.Year()-1)/1000)*1000+1, time.January, 1, 0, 0, 0, 0, loc), nil
	default:
		return time.Time{}, ie.NewInvalidRequestError(fmt.Errorf("unsupported precision %q", precision))
	}
}

func truncateByMinuteStep(ts time.Time, step int) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), (ts.Minute()/step)*step, 0, 0, ts.Location())
}

func truncateWeek(ts time.Time) time.Time {
	weekday := int(ts.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := ts.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, ts.Location())
}

type UpdateTimeseriesParams struct {
	Uuid       uuid.UUID
	ThingUuid  *uuid.UUID
	LowerBound *sql.NullFloat64
	UpperBound *sql.NullFloat64
	Name       *string
	SiUnit     *string
	Tags       *[]string
}

func (svc *TimeseriesService) UpdateTimeseries(ctx context.Context, p UpdateTimeseriesParams) (int64, error) {
	// Use a transaction for this action
	tx, err := svc.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}

	var count int64

	q := svc.q.WithTx(tx)

	if p.Name != nil {
		params := postgres.SetTimeseriesNameParams{
			Uuid: p.Uuid,
			Name: *p.Name,
		}
		c, err := q.SetTimeseriesName(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	if p.SiUnit != nil {
		// FIXME: Check SI unit against Gonum

		params := postgres.SetTimeseriesSiUnitParams{
			Uuid:   p.Uuid,
			SiUnit: *p.SiUnit,
		}
		c, err := q.SetTimeseriesSiUnit(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	if p.ThingUuid != nil {
		params := postgres.SetTimeseriesThingParams{
			Uuid:      p.Uuid,
			ThingUuid: nullableUUID(p.ThingUuid),
		}
		c, err := q.SetTimeseriesThing(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	if p.LowerBound != nil {
		params := postgres.SetTimeseriesLowerBoundParams{
			Uuid:       p.Uuid,
			LowerBound: *p.LowerBound,
		}
		c, err := q.SetTimeseriesLowerBound(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	if p.UpperBound != nil {
		params := postgres.SetTimeseriesUpperBoundParams{
			Uuid:       p.Uuid,
			UpperBound: *p.UpperBound,
		}
		c, err := q.SetTimeseriesUpperBound(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	if p.Tags != nil {
		params := postgres.SetTimeseriesTagsParams{
			Uuid: p.Uuid,
			Tags: *p.Tags,
		}
		c, err := q.SetTimeseriesTags(ctx, params)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		count += c
	}

	tx.Commit()

	return count, nil
}

func (svc *TimeseriesService) DeleteTimeseries(ctx context.Context, tsUUID uuid.UUID) (int64, error) {
	count, err := svc.q.DeleteTimeseries(ctx, tsUUID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

type DeleteTsDataParams struct {
	Uuid        uuid.UUID
	Start       time.Time
	End         time.Time
	GreaterOrEq *float32
	LessOrEq    *float32
}

func (svc *TimeseriesService) DeleteTsData(ctx context.Context, p DeleteTsDataParams) (int64, error) {
	// DeleteTsDataRange expects a list of time series
	tsuuids := []uuid.UUID{
		p.Uuid,
	}

	params := postgres.DeleteTsDataRangeParams{
		TsUuids: tsuuids,
		Start:   p.Start,
		Stop:    p.End,
		GeNull:  p.GreaterOrEq == nil,
		LeNull:  p.LessOrEq == nil,
	}

	if p.GreaterOrEq != nil {
		params.Ge = float64(*p.GreaterOrEq)
	}
	if p.LessOrEq != nil {
		params.Le = float64(*p.LessOrEq)
	}

	count, err := svc.q.DeleteTsDataRange(ctx, params)
	if err != nil {
		return 0, err
	}

	return count, nil
}
