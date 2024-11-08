# Github Prometheus Exporter (promgithub)

The `promgithub` service is a lightweight service designed to receive and process GitHub webhook events (commits, pull requests, workflow jobs and workflow runs). The webhook events are converted to prometheus metrics, allowing monitoring and insights into GitHub activities.

## Metrics Exported by the Service

The `promgithub` service exports the following Prometheus metrics:

| Name                               | Type      | Labels                                                                                         | Description                               |
|------------------------------------|-----------|------------------------------------------------------------------------------------------------|-------------------------------------------|
| `promgithub_workflow_status`       | Counter   | `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion`                       | Total number of workflow runs with status |
| `promgithub_workflow_duration`     | Histogram | `repository`, `branch`, `workflow_name`, `workflow_status`, `conclusion`                       | Duration of workflow runs                 |
| `promgithub_workflow_queued`       | Gauge     | `repository`, `branch`, `workflow_name`                                                        | Number of workflow runs queued            |
| `promgithub_workflow_in_progress`  | Gauge     | `repository`, `branch`, `workflow_name`                                                        | Number of workflow runs in progress       |
| `promgithub_workflow_completed`    | Gauge     | `repository`, `branch`, `workflow_conclusion`,`workflow_name`                                  | Number of workflow runs completed         |
| `promgithub_job_status`            | Counter   | `runner`, `repository`, `branch`, `workflow_name`, `job_name`, `job_status`, `job_conclusion`, | Total number of jobs with status          |
| `promgithub_job_duration`          | Histogram | `runner`, `repository`, `branch`, `workflow_name`, `job_name`, `job_status`, `job_conclusion`  | Duration of jobs runs in seconds          |
| `promgithub_job_queued`            | Gauge     | `runner`, `repository`, `branch`, `workflow_name`, `job_name`                                  | Number of jobs queued                     |
| `promgithub_job_in_progress`       | Gauge     | `runner`, `repository`, `branch`, `workflow_name`, `job_name`                                  | Number of jobs in progress                |
| `promgithub_job_completed`         | Gauge     | `runner`, `repository`, `branch`, `job_conclusion`, `workflow_name`, `job_name`                | Number of jobs completed                  |
| `promgithub_commit_pushed`         | Counter   | `repository`, `branch`, `commit_author`, `commit_author_email`                                 | Total number of commits pushed            |
| `promgithub_pull_request`          | Counter   | `repository`, `base_branch`, `pull_request_author`, `pull_request_status`                      | Total number of pull requests             |

## Using `promgithub` service

For usage information see [Usage documentation](./docs/usage.md)

## Contributing to `promgithub` service

For contributing guidelines see [Contributing documentation](./docs/contributing.md)