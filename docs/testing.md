# Testing and CI lanes

`promgithub` uses layered test execution so pull requests get useful feedback quickly while slower checks still run before release-sensitive changes age too long.

## CI lanes

| Lane | CI job | Runs on | Purpose | Local command |
| --- | --- | --- | --- | --- |
| Fast PR lane | `Fast PR lane` | Pull requests, pushes to `main`, weekly schedule | Static checks, security scan, build, unit tests, in-process HTTP integration tests, and coverage. This lane has no external service dependency. | `make lint`, `make security`, `make build`, `make unit-test`, `make integration-test`, `make coverage` |
| Medium Redis lane | `Medium Redis lane` | Pull requests, pushes to `main`, weekly schedule | Redis-backed deduplication and shared run/job state behavior against a real Redis service. | `PROMGITHUB_REDIS_ADDR=127.0.0.1:6379 make redis-integration-test` |
| Deep container lane | `Deep container lane` | Pushes to `main`, weekly schedule | Container build and container image security scan. This lane is intentionally kept out of normal pull request feedback because it is slower and Docker-dependent. | `make container`, `make container-security` |

## Pull request expectations

Every pull request should keep the Fast PR lane and Medium Redis lane green. Those lanes cover:

- Go unit tests.
- Lint checks.
- Vulnerability and static security checks.
- In-process HTTP webhook-to-metrics behavior.
- Redis-backed delivery deduplication and run/job state behavior.

The Deep container lane runs after merge to `main` and on the weekly scheduled workflow. It catches packaging and image-scan issues without making every pull request wait on Docker image work.

## Local verification

Before committing changes, run:

```bash
make test-all
make lint
make security
```

`make test-all` already includes the unit, integration, Redis integration, coverage, security, and lint targets. Run `make lint` and `make security` separately as explicit final checks before pushing.

## Future system smoke tests

Black-box container and Helm/deployment smoke tests are tracked separately in issue #56. When those tests exist, they should join the Deep container lane or a dedicated deep system lane rather than the Fast PR lane.
