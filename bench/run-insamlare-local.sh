#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/bench/docker/compose.yaml"
PG_URI="${PG_URI:-postgresql://postgres:mysecretpassword@127.0.0.1:5432/selfhost-bench?sslmode=disable}"
DEFAULT_LOAD_TOPIC_TEMPLATE='sensors/{{.UUID}}/temperature'
DEFAULT_TIMESTAMP_FORMAT='rfc3339'
DEFAULT_TENGO_TOPIC_TEMPLATE='factory/site{{.Index}}/modbus/temperature/{{.UUID}}'
DEFAULT_TENGO_PAYLOAD_TEMPLATE='{"meta":{"ts_uuid":"{{.UUID}}","observed_at":"{{.TimestampRFC3339}}","sequence":{{.Sequence}}},"registers":{"temperature":{"raw":{{printf "%.0f" .Value}},"scale":0.1,"offset":-40}}}'
BENCH_BIN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/insamlarebench.XXXXXX")"
SELFBENCH_BIN="$BENCH_BIN_DIR/selfbench"
INSAMLARE_BIN="$BENCH_BIN_DIR/insamlare"
INSAMLAREBENCH_BIN="$BENCH_BIN_DIR/insamlarebench"
MQTTD_BIN="$BENCH_BIN_DIR/mqttd"
MANIFEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/insamlare-manifest.XXXXXX")"
MANIFEST_PATH="${MANIFEST_PATH:-$MANIFEST_DIR/manifest.json}"
CONFIG_PATH="$BENCH_BIN_DIR/insamlare-load.yaml"
LOG_PATH="${INSAMLARE_LOG_PATH:-$BENCH_BIN_DIR/insamlare.log}"
BROKER_LOG_PATH="${BROKER_LOG_PATH:-$BENCH_BIN_DIR/broker.log}"
INSAMLARE_PID=""
BROKER_PID=""
INSAMLARE_TIMESTAMP_FORMAT="${INSAMLARE_TIMESTAMP_FORMAT:-$DEFAULT_TIMESTAMP_FORMAT}"
LOAD_TOPIC_TEMPLATE="${LOAD_TOPIC_TEMPLATE:-}"
LOAD_TIMESTAMP_FORMAT="${LOAD_TIMESTAMP_FORMAT:-}"
MQTT_HOST="${MQTT_HOST:-127.0.0.1}"
MOSQUITTO_PORT="${MOSQUITTO_PORT:-}"
EMQX_PORT="${EMQX_PORT:-}"
MQTT_BROKER=""
BROKER_IMPL="${BROKER_IMPL:-mosquitto}"
BROKER_SERVICE=""
BROKER_PORT=""
INSAMLARE_ROUTE_MODE="${INSAMLARE_ROUTE_MODE:-fixed}"
LOAD_PAYLOAD_TEMPLATE="${LOAD_PAYLOAD_TEMPLATE:-}"
LOAD_ARGS=()
KEEP_ARTIFACTS="${KEEP_ARTIFACTS:-0}"

cleanup() {
  if [[ -n "$INSAMLARE_PID" ]] && kill -0 "$INSAMLARE_PID" >/dev/null 2>&1; then
    kill "$INSAMLARE_PID" >/dev/null 2>&1 || true
    wait "$INSAMLARE_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BROKER_PID" ]] && kill -0 "$BROKER_PID" >/dev/null 2>&1; then
    kill "$BROKER_PID" >/dev/null 2>&1 || true
    wait "$BROKER_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_ARTIFACTS" == "1" ]]; then
    echo "kept artifacts in: $BENCH_BIN_DIR"
    echo "kept manifest dir: $MANIFEST_DIR"
    return
  fi
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

port_is_open() {
  local host="$1"
  local port="$2"
  bash -lc "exec 3<>/dev/tcp/$host/$port" >/dev/null 2>&1
}

wait_for_port() {
  local host="$1"
  local port="$2"
  local retries="${3:-60}"
  local i
  for ((i=0; i<retries; i++)); do
    if port_is_open "$host" "$port"; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $host:$port" >&2
  exit 1
}

pick_free_port() {
  local start="${1:-1883}"
  local end="${2:-1899}"
  local port
  for ((port=start; port<=end; port++)); do
    if ! port_is_open 127.0.0.1 "$port"; then
      printf '%s\n' "$port"
      return 0
    fi
  done
  echo "failed to find a free port in range ${start}-${end}" >&2
  exit 1
}

wait_for_process() {
  local pid="$1"
  local retries="${2:-20}"
  local i
  for ((i=0; i<retries; i++)); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "insamlare exited early; log follows:" >&2
      cat "$LOG_PATH" >&2 || true
      exit 1
    fi
    if grep -q '"Configured route"' "$LOG_PATH" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for insamlare startup; current log follows:" >&2
  cat "$LOG_PATH" >&2 || true
  exit 1
}

