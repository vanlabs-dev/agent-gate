# Conventions

## Repo layout

`test/extmcp/`, `gate/`, `agents/azure-ops/` and `versions.env` exist. Each other path is proposed and is created when its phase starts.

| Path | Content |
|---|---|
| `versions.env` | The pinned versions of the check tools: `golangci-lint`, `govulncheck`, `actionlint`, `gitleaks` |
| `test/extmcp/` | The agentgateway compatibility test: a stub gate, a stub MCP server, `kind` manifests and the pinned versions. Its own Go module |
| `gate/` | The gate, in Go: the policy, approval, audit and canonical encoding packages, the `ExtMcp` adapter, the Postgres store, the `agent-gate-audit` CLI, `kind` manifests, the end-to-end test, and the image pins in `gate/versions.env`. Its own Go module. Only `gate/internal/extmcp` imports the agentgateway bindings, which a `depguard` rule enforces |
| `agents/azure-ops/` | The reference agent, in Python. A `uv` project with `ruff`, `pytest` and one smoke test. No agent logic before Phase 3 |
| `infra/` | Terraform |
| `deploy/` | Helm charts |
| `docs/` | Vision, architecture, ADRs, roadmap, cost model |

## Branches and pull requests

- Trunk-based. `main` is always deployable.
- Each change goes through a short-lived branch and a pull request, including changes by the maintainer.
- Each pull request passes a self-review checklist: tests added, docs updated, no secrets, cost effect stated if any.

## Licence

Apache-2.0.

## Changes

- Each non-trivial change passes through four steps, each in its own session: brainstorm, proposal, review, apply.
- [OpenSpec](https://github.com/Fission-AI/OpenSpec) holds the proposal, specs, design and tasks of a change under `openspec/`.
- `docs/greenfield/<change>/` holds the brainstorm, the review notes, the verify output and the ship note of that change.
- A change is not applied before the maintainer approves its proposal.

## Commits

- Format: `type(scope): Subject`, after [Conventional Commits](https://www.conventionalcommits.org/). The scope is optional.
- Types: `feat`, `fix`, `docs`, `test`, `refactor`, `ci`, `build`, `chore`.
- Subject: imperative mood, capitalised, about 50 characters, with no full stop.
- Mark a breaking change with `!` after the type.
- One idea for each commit.
- A body only when the reason is not obvious.
- No attribution trailers.

Examples:

```text
feat(gate): Add approval request store
fix(agent): Replay the stored call after approval
docs: Add cost model
```

## Releases

- SemVer tags, starting at `v0.1.0`.
- Release notes for each tag, generated from the commit prefixes.

## CI and CD

- GitHub Actions.
- Azure login through OIDC federation. No stored cloud secrets.
- Required checks on each pull request and each push to `main`, with no path filter. Each job runs the `make` target that a developer runs locally:

  | Check | Workflow | Command |
  |---|---|---|
  | `go` | `ci.yml` | `make lint-go`, `make test-go` |
  | `python` | `ci.yml` | `make lint-python`, `make test-python` |
  | `govulncheck` | `ci.yml` | `make vuln` |
  | `secrets` | `ci.yml` | `make secrets` |
  | `actionlint` | `ci.yml` | `make lint-actions` |
  | `agentgateway compatibility test` | `extmcp.yml` | `make e2e` |
  | `gate end-to-end test` | `gate.yml` | `make e2e-gate` |

- Checks that start in Phase 2: `terraform fmt`, `terraform validate`, `tflint`, and the image and IaC scan.
- Each `uses:` line names a full commit SHA, with the version in a comment. Dependabot proposes updates of the actions, the Go modules and the Python packages each month.
- The ruleset on `main` is the file `.github/rulesets/main.json`: a pull request is required, each check above is required, and no account can bypass it. Change the file in a pull request, then apply it:

  ```sh
  gh api --method PUT repos/vanlabs-dev/agent-gate/rulesets/<id> --input .github/rulesets/main.json
  ```

  `gh api repos/vanlabs-dev/agent-gate/rulesets` lists the `<id>`.
- Terraform plan runs on the pull request. Apply runs on merge, behind an environment approval.
- Each change is tested on `kind` in CI.

## Environments

One environment, `demo`. `kind` in CI takes the place of a staging cluster.

## Dependencies

- Pin agentgateway and the `ExtMcp` bindings to an exact version. The versions are in `test/extmcp/versions.env`, `test/extmcp/go.mod` and `gate/go.mod`. Both modules use the same bindings commit.
- Upgrade through the compatibility test, `make e2e`, not by a floating tag.
- Dependabot does not update the check tools in `versions.env`, the Go version in `go.mod`, `test/extmcp/versions.env` or `gate/versions.env`. Update them by hand, one line at a time, and run `make check`.

## Secrets

- Azure Key Vault for runtime secrets. Workload identity for access.
- Nothing secret in the repo, in CI variables or in container images.

## Decision records

- One file for each decision in `docs/adr/`, numbered in order.
- Each has status, context, options, decision and consequences, and fits on one screen.
- A changed decision gets a new ADR that supersedes the old one.

## Documentation

- Each document states current truth only. History lives in git.
- Each price or third-party fact carries a source and the date checked.
- A document that no longer matches the code is a bug.

## Coding agents

- `AGENTS.md` at the repo root holds the instructions for AI coding agents.
- It links to the documents in `docs/` and does not restate them. It adds only the limits that apply to an agent, and the build, test and lint commands.

## Writing

- Plain technical English. Short sentences, active voice.
- No filler phrases.
- No em dashes.
