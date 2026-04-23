# Using `promgithub` service

## Deploying the service

The service can be deployed in your choice of infrastructure. To allow webhooks to be pushed to `promgithub` make sure your service deployment is accessible from your Github instance.

### Service Parameters

The service expects the following parameters to be set:

- **Environment Variables**:
  - `PROMGITHUB_WEBHOOK_SECRET`: The secret used to validate incoming GitHub webhook requests.
  - `PROMGITHUB_SERVICE_PORT` (optional): Service API port (default is `8080`).
  - `PROMGITHUB_REDIS_ADDR` (optional): Redis address in `host:port` form. Enables shared-state multi-instance mode.
  - `PROMGITHUB_REDIS_PASSWORD` (optional): Redis password.
  - `PROMGITHUB_REDIS_DB` (optional): Redis database number, default `0`.
  - `PROMGITHUB_REDIS_KEY_PREFIX` (optional): Prefix for Redis keys, default `promgithub`.
  - `PROMGITHUB_REDIS_DELIVERY_TTL` (optional): TTL for webhook delivery dedupe keys, default `24h`.

If Redis is not configured, `promgithub` runs without shared state and is best suited to single-instance deployments.

## Redis-backed multi-instance mode

To support horizontal scaling, configure all `promgithub` replicas to use the same Redis instance.

Redis is used for:
- deduplicating webhook deliveries with `X-GitHub-Delivery`
- persisting workflow run state by GitHub `run_id`
- persisting workflow job state by GitHub job `id`

### Docker CLI with Redis

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
      PROMGITHUB_SERVICE_PORT: 8080
    ports:
      - "8080:8080"
    depends_on:
      - redis
```

## Deploying in kubernetes

To deploy the service in a kubernetes cluster you can use the provided helm chart from the [promgithub chart repository](https://github.com/darthfork/promgithub/pkgs/container/promgithub-charts%2Fpromgithub)

### Chart.yaml

Add the helm repository as a dependency to your chart deployment:

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

### Values

Create a values file with the webhook secret and your Redis configuration.

#### Use an external Redis instance

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

#### One-stop deployment with bundled Redis

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
```

When `redis.enabled=true`, the chart configures `promgithub` to connect to the bundled Redis release service automatically.

Note: the chart now declares a Redis Helm dependency for this flow. Depending on your Helm environment, you may need to run `helm dependency update helm/promgithub` with registry access before packaging the chart.

### Ingress

Add an Ingress configuration allowing your github instance to access `promgithub` deployment. More details can be found [here](https://kubernetes.io/docs/concepts/services-networking/ingress/)

### Metrics Scraping

Create prometheus configuration resource with the chart for scraping metrics from the `/metrics` endpoint from promgithub service. For more details see the [Prometheus scraping configuration](#prometheus-scraping-configuration) below.

## Deploying service binary

The service binaries are also available under [github releases](https://github.com/darthfork/promgithub/releases) which can be deployed as the user wishes.

```bash
PROMGITHUB_WEBHOOK_SECRET="<your webhook secret>" \
PROMGITHUB_REDIS_ADDR="<redis-host:6379>" \
PROMGITHUB_SERVICE_PORT="8080" \
/path/to/binary/promgithub
```

## Setting up the Webhook in GitHub (Repository/Organization)

1. Navigate to your GitHub repository or organization settings.
2. Under **Settings**, find **Webhooks** and click **Add webhook**.
3. Enter the payload URL pointing to your `promgithub` service, e.g., `http://<your-service-url>/webhook`.
4. Set the **Content type** to `application/json`.
5. Add the **Secret**: Use the value of `PROMGITHUB_WEBHOOK_SECRET`.
6. Select the following events to trigger the webhook:
   - **push**
   - **pull request**
   - **workflow job**
   - **workflow runs**
7. Click **Add webhook** to save.

## Prometheus scraping configuration

Configure prometheus to scrape `promgithub`'s `/metrics` endpoint to extract metrics.

### Prometheus configuration

To allow prometheus to scrape `promgithub`'s `/metrics` endpoint, add the following configuration to your prometheus setup:

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

If you use victoria-metrics as your metrics provider, add a `vmservicescrape` configuration to your `promgithub` chart deployment

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
