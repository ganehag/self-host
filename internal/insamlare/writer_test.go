// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ory/dockertest/v3"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func TestWriterFlushesBufferedPoints(t *testing.T) {
	t.Parallel()

	db, cleanup := openWriterTestDB(t)
	defer cleanup()

	writer, err := NewWriter(db, WriterConfig{
		CreatedByUUID: "00000000-0000-1000-8000-000000000000",
		BatchSize:     4,
		FlushInterval: 25 * time.Millisecond,
		QueueSize:     16,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- writer.Run(ctx)
	}()

	seriesID := seedTimeseries(t, db, "writer-series")
	points := []Point{
		{TimeseriesUUID: seriesID, Timestamp: time.Unix(1710000000, 0).UTC(), Value: 1.5},
		{TimeseriesUUID: seriesID, Timestamp: time.Unix(1710000060, 0).UTC(), Value: 2.5},
	}
	if err := writer.Enqueue(ctx, points); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil && err != context.Canceled {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tsdata WHERE ts_uuid = $1`, seriesID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}

func TestWriterContinuesAfterInsertError(t *testing.T) {
	t.Parallel()

	db, cleanup := openWriterTestDB(t)
	defer cleanup()

	var errorCount atomic.Uint64
	writer, err := NewWriter(db, WriterConfig{
		CreatedByUUID: "00000000-0000-1000-8000-000000000000",
		BatchSize:     2,
		FlushInterval: 25 * time.Millisecond,
		QueueSize:     16,
		OnError: func(err error) {
			errorCount.Add(1)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- writer.Run(ctx)
	}()

	validSeriesID := seedTimeseries(t, db, "writer-valid-series")
	invalidSeriesID := uuid.MustParse("47daa6eb-bd1c-49de-9782-1e9422a206f5")

	if err := writer.Enqueue(ctx, []Point{
		{TimeseriesUUID: invalidSeriesID, Timestamp: time.Unix(1710000000, 0).UTC(), Value: 1.5},
	}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := writer.Enqueue(ctx, []Point{
		{TimeseriesUUID: validSeriesID, Timestamp: time.Unix(1710000060, 0).UTC(), Value: 2.5},
	}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil && err != context.Canceled {
		t.Fatal(err)
	}

	if errorCount.Load() == 0 {
		t.Fatal("expected insert error callback to be invoked")
	}

	stats := writer.Stats()
	if stats.FailedFlushes == 0 || stats.FailedPoints == 0 {
		t.Fatal("expected failed writer counters to increase")
	}
	if stats.InsertedPoints == 0 {
		t.Fatal("expected writer to continue and insert subsequent valid points")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tsdata WHERE ts_uuid = $1`, validSeriesID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 valid inserted row, got %d", count)
	}
}

func TestWriterIgnoresDuplicatePoints(t *testing.T) {
	t.Parallel()

	db, cleanup := openWriterTestDB(t)
	defer cleanup()

	var errorCount atomic.Uint64
	writer, err := NewWriter(db, WriterConfig{
		CreatedByUUID: "00000000-0000-1000-8000-000000000000",
		BatchSize:     4,
		FlushInterval: 25 * time.Millisecond,
		QueueSize:     16,
		OnError: func(err error) {
			errorCount.Add(1)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- writer.Run(ctx)
	}()

	seriesID := seedTimeseries(t, db, "writer-duplicate-series")
	ts := time.Unix(1710000000, 0).UTC()
	points := []Point{
		{TimeseriesUUID: seriesID, Timestamp: ts, Value: 1.5},
		{TimeseriesUUID: seriesID, Timestamp: ts, Value: 1.5},
	}
	if err := writer.Enqueue(ctx, points); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil && err != context.Canceled {
		t.Fatal(err)
	}

	if errorCount.Load() != 0 {
		t.Fatal("expected duplicate points to be ignored without invoking the error callback")
	}

	stats := writer.Stats()
	if stats.FailedFlushes != 0 || stats.FailedPoints != 0 {
		t.Fatal("expected duplicate points to avoid failed writer counters")
	}
	if stats.InsertedPoints != 1 {
		t.Fatalf("expected inserted point count to reflect only one row, got %d", stats.InsertedPoints)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tsdata WHERE ts_uuid = $1`, seriesID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 inserted row after duplicate input, got %d", count)
	}
}

func openWriterTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}

	resource, err := pool.Run("postgres", "12", []string{"POSTGRES_PASSWORD=secret"})
	if err != nil {
		t.Fatal(err)
	}
	resource.Expire(60)

	var rootDB *sql.DB
	if err := pool.Retry(func() error {
		rootDB, err = sql.Open("pgx", fmt.Sprintf("postgres://postgres:secret@localhost:%s/postgres?sslmode=disable", resource.GetPort("5432/tcp")))
		if err != nil {
			return err
		}
		return rootDB.Ping()
	}); err != nil {
		t.Fatal(err)
	}

	dbName := "insamlare_writer_test"
	if _, err := rootDB.Exec(`CREATE DATABASE ` + dbName); err != nil {
		t.Fatal(err)
	}
	_ = rootDB.Close()

	pgURL := fmt.Sprintf("postgres://postgres:secret@localhost:%s/%s?sslmode=disable", resource.GetPort("5432/tcp"), dbName)
	db, err := sql.Open("pgx", pgURL)
	if err != nil {
		t.Fatal(err)
	}

	mig, err := migrate.New("file://../../postgres/migrations", pgURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := mig.Up(); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = db.Close()
		if err := pool.Purge(resource); err != nil {
			t.Fatal(err)
		}
	}

	return db, cleanup
}

func seedTimeseries(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()

	seriesID := uuid.New()
	if _, err := db.Exec(`
INSERT INTO timeseries(uuid, name, si_unit, created_by, tags)
VALUES($1, $2, 'C', '00000000-0000-1000-8000-000000000000', ARRAY['insamlare'])
`, seriesID, name); err != nil {
		t.Fatal(err)
	}
	return seriesID
}
