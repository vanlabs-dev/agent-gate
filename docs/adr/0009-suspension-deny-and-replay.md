# 0009: Suspension contract is deny and replay

- Status: Accepted, confirmed by the Phase 0 spike on 2026-11-16 against agentgateway v1.7.0
- Date: 2026-11-16

## Context

An approval can take minutes or hours, and the approver is not the caller. The tool call must wait without an open connection.

- agentgateway's `ExtMcp` can pass, mutate or deny. It cannot return a pending result ([ADR 0002](0002-integrate-through-agentgateway-extmcp.md)).
- MCP `input_required` ([spec](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr)) asks the calling client for input. It does not fit a third-party approver.
- The MCP [tasks extension](https://modelcontextprotocol.io/extensions/tasks/overview) fits. On 2026-11-16 the [client matrix](https://modelcontextprotocol.io/extensions/client-matrix) listed no client that supports it.

## Options

1. **Deny and replay.** The gate denies with an approval reference bound to a digest of the call. After approval the agent replays the identical call and the gate passes it once.
2. **MCP tasks.** The gate returns a task handle and the client polls.
3. **Poll tool.** The gate returns a pending result and exposes a tool that the agent calls to check status.

## Decision

Option 1 now. Option 2 later, when clients support tasks.

- The digest covers tenant, agent, target, tool and canonical arguments.
- An approval is single use and expires.
- The gate marks an approval used in the same transaction that passes the call.

## Consequences

- The agent must store the exact call and replay it. If it asks the model again and the arguments change, the digest does not match and a new approval is needed.
- An agent that does not know the contract sees a tool error. The reference agent shows the correct handling.
- Canonical argument encoding must be specified and tested, or equal calls produce different digests. RFC 8785 (JCS) is the candidate. The Microsoft Agent Governance Toolkit binds approvals the same way. Compare its protocol before defining a new one.
- The gate does not execute the call. The agent does, with its own identity, so the upstream audit trail stays truthful.

## Confirmation

The compatibility test in `test/extmcp/` passed on `kind` on 2026-11-16 against agentgateway v1.7.0. A denied call passed once after approval, in a new session with a new token for the same `sub`. A replay with changed arguments, a replay by another caller and a second replay were each denied. Findings:

- The approval reference travels as text only. For `tools/call`, the client receives a tool result with `isError` true and the gate's `reason` as one text block. agentgateway drops the structured `mcp_error` payload on this path ([source](https://github.com/agentgateway/agentgateway/blob/v1.7.0/crates/agentgateway/src/mcp/guardrails/mod.rs), read on 2026-11-16).
- The digest must read only `name` and `arguments` from the params. agentgateway merges `_meta` into the params, and `_meta` can change with each call.
- The spike digest is not canonical. The canonical encoding is still to be defined.
