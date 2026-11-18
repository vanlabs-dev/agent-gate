# Brainstorm: repo-foundations

- Date: 2026-11-17
- Change: the second of two in Phase 0 of the [roadmap](../../roadmap.md). The first, `extmcp-spike`, is archived.
- Project-level direction is in the [vision](../../vision.md), the [conventions](../../conventions.md) and the [ADRs](../../adr/). This file covers this change only.

## Goal

Give the repo its working base before Phase 1: lint and unit tests for Go and Python, the security checks that have something to scan, a protected `main`, and the build, test and lint commands in `AGENTS.md`.

## Success signals

1. On a clean checkout, `make lint`, `make test`, `make check` and `make e2e` each exit 0.
2. `make test` finishes in seconds and needs no cluster.
3. A pull request runs these checks, and all pass: Go lint, Go unit tests, Python lint, Python tests, `govulncheck`, secret scan, `actionlint`, the cluster test.
4. A pull request with a lint error, a failed test or a planted fake secret fails the matching check. Each case is shown once.
5. Each `uses:` line in a workflow names a full commit SHA. Dependabot proposes updates for actions, Go modules and Python packages.
6. A direct push to `main` is rejected. A pull request with a failed check cannot merge.
7. `AGENTS.md` lists the build, test and lint commands, and they match the `Makefile`.
8. The Phase 0 row of the roadmap is complete.

## Constraints

- No cloud resources and no cost. GitHub Actions is free for a public repo.
- The Python project lives in `agents/azure-ops/`. Its virtual environment is created there, with a `uv`-managed interpreter, and is not moved.
- The Python scaffold holds no agent logic and no agent dependencies. One smoke test proves the tool chain.
- New tools install under `~/.local`. Go never creates `~/go`.
- The maintainer applies the ruleset on `main`. The agent writes the command and does not run it.
- The ruleset is applied after the new check names exist on `main`. A required check that never reports blocks each merge.
- The greenfield verify script runs root `make test`. That target must exist and must run the Python tests through `uv`.

## Non-goals

- The `gate/` module. It starts in Phase 1.
- `terraform fmt`, `terraform validate`, `tflint`, image scan and IaC scan. They start in Phase 2.
- Release automation and release notes
- Local git hooks
- Coverage thresholds
- A change to the stub gate, the stub MCP server or the 11 end-to-end scenarios

## Options considered

| Question | Options | Chosen |
|---|---|---|
| Code scaffold | Python only, Python and `gate/`, or none | Python only. `gate/` waits for its design in Phase 1 |
| Meaning of root `make test` | Fast unit tests, or unit tests and the cluster test | Fast unit tests. `make e2e` runs the cluster test. This closes review finding F13 of `extmcp-spike` |
| Extra CI checks now | Each of `govulncheck`, secret scan, SHA pins with Dependabot, `actionlint` | All four |
| Protection of `main` | Pull request and checks with no bypass, the same with admin bypass, or none | Pull request and checks, 0 approvals, no bypass, no force push |

## Chosen direction

One change with four parts.

1. **Make targets.** `make test` runs the Go and Python unit tests. `make e2e` runs the cluster test that `make test` runs today. `make lint` runs the linters. `make check` runs `lint` and `test`.
2. **Python scaffold.** `agents/azure-ops/` as a minimal `uv` project with `ruff`, `pytest` and one smoke test.
3. **CI.** Jobs for Go lint and unit tests, Python lint and tests, `govulncheck`, secret scan and `actionlint`, beside the existing cluster job. Each action is pinned by commit SHA. Dependabot keeps actions, Go modules and Python packages current.
4. **Protection and docs.** A ruleset on `main`. The commands in `AGENTS.md`. The lines in `conventions.md`, `README.md` and `test/extmcp/versions.env` that name `make test` change to `make e2e`.

The local verify script then proves lint and unit tests only. CI proves the cluster test.

## Open questions

For the proposal session.

1. Which Go linter set: `golangci-lint` with its default set, or a named list. Where its version is pinned.
2. Which Python version, and whether a type checker (`mypy`, `pyright` or `ty`) starts now or in Phase 3.
3. Where root tool versions live. `test/extmcp/versions.env` belongs to the cluster test.
4. One workflow file or several. The job names become the required checks, so they must be stable.
5. Whether the cluster job runs on each pull request or only when `test/extmcp/` changes. A required check with a path filter never reports on other pull requests. The conventions say each change is tested on `kind`.
6. How the secret scan runs: the `gitleaks` binary, the `gitleaks` action (it needs a licence key for an organisation account), or GitHub secret scanning with push protection.
7. Dependabot cadence and grouping, given about 10 hours a week from one maintainer.
8. Whether the ruleset is stored as a JSON file in the repo, and the exact step order: merge the CI change, then apply the ruleset.
9. Whether this change adds a pull request template that holds the self-review checklist from the conventions.
10. Whether `govulncheck` covers `test/extmcp/` only for now, and how it extends to `gate/` in Phase 1.
