# Hardware: Ryzen 5 7600 Workstation

Local workstation used during testing;

```txt
os: Debian GNU/Linux 13 (kernel 6.12.63-1)
cpu: AMD Ryzen 5 7600 6-Core Processor
threads: 12
memory: 61 GiB
system disk: Samsung SSD 970 EVO Plus 1TB (NVMe)
```


# Populating the database

The local benchmark workflow uses the repo-native harness in `bench/README.md`.

For the runs below, the default local seed settings were used:

- `32` time series
- `4320` points per series
- one point every `10m`
- start time `2024-01-01T00:00:00Z`

This gives roughly one month of data per series. The current codebase also maintains hourly rollups automatically through database triggers, so aggregate benchmarks exercise both the raw `tsdata` path and the rollup-backed aggregate path.


# Benchmarks

These results were collected with the local Docker workflow;

```bash
./bench/run-local.sh bench/scenarios/write-heavy.yaml
./bench/run-local.sh bench/scenarios/read-heavy.yaml
./bench/run-local.sh bench/scenarios/tsquery-heavy.yaml
```

The local benchmark config in `bench/docker/config/aapije.conf.yaml` disables request logging and OpenAPI validation for cleaner server-side measurements.


## Store data

Write-heavy scenario (`45s`, `12` workers);

```txt
results:
  total requests: 31599
  request rate:   701.95 req/s
  errors:         0 (0.00%)
  latency p50:    5.746945ms
  latency p95:    63.71539ms
  latency p99:    106.72173ms
  latency max:    267.203616ms

per operation:
  write_batch_10
    requests: 10489
    errors:   0 (0.00%)
    p50/p95/p99/max: 3.098616ms / 8.376688ms / 15.163357ms / 80.030079ms
  write_single
    requests: 21110
    errors:   0 (0.00%)
    p50/p95/p99/max: 14.430534ms / 74.872452ms / 118.116136ms / 267.203616ms
```

Notes:

- Batched writes are much cheaper than one-point writes.


## Retrieve data

Read-heavy scenario (`60s`, `24` workers);

```txt
results:
  total requests: 169337
  request rate:   2821.82 req/s
  errors:         0 (0.00%)
  latency p50:    7.090524ms
  latency p95:    20.345362ms
  latency p99:    26.246353ms
  latency max:    50.844907ms

per operation:
  read_day
    requests: 51210
    p50/p95/p99/max: 3.569052ms / 10.063758ms / 13.58485ms / 30.270559ms
  read_week
    requests: 42098
    p50/p95/p99/max: 6.23868ms / 13.373717ms / 17.425637ms / 31.711181ms
  read_month
    requests: 33886
    p50/p95/p99/max: 14.227968ms / 24.313856ms / 29.535644ms / 47.11699ms
  read_full
    requests: 25376
    p50/p95/p99/max: 14.252924ms / 24.098485ms / 29.73221ms / 50.844907ms
  list_users
    requests: 8380
    p50/p95/p99/max: 2.440601ms / 7.813389ms / 11.619663ms / 19.110063ms
  list_things
    requests: 8387
    p50/p95/p99/max: 2.159759ms / 7.153592ms / 10.367124ms / 18.834881ms
```

Notes:

- The single-series read path is fast enough that month/full range reads remain well below `30ms` at p99 in this local setup.
- Control-plane list endpoints are cheaper than the read endpoints and are no longer the obvious bottleneck.


## Year-scale raw single-series read

For comparison with the older benchmark note, a year-scale dataset was generated with:

```bash
BENCH_POINTS=52561 ./bench/run-local.sh bench/scenarios/read-heavy.yaml
```

This produces about one year of `10m` samples per series.

The local benchmark stack raises the timeseries response limits in `bench/docker/config/aapije.conf.yaml`, so the equivalent `read_full` request succeeds.

Measured result from the full `read-heavy` year-scale run:

```txt
results:
  total requests: 43029
  request rate:   716.30 req/s
  errors:         0 (0.00%)

per operation:
  read_month
    requests: 8680
    p50/p95/p99/max: 18.445845ms / 47.925852ms / 63.294322ms / 108.468362ms
  read_full
    requests: 6420
    p50/p95/p99/max: 141.065891ms / 206.153501ms / 236.653958ms / 316.692347ms
```

Notes:

- An earlier `413` on this test was caused by stale startup config; `bench/run-local.sh` now force-recreates `aapije` so config changes are applied reliably.
- Compared with the month-scale default seed, the year-scale `read_full` case is now clearly dominated by payload size and JSON encoding rather than SQL temp spill. The run completed with `0` PostgreSQL temp bytes.


## Multi-timeseries aggregate queries

Fan-out aggregate scenario (`60s`, `12` workers);

```txt
results:
  total requests: 73036
  request rate:   1217.08 req/s
  errors:         0 (0.00%)
  latency p50:    6.941ms
  latency p95:    27.342667ms
  latency p99:    32.892128ms
  latency max:    48.62768ms

per operation:
  multi_day_hourly_avg_8
    requests: 25646
    p50/p95/p99/max: 3.740763ms / 8.154217ms / 10.341801ms / 17.984408ms
  multi_week_hourly_avg_8
    requests: 18236
    p50/p95/p99/max: 7.441563ms / 12.473318ms / 15.009178ms / 23.768124ms
  multi_month_daily_avg_8
    requests: 14593
    p50/p95/p99/max: 15.487741ms / 21.2896ms / 23.912413ms / 30.648953ms
  multi_full_daily_avg_16
    requests: 7299
    p50/p95/p99/max: 27.328352ms / 34.578216ms / 37.818508ms / 48.62768ms
  multi_day_raw_8
    requests: 7262
    p50/p95/p99/max: 4.812589ms / 9.19772ms / 11.400803ms / 14.485025ms
```

Notes:

- These numbers include the hourly rollup implementation now present in the codebase.
- On this host, the rollups move long-range aggregate fan-out queries from “clearly database-bound” into a range where response encoding and transfer start to matter more than SQL aggregation.
- PostgreSQL temp spill stayed at `0` bytes during this run.
