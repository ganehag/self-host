<p align="center">
  <img src="https://raw.githubusercontent.com/self-host/self-host/main/docs/assets/logo.svg" width="160" height="160" alt="Self-host logo">
</p>

# Self-host

Self-host is a self-hosted time-series storage, ingestion, query, and automation platform built around the Self-host API.

The project exists so an organization can run the platform itself, inspect the code, modify it, and keep full control over the API, the data store, the ingestion path, and the automation logic. The system is open source, so the operator is not dependent on a hosted service or a closed implementation.

It is intended for deployments where these points matter:
- control over infrastructure, data location, and network boundaries
- a stable API surface that can be operated internally
- a platform that can scale by adding more API instances, workers, databases, or storage backends
- built-in scripting support for automation tasks such as data transformation, scheduled jobs, and worker-driven processing
- the ability to integrate time series, datasets, alerts, and scripted logic in one system

The project includes:
- `aapije`: the public REST API server
- `juvuln`: the program manager
- `malgomaj`: the program worker
- `selfctl`: the operator CLI
- benchmark and local test harnesses for API and ingestion performance

## What It Can Do

- Ingest, store, and query time-series data in infrastructure you operate yourself
- Support dataset-backed workflows alongside time-series workloads
- Run built-in scripted automation and transformation logic through the manager/worker architecture
- Store metadata in PostgreSQL and optionally keep dataset payloads in an S3-compatible object store
- Manage users, groups, policies, datasets, things, and time series through the API and `selfctl`
- Ingest and benchmark time-series workloads, including MQTT-based ingestion paths
- Validate and serve OpenAPI-described public and internal services

## Project Scope

Self-host is not just a single binary. It is a platform composed of API, background execution, storage, and operator tooling.

At a high level, it is meant to be:
- self-hosted first
- PostgreSQL-centered
- centered on time-series ingestion, storage, and query workloads
- scriptable for automation and transformation tasks
- explicit about API contracts
- suitable for both local development and production deployment
- benchmarkable and inspectable under load

## Architecture

A typical deployment consists of:
- one or more `aapije` instances exposing the public API
- one `juvuln` instance coordinating program execution
- one or more `malgomaj` workers executing program workloads
- one or more PostgreSQL databases hosting Self-host domains
- optionally an HTTP reverse proxy in front of `aapije`
- optionally an S3-compatible object store for dataset payloads

![Overview](https://raw.githubusercontent.com/self-host/self-host/main/docs/assets/overview.svg)

## Repository Layout

- `api`: OpenAPI interfaces and generated server/client types
- `bench`: local benchmark harnesses
- `cmd`: binaries such as `aapije`, `juvuln`, `malgomaj`, `selfctl`, `insamlare`
- `docs`: project and deployment documentation
- `internal`: service implementations and shared internal packages
- `middleware`: HTTP middleware
- `postgres`: migrations, generated queries, and PostgreSQL integration
- `test`: local integration test stacks, including SeaweedFS-backed dataset testing

## Quick Start

For a local API environment:

```bash
./bench/run-local.sh
```

That brings up PostgreSQL and `aapije`, applies migrations, seeds benchmark data, and leaves the API available on:

```text
http://127.0.0.1:8080
```

The API reference is available at:

```text
http://127.0.0.1:8080/reference
```

For a local SeaweedFS-backed dataset environment:

```bash
./test/seaweedfs/run-local.sh
```

For a local MQTT ingestion benchmark:

```bash
./bench/run-insamlare-local.sh
```

## CLI

`selfctl` is the main operator CLI. It supports:
- dataset create, update, upload, download, and delete
- user, group, and policy administration
- thing and timeseries administration
- local config management
- raw API requests

Build it with:

```bash
go build ./cmd/selfctl
```

Documentation:
- [selfctl guide](docs/selfctl.md)

## Documentation

Project and deployment documentation:
- [Test deployment](docs/test_deployment.md)
- [Production deployment](docs/production_deployment.md)
- [Docker deployment](docs/docker_deployment.md)
- [Kubernetes deployment](docs/k8s_deployment.md)
- [Authentication](docs/authentication.md)
- [Access control](docs/access_control.md)
- [Data partitioning](docs/data_partitioning.md)
- [Program manager and workers](docs/program_manager_worker.md)
- [External services](docs/external_services.md)
- [Rate control](docs/rate_control.md)
- [Unit handling](docs/unit_handling.md)
- [Alerts](docs/alerts.md)
- [Design](docs/design.md)
- [Glossary](docs/glossary.md)

Benchmarking and local testing:
- [Benchmark overview](docs/benchmark_overview.md)
- [Benchmark harness](docs/benchmark_harness.md)
- [SeaweedFS dataset testing](docs/dataset_seaweedfs_testing.md)
- [bench/README](bench/README.md)

## API Specification

The checked-in public OpenAPI specification is:

- [api/aapije/rest/openapiv3.yaml](api/aapije/rest/openapiv3.yaml)

The running `aapije` service also serves the generated specification at:

```text
/openapi3.json
```

and renders the interactive reference UI at:

```text
/reference
```

## Requirements

At minimum:
- Go for local builds
- PostgreSQL for the main platform
- Docker for the supplied local harnesses

Optional:
- an S3-compatible object store for dataset payloads
- an MQTT broker for ingestion testing

## License

Self-host is licensed under GPLv3. See [LICENSE](LICENSE).
