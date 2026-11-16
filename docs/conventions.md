# Conventions

## Repo layout

Proposed. Nothing below exists until its phase starts.

| Path | Content |
|---|---|
| `gate/` | The gate, in Go |
| `agents/azure-ops/` | The reference agent, in Python |
| `infra/` | Terraform |
| `deploy/` | Helm charts |
| `docs/` | Vision, architecture, ADRs, roadmap, cost model |

## Branches and pull requests

- Trunk-based. `main` is always deployable.
- Each change goes through a short-lived branch and a pull request, including changes by the maintainer.
- Each pull request passes a self-review checklist: tests added, docs updated, no secrets, cost effect stated if any.

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
- Required checks on each pull request:
  - Lint and tests for Go and Python
  - `terraform fmt`, `terraform validate`, `tflint`
  - `govulncheck`
  - Image and IaC scan
  - Secret scan
- Terraform plan runs on the pull request. Apply runs on merge, behind an environment approval.
- Each change is tested on `kind` in CI.

## Environments

One environment, `demo`. `kind` in CI takes the place of a staging cluster.

## Dependencies

- Pin agentgateway and the `ExtMcp` bindings to an exact version.
- Upgrade through a compatibility test, not by a floating tag.

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
