package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

type renderContext struct {
	Manifest *Manifest
}

type compiledOperation struct {
	name             string
	weight           int
	method           string
	pathTemplate     *template.Template
	bodyTemplate     *template.Template
	headerTemplates  map[string]*template.Template
	expectedStatuses map[int]struct{}
}

type requestOutcome struct {
	opName     string
	statusCode int
	err        error
	duration   time.Duration
	bytesRead  int64
}

type opStats struct {
	Name      string
	Count     int64
	Errors    int64
	BytesRead int64
	Latencies []int64
	Statuses  map[int]int64
}

type dbSnapshot struct {
	TakenAt      time.Time
	NumBackends  int64
	XactCommit   int64
	XactRollback int64
	BlksRead     int64
	BlksHit      int64
	TupReturned  int64
	TupFetched   int64
	TupInserted  int64
	TupUpdated   int64
	TupDeleted   int64
	TempFiles    int64
	TempBytes    int64
	Deadlocks    int64
}

type activityMax struct {
	Active            int64
	Idle              int64
	IdleInTransaction int64
}

var (
	runConfigPath string
	runManifest   string
	runPGURI      string
	lastNowNano   int64
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run an HTTP benchmark scenario",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runConfigPath == "" {
			return fmt.Errorf("--config is required")
		}

		cfg, err := loadRunConfig(runConfigPath)
		if err != nil {
			return err
		}
		if cfg.Concurrency <= 0 || cfg.Duration <= 0 || cfg.Timeout <= 0 {
			return fmt.Errorf("concurrency, duration and timeout must be > 0")
		}

		var manifest *Manifest
		if runManifest != "" {
			manifest, err = loadManifest(runManifest)
			if err != nil {
				return err
			}
		} else {
			manifest = &Manifest{}
		}

		ops, err := compileOperations(cfg, manifest)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			return fmt.Errorf("no operations configured")
		}

		client := &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
				MaxIdleConns:        cfg.Concurrency * 4,
				MaxConnsPerHost:     cfg.Concurrency * 4,
				MaxIdleConnsPerHost: cfg.Concurrency * 4,
			},
		}

		var db *sql.DB
		var before *dbSnapshot
		var after *dbSnapshot
		var maxima activityMax

		if runPGURI != "" {
			db, err = sql.Open("pgx", runPGURI)
			if err != nil {
				return err
			}
			defer db.Close()

			before, err = captureDBSnapshot(context.Background(), db)
			if err != nil {
				return fmt.Errorf("capture pre-run db snapshot: %w", err)
			}
		}

		fmt.Printf("scenario: %s\n", cfg.Name)
		fmt.Printf("target:   %s\n", cfg.BaseURL)
		fmt.Printf("duration: %s\n", cfg.Duration)
		fmt.Printf("warmup:   %s\n", cfg.Warmup)
		fmt.Printf("workers:  %d\n", cfg.Concurrency)
		fmt.Printf("ops:      %s\n\n", strings.Join(operationNames(ops), ", "))

		if cfg.Warmup > 0 {
			if err := runWorkers(client, cfg, ops, manifest, cfg.Warmup, nil, nil); err != nil {
				return err
			}
		}

		outcomes := make(chan requestOutcome, cfg.Concurrency*8)
		doneStats := make(chan map[string]*opStats, 1)
		go aggregateOutcomes(outcomes, doneStats)

		stopSampler := make(chan struct{})
		if db != nil {
			go sampleActivity(db, stopSampler, &maxima)
		}

		start := time.Now()
		if err := runWorkers(client, cfg, ops, manifest, cfg.Duration, outcomes, &start); err != nil {
			close(stopSampler)
			close(outcomes)
			<-doneStats
			return err
		}
		close(stopSampler)
		close(outcomes)
		stats := <-doneStats
		finish := time.Now()

		if db != nil {
			after, err = captureDBSnapshot(context.Background(), db)
			if err != nil {
				return fmt.Errorf("capture post-run db snapshot: %w", err)
			}
		}

		printReport(start, finish, stats, before, after, maxima)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runConfigPath, "config", "", "benchmark scenario config file")
	runCmd.Flags().StringVar(&runManifest, "manifest", "", "seed manifest file")
	runCmd.Flags().StringVar(&runPGURI, "pg-uri", "", "optional PostgreSQL connection URI for snapshots")
}

