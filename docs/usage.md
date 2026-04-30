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
- `PROMGITHUB_ENABLE_DETAILED_METRICS` (optional): When `true`, also emits higher-cardinality `*_detailed` metric families with labels such as `branch`, `workflow_name`, and `base_branch`. Default `false`.

If Redis is configured, the service stores delivery and run state in Redis.

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
PROMGITHUB_ENABLE_DETAILED_METRICS="true" \
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
  -e PROMGITHUB_ENABLE_DETAILED_METRICS=true \
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
      PROMGITHUB_ENABLE_DETAILED_METRICS: "true"
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
  metrics:
    enableDetailed: false
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
  metrics:
    enableDetailed: true
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
