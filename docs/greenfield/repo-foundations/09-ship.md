# Ship: repo-foundations

- Date: 2026-11-18
- Result: the repo has its checks, and `main` is protected. Phase 0 of the [roadmap](../../roadmap.md) is complete.

## What shipped

- `make check`, `make lint`, `make test`, `make vuln`, `make secrets` and `make e2e`. `make test` runs the unit tests in under one second and needs no cluster. The cluster test is now `make e2e`.
- The check tools run with `go run <module>@<version>`, pinned in [`versions.env`](../../../versions.env).
- [`agents/azure-ops/`](../../../agents/azure-ops/): a `uv` project on Python 3.14 with `ruff`, `pytest` and one smoke test. No agent logic.
- The workflow `.github/workflows/ci.yml` with the checks `go`, `python`, `govulncheck`, `secrets` and `actionlint`. Together with `agentgateway compatibility test`, these are the six required checks.
- Each action pinned by commit SHA. Dependabot for actions, Go modules and Python packages, monthly and grouped. Dependabot security updates on.
- The ruleset `27522673` on `main`, from [`.github/rulesets/main.json`](../../../.github/rulesets/main.json): pull request required, six checks required, no bypass, no force push, no deletion.
- The pull request template and the "Commands" section of `AGENTS.md`.
- The main spec [`repo-checks`](../../../openspec/specs/repo-checks/spec.md). The main spec `agentgateway-integration` names `make e2e`.

## Changes during apply

- `make secrets` runs with `--verbose`. Without it, a failed scan in CI prints a count only. Decided by the maintainer on 2026-11-18.
- Dependabot security updates need vulnerability alerts first. The design named one setting. The maintainer enabled both.
- `golangci-lint run` reports format findings itself, so `lint-go` has no separate `fmt` step.

## Evidence

- [08-verify.md](08-verify.md): verify script PASS, `make check` exit 0, and the result of each task.
- PR #3 and `main` at `ccc1b02`: six checks passed.
- PR #5: three planted defects failed `go`, `python` and `secrets`. With the ruleset active, its merge state was `BLOCKED`. Closed, not merged.
- A direct push to `main` was rejected with `GH013`.

## Where the artifacts live

| Artifact | Path |
|---|---|
| Brainstorm, review, verify output, this note | `docs/greenfield/repo-foundations/` |
| Proposal, design, tasks, delta specs | `openspec/changes/archive/2026-11-18-repo-foundations/` |
| Main specs | `openspec/specs/repo-checks/spec.md`, `openspec/specs/agentgateway-integration/spec.md` |
| Ruleset | `.github/rulesets/main.json`, live as ruleset `27522673` |

## Follow-ups

- Review Dependabot PR #4 (`uv_build` range `<0.14.0`).
- GitHub adds `require_extra_approval_for_unattributed_changes: true` to the ruleset by default. The merge of this pull request shows whether it affects a solo merge.
- Phase 1 adds `gate` to `GO_MODULES` in the `Makefile`.