wait_for_broker() {
  local host="$1"
  local port="$2"
  local retries="${3:-60}"
  local i
  for ((i=0; i<retries; i++)); do
    if port_is_open "$host" "$port"; then
      return 0
    fi
    if [[ -n "$BROKER_PID" ]] && ! kill -0 "$BROKER_PID" >/dev/null 2>&1; then
      echo "broker exited early; log follows:" >&2
      cat "$BROKER_LOG_PATH" >&2 || true
      exit 1
    fi
    sleep 1
  done
  echo "timed out waiting for $host:$port" >&2
  if [[ -f "$BROKER_LOG_PATH" ]]; then
    echo "broker log follows:" >&2
    cat "$BROKER_LOG_PATH" >&2 || true
  fi
  exit 1
}

require docker
require go
require psql

cd "$ROOT_DIR"

case "$BROKER_IMPL" in
  mosquitto)
    BROKER_SERVICE="mosquitto"
    if [[ -z "$MOSQUITTO_PORT" ]]; then
      MOSQUITTO_PORT="$(pick_free_port 1883 1899)"
    fi
    BROKER_PORT="$MOSQUITTO_PORT"
    export MOSQUITTO_PORT
    ;;
  emqx)
    BROKER_SERVICE="emqx"
    if [[ -z "$EMQX_PORT" ]]; then
      EMQX_PORT="$(pick_free_port 1883 1899)"
    fi
    BROKER_PORT="$EMQX_PORT"
    export EMQX_PORT
    ;;
  gomqtt)
    BROKER_SERVICE=""
    BROKER_PORT="$(pick_free_port 1883 1899)"
    ;;
  *)
    echo "unsupported BROKER_IMPL: $BROKER_IMPL" >&2
    exit 1
    ;;
esac

MQTT_BROKER="${MQTT_BROKER:-tcp://${MQTT_HOST}:${BROKER_PORT}}"

echo "bringing up postgres"
docker compose -f "$COMPOSE_FILE" up -d postgres

if [[ -n "$BROKER_SERVICE" ]]; then
  echo "bringing up ${BROKER_IMPL} on ${BROKER_PORT}"
  docker compose -f "$COMPOSE_FILE" up -d "$BROKER_SERVICE"
else
  echo "broker ${BROKER_IMPL} will run as a local Go process on ${BROKER_PORT}"
fi

echo "waiting for postgres"
wait_for_port 127.0.0.1 5432

echo "applying migrations"
migrate_output=""
if ! migrate_output="$(go run ./cmd/selfctl db migrate up --database "$PG_URI" 2>&1)"; then
  if [[ "$migrate_output" != *"no change"* ]] && [[ "$migrate_output" != *"Already on the latest version."* ]]; then
    printf '%s\n' "$migrate_output" >&2
    exit 1
  fi
fi

echo "building load-test binaries"
go build -o "$SELFBENCH_BIN" ./cmd/selfbench
go build -o "$INSAMLARE_BIN" ./cmd/insamlare
go build -o "$INSAMLAREBENCH_BIN" ./cmd/insamlarebench
if [[ "$BROKER_IMPL" == "gomqtt" ]]; then
  go build -o "$MQTTD_BIN" ./cmd/mqttd
fi

echo "seeding benchmark dataset"
"$SELFBENCH_BIN" seed \
  --pg-uri "$PG_URI" \
  --domain test \
  --series "${BENCH_SERIES:-64}" \
  --points "${BENCH_POINTS:-4320}" \
  --batch "${BENCH_BATCH:-512}" \
  --step "${BENCH_STEP:-10m}" \
  --start "${BENCH_START:-2024-01-01T00:00:00Z}" \
  --manifest "$MANIFEST_PATH"

case "$INSAMLARE_ROUTE_MODE" in
  fixed)
    if [[ -z "$LOAD_TOPIC_TEMPLATE" ]]; then
      LOAD_TOPIC_TEMPLATE="$DEFAULT_LOAD_TOPIC_TEMPLATE"
    fi
    if [[ -z "$LOAD_TIMESTAMP_FORMAT" ]]; then
      LOAD_TIMESTAMP_FORMAT="$INSAMLARE_TIMESTAMP_FORMAT"
    fi
    cat >"$CONFIG_PATH" <<EOF
mqtt:
  broker: ${MQTT_BROKER}
  client_id: selfhost-insamlare-load
  qos: ${INSAMLARE_QOS:-1}
  clean_session: false

postgres:
  dsn: ${PG_URI}
  created_by_uuid: 00000000-0000-1000-8000-000000000000

ingest:
  batch_size: ${INSAMLARE_BATCH_SIZE:-5000}
  flush_interval: ${INSAMLARE_FLUSH_INTERVAL:-500ms}
  workers: ${INSAMLARE_WORKERS:-8}
  queue_size: ${INSAMLARE_QUEUE_SIZE:-50000}

load_log:
  interval: ${INSAMLARE_LOAD_LOG_INTERVAL:-10s}

transform:
  timeout: ${INSAMLARE_TRANSFORM_TIMEOUT:-250ms}

routes:
  - name: dynamic_json_value
    topic_regex: ^sensors/([^/]+)/([^/]+)$
    timeseries_uuid: \$1
    payload_format: json
    value_key: \$2
    timestamp_key: ts
    timestamp_format: ${INSAMLARE_TIMESTAMP_FORMAT}
