#!/usr/bin/env bash

set -euo pipefail

SMOKE_SECRET="${SMOKE_SECRET:-system-smoke-secret}"
SMOKE_REPOSITORY="${SMOKE_REPOSITORY:-user/repo}"

fail() {
  printf 'system smoke test failed: %s\n' "$*" >&2
  exit 1
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-60}"

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --silent --show-error --fail "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  fail "timed out waiting for ${url}"
}

wait_for_metric() {
  local url="$1"
  local metric="$2"
  local attempts="${3:-30}"

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --silent --show-error --fail "$url" | grep --fixed-strings --quiet "$metric"; then
      return 0
    fi
    sleep 1
  done

  fail "metric did not appear: ${metric}"
}

signature_for() {
  local payload="$1"
  printf 'sha256=%s' "$(openssl dgst -sha256 -hmac "$SMOKE_SECRET" "$payload" | awk '{print $NF}')"
}

assert_service_smoke() {
  local base_url="$1"
  local delivery_id="$2"
  local payload="${3:-test_data/push.json}"
  local health
  local signature

  wait_for_http "${base_url}/health"

  health="$(curl --silent --show-error --fail "${base_url}/health")"
  grep --fixed-strings --quiet '"status":"ok"' <<<"$health" || fail "unexpected health response: ${health}"

  curl --silent --show-error --fail "${base_url}/metrics" \
    | grep --fixed-strings --quiet 'promgithub_api_calls_total' \
    || fail "metrics endpoint did not expose service metrics"

  signature="$(signature_for "$payload")"
  curl --silent --show-error --fail \
    --request POST \
    --header 'Content-Type: application/json' \
    --header 'X-GitHub-Event: push' \
    --header "X-GitHub-Delivery: ${delivery_id}" \
    --header "X-Hub-Signature-256: ${signature}" \
    --data-binary "@${payload}" \
    "${base_url}/webhook" >/dev/null

  wait_for_metric \
    "${base_url}/metrics" \
    "promgithub_commit_pushed{repository=\"${SMOKE_REPOSITORY}\"} 1"
}
