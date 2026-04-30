# Github Prometheus Exporter (promgithub)

`promgithub` receives GitHub webhook events and exposes Prometheus metrics for repository activity, workflow runs, workflow jobs, commits, and pull requests.

It can run either:
- as a single instance
- as multiple instances with Redis for shared deduplication and state

## Metrics exported

### Default metrics

The default metric set is bounded-cardinality and production-safe for larger repository sets.

| Name                              | Type      | Labels                                         | Description                               |
|-----------------------------------|-----------|------------------------------------------------|-------------------------------------------|
| `promgithub_workflow_status`      | Counter   | `repository`, `workflow_status`, `conclusion`  | Total number of workflow runs with status |
| `promgithub_workflow_duration`    | Histogram | `repository`, `workflow_status`, `conclusion`  | Duration of workflow runs                 |
| `promgithub_workflow_queued`      | Gauge     | `repository`                                   | Number of workflow runs queued            |
| `promgithub_workflow_in_progress` | Gauge     | `repository`                                   | Number of workflow runs in progress       |
| `promgithub_workflow_completed`   | Gauge     | `repository`, `workflow_conclusion`            | Number of workflow runs completed         |
| `promgithub_job_status`           | Counter   | `repository`, `job_status`, `job_conclusion`   | Total number of jobs with status          |
| `promgithub_job_duration`         | Histogram | `repository`, `job_status`, `job_conclusion`   | Duration of jobs runs in seconds          |
| `promgithub_job_queued`           | Gauge     | `repository`                                   | Number of jobs queued                     |
| `promgithub_job_in_progress`      | Gauge     | `repository`                                   | Number of jobs in progress                |
| `promgithub_job_completed`        | Gauge     | `repository`, `job_conclusion`                 | Number of jobs completed                  |
| `promgithub_commit_pushed`        | Counter   | `repository`                                   | Total number of commits pushed            |
| `promgithub_pull_request`         | Counter   | `repository`, `pull_request_status`            | Total number of pull requests             |

### Optional detailed metrics

Set `PROMGITHUB_ENABLE_DETAILED_METRICS=true` to also emit opt-in detailed metric families with higher-cardinality labels:

- `promgithub_workflow_status_detailed`
- `promgithub_workflow_duration_detailed`
- `promgithub_workflow_queued_detailed`
- `promgithub_workflow_in_progress_detailed`
- `promgithub_workflow_completed_detailed`
- `promgithub_job_status_detailed`
- `promgithub_job_duration_detailed`
- `promgithub_job_queued_detailed`
- `promgithub_job_in_progress_detailed`
- `promgithub_job_completed_detailed`
- `promgithub_pull_request_detailed`

These detailed metrics preserve labels such as `branch`, `workflow_name`, and `base_branch`. They are disabled by default because they can grow quickly in larger GitHub environments.

## Metric model

The exporter now defaults to repository-level operational metrics and keeps higher-cardinality dimensions as an explicit opt-in. This avoids unbounded series growth from branch churn, workflow-name sprawl, and pull-request base-branch fragmentation while still allowing teams to enable richer labels when they understand the cost.

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
