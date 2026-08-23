#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=smoke-lib.sh
source "${SCRIPT_DIR}/smoke-lib.sh"

IMAGE="${SMOKE_IMAGE:-ghcr.io/darthfork/promgithub:0.0.7}"
CLUSTER_NAME="${SMOKE_KIND_CLUSTER:-promgithub-smoke-${RANDOM}}"
NAMESPACE="${SMOKE_NAMESPACE:-promgithub-smoke}"
RELEASE_NAME="${SMOKE_RELEASE:-promgithub-smoke}"
LOCAL_PORT="${SMOKE_LOCAL_PORT:-18080}"
PORT_FORWARD_PID=""
PORT_FORWARD_LOG="$(mktemp)"
CLUSTER_CREATED=false

start_port_forward() {
  : >"$PORT_FORWARD_LOG"
  kubectl --namespace "$NAMESPACE" port-forward \
    "service/${RELEASE_NAME}" "${LOCAL_PORT}:8080" >"$PORT_FORWARD_LOG" 2>&1 &
  PORT_FORWARD_PID=$!

  for ((attempt = 1; attempt <= 30; attempt++)); do
    kill -0 "$PORT_FORWARD_PID" >/dev/null 2>&1 \
      || fail "port-forward exited before the service became reachable: $(cat "$PORT_FORWARD_LOG")"
    if curl --silent --show-error --fail "http://127.0.0.1:${LOCAL_PORT}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  fail "timed out waiting for port-forwarded service health"
}

stop_port_forward() {
  [[ -n "$PORT_FORWARD_PID" ]] || return 0
  kill "$PORT_FORWARD_PID" >/dev/null 2>&1 || true
  wait "$PORT_FORWARD_PID" >/dev/null 2>&1 || true
  PORT_FORWARD_PID=""
}

cluster_diagnostics() {
  [[ "$CLUSTER_CREATED" == true ]] || return 0

  printf '\nHelm smoke failure diagnostics\n' >&2
  kubectl --namespace "$NAMESPACE" get pods,services,endpoints,deployments >&2 || true
  kubectl --namespace "$NAMESPACE" describe pods >&2 || true
  kubectl --namespace "$NAMESPACE" logs \
    --selector "app.kubernetes.io/instance=${RELEASE_NAME}" \
    --all-containers --tail=100 >&2 || true
  kubectl --namespace "$NAMESPACE" get events --sort-by=.lastTimestamp >&2 || true
  if [[ -s "$PORT_FORWARD_LOG" ]]; then
    printf '\nPort-forward output\n' >&2
    cat "$PORT_FORWARD_LOG" >&2
  fi
}

cleanup() {
  local status=$?
  trap - EXIT

  if ((status != 0)); then
    cluster_diagnostics
  fi
  stop_port_forward
  if [[ "$CLUSTER_CREATED" == true ]]; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
  fi
  rm -f "$PORT_FORWARD_LOG"
  exit "$status"
}
trap cleanup EXIT

cd "$REPO_ROOT"

for command in curl docker helm kind kubectl openssl; do
  command -v "$command" >/dev/null || fail "required command not found: ${command}"
done

printf 'Linting and rendering Helm chart\n'
helm dependency build helm/promgithub
helm lint helm/promgithub \
  --set secrets.github_webhook_secret="$SMOKE_SECRET" \
  --set autoscaling.enabled=false
helm template "$RELEASE_NAME" helm/promgithub \
  --namespace "$NAMESPACE" \
  --set secrets.github_webhook_secret="$SMOKE_SECRET" \
  --set autoscaling.enabled=false \
  --set redis.enabled=true \
  --set redis.auth.password=system-smoke-redis >/dev/null

printf 'Creating kind cluster and loading %s\n' "$IMAGE"
kind create cluster --name "$CLUSTER_NAME" --wait 120s
CLUSTER_CREATED=true
kind load docker-image "$IMAGE" --name "$CLUSTER_NAME"

printf 'Installing standalone Helm deployment\n'
helm install "$RELEASE_NAME" helm/promgithub \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --set image.repository="${IMAGE%:*}" \
  --set image.tag="${IMAGE##*:}" \
  --set image.pullPolicy=Never \
  --set secrets.github_webhook_secret="$SMOKE_SECRET" \
  --set autoscaling.enabled=false \
  --wait \
  --timeout 3m

kubectl --namespace "$NAMESPACE" rollout status "deployment/${RELEASE_NAME}" --timeout=120s
start_port_forward
assert_service_smoke "http://127.0.0.1:${LOCAL_PORT}" "helm-standalone-${RANDOM}"

printf 'Upgrading Helm deployment to external Redis-backed mode\n'
kubectl --namespace "$NAMESPACE" create deployment redis --image=redis:7-alpine
kubectl --namespace "$NAMESPACE" expose deployment redis --port=6379
kubectl --namespace "$NAMESPACE" rollout status deployment/redis --timeout=120s

helm upgrade "$RELEASE_NAME" helm/promgithub \
  --namespace "$NAMESPACE" \
  --reuse-values \
  --set redisConfig.addr=redis:6379 \
  --wait \
  --timeout 3m

kubectl --namespace "$NAMESPACE" rollout status "deployment/${RELEASE_NAME}" --timeout=120s
stop_port_forward
start_port_forward
assert_service_smoke "http://127.0.0.1:${LOCAL_PORT}" "helm-redis-${RANDOM}"

printf 'Helm deployment smoke test passed\n'