func compileOperations(cfg *RunConfig, manifest *Manifest) ([]compiledOperation, error) {
	funcs := template.FuncMap{
		"seriesUUID": func(index int) string {
			if manifest == nil || index < 0 || index >= len(manifest.Series) {
				return ""
			}
			return manifest.Series[index].UUID
		},
		"seriesQuery": func(start, count int) string {
			if manifest == nil || count <= 0 || start < 0 || start >= len(manifest.Series) {
				return ""
			}

			end := start + count
			if end > len(manifest.Series) {
				end = len(manifest.Series)
			}

			parts := make([]string, 0, end-start)
			for i := start; i < end; i++ {
				parts = append(parts, "uuids="+manifest.Series[i].UUID)
			}

			return strings.Join(parts, "&")
		},
		"windowStart": func(name string) string {
			for _, window := range manifest.Windows {
				if window.Name == name {
					return window.Start
				}
			}
			return ""
		},
		"windowEnd": func(name string) string {
			for _, window := range manifest.Windows {
				if window.Name == name {
					return window.End
				}
			}
			return ""
		},
		"nowRFC3339Nano": func() string {
			return uniqueNowUTC().Format(time.RFC3339Nano)
		},
		"nowUnixMilli": func() int64 {
			return uniqueNowUTC().UnixMilli()
		},
	}

	ops := make([]compiledOperation, 0, len(cfg.Operations))
	for _, op := range cfg.Operations {
		if op.Weight <= 0 {
			continue
		}

		pathTemplate, err := template.New(op.Name + "-path").Funcs(funcs).Parse(op.Path)
		if err != nil {
			return nil, fmt.Errorf("parse path template for %s: %w", op.Name, err)
		}

		var bodyTemplate *template.Template
		if op.Body != "" {
			bodyTemplate, err = template.New(op.Name + "-body").Funcs(funcs).Parse(op.Body)
			if err != nil {
				return nil, fmt.Errorf("parse body template for %s: %w", op.Name, err)
			}
		}

		headerTemplates := make(map[string]*template.Template, len(op.Headers))
		for key, value := range op.Headers {
			headerTemplates[key], err = template.New(op.Name + "-header-" + key).Funcs(funcs).Parse(value)
			if err != nil {
				return nil, fmt.Errorf("parse header template for %s/%s: %w", op.Name, key, err)
			}
		}

		expected := make(map[int]struct{}, len(op.ExpectedStatuses))
		for _, code := range op.ExpectedStatuses {
			expected[code] = struct{}{}
		}
		if len(expected) == 0 {
			expected[http.StatusOK] = struct{}{}
		}

		ops = append(ops, compiledOperation{
			name:             op.Name,
			weight:           op.Weight,
			method:           strings.ToUpper(op.Method),
			pathTemplate:     pathTemplate,
			bodyTemplate:     bodyTemplate,
			headerTemplates:  headerTemplates,
			expectedStatuses: expected,
		})
	}
	return ops, nil
}

func runWorkers(client *http.Client, cfg *RunConfig, ops []compiledOperation, manifest *Manifest, duration time.Duration, outcomes chan<- requestOutcome, startTime *time.Time) error {
	var wg sync.WaitGroup
	weighted := buildWeightedOperations(ops)
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.Domain+":"+cfg.Token))
	deadline := time.Now().Add(duration)

	for workerID := 0; workerID < cfg.Concurrency; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id*997)))
			renderData := renderContext{Manifest: manifest}

			for {
				if time.Now().After(deadline) {
					return
				}

				op := weighted[rng.Intn(len(weighted))]
				outcome := executeOperation(client, cfg, op, renderData, authHeader)
				if outcomes != nil {
					outcomes <- outcome
				}
			}
		}(workerID)
	}

	wg.Wait()
	return nil
}

func buildWeightedOperations(ops []compiledOperation) []compiledOperation {
	total := 0
	for _, op := range ops {
		total += op.weight
	}
	weighted := make([]compiledOperation, 0, total)
	for _, op := range ops {
		for i := 0; i < op.weight; i++ {
			weighted = append(weighted, op)
		}
	}
	return weighted
}

