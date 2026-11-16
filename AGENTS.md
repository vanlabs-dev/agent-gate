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

## Limits

Stop and ask the maintainer before you:

- Run a command that creates, changes, starts or deletes a cloud resource, for example `terraform apply` or `az aks start`.
- Do anything else that spends money.
- Change the decision in an accepted ADR. Draft a new ADR with status "Proposed" in its place.
- Start work that is outside the current roadmap phase, or that the vision lists as a non-goal.
