# 0002: Integrate through the agentgateway ExtMcp extension

- Status: Accepted, to be confirmed by a spike in Phase 0
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
- The project depends on a young API. Pin the agentgateway version and upgrade through a compatibility test.
- Routes must attach a `jwtAuth` policy, or the gate receives no claims ([issue #3485](https://github.com/agentgateway/agentgateway/issues/3485)).
- The callout needs an explicit deadline.
- The Phase 0 spike must show a deny followed by a passed replay on `kind`. If it fails, change this ADR to option 3 or 2.