func executeOperation(client *http.Client, cfg *RunConfig, op compiledOperation, data renderContext, authHeader string) requestOutcome {
	start := time.Now()

	path, err := renderTemplate(op.pathTemplate, data)
	if err != nil {
		return requestOutcome{opName: op.name, err: err, duration: time.Since(start)}
	}

	var body io.Reader
	if op.bodyTemplate != nil {
		renderedBody, err := renderTemplate(op.bodyTemplate, data)
		if err != nil {
			return requestOutcome{opName: op.name, err: err, duration: time.Since(start)}
		}
		body = strings.NewReader(renderedBody)
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, op.method, strings.TrimRight(cfg.BaseURL, "/")+path, body)
	if err != nil {
		return requestOutcome{opName: op.name, err: err, duration: time.Since(start)}
	}

	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	for key, tmpl := range op.headerTemplates {
		rendered, err := renderTemplate(tmpl, data)
		if err != nil {
			return requestOutcome{opName: op.name, err: err, duration: time.Since(start)}
		}
		req.Header.Set(key, rendered)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := client.Do(req)
	if err != nil {
		return requestOutcome{opName: op.name, err: err, duration: time.Since(start)}
	}
	defer resp.Body.Close()

	n, readErr := io.Copy(io.Discard, resp.Body)
	duration := time.Since(start)

	_, ok := op.expectedStatuses[resp.StatusCode]
	if readErr != nil {
		return requestOutcome{opName: op.name, statusCode: resp.StatusCode, err: readErr, duration: duration, bytesRead: n}
	}
	if !ok {
		return requestOutcome{opName: op.name, statusCode: resp.StatusCode, err: fmt.Errorf("unexpected status %d", resp.StatusCode), duration: duration, bytesRead: n}
	}

	return requestOutcome{opName: op.name, statusCode: resp.StatusCode, duration: duration, bytesRead: n}
}

func renderTemplate(tmpl *template.Template, data renderContext) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func aggregateOutcomes(outcomes <-chan requestOutcome, done chan<- map[string]*opStats) {
	stats := make(map[string]*opStats)
	for outcome := range outcomes {
		stat := stats[outcome.opName]
		if stat == nil {
			stat = &opStats{
				Name:      outcome.opName,
				Statuses:  make(map[int]int64),
				Latencies: make([]int64, 0, 1024),
			}
			stats[outcome.opName] = stat
		}

		stat.Count++
		stat.BytesRead += outcome.bytesRead
		stat.Latencies = append(stat.Latencies, outcome.duration.Nanoseconds())
		stat.Statuses[outcome.statusCode]++
		if outcome.err != nil {
			stat.Errors++
		}
	}
	done <- stats
}

func captureDBSnapshot(ctx context.Context, db *sql.DB) (*dbSnapshot, error) {
	const query = `
SELECT
	numbackends,
	xact_commit,
	xact_rollback,
	blks_read,
	blks_hit,
	tup_returned,
	tup_fetched,
	tup_inserted,
	tup_updated,
	tup_deleted,
	temp_files,
	temp_bytes,
	deadlocks
FROM pg_stat_database
WHERE datname = current_database()
`
	snapshot := &dbSnapshot{TakenAt: time.Now()}
	if err := db.QueryRowContext(ctx, query).Scan(
		&snapshot.NumBackends,
		&snapshot.XactCommit,
		&snapshot.XactRollback,
		&snapshot.BlksRead,
		&snapshot.BlksHit,
		&snapshot.TupReturned,
		&snapshot.TupFetched,
		&snapshot.TupInserted,
		&snapshot.TupUpdated,
		&snapshot.TupDeleted,
		&snapshot.TempFiles,
		&snapshot.TempBytes,
		&snapshot.Deadlocks,
	); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func sampleActivity(db *sql.DB, stop <-chan struct{}, maxima *activityMax) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			rows, err := db.Query(`
SELECT state, count(*)
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY state
`)
			if err != nil {
				continue
			}

			var active, idle, idleTx int64
			for rows.Next() {
				var state sql.NullString
				var count int64
				if err := rows.Scan(&state, &count); err != nil {
					continue
				}
				switch state.String {
				case "active":
					active = count
				case "idle":
					idle = count
				case "idle in transaction":
					idleTx = count
				}
			}
			rows.Close()

			updateMax(&maxima.Active, active)
			updateMax(&maxima.Idle, idle)
			updateMax(&maxima.IdleInTransaction, idleTx)
		}
	}
}

func updateMax(target *int64, candidate int64) {
	for {
		current := atomic.LoadInt64(target)
		if candidate <= current {
			return
		}
		if atomic.CompareAndSwapInt64(target, current, candidate) {
			return
		}
	}
}

