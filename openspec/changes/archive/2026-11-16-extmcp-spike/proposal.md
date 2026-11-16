# Proposal

## Why

[ADR 0002](../../../../docs/adr/0002-integrate-through-agentgateway-extmcp.md) and [ADR 0009](../../../../docs/adr/0009-suspension-deny-and-replay.md) rest on one unproven claim: agentgateway can call an external gate for each MCP `tools/call`, the gate can deny with an approval reference, and an identical replay passes after approval. Every later phase builds on that claim, so it must be proven or disproven first. Scope and success signals are in the [brainstorm](../../../../docs/greenfield/extmcp-spike/01-brainstorm.md).

## What Changes

- Add a Go module at `test/extmcp/` with a stub `ExtMcp` gate, a stub MCP server and one end-to-end test.
- Add Kubernetes manifests that deploy agentgateway v1.7.0, both stubs, a route, a JWT policy and an `mcp.guardrails` policy on `kind`.
- Add a root `Makefile` with a `test` target. It creates the cluster, deploys, runs the test and exits 0 on success.
- Add one GitHub Actions job that runs `make test`.
- Add `LICENSE` (Apache-2.0) and a short `README.md`. Create the public repo `vanlabs-dev/agent-gate` and push.
- Install Go, `kind`, `kubectl` and `helm` on the development machine.
- Record the result: mark ADR 0002 and ADR 0009 confirmed or replace them, and correct the docs where the source read found differences.

## Capabilities

### New Capabilities

- `agentgateway-integration`: the contract between agentgateway and a gate behind its `ExtMcp` extension. It covers what the gate receives, how a deny reaches the client, the deny and replay sequence, the behaviour when the gate is unavailable, and the pinned version the contract is tested against.

### Modified Capabilities

None.

## Impact

- **New code:** `test/extmcp/` (Go module, manifests, Dockerfile), `Makefile`, `.github/workflows/`.
- **New dependencies:** agentgateway v1.7.0 (charts and images from `ghcr.io/agentgateway`), the Go module `github.com/agentgateway/agentgateway/api`, the MCP Go SDK, gRPC for Go.
- **Docs:** ADR 0002, ADR 0009, `docs/architecture/options.md`, `docs/conventions.md` (repo layout), `docs/vision.md` (status), `docs/roadmap.md` if the result changes option A.
- **Development machine:** new tools under `~/.local`, new variables in `~/.bashrc`.
- **Outside the machine:** one public GitHub repo. GitHub Actions on a public repo has no cost.
- **Cost:** none. No cloud resources.

Approved: 2026-11-16 (vanlabs-dev)

Review changes approved: 2026-11-16 (vanlabs-dev). See the [review](../../../../docs/greenfield/extmcp-spike/07-review.md).
