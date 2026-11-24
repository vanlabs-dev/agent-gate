# Spec Delta

## MODIFIED Requirements

### Requirement: The contract is tested against one pinned agentgateway version
The repository SHALL pin agentgateway and the `ExtMcp` bindings to one exact version. One command SHALL create a `kind` cluster, deploy that version with the test services, run every scenario in this spec, and exit 0 only when all of them pass. The command SHALL exit non-zero when no scenario ran. The same command SHALL run in CI.

#### Scenario: Local run
- **WHEN** a developer with the toolchain installed runs `make e2e` from the repo root
- **THEN** the command exits 0 and reports each scenario as passed

#### Scenario: CI run
- **WHEN** a pull request is opened or `main` is updated
- **THEN** a GitHub Actions job runs the same command and must pass