func uniqueNowUTC() time.Time {
	for {
		now := time.Now().UTC().UnixNano()
		now -= now % int64(time.Microsecond)
		last := atomic.LoadInt64(&lastNowNano)
		if now <= last {
			now = last + int64(time.Microsecond)
		}
		if atomic.CompareAndSwapInt64(&lastNowNano, last, now) {
			return time.Unix(0, now).UTC()
		}
	}
}

func printReport(start, finish time.Time, stats map[string]*opStats, before, after *dbSnapshot, maxima activityMax) {
	elapsed := finish.Sub(start)
	totalCount := int64(0)
	totalErrors := int64(0)
	allLatencies := make([]int64, 0)

	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		stat := stats[name]
		totalCount += stat.Count
		totalErrors += stat.Errors
		allLatencies = append(allLatencies, stat.Latencies...)
	}

	fmt.Printf("results:\n")
	fmt.Printf("  total requests: %d\n", totalCount)
	fmt.Printf("  request rate:   %.2f req/s\n", float64(totalCount)/elapsed.Seconds())
	fmt.Printf("  errors:         %d (%.2f%%)\n", totalErrors, percent(totalErrors, totalCount))
	overall := summarizeLatencies(allLatencies)
	fmt.Printf("  latency p50:    %s\n", overall.P50)
	fmt.Printf("  latency p95:    %s\n", overall.P95)
	fmt.Printf("  latency p99:    %s\n", overall.P99)
	fmt.Printf("  latency max:    %s\n", overall.Max)

	fmt.Printf("\nper operation:\n")
	for _, name := range names {
		stat := stats[name]
		summary := summarizeLatencies(stat.Latencies)
		fmt.Printf("  %s\n", name)
		fmt.Printf("    requests: %d\n", stat.Count)
		fmt.Printf("    errors:   %d (%.2f%%)\n", stat.Errors, percent(stat.Errors, stat.Count))
		fmt.Printf("    p50/p95/p99/max: %s / %s / %s / %s\n", summary.P50, summary.P95, summary.P99, summary.Max)
		fmt.Printf("    statuses: %s\n", formatStatuses(stat.Statuses))
	}

	if before != nil && after != nil {
		fmt.Printf("\npostgres delta:\n")
		fmt.Printf("  xact_commit:  %d\n", after.XactCommit-before.XactCommit)
		fmt.Printf("  xact_rollback:%d\n", after.XactRollback-before.XactRollback)
		fmt.Printf("  blks_read:    %d\n", after.BlksRead-before.BlksRead)
		fmt.Printf("  blks_hit:     %d\n", after.BlksHit-before.BlksHit)
		fmt.Printf("  tup_returned: %d\n", after.TupReturned-before.TupReturned)
		fmt.Printf("  tup_fetched:  %d\n", after.TupFetched-before.TupFetched)
		fmt.Printf("  tup_inserted: %d\n", after.TupInserted-before.TupInserted)
		fmt.Printf("  tup_updated:  %d\n", after.TupUpdated-before.TupUpdated)
		fmt.Printf("  tup_deleted:  %d\n", after.TupDeleted-before.TupDeleted)
		fmt.Printf("  temp_files:   %d\n", after.TempFiles-before.TempFiles)
		fmt.Printf("  temp_bytes:   %d\n", after.TempBytes-before.TempBytes)
		fmt.Printf("  deadlocks:    %d\n", after.Deadlocks-before.Deadlocks)
		fmt.Printf("  max activity: active=%d idle=%d idle_in_tx=%d\n", maxima.Active, maxima.Idle, maxima.IdleInTransaction)
	}
}

type latencySummary struct {
	P50 string
	P95 string
	P99 string
	Max string
}

func summarizeLatencies(latencies []int64) latencySummary {
	if len(latencies) == 0 {
		return latencySummary{P50: "-", P95: "-", P99: "-", Max: "-"}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	return latencySummary{
		P50: time.Duration(percentile(latencies, 0.50)).String(),
		P95: time.Duration(percentile(latencies, 0.95)).String(),
		P99: time.Duration(percentile(latencies, 0.99)).String(),
		Max: time.Duration(latencies[len(latencies)-1]).String(),
	}
}

func percentile(values []int64, pct float64) int64 {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(values))*pct)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func percent(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func formatStatuses(statuses map[int]int64) string {
	keys := make([]int, 0, len(statuses))
	for code := range statuses {
		keys = append(keys, code)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, code := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", code, statuses[code]))
	}
	return strings.Join(parts, ", ")
}

func operationNames(ops []compiledOperation) []string {
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.name)
	}
	return names
}
