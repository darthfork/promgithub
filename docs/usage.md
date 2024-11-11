# Using `promgithub` service

## Deploying the service

The service can be deployed in your choice of infrastructure. To allow webhooks to be pushed to `promgithub` make sure your service deployment is accessible from your Github instance.

### Service Parameters

The service expects the following parameters to be set:

- **Environment Variables**:
  - `PROMGITHUB_WEBHOOK_SECRET`: The secret used to validate incoming GitHub webhook requests.
  - `PROMGITHUB_SERVICE_PORT` (optional): Service API port (default is `8080`).

## Deploying in kubernetes

To deploy the service in a kubernetes cluster you can use the provided helm chart from the [promgithub chart repository](https://github.com/darthfork/promgithub/pkgs/container/promgithub-charts%2Fpromgithub)

When deploying with kubernetes add the following resources and configurations to your `promgithub` deployment

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

### Ingress

Add an Ingress configuration allowing your github instance to access `promgithub` deployment. More details can be found [here](https://kubernetes.io/docs/concepts/services-networking/ingress/)


### Values

Create a values file with the webhook secret and (optional) service port values for your chart

```yaml
promgithub:
  secrets:
    github_webhook_secret: <your webhook secret> # Mounted as PROMGITHUB_WEBHOOK_SECRET in the deployment
  service:
    port: <service port> # optional (default is 8080)
```

**Default values**: The default values file for promgithub can be found [here](https://github.com/darthfork/promgithub/blob/main/helm/promgithub/values.yaml)

### Metrics Scraping

Create prometheus configuration resource with the chart for scraping metrics from the `/metrics` endpoint from promgithub service. For more details see the [Prometheus scraping configuration](#prometheus-scraping-configuration) below.

## Deploying service in a container

To deploy the service in a container using a container management environment like fargate/docker-compose, you can use the `promgithub` container from the [GHCR container repository](https://github.com/darthfork/promgithub/pkgs/container/promgithub)

### Docker CLI

Run the service using docker cli as follows:

```bash
docker run\
    -e PROMGITHUB_WEBHOOK_SECRET=<your webhook secret>\
    -e PROMGITHUB_SERVICE_PORT=<service port>\
    -p <HOST_PORT>:<CONTAINER_PORT>
    ghcr.io/darthfork/promgithub:<version>
```

### Docker Compose

To run the service in docker compose, create a compose file as below:

```yaml
# promgithub-compose.yaml
services:
  promgithub:
    image: ghcr.io/darthfork/promgithub:<version>
    hostname: promgithub
    stdin_open: false
    tty: false
    environment:
      - PROMGITHUB_WEBHOOK_SECRET=<your webhook secret>
      - PROMGITHUB_SERVICE_PORT=<service port (optional)>
    ports:
      - <HOST_PORT>:<CONTAINER_PORT>
```

To start the `promgithub` container with compose run:

```bash
docker-compose -f promgithub-compose.yaml run --rm promgithub
```

## Deploying service binary

The service binaries are also available under [github releases](https://github.com/darthfork/promgithub/releases) which can be deployed as the user wishes.

```bash
PROMGITHUB_WEBHOOK_SECRET="<your webhook secret>" PROMGITHUB_SERVICE_PORT="<service port>" /path/to/binary/promgithub
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