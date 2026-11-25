# Agent instructions

Instructions for AI coding agents that work in this repo.

## Read first

| Question | Document |
|---|---|
| What is this project, and what is out of scope? | [docs/vision.md](docs/vision.md) |
| How do the parts fit, and what is still open? | [docs/architecture/options.md](docs/architecture/options.md) |
| Why was a decision made? | [docs/adr/](docs/adr/) |
| What is being built now? | [docs/roadmap.md](docs/roadmap.md) |
| What may this cost? | [docs/cost-model.md](docs/cost-model.md) |
| How are commits, pull requests, CI, dependencies, secrets and docs handled? | [docs/conventions.md](docs/conventions.md) |

## Commands

Run each from the repo root. Run `make check` before each push.

| Command | Runs | Needs |
|---|---|---|
| `make check` | `make lint` and `make test` | Go, `uv` |
| `make lint` | Go lint (`golangci-lint`), Python lint (`ruff`), workflow lint (`actionlint`) | Go, `uv` |
| `make test` | Go unit tests and Python tests. No cluster | Go, `uv` |
| `make vuln` | `govulncheck` on each Go module | Go, network |
| `make secrets` | `gitleaks` on the history of `HEAD` | Go |
| `make e2e` | The agentgateway compatibility test on a `kind` cluster | Go, Docker, `kind`, `kubectl`, Helm 4 |
| `make e2e-gate` | The gate end-to-end test on its own `kind` cluster: agentgateway, Dex, Postgres, the gate and the stub MCP server | Go, Docker, `kind`, `kubectl`, Helm 4 |
| `make clean` | Deletes the `kind` clusters of `make e2e` and `make e2e-gate` | `kind` |

The Go tools run with `go run <module>@<version>`. Their versions are in [`versions.env`](versions.env). Nothing needs to be installed beyond Go and `uv`.

## Limits

Stop and ask the maintainer before you:

- Run a command that creates, changes, starts or deletes a cloud resource, for example `terraform apply` or `az aks start`.
- Do anything else that spends money.
- Change the decision in an accepted ADR. Draft a new ADR with status "Proposed" in its place.
- Start work that is outside the current roadmap phase, or that the vision lists as a non-goal.
