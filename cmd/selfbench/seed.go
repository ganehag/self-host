package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/spf13/cobra"
)

const (
	rootUserUUID        = "00000000-0000-1000-8000-000000000000"
	insertTimeseriesSQL = `
INSERT INTO timeseries(
	uuid,
	name,
	si_unit,
	lower_bound,
	upper_bound,
	created_by,
	tags
) VALUES ($1, $2, $3, $4, $5, $6, $7)
`
	insertTsDataJSONSQL = `
INSERT INTO tsdata(ts_uuid, value, ts, created_by)
SELECT $1::uuid, x.v, x.ts, $2::uuid
FROM json_to_recordset($3::json) AS x("v" double precision, "ts" timestamptz)
`
)

type seedPoint struct {
	Value float64   `json:"v"`
	TS    time.Time `json:"ts"`
}

var (
	seedPGURI       string
	seedManifest    string
	seedDomain      string
	seedPrefix      string
	seedSeriesCount int
	seedPoints      int
	seedBatch       int
	seedStep        time.Duration
	seedStart       time.Time
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed benchmark data directly into PostgreSQL",
	RunE: func(cmd *cobra.Command, args []string) error {
		if seedPGURI == "" {
			return fmt.Errorf("--pg-uri is required")
		}
		if seedManifest == "" {
			return fmt.Errorf("--manifest is required")
		}
		if seedSeriesCount <= 0 || seedPoints <= 0 || seedBatch <= 0 {
			return fmt.Errorf("--series, --points and --batch must be > 0")
		}

		db, err := sql.Open("pgx", seedPGURI)
		if err != nil {
			return err
		}
		defer db.Close()

		ctx := context.Background()
		rootID, err := uuid.Parse(rootUserUUID)
		if err != nil {
			return err
		}

		manifest := &Manifest{
			GeneratedAt:  time.Now().UTC(),
			Domain:       seedDomain,
			RootUserUUID: rootUserUUID,
			SeriesPrefix: seedPrefix,
			Step:         seedStep.String(),
			Series:       make([]ManifestSeries, 0, seedSeriesCount),
		}

		rng := rand.New(rand.NewSource(seedStart.UnixNano()))
		end := seedStart.Add(time.Duration(seedPoints-1) * seedStep)

		for i := 0; i < seedSeriesCount; i++ {
			seriesID := uuid.New()
			name := fmt.Sprintf("%s-%04d", seedPrefix, i)
			tags := []string{
				"bench",
				fmt.Sprintf("bench:%s", seedPrefix),
				fmt.Sprintf("bench-series:%d", i),
			}

			if _, err := db.ExecContext(ctx, insertTimeseriesSQL,
				seriesID,
				name,
				"C",
				-50.0,
				50.0,
				rootID,
				tags,
			); err != nil {
				return fmt.Errorf("insert timeseries %s: %w", name, err)
			}

			if err := seedSeriesData(ctx, db, rootID, seriesID, i, rng); err != nil {
				return err
			}

			manifest.Series = append(manifest.Series, ManifestSeries{
				UUID:   seriesID.String(),
				Name:   name,
				Unit:   "C",
				Tags:   tags,
				Points: seedPoints,
				Start:  seedStart.UTC().Format(time.RFC3339),
				End:    end.UTC().Format(time.RFC3339),
			})
		}

		manifest.Windows = buildWindows(seedStart, end)
		return writeManifest(seedManifest, manifest)
	},
}

func init() {
	rootCmd.AddCommand(seedCmd)

	seedCmd.Flags().StringVar(&seedPGURI, "pg-uri", "", "PostgreSQL connection URI")
	seedCmd.Flags().StringVar(&seedManifest, "manifest", "bench/manifest.json", "output manifest path")
	seedCmd.Flags().StringVar(&seedDomain, "domain", "test", "domain name used by the API")
	seedCmd.Flags().StringVar(&seedPrefix, "prefix", "bench", "prefix for benchmark series names")
	seedCmd.Flags().IntVar(&seedSeriesCount, "series", 32, "number of time series to create")
	seedCmd.Flags().IntVar(&seedPoints, "points", 4320, "number of points per series")
	seedCmd.Flags().IntVar(&seedBatch, "batch", 512, "insert batch size per statement")
	seedCmd.Flags().DurationVar(&seedStep, "step", 10*time.Minute, "time between data points")
	seedCmd.Flags().Var(newRFC3339Value(&seedStart, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)), "start", "start timestamp in RFC3339")
}

func seedSeriesData(ctx context.Context, db *sql.DB, rootID uuid.UUID, seriesID uuid.UUID, seriesIndex int, rng *rand.Rand) error {
	for offset := 0; offset < seedPoints; offset += seedBatch {
		limit := minInt(seedBatch, seedPoints-offset)
		points := make([]seedPoint, 0, limit)
		for j := 0; j < limit; j++ {
			pointIndex := offset + j
			ts := seedStart.Add(time.Duration(pointIndex) * seedStep)
			phase := float64(pointIndex)/144.0 + float64(seriesIndex)*0.15
			value := math.Sin(phase)*18 + math.Cos(phase/4)*6 + rng.Float64()*1.5
			points = append(points, seedPoint{
				Value: math.Round(value*100) / 100,
				TS:    ts.UTC(),
			})
		}

		payload, err := json.Marshal(points)
		if err != nil {
			return fmt.Errorf("marshal points: %w", err)
		}
		if _, err := db.ExecContext(ctx, insertTsDataJSONSQL, seriesID, rootID, payload); err != nil {
			return fmt.Errorf("insert tsdata for %s: %w", seriesID.String(), err)
		}
	}
	return nil
}

func buildWindows(start, end time.Time) []ManifestWindow {
	candidates := []struct {
		name string
		dur  time.Duration
	}{
		{name: "day", dur: 24 * time.Hour},
		{name: "week", dur: 7 * 24 * time.Hour},
		{name: "month", dur: 30 * 24 * time.Hour},
		{name: "full", dur: end.Sub(start)},
	}

	windows := make([]ManifestWindow, 0, len(candidates))
	for _, candidate := range candidates {
		windowEnd := start.Add(candidate.dur)
		if windowEnd.After(end) {
			windowEnd = end
		}
		windows = append(windows, ManifestWindow{
			Name:  candidate.name,
			Start: start.UTC().Format(time.RFC3339),
			End:   windowEnd.UTC().Format(time.RFC3339),
		})
	}
	return windows
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type rfc3339Value struct {
	target *time.Time
}

func newRFC3339Value(target *time.Time, defaultValue time.Time) *rfc3339Value {
	*target = defaultValue
	return &rfc3339Value{target: target}
}

func (v *rfc3339Value) String() string {
	if v == nil || v.target == nil {
		return ""
	}
	return v.target.UTC().Format(time.RFC3339)
}

func (v *rfc3339Value) Set(value string) error {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	*v.target = parsed
	return nil
}

func (v *rfc3339Value) Type() string {
	return "rfc3339"
}
