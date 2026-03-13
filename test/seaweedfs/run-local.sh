#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="$ROOT_DIR/test/seaweedfs"
COMPOSE_FILE="$TEST_DIR/docker/compose.yaml"
PG_URI="${PG_URI:-postgresql://postgres:mysecretpassword@127.0.0.1:55432/selfhost-seaweed?sslmode=disable}"
API_URL="${API_URL:-http://127.0.0.1:18080}"
AUTH="${AUTH:-test:root}"
DATASET_BUCKET="${DATASET_BUCKET:-selfhost-datasets}"

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

ensure_database() {
  local retries="${1:-30}"
  local i
  for ((i=0; i<retries; i++)); do
    if docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U postgres -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'selfhost-seaweed'" | grep -q 1; then
      return 0
    fi
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U postgres -d postgres -c "CREATE DATABASE \"selfhost-seaweed\"" >/dev/null 2>&1 || true
    sleep 1
  done
  echo "timed out creating selfhost-seaweed database" >&2
  exit 1
}

wait_for_seaweed_s3() {
  local retries="${1:-60}"
  local i
  for ((i=0; i<retries; i++)); do
    if docker compose -f "$COMPOSE_FILE" exec -T mc mc alias set local http://seaweedfs:8333 selfhost selfhost-secret >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for SeaweedFS S3 endpoint" >&2
  exit 1
}

json_field() {
  local key="$1"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p"
}

md5hex() {
  md5sum "$1" | awk '{print $1}'
}

require docker
require go
require curl
require md5sum
require base64
require dd

cd "$ROOT_DIR"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/selfhost-seaweed.XXXXXX")"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

small_payload="small-s3-roundtrip"
small_payload_b64="$(printf '%s' "$small_payload" | base64 -w0)"

part1="$tmpdir/part1.bin"
part2="$tmpdir/part2.bin"
cat_payload="$tmpdir/full.bin"
dd if=/dev/zero of="$part1" bs=1M count=5 status=none
printf 'tail-data\n' > "$part2"
cat "$part1" "$part2" > "$cat_payload"

part1_md5="$(md5hex "$part1")"
part2_md5="$(md5hex "$part2")"
full_md5="$(md5hex "$cat_payload")"

echo "bringing up SeaweedFS dataset test stack"
docker compose -f "$COMPOSE_FILE" up -d postgres seaweedfs
docker compose -f "$COMPOSE_FILE" up -d --build --force-recreate aapije mc

echo "waiting for api"
wait_for_http "$API_URL/status"

echo "ensuring database exists"
ensure_database

echo "applying migrations"
migrate_output=""
if ! migrate_output="$(go run ./cmd/selfctl db migrate up --database "$PG_URI" 2>&1)"; then
  if [[ "$migrate_output" != *"no change"* ]] && [[ "$migrate_output" != *"Already on the latest version."* ]]; then
    printf '%s\n' "$migrate_output" >&2
    exit 1
  fi
fi

echo "waiting for SeaweedFS S3"
wait_for_seaweed_s3

echo "provisioning SeaweedFS bucket"
docker compose -f "$COMPOSE_FILE" exec -T mc mc mb --ignore-existing "local/$DATASET_BUCKET" >/dev/null

echo "creating direct dataset"
create_response="$tmpdir/create.json"
curl -fsS -u "$AUTH" \
  -H 'Content-Type: application/json' \
  -X POST "$API_URL/v2/datasets" \
  -d "{\"name\":\"seaweed-small\",\"format\":\"json\",\"content\":\"$small_payload_b64\"}" \
  > "$create_response"

dataset_uuid="$(json_field uuid < "$create_response")"
if [[ -z "$dataset_uuid" ]]; then
  echo "failed to parse dataset uuid from create response" >&2
  cat "$create_response" >&2
  exit 1
fi

echo "verifying direct dataset download"
download_small="$tmpdir/download-small.bin"
curl -fsS -u "$AUTH" "$API_URL/v2/datasets/$dataset_uuid/raw" -o "$download_small"
if [[ "$(cat "$download_small")" != "$small_payload" ]]; then
  echo "direct dataset payload mismatch" >&2
  exit 1
fi

echo "initializing multipart upload"
upload_init="$tmpdir/upload-init.json"
curl -fsS -u "$AUTH" -X POST "$API_URL/v2/datasets/$dataset_uuid/uploads" > "$upload_init"
upload_id="$(json_field uploadId < "$upload_init")"
if [[ -z "$upload_id" ]]; then
  echo "failed to parse upload id" >&2
  cat "$upload_init" >&2
  exit 1
fi

echo "uploading multipart dataset"
curl -fsS -u "$AUTH" -X PUT \
  -H "Content-MD5: $part1_md5" \
  --data-binary "@$part1" \
  "$API_URL/v2/datasets/$dataset_uuid/parts?partNumber=1&uploadId=$upload_id" >/dev/null

curl -fsS -u "$AUTH" -X PUT \
  -H "Content-MD5: $part2_md5" \
  --data-binary "@$part2" \
  "$API_URL/v2/datasets/$dataset_uuid/parts?partNumber=2&uploadId=$upload_id" >/dev/null

echo "assembling multipart dataset"
curl -fsS -u "$AUTH" -X POST \
  -H "Content-MD5: $full_md5" \
  "$API_URL/v2/datasets/$dataset_uuid/assemble?partNumber=1&uploadId=$upload_id" >/dev/null

echo "verifying assembled dataset download"
download_full="$tmpdir/download-full.bin"
curl -fsS -u "$AUTH" "$API_URL/v2/datasets/$dataset_uuid/raw" -o "$download_full"
cmp "$cat_payload" "$download_full"

echo "verifying object exists in SeaweedFS"
object_listing="$tmpdir/object-list.txt"
docker compose -f "$COMPOSE_FILE" exec -T mc mc find "local/$DATASET_BUCKET/datasets/test/$dataset_uuid" > "$object_listing"
if ! grep -q '/content$' "$object_listing"; then
  echo "expected dataset object not found in SeaweedFS bucket" >&2
  cat "$object_listing" >&2
  exit 1
fi

echo "deleting dataset"
curl -fsS -u "$AUTH" -X DELETE "$API_URL/v2/datasets/$dataset_uuid" >/dev/null

echo "verifying object was removed from SeaweedFS"
if docker compose -f "$COMPOSE_FILE" exec -T mc mc find "local/$DATASET_BUCKET/datasets/test/$dataset_uuid" | grep -q .; then
  echo "dataset objects still present after delete" >&2
  exit 1
fi

echo "SeaweedFS dataset smoke test passed"
