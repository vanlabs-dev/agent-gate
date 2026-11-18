# 0002: Integrate through the agentgateway ExtMcp extension

- Status: Accepted, confirmed by the Phase 0 spike on 2026-11-16 against agentgateway v1.7.0
- Date: 2026-11-16
- Review: on or after 2026-11-23

## Context

The gate needs a place in the request path where it can see each MCP tool call and the caller's identity, and stop the call.

[agentgateway](https://github.com/agentgateway/agentgateway) (Apache-2.0, Linux Foundation) has an MCP-aware gRPC extension, [`ExtMcp`](https://github.com/agentgateway/agentgateway/blob/main/crates/protos/proto/ext_mcp.proto). It sends the gate the MCP method, the JSON-RPC params and selected identity claims. The gate can answer pass, mutate or deny. It cannot answer the client directly. [Issue #3003](https://github.com/agentgateway/agentgateway/issues/3003) requests that and is open.

## Options

1. **Extension:** a Go service behind `ExtMcp`.
2. **Proxy:** a standalone MCP reverse proxy in Go.
3. **MCP target:** a Go MCP server that agentgateway routes gated tools to.

[Architecture options](../architecture/options.md) compares them.

## Decision

Option 1. The gate is an `ExtMcp` server. Policy, approval and audit code stay independent of `ExtMcp` types, so that options 2 or 3 remain possible.

## Consequences

- The gate cannot suspend a call. See [ADR 0009](0009-suspension-deny-and-replay.md).
- The project depends on a young API. The agentgateway version is pinned in `test/extmcp/versions.env`. An upgrade must pass the compatibility test, `make e2e`.
- Routes must attach a `jwtAuthentication` policy, or the gate receives no claims ([issue #3485](https://github.com/agentgateway/agentgateway/issues/3485)).
- The callout needs an explicit deadline.

## Confirmation

The compatibility test in `test/extmcp/` passed on `kind` on 2026-11-16 against agentgateway v1.7.0 (commit `26f5aebe`). It proves the [integration contract](../../openspec/specs/agentgateway-integration/spec.md). Findings:

- The feature is configured as `backend.mcp.guardrails` in an `AgentgatewayPolicy`. `ExtMcp` names only the gRPC service.
- The gate receives claims as `metadata` expressions such as `jwt.sub`, and never the token. A failed expression is skipped, so the gate must deny a call that carries no identity claim.
- The gate receives the gateway's re-encoding of the params with `_meta` merged in, not the client's bytes.
- An empty `allowedRequestHeaders` forwards every request header to the gate. Set the list.
- A gate outage fails closed. The client receives the JSON-RPC internal error `guardrail check failed`, and the upstream server receives no call.

Source for each finding: the agentgateway [v1.7.0 source](https://github.com/agentgateway/agentgateway/blob/v1.7.0/crates/agentgateway/src/mcp/guardrails), read on 2026-11-16, and the test result.
