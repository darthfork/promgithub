# promgithub Service

The `promgithub` service is a lightweight tool designed to receive and process GitHub webhook events, such as commits, pull requests, and workflow runs. It exports metrics to Prometheus, allowing you to monitor and gain insights into your GitHub activities in a straightforward manner. This service is ideal for integrating GitHub events into your observability stack, making it easier to track CI/CD workflows, job durations, and more.

## Metrics Exported by the Service

The `promgithub` service exports the following Prometheus metrics:

- **promgithub_workflow_status**:

  - Type: Counter
  - Description: Total number of workflow runs with status
  - Labels: `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion`

- **promgithub_workflow_duration**:

  - Type: Histogram
  - Description: Duration of workflow runs
  - Labels: `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion`

- **promgithub_workflow_queued**:

  - Type: Gauge
  - Description: Number of workflow runs queued
  - Labels: `repository`, `branch`, `workflow_name`

- **promgithub_workflow_in_progress**:

  - Type: Gauge
  - Description: Number of workflow runs in progress
  - Labels: `repository`, `branch`, `workflow_name`

- **promgithub_workflow_completed**:

  - Type: Gauge
  - Description: Number of workflow runs completed
  - Labels: `repository`, `branch`, `workflow_name`

- **promgithub_job_status**:

  - Type: Counter
  - Description: Total number of jobs with status
  - Labels: `runner`, `repository`, `branch`, `workflow_name`, `job_name`, `job_status`, `job_conclusion`

- **promgithub_job_duration**:

  - Type: Histogram
  - Description: Duration of jobs runs in seconds
  - Labels: `runner`, `repository`, `branch`, `workflow_name`, `job_name`, `job_status`, `job_conclusion`

- **promgithub_job_queued**:

  - Type: Gauge
  - Description: Number of jobs queued
  - Labels: `runner`, `repository`, `branch`, `workflow_name`, `job_name`

- **promgithub_job_in_progress**:

  - Type: Gauge
  - Description: Number of jobs in progress
  - Labels: `runner`, `repository`, `branch`, `workflow_name`, `job_name`

- **promgithub_job_completed**:

  - Type: Gauge
  - Description: Number of jobs completed
  - Labels: `runner`, `repository`, `branch`, `workflow_name`, `job_name`

- **promgithub_commit_pushed**:

  - Type: Counter
  - Description: Total number of commits pushed
  - Labels: `repository`, `branch`, `commit_author`, `commit_author_email`

- **promgithub_pull_request**:

  - Type: Counter
  - Description: Total number of pull requests
  - Labels: `repository`, `base_branch`, `head_branch`, `pull_request_author`, `pull_request_author_email`, `pull_request_status`

- promgithub_api_calls_total:

  - Type: Counter
  - Description: Number of API calls
  - Labels: `status`, `method`, `path`

- **promgithub_request_duration_seconds**:

  - Type: Histogram
  - Description: Request duration in seconds
  - Labels: `path`, `method`

## How to Use This Service

### Setting up the Webhook in GitHub (Repository/Organization)

1. Navigate to your GitHub repository or organization settings.
2. Under **Settings**, find **Webhooks** and click **Add webhook**.
3. Enter the payload URL pointing to your `promgithub` service, e.g., `http://<your-service-url>/webhook`.
4. Set the **Content type** to `application/json`.
5. Add the **Secret**: Use the value of `PROMGITHUB_WEBHOOK_SECRET`.
6. Select the events to trigger the webhook:
   - You can select **Just the push event** or **Send me everything**, depending on your needs.
7. Click **Add webhook** to save.

### Requirements to Run the Service

- **Redis**: The service uses Redis for caching metrics. Ensure you have a Redis instance available and reachable by the service.

### Required Parameters

- **Environment Variables**:
  - `PROMGITHUB_WEBHOOK_SECRET`: The secret used to validate incoming GitHub webhook requests.
  - `PROMGITHUB_SERVICE_PORT` (optional): The port to listen on (default is `8080`).
