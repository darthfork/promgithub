# Testing and CI lanes

`promgithub` uses layered test execution so pull requests get useful feedback quickly while slower checks still run before release-sensitive changes age too long.

## CI lanes

| Lane | CI job | Runs on | Purpose | Local command |
| --- | --- | --- | --- | --- |
| Fast PR lane | `Fast PR lane` | Pull requests, pushes to `main`, weekly schedule | Static checks, security scan, build, unit tests, in-process HTTP integration tests, and coverage. This lane has no external service dependency. | `make lint`, `make security`, `make build`, `make unit-test`, `make integration-test`, `make coverage` |
| Medium Redis lane | `Medium Redis lane` | Pull requests, pushes to `main`, weekly schedule | Redis-backed deduplication and shared run/job state behavior against a real Redis service. | `PROMGITHUB_REDIS_ADDR=127.0.0.1:6379 make redis-integration-test` |
| Deep container and system lane | `Deep container and system lane` | Pushes to `main`, weekly schedule | Container build/security scan plus black-box container and kind/Helm deployment smoke tests. It validates startup configuration, health, metrics, signed webhook ingestion, chart readiness, and standalone and Redis-backed deployment modes. | `make system-smoke`, `make container-security` |

## Pull request expectations

Every pull request should keep the Fast PR lane and Medium Redis lane green. Those lanes cover:

- Go unit tests.
- Lint checks.
- Vulnerability and static security checks.
- In-process HTTP webhook-to-metrics behavior.
- Redis-backed delivery deduplication and run/job state behavior.

The Deep container and system lane runs after merge to `main` and on the weekly scheduled workflow. It catches packaging, deployment, and image-scan issues without making every pull request wait on Docker and kind work.

## Local verification

Before committing changes, run:

```bash
make test-all
make lint
make security
```

`make test-all` already includes the unit, integration, Redis integration, coverage, security, and lint targets. Run `make lint` and `make security` separately as explicit final checks before pushing.

## System smoke tests

Run the complete black-box suite with:

```bash
make system-smoke
```

The suite requires Docker, Helm, kind, kubectl, curl, and OpenSSL. It builds the production image and then:

- starts the container from the outside and checks `/health` and `/metrics`;
- submits a signed push webhook and waits for its exported metric;
- confirms startup fails when `PROMGITHUB_WEBHOOK_SECRET` is missing;
- lints and renders the Helm chart, including its bundled Redis configuration;
- installs the chart into a disposable kind cluster and checks pod readiness and endpoints;
- upgrades the deployment to use a real external Redis instance and repeats the webhook-to-metric check.

The scripts delete their temporary container and kind cluster on exit. Override `SMOKE_IMAGE`, `SMOKE_KIND_CLUSTER`, or `SMOKE_LOCAL_PORT` when local names or ports conflict.
