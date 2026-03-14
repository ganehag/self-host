// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type manifest struct {
	Series []manifestSeries `json:"series" yaml:"series"`
}

type manifestSeries struct {
	UUID string `json:"uuid" yaml:"uuid"`
}

type topicTemplateData struct {
	Index              int
	UUID               string
	Sequence           uint64
	Value              float64
	TimestampUnixNano  int64
	TimestampRFC3339   string
	TimestampUnix      int64
	TimestampUnixMilli int64
}

type publisherCounters struct {
	Attempted atomic.Uint64
	Published atomic.Uint64
	Errors    atomic.Uint64
}

var (
	runBroker           string
	runClientIDPrefix   string
	runUsername         string
	runPassword         string
	runQoS              int
	runWorkers          int
	runDuration         time.Duration
	runMessages         int
	runReportInterval   time.Duration
	runSettle           time.Duration
	runManifest         string
	runSeriesUUIDs      []string
	runSeriesLimit      int
	runPGURI            string
	runTopicTemplate    string
	runPayloadTemplate  string
	runPointsPerMessage int
	runValueKey         string
	runTimestampKey     string
	runTimestampFmt     string
	runValueBase        float64
	runValueDelta       float64
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a sustained MQTT load test against insamlare",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runBroker) == "" {
			return fmt.Errorf("--broker is required")
		}
		if runWorkers <= 0 {
			return fmt.Errorf("--workers must be > 0")
		}
		if runDuration <= 0 && runMessages <= 0 {
			return fmt.Errorf("set --duration or --messages")
		}
		if runReportInterval <= 0 {
			return fmt.Errorf("--report-interval must be > 0")
		}
		if runQoS < 0 || runQoS > 2 {
			return fmt.Errorf("--qos must be 0, 1 or 2")
		}

		seriesIDs, err := loadSeriesUUIDs(runManifest, runSeriesUUIDs, runSeriesLimit)
		if err != nil {
			return err
		}
		if len(seriesIDs) == 0 {
			return fmt.Errorf("no series UUIDs resolved")
		}

		topicTmpl, err := template.New("topic").Parse(runTopicTemplate)
		if err != nil {
			return fmt.Errorf("parse topic template: %w", err)
		}
		var payloadTmpl *template.Template
		if strings.TrimSpace(runPayloadTemplate) != "" {
			payloadTmpl, err = template.New("payload").Parse(runPayloadTemplate)
			if err != nil {
				return fmt.Errorf("parse payload template: %w", err)
			}
		}

		var db *sql.DB
		var beforeCount int64
		if strings.TrimSpace(runPGURI) != "" {
			db, err = sql.Open("pgx", runPGURI)
			if err != nil {
				return fmt.Errorf("open postgres: %w", err)
			}
			defer db.Close()
			beforeCount, err = countExistingPoints(context.Background(), db, seriesIDs)
			if err != nil {
				return fmt.Errorf("count initial tsdata rows: %w", err)
			}
		}

		fmt.Printf("broker:          %s\n", runBroker)
		fmt.Printf("workers:         %d\n", runWorkers)
		fmt.Printf("qos:             %d\n", runQoS)
		if runMessages > 0 {
			fmt.Printf("messages:        %d\n", runMessages)
		}
		if runDuration > 0 {
			fmt.Printf("duration:        %s\n", runDuration)
		}
		fmt.Printf("series:          %d\n", len(seriesIDs))
		fmt.Printf("points/message:  %d\n", runPointsPerMessage)
		fmt.Printf("topic_template:  %s\n", runTopicTemplate)
		if db != nil {
			fmt.Printf("initial_rows:    %d\n", beforeCount)
		}
		fmt.Println()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := time.Now()
		payloadBaseTime := start.UTC()
		var counters publisherCounters
		var sequence atomic.Uint64
		var wg sync.WaitGroup
		errCh := make(chan error, runWorkers)
		workerDone := make(chan struct{})

		for worker := 0; worker < runWorkers; worker++ {
			clientID := fmt.Sprintf("%s-%02d-%d", runClientIDPrefix, worker, time.Now().UnixNano())
			client, err := connectPublisher(runBroker, clientID, runUsername, runPassword)
			if err != nil {
				cancel()
				return err
			}
			defer client.Disconnect(250)

			wg.Add(1)
			go func(client mqtt.Client) {
				defer wg.Done()
				if err := publishWorker(ctx, client, topicTmpl, payloadTmpl, seriesIDs, payloadBaseTime, &sequence, &counters); err != nil {
					errCh <- err
					cancel()
				}
			}(client)
		}
		go func() {
			wg.Wait()
			close(workerDone)
		}()

		timer := (*time.Timer)(nil)
		if runDuration > 0 {
			timer = time.NewTimer(runDuration)
			defer timer.Stop()
		}

		reportTicker := time.NewTicker(runReportInterval)
		defer reportTicker.Stop()

	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case <-reportTicker.C:
				printProgress(start, db, seriesIDs, beforeCount, &counters)
			case err := <-errCh:
				return err
			case <-workerDone:
				cancel()
				break loop
			case <-timerC(timer):
				cancel()
			}
		}

		if runSettle > 0 {
			time.Sleep(runSettle)
		}

		var finalCount int64
		if db != nil {
			finalCount, err = countExistingPoints(context.Background(), db, seriesIDs)
			if err != nil {
				return fmt.Errorf("count final tsdata rows: %w", err)
			}
		}

		elapsed := time.Since(start)
		attempted := counters.Attempted.Load()
		published := counters.Published.Load()
		errors := counters.Errors.Load()

		fmt.Println("summary:")
		fmt.Printf("  elapsed:               %s\n", elapsed.Round(time.Millisecond))
		fmt.Printf("  attempted_messages:    %d\n", attempted)
		fmt.Printf("  published_messages:    %d\n", published)
		fmt.Printf("  publish_errors:        %d\n", errors)
		if elapsed > 0 {
			fmt.Printf("  publish_rate:          %.0f msg/s\n", float64(published)/elapsed.Seconds())
		}
		if db != nil {
			inserted := finalCount - beforeCount
			fmt.Printf("  inserted_rows:         %d\n", inserted)
			if elapsed > 0 {
				fmt.Printf("  insert_rate:           %.0f rows/s\n", float64(inserted)/elapsed.Seconds())
			}
			fmt.Printf("  ingest_gap:            %d\n", int64(published)-inserted)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runBroker, "broker", "tcp://127.0.0.1:1883", "MQTT broker URL")
	runCmd.Flags().StringVar(&runClientIDPrefix, "client-id-prefix", "insamlarebench", "MQTT client ID prefix")
	runCmd.Flags().StringVar(&runUsername, "username", "", "MQTT username")
	runCmd.Flags().StringVar(&runPassword, "password", "", "MQTT password")
	runCmd.Flags().IntVar(&runQoS, "qos", 1, "MQTT QoS level")
	runCmd.Flags().IntVar(&runWorkers, "workers", 4, "number of parallel MQTT publishers")
	runCmd.Flags().DurationVar(&runDuration, "duration", 30*time.Second, "publish duration")
	runCmd.Flags().IntVar(&runMessages, "messages", 0, "total messages to publish; 0 means unbounded for the duration")
	runCmd.Flags().DurationVar(&runReportInterval, "report-interval", 5*time.Second, "progress report interval")
	runCmd.Flags().DurationVar(&runSettle, "settle", 2*time.Second, "extra wait after publishing to let inserts land")
	runCmd.Flags().StringVar(&runManifest, "manifest", "", "optional selfbench manifest file to source series UUIDs from")
	runCmd.Flags().StringSliceVar(&runSeriesUUIDs, "series-uuids", nil, "explicit series UUID list")
	runCmd.Flags().IntVar(&runSeriesLimit, "series-limit", 0, "maximum number of series UUIDs to use from the manifest")
	runCmd.Flags().StringVar(&runPGURI, "pg-uri", "", "optional PostgreSQL connection URI for inserted-row verification")
	runCmd.Flags().StringVar(&runTopicTemplate, "topic-template", "sensors/{{.UUID}}/temperature", "Go template for MQTT topic; supports {{.UUID}} and {{.Index}}")
	runCmd.Flags().StringVar(&runPayloadTemplate, "payload-template", "", "optional Go template for payload; supports UUID, Index, Sequence, Value, TimestampRFC3339, TimestampUnix, TimestampUnixMilli")
	runCmd.Flags().IntVar(&runPointsPerMessage, "points-per-message", 1, "number of points to include in each generated payload message")
	runCmd.Flags().StringVar(&runValueKey, "value-key", "temperature", "JSON payload key for the value field")
	runCmd.Flags().StringVar(&runTimestampKey, "timestamp-key", "ts", "JSON payload key for the timestamp field")
	runCmd.Flags().StringVar(&runTimestampFmt, "timestamp-format", "rfc3339", "timestamp format to generate: rfc3339, unix or unix_ms")
	runCmd.Flags().Float64Var(&runValueBase, "value-base", 20.0, "base numeric value in generated payloads")
	runCmd.Flags().Float64Var(&runValueDelta, "value-delta", 5.0, "sinusoidal delta applied to generated values")
}

