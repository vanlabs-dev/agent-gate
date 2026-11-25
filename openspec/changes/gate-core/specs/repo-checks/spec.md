# Spec Delta

## MODIFIED Requirements

### Requirement: Each pull request runs the checks
Each pull request, and each update of `main`, SHALL run these checks: Go lint and unit tests, Python lint and tests, a Go vulnerability check, a secret scan, a workflow lint, the agentgateway compatibility test and the gate end-to-end test. Each check SHALL run on each pull request, whatever files changed. Each check SHALL run the same command that a developer runs locally.

#### Scenario: Clean pull request
- **WHEN** a pull request with no defect is opened
- **THEN** each check reports a pass

#### Scenario: Lint error
- **WHEN** a pull request adds a lint finding
- **THEN** the check of that language fails

#### Scenario: Failed test
- **WHEN** a pull request adds a unit test that fails
- **THEN** the check of that language fails

#### Scenario: Planted secret
- **WHEN** a pull request adds a commit that contains a fake credential
- **THEN** the secret scan fails

#### Scenario: Secret on another branch
- **WHEN** a branch that is not part of a pull request contains a fake credential
- **THEN** the secret scan of that pull request passes

## ADDED Requirements

### Requirement: The gate end-to-end test runs with one command
`make e2e-gate` from the repo root SHALL create its own `kind` cluster, deploy agentgateway at the pinned version, the OIDC issuer, Postgres, the gate and the stub MCP server, run every end-to-end scenario of the gate, and exit 0 only when all of them pass. It SHALL exit non-zero when no scenario ran. It SHALL NOT read or change `~/.kube`.

#### Scenario: Local run
- **WHEN** a developer with the toolchain installed runs `make e2e-gate`
- **THEN** the command exits 0 and reports each scenario as passed

#### Scenario: No scenario ran
- **WHEN** the end-to-end test is run with a wrong build tag
- **THEN** `make e2e-gate` exits non-zero
