# Using `promgithub` service

## Deploying the service

The service can be deployed in your choice of infrastructure

### Deploying in kubernetes

To deploy the service in a kubernetes cluster you can use the provided helm chart.

**TODO:** Add helm chart details

## Deploying service in a container

To deploy the service in a container using a container management environment like fargate/docker-compose, you can use the `promgithub` container from the [GHCR container repository](https://github.com/darthfork/promgithub/pkgs/container/promgithub)

## Deploying service binary

The service binaries are also available under [github releases](https://github.com/darthfork/promgithub/releases) which can be deployed as the user wishes.


### Service Parameters

The service expects the following parameters to be set:

- **Environment Variables**:
  - `PROMGITHUB_WEBHOOK_SECRET`: The secret used to validate incoming GitHub webhook requests.
  - `PROMGITHUB_SERVICE_PORT` (optional): Service API port (default is `8080`).

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

Configure prometheus to scrape `promgithub`'s `/metrics` endpoint to extract metrics

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

If you use victoria-metrics as your metrics provider, add a `vmservicescrape`  configuration to your `promgithub` chart deployment

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