# Github Prometheus Exporter (promgithub)

`promgithub` is a service that receives GitHub webhook events and exposes Prometheus metrics for repository activity, workflow runs, workflow jobs, commits, and pull requests.

It is designed to be simple to deploy and can run either:
- as a single instance
- as multiple instances with Redis for shared deduplication and state

## Metrics exported

`promgithub` exports the following metrics:

| Name | Type | Labels | Description |
|---|---|---|---|
| `promgithub_workflow_status` | Counter | `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion` | Total number of workflow runs with status |
| `promgithub_workflow_duration` | Histogram | `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion` | Duration of workflow runs |
| `promgithub_workflow_queued` | Gauge | `repository`, `branch`, `workflow_name` | Number of workflow runs queued |
| `promgithub_workflow_in_progress` | Gauge | `repository`, `branch`, `workflow_name` | Number of workflow runs in progress |
| `promgithub_workflow_completed` | Gauge | `repository`, `branch`, `workflow_conclusion`, `workflow_name` | Number of workflow runs completed |
| `promgithub_job_status` | Counter | `repository`, `branch`, `workflow_name`, `job_status`, `job_conclusion` | Total number of jobs with status |
| `promgithub_job_duration` | Histogram | `repository`, `branch`, `workflow_name`, `job_status`, `job_conclusion` | Duration of job runs in seconds |
| `promgithub_job_queued` | Gauge | `repository`, `branch`, `workflow_name` | Number of jobs queued |
| `promgithub_job_in_progress` | Gauge | `repository`, `branch`, `workflow_name` | Number of jobs in progress |
| `promgithub_job_completed` | Gauge | `repository`, `branch`, `job_conclusion`, `workflow_name` | Number of jobs completed |
| `promgithub_commit_pushed` | Counter | `repository` | Total number of commits pushed |
| `promgithub_pull_request` | Counter | `repository`, `base_branch`, `pull_request_status` | Total number of pull requests |
| `promgithub_event_queue_depth` | Gauge | none | Current number of queued webhook events awaiting processing |
| `promgithub_event_queue_capacity` | Gauge | none | Configured capacity of the webhook event queue |
| `promgithub_event_worker_count` | Gauge | none | Configured number of async webhook event workers |
| `promgithub_event_processed_total` | Counter | `event_type` | Total number of webhook events processed asynchronously |
| `promgithub_event_dropped_total` | Counter | `event_type`, `reason` | Total number of webhook events dropped before processing |
| `promgithub_event_processing_failures_total` | Counter | `event_type` | Total number of async webhook processing failures |
| `promgithub_event_processing_duration_seconds` | Histogram | `event_type` | Duration of async webhook event processing |
| `promgithub_duplicate_deliveries_seen_total` | Counter | `event_type` | Duplicate webhook deliveries observed |
| `promgithub_duplicate_deliveries_dropped_total` | Counter | `event_type` | Duplicate webhook deliveries dropped |

## Metric model

The exporter focuses on repository and workflow health signals while avoiding noisy per-entity labels such as runner names, job names, commit author identities, and pull request authors.

This keeps the default metric set compact and practical for Prometheus while still preserving the `branch` label for branch-specific workflow and job visibility.

## Redis-backed multi-instance mode

When Redis is configured, `promgithub` uses it for:
- webhook delivery deduplication using `X-GitHub-Delivery`
- shared workflow run state storage
- shared workflow job state storage

This allows multiple `promgithub` instances to share delivery and run state through a common backend.

## Using promgithub

See [Usage documentation](./docs/usage.md) for deployment and configuration examples.

## Contributing

See [Contributing documentation](./docs/contributing.md).
