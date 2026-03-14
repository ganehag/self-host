# Benchmark Harness

This folder contains a repo-native benchmark harness for self-host.

It has two parts:

- `selfbench seed`: create reproducible benchmark time series directly in PostgreSQL and write a manifest file with generated UUIDs and useful time windows.
- `selfbench run`: execute weighted concurrent HTTP scenarios against `aapije`, with optional PostgreSQL snapshots before and after the run.

It also includes a local Docker workflow:

- `bench/run-local.sh`: bring up PostgreSQL + `aapije`, apply migrations, seed data, and optionally run a scenario
- `bench/run-insamlare-local.sh`: bring up PostgreSQL + Mosquitto, seed data, run a tuned `insamlare`, and execute an MQTT load test
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

Set up the local stack and seed data:

```bash
./bench/run-local.sh
```

Run the smoke scenario after setup:

```bash
./bench/run-local.sh bench/scenarios/local-smoke.yaml
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


## One-command `insamlare` load test

This path benchmarks MQTT-to-PostgreSQL ingestion instead of the HTTP API.

```bash
./bench/run-insamlare-local.sh
```

The script will:

- bring up PostgreSQL and Mosquitto in Docker
- apply database migrations
- seed benchmark time series and write a manifest
- start a tuned local `insamlare`
- run `insamlarebench` against the broker and report publish and insert rates

Useful overrides:

```bash
BENCH_SERIES=256 LOAD_WORKERS=64 LOAD_DURATION=2m ./bench/run-insamlare-local.sh
```

```bash
INSAMLARE_TIMESTAMP_FORMAT=rfc3339 LOAD_TIMESTAMP_FORMAT=rfc3339 ./bench/run-insamlare-local.sh
```

```bash
MOSQUITTO_PORT=1888 ./bench/run-insamlare-local.sh
```

```bash
BROKER_IMPL=emqx ./bench/run-insamlare-local.sh
```

```bash
BROKER_IMPL=gomqtt ./bench/run-insamlare-local.sh
```

```bash
LOAD_POINTS_PER_MESSAGE=16 ./bench/run-insamlare-local.sh
```

```bash
INSAMLARE_ROUTE_MODE=tengo ./bench/run-insamlare-local.sh
```

Notes:

- `rfc3339` is the right default for throughput tests. `unix_ms` will quickly hit the `(ts_uuid, ts)` uniqueness constraint if you publish multiple points for the same series inside one millisecond.
- If `MOSQUITTO_PORT` is not set, the script will pick a free host port in the `1883-1899` range automatically.
- `BROKER_IMPL=emqx` switches the harness to the official `emqx/emqx:latest` broker image. Source: https://hub.docker.com/r/emqx/emqx/
- `BROKER_IMPL=gomqtt` builds and runs the new local [mqttd](/home/mikael/Project/github/self-host/cmd/mqttd/main.go) command, which wraps Mochi MQTT with allow-all auth and a tuned TCP listener. Source for the library: https://github.com/mochi-mqtt/server
- `LOAD_POINTS_PER_MESSAGE` makes `insamlarebench` pack multiple JSON points into each MQTT message, which is the most important next ingestion optimization to test.
- `INSAMLARE_ROUTE_MODE=tengo` switches the harness to a realistic scripted route that decodes a nested Modbus-style JSON payload and computes the final engineering value in Tengo before inserting.
- The script writes the `insamlare` log to a temporary file and prints the path at the end.


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
