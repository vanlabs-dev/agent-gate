# Design

## Context

Motivation and scope: see `proposal.md` and the [brainstorm](../../../docs/greenfield/repo-foundations/01-brainstorm.md). This file answers the 10 open questions of the brainstorm.

Current state, read on 2026-11-17:

- The root `Makefile` has one entry point, `make test`. It builds a `kind` cluster and runs `TestE2E` with the build tag `e2e`.
- `test/extmcp/` is the only Go module (Go 1.27.4). `go test ./...` with no tag runs the stub unit tests in under one second.
- `.github/workflows/extmcp.yml` has one job, named `agentgateway compatibility test`. Its actions are pinned by tag (`@v7`).
- `main` has no protection. The repo is public and belongs to a user account, not an organisation.
- No Python code exists. `uv` 0.12.5 is installed, with managed interpreters for Python 3.12, 3.13 and 3.14.
- The greenfield verify script runs `make test` at the repo root. The root holds no `pyproject.toml`, so the script does not call `pytest` directly.

Probe on 2026-11-17 with Go 1.27.4, each tool started with `go run <module>@<version>`:

| Tool | Version | First build | Result on this repo |
|---|---|---|---|
| `golangci-lint` | v2.15.0 | 46 s | 4 to 7 `errcheck` findings, all an unchecked `Close` in a test file. Format is clean |
| `govulncheck` | v1.9.0 | 6 s | No vulnerabilities |
| `actionlint` | v1.7.12 | 5 s | No findings |
| `gitleaks` | v8.30.1 | 13 s | 27 commits, no leaks |

## Goals / Non-Goals

**Goals:**

- One mechanism for each tool, the same on the development machine and in CI.
- Check names that stay stable, because the ruleset names them.
- A change that Phase 1 extends by one line for `gate/`.

**Non-Goals:**

- A type checker for Python. It starts in Phase 3 with the agent code and its dependencies. No Python logic exists before then, so nothing is lost.
- Automatic merge of Dependabot pull requests.
- A path filter on a job.
- Automatic updates of the tool versions in the root `versions.env`.

## Decisions

### 1. Make targets

| Target | Runs | Needs |
|---|---|---|
| `test` | `test-go`, `test-python` | Go, `uv` |
| `lint` | `lint-go`, `lint-python`, `lint-actions` | Go, `uv` |
| `check` | `lint`, `test` | Go, `uv` |
| `vuln` | `govulncheck` on each Go module | Go, network |
| `secrets` | `gitleaks` on the git history | Go |
| `e2e` | The full cluster test: `tools cluster gateway image deploy e2e-run` | Docker, `kind`, `kubectl`, Helm |
| `clean` | Deletes the cluster. No change | `kind` |

- The step that runs `go test -tags e2e` is renamed from `e2e` to `e2e-run`.
- A variable `GO_MODULES := test/extmcp` lists the Go modules. `test-go`, `lint-go` and `vuln` loop over it. Phase 1 adds `gate` to that line.
- `check` leaves out `vuln` and `secrets`. Both need the network or the full history, and CI runs them on each pull request.
- Alternative: keep `make test` as the cluster test and add `make unit`. Rejected in the brainstorm (finding F13).

### 2. Tools run through `go run <module>@<version>`

`golangci-lint`, `govulncheck`, `actionlint` and `gitleaks` are Go programs. The `Makefile` starts each with `go run <module>@<version>`. The versions are in a new root file `versions.env`:

```text
GOLANGCI_LINT_VERSION=v2.15.0
GOVULNCHECK_VERSION=v1.9.0
ACTIONLINT_VERSION=v1.7.12
GITLEAKS_VERSION=v8.30.1
```

