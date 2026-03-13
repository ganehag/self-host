# Benchmark Harness

This folder contains a repo-native benchmark harness for self-host.

It has two parts:

- `selfbench seed`: create reproducible benchmark time series directly in PostgreSQL and write a manifest file with generated UUIDs and useful time windows.
- `selfbench run`: execute weighted concurrent HTTP scenarios against `aapije`, with optional PostgreSQL snapshots before and after the run.

It also includes a local Docker workflow:

- `bench/run-local.sh`: bring up PostgreSQL + `aapije`, apply migrations, seed data, and run a scenario
- `bench/down-local.sh`: tear the local stack down

`bench/run-local.sh` builds `selfbench` and writes its manifest into temporary paths by default, so it does not leave generated files in the repo unless `MANIFEST_PATH` is set explicitly.


## Build

```bash
go build ./cmd/selfbench
```


## Seed a dataset

This creates tagged benchmark time series and writes a manifest file that the load profiles can reference.

```bash
./selfbench seed \
  --pg-uri 'postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable' \
  --domain test \
  --series 64 \
  --points 4320 \
  --batch 512 \
  --step 10m \
  --start 2024-01-01T00:00:00Z \
  --manifest bench/manifest.json
```

Notes:

- The seed command writes directly to PostgreSQL, not through the public API.
- It assumes a migrated database with the default root user from the project migrations.
- Generated series are tagged with `bench` and `bench:<prefix>`.


## Run a scenario

Smoke test:

```bash
./selfbench run \
  --config bench/scenarios/local-smoke.yaml \
  --manifest bench/manifest.json \
  --pg-uri 'postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable'
```

Read-heavy:

```bash
./selfbench run \
  --config bench/scenarios/read-heavy.yaml \
  --manifest bench/manifest.json \
  --pg-uri 'postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable'
```

Mixed:

```bash
./selfbench run \
  --config bench/scenarios/mixed.yaml \
  --manifest bench/manifest.json \
  --pg-uri 'postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable'
```


## One-command local Docker run

Run the default smoke scenario:

```bash
./bench/run-local.sh
```

Run a different scenario:

```bash
./bench/run-local.sh bench/scenarios/read-heavy.yaml
```

Useful environment overrides:

```bash
BENCH_SERIES=128 BENCH_POINTS=52561 ./bench/run-local.sh bench/scenarios/mixed.yaml
```

Persist the generated manifest instead of using the default temp file:

```bash
MANIFEST_PATH=bench/manifest.json ./bench/run-local.sh
```

Teardown:

```bash
./bench/down-local.sh
```


## What it reports

`selfbench run` prints:

- total requests
- request rate
- error rate
- overall latency p50/p95/p99/max
- per-operation latency and HTTP status breakdown
- PostgreSQL deltas from `pg_stat_database`
- maximum observed `pg_stat_activity` counts for `active`, `idle`, and `idle in transaction`


## Template helpers

Scenario files use Go templates in paths, bodies, and operation headers.

Available helpers:

- `{{ seriesUUID 0 }}`
- `{{ windowStart "day" }}`
- `{{ windowEnd "day" }}`
- `{{ nowRFC3339Nano }}`
- `{{ nowUnixMilli }}`


## Recommended workflow

1. Seed one medium dataset.
2. Run `local-smoke.yaml` to confirm the environment is healthy.
3. Run `read-heavy.yaml` to isolate read-path behavior.
4. Run `write-heavy.yaml` to isolate insert behavior.
5. Run `mixed.yaml` for a production-like blend.


## Caveats

- The harness currently targets `aapije` and optional PostgreSQL snapshots. It does not yet benchmark `juvuln` and `malgomaj` control-plane behavior directly.
- Run the load generator from a different machine than the server if you want cleaner server-side numbers.
- The API rate limiter can affect results. For benchmarking, raise `rate_control.req_per_hour` and `rate_control.maxburst` in the `aapije` config.
