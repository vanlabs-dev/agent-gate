# Proposal

## Why

Phase 1 starts the gate. The repo has one CI job and no lint, no fast test command, no security checks and no protection on `main`. The root `make test` is a cluster test that takes minutes (review finding F13 of `extmcp-spike`). This change gives the repo its working base and completes Phase 0 of the [roadmap](../../../docs/roadmap.md). Scope and success signals are in the [brainstorm](../../../docs/greenfield/repo-foundations/01-brainstorm.md).

## What Changes

- **BREAKING** `make test` runs the Go and Python unit tests and needs no cluster. `make e2e` runs the cluster test that `make test` runs today.
- Add `make lint` (Go lint, Python lint, workflow lint), `make vuln`, `make secrets` and `make check` (`lint` and `test`).
- Add `agents/azure-ops/` as a minimal `uv` project with `ruff`, `pytest` and one smoke test. It holds no agent logic.
- Add a workflow with five jobs: Go, Python, `govulncheck`, secret scan, `actionlint`. The cluster job stays in its own workflow and runs `make e2e` on each pull request.
- Pin each action by full commit SHA. Add Dependabot for actions, Go modules and Python packages.
- Add a ruleset for `main` as a JSON file: pull request required, six required checks, no bypass, no force push, no deletion. The maintainer applies it after the change is merged.
- Add a pull request template with the self-review checklist from the [conventions](../../../docs/conventions.md).
- Add the build, test and lint commands to `AGENTS.md`. Change each doc line that names `make test` as the cluster test.

## Capabilities

### New Capabilities

- `repo-checks`: the checks that guard the repo. It covers the local commands, the checks on each pull request, the pinning and update of dependencies, and the protection of `main`.

### Modified Capabilities

- `agentgateway-integration`: the scenario "Local run" names `make e2e` in place of `make test`. No behaviour of the contract changes.

## Impact

- **Changed code:** `Makefile`, `.github/workflows/extmcp.yml`, one comment in `test/extmcp/e2e_test.go`, one comment in `test/extmcp/versions.env`.
- **New code:** `agents/azure-ops/`, `versions.env`, `.golangci.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml`, `.github/rulesets/main.json`, `.github/pull_request_template.md`.
- **New dependencies:** `golangci-lint`, `govulncheck`, `actionlint`, `gitleaks`, `ruff`, `pytest`, the build backend `uv_build`, and the action `astral-sh/setup-uv`.
- **Docs:** `AGENTS.md`, `README.md`, `docs/conventions.md`, `docs/roadmap.md`, `docs/vision.md` if its status line names Phase 0. In `docs/adr/0002-integrate-through-agentgateway-extmcp.md`, the command name `make test` changes to `make e2e`. The decision does not change.
- **Development machine:** a `uv`-managed Python interpreter and a virtual environment in `agents/azure-ops/.venv`. No new tool binaries.
- **Outside the machine:** one ruleset on `main` and one repo setting (Dependabot security updates). The maintainer applies both. Secret scanning with push protection is on already.
- **Cost:** none. No cloud resources.

Approved: 2026-11-18 (vanlabs-dev)

Revised: 2026-11-18, after the [review](../../../docs/greenfield/repo-foundations/07-review.md). The scope did not change.