func publishWorker(
	ctx context.Context,
	client mqtt.Client,
	topicTmpl *template.Template,
	payloadTmpl *template.Template,
	seriesIDs []uuid.UUID,
	payloadBaseTime time.Time,
	sequence *atomic.Uint64,
	counters *publisherCounters,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		seq := sequence.Add(1) - 1
		if runMessages > 0 && seq >= uint64(runMessages) {
			return nil
		}

		seriesIndex := int(seq % uint64(len(seriesIDs)))
		seriesID := seriesIDs[seriesIndex]
		payloadData := buildPayloadData(payloadBaseTime, seq, seriesIndex, seriesID)
		topic, err := renderTopic(topicTmpl, topicTemplateData{
			Index:              payloadData.Index,
			UUID:               payloadData.UUID,
			Sequence:           payloadData.Sequence,
			Value:              payloadData.Value,
			TimestampRFC3339:   payloadData.TimestampRFC3339,
			TimestampUnix:      payloadData.TimestampUnix,
			TimestampUnixMilli: payloadData.TimestampUnixMilli,
		})
		if err != nil {
			counters.Errors.Add(1)
			return err
		}

		counters.Attempted.Add(1)
		payload, err := buildPayload(payloadTmpl, payloadData)
		if err != nil {
			counters.Errors.Add(1)
			return err
		}
		token := client.Publish(topic, byte(runQoS), false, payload)
		if !token.WaitTimeout(30 * time.Second) {
			counters.Errors.Add(1)
			return fmt.Errorf("publish timeout on topic %q", topic)
		}
		if err := token.Error(); err != nil {
			counters.Errors.Add(1)
			return fmt.Errorf("publish to %q: %w", topic, err)
		}
		counters.Published.Add(1)
	}
}

