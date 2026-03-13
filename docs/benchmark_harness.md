# Benchmark harness

For a repeatable local benchmark workflow, use the in-repo `selfbench` command under [cmd/selfbench](/home/mikael/Project/github/self-host/cmd/selfbench).

It is intended to cover the gaps in the older benchmark notes:

- reproducible dataset seeding
- repeatable scenario definitions
- latency percentiles
- request/error accounting
- lightweight PostgreSQL snapshots during a run


## Build

```text
go build ./cmd/selfbench
```


## Seed a dataset

```text
./selfbench seed --pg-uri postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable --domain test --series 64 --points 4320 --manifest bench/manifest.json
```

This writes benchmark time series directly to PostgreSQL and creates a manifest file used by the benchmark scenarios.


## Run scenarios

```text
./selfbench run --config bench/scenarios/local-smoke.yaml --manifest bench/manifest.json --pg-uri postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable
./selfbench run --config bench/scenarios/read-heavy.yaml --manifest bench/manifest.json --pg-uri postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable
./selfbench run --config bench/scenarios/write-heavy.yaml --manifest bench/manifest.json --pg-uri postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable
./selfbench run --config bench/scenarios/mixed.yaml --manifest bench/manifest.json --pg-uri postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-test?sslmode=disable
```


## Local Docker workflow

To bring up a local benchmark stack and run a scenario in one step:

```text
./bench/run-local.sh
./bench/run-local.sh bench/scenarios/read-heavy.yaml
```

To keep the generated manifest instead of using the default temp file:

```text
MANIFEST_PATH=bench/manifest.json ./bench/run-local.sh
```

To tear it down:

```text
./bench/down-local.sh
```

The workflow uses:

- PostgreSQL in Docker
- `aapije` in Docker
- `selfctl` and `selfbench` on the host for migrations, seeding, and scenario execution

The script builds `selfbench` and writes its manifest into temporary paths by default, so a normal local run does not leave generated artifacts in the repo.


## What to compare

Run each scenario while varying one axis at a time:

- API instance count
- PostgreSQL size / tuning
- request validation on or off
- rate-control settings
- number of domains
- worker / manager placement

Keep the dataset and scenario file fixed while changing infrastructure. That makes the output comparable across runs.
