# Spec Delta

## Purpose

Defines the checks that guard the repo: the commands a developer runs before a push, the checks that run on each pull request, how dependencies are pinned and updated, and how `main` is protected.

## ADDED Requirements

### Requirement: Unit tests run with one fast command
`make test` from the repo root SHALL run the unit tests of each Go module and each Python project. It SHALL need no cluster and no container runtime. It SHALL exit non-zero when a test fails.

#### Scenario: Clean checkout
- **WHEN** a developer with Go and `uv` installed runs `make test` on a clean checkout
- **THEN** the command exits 0
- **AND** it reports the Go unit tests and the Python tests as passed
- **AND** no `kind` cluster is created

#### Scenario: Failed unit test
- **WHEN** one unit test fails
- **THEN** `make test` exits non-zero

### Requirement: Lint runs with one command
`make lint` from the repo root SHALL lint the Go code, the Python code and the workflow files. It SHALL exit non-zero when one linter reports a finding.

#### Scenario: Clean checkout
- **WHEN** a developer runs `make lint` on a clean checkout
- **THEN** the command exits 0

#### Scenario: Lint finding
- **WHEN** a Go file, a Python file or a workflow file has a lint finding
- **THEN** `make lint` exits non-zero and names the file

### Requirement: One command runs lint and unit tests
`make check` from the repo root SHALL run `make lint` and `make test`, and SHALL exit 0 only when both pass.

#### Scenario: Clean checkout
- **WHEN** a developer runs `make check` on a clean checkout
- **THEN** the command exits 0

### Requirement: Each pull request runs the checks
Each pull request, and each update of `main`, SHALL run these checks: Go lint and unit tests, Python lint and tests, a Go vulnerability check, a secret scan, a workflow lint, and the cluster test. Each check SHALL run on each pull request, whatever files changed. Each check SHALL run the same command that a developer runs locally.

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

### Requirement: Workflow dependencies are pinned
Each `uses:` line in a workflow SHALL name a full commit SHA. Each tool that a check runs SHALL have its version pinned in the repo.

#### Scenario: Action reference
- **WHEN** the workflow files are searched for `uses:` lines
- **THEN** each line names a 40-character commit SHA

#### Scenario: Tool version
- **WHEN** a check runs two times on the same commit
- **THEN** both runs use the same version of each tool

### Requirement: Dependency updates are proposed automatically
The repo SHALL receive automatic pull requests that update the actions, the Go modules and the Python packages. The agentgateway bindings SHALL be left out, because the compatibility test owns that version.

#### Scenario: New version of a dependency
- **WHEN** a newer version of an action, a Go module or a Python package exists
- **THEN** a pull request proposes the update
- **AND** the checks run on that pull request

#### Scenario: New commit of the agentgateway bindings
- **WHEN** a newer version of `github.com/agentgateway/agentgateway/api` exists
- **THEN** no automatic pull request proposes it

### Requirement: The main branch is protected
A change SHALL reach `main` only through a pull request whose checks all pass. No account SHALL bypass this rule. A force push to `main` and a deletion of `main` SHALL be rejected.

#### Scenario: Direct push
- **WHEN** the maintainer pushes a commit to `main` directly
- **THEN** GitHub rejects the push

#### Scenario: Pull request with a failed check
- **WHEN** a pull request has one failed check
- **THEN** GitHub does not allow the merge

#### Scenario: Pull request with all checks passed
- **WHEN** each check of a pull request has passed
- **THEN** the maintainer can merge it with no approval from a second account

### Requirement: The agent instructions list the commands
`AGENTS.md` SHALL list the build, test and lint commands. Each listed command SHALL exist and SHALL do what the list says.

#### Scenario: Listed command
- **WHEN** a command from the list in `AGENTS.md` is run on a clean checkout
- **THEN** the command exists and exits 0