func buildPayloadData(base time.Time, seq uint64, seriesIndex int, seriesID uuid.UUID) topicTemplateData {
	pointsPerMessage := maxInt(runPointsPerMessage, 1)
	globalPointIndex := seq * uint64(pointsPerMessage)
	ts := base.Add(time.Duration(globalPointIndex) * time.Microsecond).UTC()
	value := runValueBase + math.Sin(float64(seq)/180.0)*runValueDelta
	return topicTemplateData{
		Index:              seriesIndex,
		UUID:               seriesID.String(),
		Sequence:           seq,
		Value:              value,
		TimestampUnixNano:  ts.UnixNano(),
		TimestampRFC3339:   ts.Format(time.RFC3339Nano),
		TimestampUnix:      ts.Unix(),
		TimestampUnixMilli: ts.UnixMilli(),
	}
}

func buildPayload(payloadTmpl *template.Template, data topicTemplateData) ([]byte, error) {
	if payloadTmpl != nil {
		var b strings.Builder
		if err := payloadTmpl.Execute(&b, data); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	}

	if runPointsPerMessage > 1 {
		return buildBatchPayload(data)
	}

	switch runTimestampFmt {
	case "rfc3339":
		return []byte(fmt.Sprintf(`{"%s":%.4f,"%s":"%s"}`, runValueKey, data.Value, runTimestampKey, data.TimestampRFC3339)), nil
	case "unix":
		return []byte(fmt.Sprintf(`{"%s":%.4f,"%s":%d}`, runValueKey, data.Value, runTimestampKey, data.TimestampUnix)), nil
	case "unix_ms":
		return []byte(fmt.Sprintf(`{"%s":%.4f,"%s":%d}`, runValueKey, data.Value, runTimestampKey, data.TimestampUnixMilli)), nil
	default:
		return nil, fmt.Errorf("unsupported --timestamp-format %q", runTimestampFmt)
	}
}