EOF
    ;;
  tengo)
    if [[ -z "$LOAD_TOPIC_TEMPLATE" ]]; then
      LOAD_TOPIC_TEMPLATE="$DEFAULT_TENGO_TOPIC_TEMPLATE"
    fi
    if [[ -z "$LOAD_TIMESTAMP_FORMAT" ]]; then
      LOAD_TIMESTAMP_FORMAT="rfc3339"
    fi
    if [[ -z "$LOAD_PAYLOAD_TEMPLATE" ]]; then
      LOAD_PAYLOAD_TEMPLATE="$DEFAULT_TENGO_PAYLOAD_TEMPLATE"
    fi
    SCRIPT_PATH="$BENCH_BIN_DIR/modbus_transform.tengo"
    cat >"$SCRIPT_PATH" <<'EOF'
payload_map := import("json").decode(payload)
meta := payload_map["meta"]
registers := payload_map["registers"]
temperature := registers["temperature"]
raw := temperature["raw"]
scale := temperature["scale"]
offset := temperature["offset"]

points = [{
  ts_uuid: meta["ts_uuid"],
  value: raw * scale + offset,
  ts: meta["observed_at"]
}]
EOF
    cat >"$CONFIG_PATH" <<EOF
mqtt:
  broker: ${MQTT_BROKER}
  client_id: selfhost-insamlare-load
  qos: ${INSAMLARE_QOS:-1}
  clean_session: false

postgres:
  dsn: ${PG_URI}
  created_by_uuid: 00000000-0000-1000-8000-000000000000

ingest:
  batch_size: ${INSAMLARE_BATCH_SIZE:-5000}
  flush_interval: ${INSAMLARE_FLUSH_INTERVAL:-500ms}
  workers: ${INSAMLARE_WORKERS:-8}
  queue_size: ${INSAMLARE_QUEUE_SIZE:-50000}

load_log:
  interval: ${INSAMLARE_LOAD_LOG_INTERVAL:-10s}

transform:
  timeout: ${INSAMLARE_TRANSFORM_TIMEOUT:-250ms}

routes:
  - name: scripted_modbus_temperature
    topic: factory/+/modbus/temperature/+
    script_path: ${SCRIPT_PATH}
EOF
    ;;
  *)
    echo "unsupported INSAMLARE_ROUTE_MODE: $INSAMLARE_ROUTE_MODE" >&2
    exit 1
    ;;
esac

if [[ -n "$LOAD_PAYLOAD_TEMPLATE" ]]; then
  LOAD_ARGS+=(--payload-template "$LOAD_PAYLOAD_TEMPLATE")
fi

if [[ "$BROKER_IMPL" == "gomqtt" ]]; then
  echo "starting gomqtt broker"
  "$MQTTD_BIN" \
    --listen ":${BROKER_PORT}" \
    --write-buffer "${GOMQTT_WRITE_BUFFER:-65536}" \
    --read-buffer "${GOMQTT_READ_BUFFER:-65536}" \
    --max-pending "${GOMQTT_MAX_PENDING:-16384}" \
    --sys-topic-seconds "${GOMQTT_SYS_TOPIC_SECONDS:-30}" \
    >"$BROKER_LOG_PATH" 2>&1 &
  BROKER_PID="$!"
fi

echo "waiting for ${BROKER_IMPL}"
if [[ -n "$BROKER_PID" ]]; then
  wait_for_broker "$MQTT_HOST" "$BROKER_PORT"
else
  wait_for_port "$MQTT_HOST" "$BROKER_PORT"
fi

echo "starting insamlare"
(
  cd "$BENCH_BIN_DIR"
  CONFIG_FILENAME=insamlare-load "$INSAMLARE_BIN" >"$LOG_PATH" 2>&1
) &
INSAMLARE_PID="$!"
wait_for_process "$INSAMLARE_PID"

echo "running insamlare load test"
"$INSAMLAREBENCH_BIN" run \
  --broker "$MQTT_BROKER" \
  --pg-uri "$PG_URI" \
  --manifest "$MANIFEST_PATH" \
  --series-limit "${LOAD_SERIES_LIMIT:-${BENCH_SERIES:-64}}" \
  --workers "${LOAD_WORKERS:-32}" \
  --duration "${LOAD_DURATION:-30s}" \
  --points-per-message "${LOAD_POINTS_PER_MESSAGE:-1}" \
  --qos "${LOAD_QOS:-1}" \
  --topic-template "${LOAD_TOPIC_TEMPLATE}" \
  --timestamp-format "${LOAD_TIMESTAMP_FORMAT}" \
  "${LOAD_ARGS[@]}" \
  --report-interval "${LOAD_REPORT_INTERVAL:-5s}" \
  --settle "${LOAD_SETTLE:-5s}"

echo "insamlare log: $LOG_PATH"
if [[ -n "$BROKER_PID" ]]; then
  echo "broker log: $BROKER_LOG_PATH"
fi
LOAD_ARGS=()
