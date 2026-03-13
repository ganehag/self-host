#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/bench/docker/compose.yaml"
SCENARIO_PATH="${1:-bench/scenarios/local-smoke.yaml}"
PG_URI="${PG_URI:-postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-bench?sslmode=disable}"
BENCH_BIN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/selfbench.XXXXXX")"
BENCH_BIN="$BENCH_BIN_DIR/selfbench"
MANIFEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/selfbench-manifest.XXXXXX")"
MANIFEST_PATH="${MANIFEST_PATH:-$MANIFEST_DIR/manifest.json}"

cleanup() {
  rm -rf "$BENCH_BIN_DIR"
  rm -rf "$MANIFEST_DIR"
}

trap cleanup EXIT

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

wait_for_http() {
  local url="$1"
  local retries="${2:-60}"
  local i
  for ((i=0; i<retries; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  exit 1
}

require docker
require go
require curl

cd "$ROOT_DIR"

echo "bringing up benchmark stack"
docker compose -f "$COMPOSE_FILE" up -d postgres
docker compose -f "$COMPOSE_FILE" up -d --build --force-recreate aapije

echo "waiting for api"
wait_for_http "http://127.0.0.1:8080/status"

echo "applying migrations"
migrate_output=""
if ! migrate_output="$(go run ./cmd/selfctl db migrate up --database "$PG_URI" 2>&1)"; then
  if [[ "$migrate_output" != *"no change"* ]] && [[ "$migrate_output" != *"Already on the latest version."* ]]; then
    printf '%s\n' "$migrate_output" >&2
    exit 1
  fi
fi

echo "building selfbench"
go build -o "$BENCH_BIN" ./cmd/selfbench

echo "seeding benchmark dataset"
"$BENCH_BIN" seed \
  --pg-uri "$PG_URI" \
  --domain test \
  --series "${BENCH_SERIES:-64}" \
  --points "${BENCH_POINTS:-4320}" \
  --batch "${BENCH_BATCH:-512}" \
  --step "${BENCH_STEP:-10m}" \
  --start "${BENCH_START:-2024-01-01T00:00:00Z}" \
  --manifest "$MANIFEST_PATH"

echo "running scenario: $SCENARIO_PATH"
"$BENCH_BIN" run \
  --config "$SCENARIO_PATH" \
  --manifest "$MANIFEST_PATH" \
  --pg-uri "$PG_URI"
