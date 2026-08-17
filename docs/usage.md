# Using `promgithub` service

## Overview

`promgithub` receives GitHub webhook events and exposes Prometheus metrics over HTTP.

It can be deployed:
- as a single instance with only the webhook secret configured
- as a multi-instance deployment with Redis configured for shared deduplication and state

## Configuration

### Environment variables

The service supports the following environment variables:

- `PROMGITHUB_WEBHOOK_SECRET`: Secret used to validate incoming GitHub webhook requests.
- `PROMGITHUB_SERVICE_PORT` (optional): HTTP port for the service, default `8080`.
- `PROMGITHUB_REDIS_ADDR` (optional): Redis address in `host:port` form.
- `PROMGITHUB_REDIS_PASSWORD` (optional): Redis password.
- `PROMGITHUB_REDIS_DB` (optional): Redis database number, default `0`.
- `PROMGITHUB_REDIS_KEY_PREFIX` (optional): Prefix used for Redis keys, default `promgithub`.
- `PROMGITHUB_REDIS_DELIVERY_TTL` (optional): TTL for webhook delivery dedupe keys, default `24h`.
- `PROMGITHUB_EVENT_WORKERS` (optional): Number of async webhook processing workers, default `4`.
- `PROMGITHUB_EVENT_QUEUE_SIZE` (optional): Bounded async webhook queue size, default `256`.
- `PROMGITHUB_REPO_ALLOWLIST` (optional): Comma-separated `owner/repo` list. When set, only these repositories are recorded.
- `PROMGITHUB_REPO_DENYLIST` (optional): Comma-separated `owner/repo` list. Matching repositories are dropped.
- `PROMGITHUB_BRANCH_ALLOW_REGEX` (optional): If set, only events whose branch matches this regular expression are recorded.
- `PROMGITHUB_BRANCH_DENY_REGEX` (optional): If set, events whose branch matches this regular expression are dropped.
- `PROMGITHUB_WORKFLOW_ALLOW_REGEX` (optional): If set, only workflow and job events whose workflow name matches are recorded. Push and pull request events skip this filter.
- `PROMGITHUB_WORKFLOW_DENY_REGEX` (optional): If set, matching workflow and job events are dropped.
- `PROMGITHUB_NORMALIZE_BRANCHES` (optional): When `true`, replace raw branch labels with `default`, `release`, or `feature`. Default `false`.
- `PROMGITHUB_DEFAULT_BRANCHES` (optional): Comma-separated branch names classified as `default` when normalization is enabled. Default `main,master`.
- `PROMGITHUB_RELEASE_BRANCH_REGEX` (optional): Branches matching this expression are classified as `release` when normalization is enabled. Default `^(release/|hotfix/).+`.

If Redis is configured, the service stores delivery and run state in Redis.

### Label normalization and event filtering

Filtered events are still signature-checked, deduplicated, and acknowledged so GitHub does not retry them. They do not update business metrics or run state. Drops are counted on `promgithub_event_filtered_total{event_type,reason}` with `reason` of `repository`, `branch`, or `workflow`.

Branch filters apply to:

- workflow and job `head_branch`
- push refs after stripping `refs/heads/` or `refs/tags/`
- pull request `base` refs

Allowlists and denylists can be combined. A repository must be in the allowlist when one is set, and must not be in the denylist.

Enabling `PROMGITHUB_NORMALIZE_BRANCHES` changes the `branch` and `base_branch` label values:

| Class | Default match |
| --- | --- |
| `default` | `main` or `master`, or names in `PROMGITHUB_DEFAULT_BRANCHES` |
| `release` | `release/*` or `hotfix/*`, or `PROMGITHUB_RELEASE_BRANCH_REGEX` |
| `feature` | every other non-empty branch |

This is a scrape-breaking change for existing dashboards. Enable it on new deployments, or expect old raw-branch series to go stale.

Recommended production starting point for a multi-repo organization:

```bash
PROMGITHUB_REPO_ALLOWLIST="acme/api,acme/web,acme/worker"
PROMGITHUB_BRANCH_DENY_REGEX="^(dependabot/|renovate/)"
PROMGITHUB_NORMALIZE_BRANCHES="true"
```

Keep `PROMGITHUB_NORMALIZE_BRANCHES=false` when you still need per-branch workflow health. In that case, prefer `PROMGITHUB_BRANCH_ALLOW_REGEX` such as `^(main|release/.+)$` so feature-branch series do not accumulate.

### Async processing and backpressure

Webhook requests are acknowledged after signature validation, duplicate-delivery recording, and enqueueing into the bounded async processor.

- Accepted events return `202 Accepted` and are processed by background workers.
- Duplicate deliveries return `200 OK` and do not enqueue duplicate work.
- If the queue is full, the request returns `503 Service Unavailable` and increments `promgithub_event_queue_dropped_total{event_type="<event>"}`.
- Processing panics are recovered, logged, and exposed via `promgithub_event_processing_failures_total`; workers continue handling later events.
- On graceful termination, the processor stops accepting new events and drains accepted in-flight/queued events before exit.

