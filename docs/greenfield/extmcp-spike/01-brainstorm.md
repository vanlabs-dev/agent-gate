# Brainstorm: extmcp-spike

- Date: 2026-11-16
- Change: the first of two in Phase 0 of the [roadmap](../../roadmap.md). The second is `repo-foundations`.
- Project-level direction is in the [vision](../../vision.md), the [architecture options](../../architecture/options.md) and the [ADRs](../../adr/). This file covers this change only.

## Goal

Prove that agentgateway can call an external Go service through `ExtMcp` for each MCP `tools/call`, that the service can deny a call with an approval reference, and that an identical replay passes after approval. The result confirms or changes [ADR 0002](../../adr/0002-integrate-through-agentgateway-extmcp.md) and [ADR 0009](../../adr/0009-suspension-deny-and-replay.md).

## Success signals

One command creates a `kind` cluster, deploys a pinned agentgateway, a stub MCP server and a stub gate, runs the test, and exits 0. The test shows:

1. An ungated call returns the upstream result.
2. A gated call returns a tool error that carries an approval reference. The upstream server does not receive the call.
3. After the approval is recorded, an identical replay reaches the upstream server and returns its result.
4. A replay with changed arguments is denied.
5. A second replay of a used approval is denied.
6. The gate receives the tool name, the arguments and at least one identity claim from a validated token.
7. `CheckResponse` runs with the result of a passed call.

The same test passes in GitHub Actions on the public repo. ADR 0002 and 0009 are then marked confirmed, or replaced.

## Constraints

- Time box: 20 hours. If the test does not pass by then, record what failed and choose between options B and C.
- Pin agentgateway to one exact version. Record it.
- The stub gate holds state in memory, gates one hard-coded tool, and takes approvals through a test-only endpoint.
- The token comes from a local test issuer. No Entra.
- No cloud resources and no cost.
- The repo `vanlabs-dev/agent-gate` goes public when this change is applied. The first push includes the Apache-2.0 licence file and a short README.
- The Go toolchain, `kind`, `kubectl` and `helm` are installed on the development machine as part of this change. `GOPATH` must be set explicitly and must not be `~/go`.

## Non-goals

- A policy engine, Postgres, the inbox, the audit chain
- Lint, the Python scaffold, the `AGENTS.md` commands. These belong to `repo-foundations`.
- Performance measurement

## Options considered

| Question | Options | Chosen |
|---|---|---|
| How to split Phase 0 | Spike first and scaffold second, or one change | Spike first. If it fails, ADR 0002 changes before a layout is built around it |
| When the repo goes public | From the first change, or private until the spike passes | From the first change |
| Toolchain | Installed on the machine, or containers only | Installed on the machine |
| Licence | Apache-2.0, or MIT | Apache-2.0. It matches agentgateway and carries a patent grant |
| Scope after the prior-art read | Continue, or pause and rethink | Continue. The spike tests the agentgateway placement, the main difference that remains ([ADR 0001](../../adr/0001-scope-approval-gate.md)) |

## Chosen direction

A small Go module with a stub `ExtMcp` server, a stub MCP server, manifests for `kind`, one end-to-end test and one CI job. The spike code stays in the repo and becomes the seed of the agentgateway compatibility test that [conventions](../../conventions.md) requires.

## Open questions

For the proposal session.

1. Which agentgateway version to pin, and whether its Kubernetes mode installs cleanly on `kind`.
2. How `ExtMcp` is configured on a route, and which field maps token claims into `metadata_context`. Research did not confirm this.
3. Whether an `error` result from `CheckRequest` always skips the upstream call. The proto does not say so.
4. What the client receives on a deny: a tool result with `isError`, or the `mcp_error` payload. The proto comment and the docs disagree.
5. Where the Go bindings come from: generated from a vendored copy of the pinned proto, or an upstream module.
6. Which MCP client the test uses: the MCP Go SDK or raw JSON-RPC over HTTP.
7. Where the spike lives in the repo, given that it becomes the compatibility test.
8. Whether the Ignite review due on or after 2026-11-23 happens before the apply session.