- Why: the Go checksum database verifies each module, so a version cannot change after it is pinned. Nothing is installed on the machine. CI and the developer run the same `make` target.
- `test/extmcp/versions.env` keeps the versions of the cluster test only.
- CI: `actions/setup-go` caches the build. Its `cache-dependency-path` names `versions.env` and `test/extmcp/go.sum`, so a new tool version makes a new cache.
- Alternative: release binaries with a checksum, as the workflow does for Helm. Rejected: an install step for four tools, on the machine and in CI.
- Alternative: the official action of each tool. Rejected: the action picks the tool version, so CI and the machine can differ.
- Trade-off: the `golangci-lint` project recommends its release binary. The probe shows that the `go run` build works. If a later version does not build, that tool changes to the release binary with a pinned checksum.

### 3. Go lint set

`.golangci.yml` at the repo root, config version 2:

- The `standard` default set: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`.
- The formatters `gofmt` and `goimports`.
- The build tag `e2e`, so the end-to-end test file is linted.
- The exclusion preset `std-error-handling`. It accepts an unchecked `Close`, which covers each finding of the probe.

No Go source file changes. If a finding remains after the preset, the fix goes into the test file and changes no behaviour. A named list of more linters waits for real code in `gate/`.

### 4. Python scaffold

```text
agents/azure-ops/
├── .python-version      # 3.14
├── pyproject.toml       # project azure-ops, dev group: ruff, pytest
├── uv.lock
├── src/azure_ops/__init__.py
└── tests/test_smoke.py  # imports azure_ops
```

- Python 3.14: the newest version with one year of package support. Python 3.15 is one month old.
- `ruff` and `pytest` are in the `dev` dependency group and locked in `uv.lock`. No runtime dependency.
- `lint-python` runs `ruff check` and `ruff format --check`. `test-python` runs `pytest`. Each goes through `uv run --locked`, so a stale lock file fails the target.
- `ruff` uses its default rule set. A larger set waits for Phase 3.
- The virtual environment is `agents/azure-ops/.venv`, made with `--python-preference only-managed`. `.gitignore` gets `.venv/`, `__pycache__/`, `*.pyc`, `.ruff_cache/` and `.pytest_cache/`.
- CI: `astral-sh/setup-uv`, pinned by SHA. `pyproject.toml` sets `required-version` for `uv`, and the action reads it.

### 5. Workflows and check names

Two workflow files. Each job runs on `pull_request` and on a push to `main`, with no path filter, with `permissions: contents: read` and a timeout.

| File | Job name (the check name) | Command |
|---|---|---|
| `ci.yml` | `go` | `make lint-go`, then `make test-go` |
| `ci.yml` | `python` | `make lint-python`, then `make test-python` |
| `ci.yml` | `govulncheck` | `make vuln` |
| `ci.yml` | `secrets` | `make secrets`, after a checkout with the full history |
| `ci.yml` | `actionlint` | `make lint-actions` |
| `extmcp.yml` | `agentgateway compatibility test` | `make e2e` |

- In `go` and `python`, the test step also runs when the lint step fails. One run then shows both results.
- The cluster job runs on each pull request. The conventions require it, the job takes a few minutes, and a required check with a path filter never reports on other pull requests.
- Alternative: one job for each of lint and test. Rejected: eight required names and two more toolchain setups for no new signal.
- Alternative: one workflow file. Rejected: the cluster job has its own log collection, and its name already reports on `main`.

### 6. Secret scan

- `make secrets` runs `gitleaks git --redact` on the full history. The CI job checks out with `fetch-depth: 0`.
- A false positive goes into `.gitleaksignore` by fingerprint, with a comment that gives the reason.
- The maintainer also enables GitHub secret scanning with push protection. It is free for a public repo and stops a known credential format before it reaches GitHub.
- Alternative: the `gitleaks` action. Rejected for the reason in decision 2.
- Alternative: GitHub secret scanning alone. Rejected: it reports no check on a pull request, so it cannot be a required check.

### 7. `govulncheck` scope

`make vuln` runs `govulncheck ./...` in each module of `GO_MODULES`. Today that is `test/extmcp` only. It reports a vulnerability only when the code can reach it.

### 8. Pins and updates

- Each `uses:` line names a 40-character commit SHA, with the version in a comment. This includes the three actions in `extmcp.yml`.
- `.github/dependabot.yml` has three entries: `github-actions` at `/`, `gomod` at `/test/extmcp`, `uv` at `/agents/azure-ops`.
- Each entry runs monthly and groups its updates into one pull request. That is at most three pull requests a month, which fits 10 hours a week. `ruff` alone has a release about each week.
- The `gomod` entry ignores `github.com/agentgateway/agentgateway/api`. That version follows `AGENTGATEWAY_COMMIT` in `test/extmcp/versions.env` and changes only with it.
- The maintainer enables Dependabot security updates. A security fix then does not wait for the monthly run.
- Not updated by Dependabot: the four tool versions in `versions.env`, the Go version in `go.mod`, and `test/extmcp/versions.env`. The maintainer updates them by hand.

### 9. Ruleset on `main`

The ruleset is the file `.github/rulesets/main.json`:

- Target: the default branch. Enforcement: active. Bypass list: empty.
- Rules: no deletion, no force push, pull request required with 0 approvals, and the six checks of decision 5 required.
- The rule "branch must be up to date before merge" is off. One maintainer gains nothing from a rebase of each pull request.

The maintainer applies it. The agent does not.

```sh
gh api --method POST repos/vanlabs-dev/agent-gate/rulesets --input .github/rulesets/main.json
```

- Why a file: the rule is reviewed in a pull request, and the repo shows what protects `main`.
- Trade-off: the file and the live ruleset can differ. A task compares them after the maintainer applies the file.
- Alternative: classic branch protection. Rejected: no file form, and rulesets are the current GitHub feature.

### 10. Pull request template

`.github/pull_request_template.md` holds the four lines of the self-review checklist in the conventions: tests added, docs updated, no secrets, cost effect stated. No other text.

### 11. Proof of a failed check

One branch `demo/failed-checks` from this change, with three commits: a Go lint finding, a Python test that fails, and a fake credential in a format that `gitleaks` detects. One pull request shows the checks `go`, `python` and `secrets` as failed. After the ruleset is active, the same pull request shows that GitHub blocks the merge. The pull request is then closed, not merged, and the branch is deleted.

The fake credential is pushed before push protection is on. Push protection can reject it.

## Risks / Trade-offs

- [A required check never reports, and each merge is blocked] → No path filters. The ruleset is applied only after the pull request of this change is merged. A task compares the six names in the file with the names that GitHub reports.
- [No bypass, and a broken check blocks its own fix] → The fix is a pull request like each other change. If the check cannot pass, the maintainer edits the ruleset. GitHub records that edit.
- [A new advisory fails `govulncheck` on an unrelated pull request] → Accepted. The fix is an update of the module or of Go.
- [The cluster job fails for a reason outside the change] → Run the job again. Record each case. Three cases in one month start a fix.
- [A Dependabot update of a Go module breaks the contract with agentgateway] → The cluster job runs on that pull request.
- [The first `golangci-lint` build takes about 46 s on the machine] → The Go build cache holds it. `make test` does not use it.
- [`make test` changes its meaning] → `README.md`, `docs/conventions.md`, the spec and two comments change in the same change.

## Migration Plan

1. The branch `repo-foundations` gets all files. One pull request. The six checks pass.
2. The pull request `demo/failed-checks` shows the three failed checks. It stays open.
3. The maintainer merges the pull request of the change. The six checks report on `main`.
4. The maintainer applies the ruleset. A direct push is rejected. The demo pull request is blocked, then closed.
5. The maintainer enables push protection and Dependabot security updates.
6. A second pull request holds the verify output, the ship note, the archive and the doc lines that close Phase 0. It passes through the ruleset.

Rollback: the maintainer deletes the ruleset with `gh api --method DELETE repos/vanlabs-dev/agent-gate/rulesets/<id>`. Each other part is a file and reverts with a pull request.
