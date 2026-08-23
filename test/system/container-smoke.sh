#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=smoke-lib.sh
source "${SCRIPT_DIR}/smoke-lib.sh"

IMAGE="${SMOKE_IMAGE:-ghcr.io/darthfork/promgithub:0.0.7}"
CONTAINER_NAME="promgithub-system-smoke-${RANDOM}"
FAILURE_CONTAINER_NAME="${CONTAINER_NAME}-missing-config"
LOG_FILE="$(mktemp)"

cleanup() {
  docker rm --force "$CONTAINER_NAME" "$FAILURE_CONTAINER_NAME" >/dev/null 2>&1 || true
  rm -f "$LOG_FILE"
}
trap cleanup EXIT

cd "$REPO_ROOT"

printf 'Starting black-box container smoke test for %s\n' "$IMAGE"
docker run --detach --name "$CONTAINER_NAME" --publish 127.0.0.1::8080 \
  --env "PROMGITHUB_WEBHOOK_SECRET=${SMOKE_SECRET}" \
  "$IMAGE" >/dev/null

HOST_PORT="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "8080/tcp") 0).HostPort}}' "$CONTAINER_NAME")"
assert_service_smoke "http://127.0.0.1:${HOST_PORT}" "container-smoke-${RANDOM}"

printf 'Verifying startup rejects missing required configuration\n'
if docker run --name "$FAILURE_CONTAINER_NAME" "$IMAGE" >"$LOG_FILE" 2>&1; then
  fail 'container started without PROMGITHUB_WEBHOOK_SECRET'
fi

grep --fixed-strings --quiet 'PROMGITHUB_WEBHOOK_SECRET is not set' "$LOG_FILE" \
  || fail 'missing-secret startup failure did not explain the invalid configuration'

printf 'Container smoke test passed\n'
