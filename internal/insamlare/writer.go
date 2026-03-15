// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const insertTsDataJSONSQL = `
INSERT INTO tsdata(ts_uuid, value, ts, created_by)
SELECT $1::uuid, x.v, x.ts, $2::uuid
FROM json_to_recordset($3::json) AS x("v" double precision, "ts" timestamptz)
ON CONFLICT (ts_uuid, ts) DO NOTHING
`

type WriterConfig struct {
	CreatedByUUID string
	BatchSize     int
	FlushInterval time.Duration
	QueueSize     int
	OnError       func(error)
}

type writerPoint struct {
	Value float64   `json:"v"`
	TS    time.Time `json:"ts"`
}

type Writer struct {
	db            *sql.DB
	createdByUUID uuid.UUID
	batchSize     int
	flushInterval time.Duration
	queue         chan Point
	enqueued      atomic.Uint64
	inserted      atomic.Uint64
	flushes       atomic.Uint64
	failedPoints  atomic.Uint64
	failedFlushes atomic.Uint64
	onError       func(error)
}

type WriterStats struct {
	QueueLen       int
	QueueCap       int
	EnqueuedPoints uint64
	InsertedPoints uint64
	Flushes        uint64
	FailedPoints   uint64
	FailedFlushes  uint64
}

func NewWriter(db *sql.DB, cfg WriterConfig) (*Writer, error) {
	createdByUUID, err := uuid.Parse(cfg.CreatedByUUID)
	if err != nil {
		return nil, fmt.Errorf("parse created_by_uuid: %w", err)
	}

	return &Writer{
		db:            db,
		createdByUUID: createdByUUID,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		queue:         make(chan Point, cfg.QueueSize),
		onError:       cfg.OnError,
	}, nil
}

func (w *Writer) Enqueue(ctx context.Context, points []Point) error {
	for _, point := range points {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case w.queue <- point:
			w.enqueued.Add(1)
		}
	}
	return nil
}

func (w *Writer) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	buffer := make(map[uuid.UUID][]writerPoint)
	total := 0

	flush := func() error {
		if total == 0 {
			return nil
		}
		if err := w.flush(ctx, buffer); err != nil {
			if w.onError != nil {
				w.onError(err)
			}
			clear(buffer)
			total = 0
			return nil
		}
		clear(buffer)
		total = 0
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return flush()
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		case point := <-w.queue:
			buffer[point.TimeseriesUUID] = append(buffer[point.TimeseriesUUID], writerPoint{
				Value: point.Value,
				TS:    point.Timestamp.UTC(),
			})
			total++
			if total >= w.batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}

func (w *Writer) flush(ctx context.Context, grouped map[uuid.UUID][]writerPoint) error {
	for seriesID, points := range grouped {
		if len(points) == 0 {
			continue
		}
		payload, err := json.Marshal(points)
		if err != nil {
			w.failedPoints.Add(uint64(len(points)))
			w.failedFlushes.Add(1)
			return fmt.Errorf("marshal points for %s: %w", seriesID, err)
		}
		result, err := w.db.ExecContext(ctx, insertTsDataJSONSQL, seriesID, w.createdByUUID, payload)
		if err != nil {
			w.failedPoints.Add(uint64(len(points)))
			w.failedFlushes.Add(1)
			return fmt.Errorf("insert tsdata for %s: %w", seriesID, err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read inserted row count for %s: %w", seriesID, err)
		}
		w.inserted.Add(uint64(inserted))
	}
	w.flushes.Add(1)
	return nil
}

func (w *Writer) Stats() WriterStats {
	return WriterStats{
		QueueLen:       len(w.queue),
		QueueCap:       cap(w.queue),
		EnqueuedPoints: w.enqueued.Load(),
		InsertedPoints: w.inserted.Load(),
		Flushes:        w.flushes.Load(),
		FailedPoints:   w.failedPoints.Load(),
		FailedFlushes:  w.failedFlushes.Load(),
	}
}

func clear[K comparable, V any](m map[K]V) {
	for k := range m {
		delete(m, k)
	}
}