func buildBatchPayload(data topicTemplateData) ([]byte, error) {
	type batchPoint struct {
		Value any `json:"value"`
		TS    any `json:"ts"`
	}

	points := make([]batchPoint, 0, runPointsPerMessage)
	for i := 0; i < runPointsPerMessage; i++ {
		pointData := data
		pointData.Sequence += uint64(i)
		pointData.Value = runValueBase + math.Sin(float64(pointData.Sequence)/180.0)*runValueDelta
		ts := time.Unix(0, data.TimestampUnixNano).Add(time.Duration(i) * time.Microsecond).UTC()

		switch runTimestampFmt {
		case "rfc3339":
			points = append(points, batchPoint{
				Value: pointData.Value,
				TS:    ts.Format(time.RFC3339Nano),
			})
		case "unix":
			points = append(points, batchPoint{
				Value: pointData.Value,
				TS:    ts.Unix(),
			})
		case "unix_ms":
			points = append(points, batchPoint{
				Value: pointData.Value,
				TS:    ts.UnixMilli() + int64(i),
			})
		default:
			return nil, fmt.Errorf("unsupported --timestamp-format %q", runTimestampFmt)
		}
	}

	normalized := make([]map[string]any, 0, len(points))
	for _, point := range points {
		normalized = append(normalized, map[string]any{
			runValueKey:     point.Value,
			runTimestampKey: point.TS,
		})
	}

	return json.Marshal(normalized)
}

func connectPublisher(broker, clientID, username, password string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetCleanSession(true)
	opts.SetOrderMatters(false)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		return nil, fmt.Errorf("mqtt connect did not complete for %s", clientID)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect for %s: %w", clientID, err)
	}
	return client, nil
}

func renderTopic(tmpl *template.Template, data topicTemplateData) (string, error) {
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func loadSeriesUUIDs(manifestPath string, explicit []string, limit int) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{})
	out := make([]uuid.UUID, 0)

	add := func(raw string) error {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return err
		}
		if _, ok := seen[id]; ok {
			return nil
		}
		seen[id] = struct{}{}
		out = append(out, id)
		return nil
	}

	for _, raw := range explicit {
		if err := add(raw); err != nil {
			return nil, fmt.Errorf("parse --series-uuids entry %q: %w", raw, err)
		}
	}

	if strings.TrimSpace(manifestPath) != "" {
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		for _, series := range manifest.Series {
			if limit > 0 && len(out) >= limit {
				break
			}
			if err := add(series.UUID); err != nil {
				return nil, fmt.Errorf("parse manifest series UUID %q: %w", series.UUID, err)
			}
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

func loadManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest manifest
	switch {
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		err = yaml.Unmarshal(data, &manifest)
	default:
		err = json.Unmarshal(data, &manifest)
	}
	if err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}

func countExistingPoints(ctx context.Context, db *sql.DB, seriesIDs []uuid.UUID) (int64, error) {
	if len(seriesIDs) == 0 {
		return 0, nil
	}

	args := make([]any, 0, len(seriesIDs))
	placeholders := make([]string, 0, len(seriesIDs))
	for i, id := range seriesIDs {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	query := `SELECT COUNT(*) FROM tsdata WHERE ts_uuid IN (` + strings.Join(placeholders, ", ") + `)`
	var count int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func printProgress(start time.Time, db *sql.DB, seriesIDs []uuid.UUID, beforeCount int64, counters *publisherCounters) {
	elapsed := time.Since(start)
	attempted := counters.Attempted.Load()
	published := counters.Published.Load()
	errors := counters.Errors.Load()

	line := fmt.Sprintf(
		"elapsed=%s attempted=%d published=%d errors=%d publish_rate=%.0f msg/s",
		elapsed.Round(time.Second),
		attempted,
		published,
		errors,
		float64(published)/maxFloat(elapsed.Seconds(), 1),
	)

	if db != nil {
		count, err := countExistingPoints(context.Background(), db, seriesIDs)
		if err != nil {
			fmt.Printf("%s db_error=%q\n", line, err)
			return
		}
		inserted := count - beforeCount
		fmt.Printf(
			"%s inserted=%d insert_rate=%.0f rows/s ingest_gap=%d\n",
			line,
			inserted,
			float64(inserted)/maxFloat(elapsed.Seconds(), 1),
			int64(published)-inserted,
		)
		return
	}

	fmt.Println(line)
}

func timerC(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}
	return timer.C
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