Watch `promgithub_event_queue_depth`, `promgithub_event_queue_capacity`, `promgithub_event_worker_count`, `promgithub_event_processed_total`, `promgithub_event_queue_dropped_total`, `promgithub_event_unsupported_total`, and `promgithub_event_processing_failures_total` to tune worker and queue settings.

## Running the service

### Run the binary

```bash
PROMGITHUB_WEBHOOK_SECRET="<your webhook secret>" \
PROMGITHUB_SERVICE_PORT="8080" \
/path/to/binary/promgithub
```

### Run the binary with Redis

```bash
PROMGITHUB_WEBHOOK_SECRET="<your webhook secret>" \
PROMGITHUB_REDIS_ADDR="<redis-host:6379>" \
PROMGITHUB_REDIS_PASSWORD="<redis password>" \
PROMGITHUB_REDIS_DB="0" \
PROMGITHUB_REDIS_KEY_PREFIX="promgithub" \
PROMGITHUB_REDIS_DELIVERY_TTL="24h" \
PROMGITHUB_SERVICE_PORT="8080" \
/path/to/binary/promgithub
```

### Docker

```bash
docker run \
  -e PROMGITHUB_WEBHOOK_SECRET=<your webhook secret> \
  -e PROMGITHUB_SERVICE_PORT=8080 \
  -p 8080:8080 \
  ghcr.io/darthfork/promgithub:<version>
```

### Docker with Redis

```bash
docker run \
  -e PROMGITHUB_WEBHOOK_SECRET=<your webhook secret> \
  -e PROMGITHUB_REDIS_ADDR=<redis-host:6379> \
  -e PROMGITHUB_REDIS_PASSWORD=<redis password> \
  -e PROMGITHUB_REDIS_DB=0 \
  -e PROMGITHUB_REDIS_KEY_PREFIX=promgithub \
  -e PROMGITHUB_REDIS_DELIVERY_TTL=24h \
  -e PROMGITHUB_SERVICE_PORT=8080 \
  -p 8080:8080 \
  ghcr.io/darthfork/promgithub:<version>
```

### Docker Compose with Redis

```yaml
services:
  redis:
    image: redis:7
    command: ["redis-server", "--appendonly", "yes"]
    ports:
      - "6379:6379"

  promgithub:
    image: ghcr.io/darthfork/promgithub:<version>
    environment:
      PROMGITHUB_WEBHOOK_SECRET: <your webhook secret>
      PROMGITHUB_REDIS_ADDR: redis:6379
      PROMGITHUB_REDIS_PASSWORD: <redis password>
      PROMGITHUB_REDIS_DB: 0
      PROMGITHUB_REDIS_KEY_PREFIX: promgithub
      PROMGITHUB_REDIS_DELIVERY_TTL: 24h
      PROMGITHUB_SERVICE_PORT: 8080
    ports:
      - "8080:8080"
    depends_on:
      - redis
```

## Deploying with Kubernetes

`promgithub` includes a Helm chart.

### Add the chart dependency

```yaml
apiVersion: v2
name: promgithub
description: Deployment of promgithub
type: application
version: <chart version>

dependencies:
  - name: promgithub
    version: "<promgithub-charts version>"
    repository: "oci://ghcr.io/darthfork/promgithub-charts"
```

### Values for an external Redis instance

```yaml
promgithub:
  secrets:
    github_webhook_secret: <your webhook secret>
    redis_password: <redis password>
  redisConfig:
    addr: redis.example.internal:6379
    db: 0
    keyPrefix: promgithub
    deliveryTTL: 24h
```

### Values for a bundled Redis deployment

```yaml
promgithub:
  secrets:
    github_webhook_secret: <your webhook secret>
  redis:
    enabled: true
    auth:
      enabled: true
      password: <redis password>
  redisConfig:
    db: 0
    keyPrefix: promgithub
    deliveryTTL: 24h
  labelPolicy:
    repoAllowlist: "acme/api,acme/web"
    branchDenyRegex: "^(dependabot/|renovate/)"
    normalizeBranches: true
```

When `redis.enabled=true`, the chart deploys Redis as a dependency and configures `promgithub` to connect to it automatically.

### Ingress

Expose the `/webhook` endpoint to GitHub using your preferred Kubernetes ingress setup.

## Setting up the GitHub webhook

1. Navigate to your GitHub repository or organization settings.
2. Under **Settings**, open **Webhooks** and click **Add webhook**.
3. Set the payload URL to your `promgithub` webhook endpoint, for example `https://<your-service-url>/webhook`.
4. Set **Content type** to `application/json`.
5. Set the **Secret** to the value used for `PROMGITHUB_WEBHOOK_SECRET`.
6. Subscribe to these events:
   - **push**
   - **pull request**
   - **workflow job**
   - **workflow runs**
7. Save the webhook.

## Scraping metrics

`promgithub` exposes Prometheus metrics on `/metrics`.

### Prometheus configuration

```yaml
scrape_configs:
  - job_name: 'promgithub'
    scrape_interval: 15s
    metrics_path: '/metrics'
    static_configs:
      - targets: ['promgithub:8080']
        labels:
          service: 'promgithub'
```

### VictoriaMetrics configuration

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  name: promgithub
  namespace: promgithub
spec:
  endpoints:
    - path: /metrics
      interval: 15s
      port: http
```
