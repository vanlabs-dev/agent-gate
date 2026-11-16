# Architecture options

Three shapes were considered. Option A is the chosen direction. A spike in Phase 0 must confirm it. Facts about other projects were checked on 2026-11-16.

## Summary

| | A: gate as extension (chosen) | B: gate as proxy | C: gate as MCP target |
|---|---|---|---|
| Shape | Go service that agentgateway calls through `ExtMcp` | Standalone MCP reverse proxy in Go | Go MCP server that agentgateway routes gated tools to |
| Code this project owns | Policy, approval, audit, inbox | The same, plus MCP proxying | The same, plus upstream execution |
| Can suspend a call | No: deny, then replay | Yes, through MCP tasks, which no client supports yet | Yes, with the same limit |
| Main risk | Early extension API on a project with monthly releases | Rebuilds a gateway alone against a moving spec | Two hops and duplicated routing |

## Option A: gate as extension

agentgateway carries MCP and LLM traffic. For each MCP call it asks agent-gate for a decision through its `ExtMcp` gRPC extension.

```mermaid
flowchart LR
  agent[Python agent] -->|tools/call with Entra token| agw[agentgateway]
  agw -->|ExtMcp CheckRequest| gate[agent-gate]
  agw -->|passed calls| mcp[MCP servers]
  gate --> pg[(Postgres)]
  gate --> blob[("Immutable Blob storage")]
  human[Approver] -->|Entra sign-in| inbox[Approval inbox]
  inbox --> gate
```

### Components

- **agentgateway:** validates the Entra token, routes MCP and LLM traffic, enforces token budgets.
- **agent-gate:** `ExtMcp` server, policy engine, approval API and inbox, audit appender, checkpoint signer.
- **Postgres:** approval requests, audit entries, checkpoints.
- **Verifier CLI:** checks the audit chain against the signed checkpoints.
- **Python agent:** the reference agent. See [ADR 0004](../adr/0004-anchor-agent-azure-ops.md).

### What the extension point allows

The `ExtMcp` service has two calls, `CheckRequest` and `CheckResponse` ([proto](https://github.com/agentgateway/agentgateway/blob/main/crates/protos/proto/ext_mcp.proto)). Each can return one of three results:

- `pass`
- `mutated`: replace the params or the result
- `error`: deny

No result answers the client without a call to the upstream server. [Issue #3003](https://github.com/agentgateway/agentgateway/issues/3003) requests one and is open. The gate therefore cannot return a pending result. It denies, and the agent replays the call after approval.

### Request flow for a call that needs approval

```mermaid
sequenceDiagram
  participant A as Agent
  participant G as agentgateway
  participant X as agent-gate
  participant H as Approver
  participant S as MCP server
  A->>G: tools/call with Entra token
  G->>X: CheckRequest with params and claims
  X->>X: Evaluate policy, result is needs approval
  X->>X: Store pending request, append audit entry
  X-->>G: error with approval reference
  G-->>A: tool error with approval reference
  H->>X: Approve in inbox
  X->>X: Append audit entry
  A->>G: tools/call, identical
  G->>X: CheckRequest
  X->>X: Match digest, mark approval used
  X-->>G: pass
  G->>S: tools/call
  S-->>G: result
  G->>X: CheckResponse
  X->>X: Append result digest
  G-->>A: result
```

1. The agent gets an Entra token and calls `tools/call` through agentgateway.
2. agentgateway validates the token and sends the call and identity claims to the gate.
3. The gate evaluates policy. The result is allow, deny or needs approval.
4. For needs approval, the gate stores a pending request. The key is a digest of tenant, agent, target, tool and canonical arguments. The gate appends an audit entry and shows the request in the inbox.
5. The gate denies with an approval reference.
6. The agent stores the exact call and waits.
7. A human signs in to the inbox and decides. The gate rejects an approver who is the requester. It appends an audit entry.
8. The agent replays the call. The gate matches the digest, marks the approval used, and passes the call.
9. The gate records a digest of the result.

### Known limits of A

- The agent must replay the stored call. If it asks the model again and the arguments change, the digest does not match.
- The gate sees claims only when a `jwtAuth` policy is attached to the route ([issue #3485](https://github.com/agentgateway/agentgateway/issues/3485)).
- `CheckResponse` does not run for upstream errors ([issue #4245](https://github.com/agentgateway/agentgateway/issues/4245)), so a failed execution leaves no result digest.
- The callout has no deadline by default. Set one.
- `ExtMcp` is a young API. Pin the agentgateway version and upgrade through a compatibility test.

## Option B: gate as proxy

agent-gate is its own MCP reverse proxy. An adopted component still carries LLM traffic.

```mermaid
flowchart LR
  agent[Python agent] -->|MCP| gate[agent-gate proxy]
  gate --> mcp[MCP servers]
  gate --> pg[(Postgres)]
  agent -->|LLM calls| agw[agentgateway] --> llm[Models]
```

### Request flow for a call that needs approval

1. The agent calls `tools/call` on the gate with an Entra token.
2. The gate validates the token and evaluates policy.
3. The gate stores a pending request and appends an audit entry.
4. If the client declared the MCP tasks extension, the gate returns a task handle. If not, it denies with an approval reference, as in option A.
5. A human decides in the inbox.
6. With tasks, the gate calls the upstream server itself and serves the result through `tasks/get`. Without tasks, the agent replays the call.

B gives true suspension, but only through the [MCP tasks extension](https://modelcontextprotocol.io/extensions/tasks/overview). On 2026-11-16 the [client matrix](https://modelcontextprotocol.io/extensions/client-matrix) listed no client that supports it. B also means that this project maintains an MCP proxy against a moving spec.

## Option C: gate as MCP target

agentgateway routes gated tools to agent-gate, which is an MCP server that calls the real upstream after approval.

```mermaid
flowchart LR
  agent[Python agent] --> agw[agentgateway]
  agw -->|gated tools| gate[agent-gate MCP server]
  gate --> mcp[MCP servers]
  agw -->|other tools| mcp
```

C keeps agentgateway in front and lets the gate execute after approval. It adds a second hop and duplicates routing configuration in two places.

## Cross-cutting design

### Audit

- Each entry stores SHA-256 of the previous hash plus the canonical JSON of the entry.
- One appender writes under a lock, so the chain has one order.
- On a schedule, the gate signs the chain head with a Key Vault key and writes the checkpoint to immutable Blob storage.
- The verifier CLI recomputes the chain and compares it with the checkpoints.

### Identity

- AKS workload identity gives the agent pod a managed identity. That identity is the credential for an Entra Agent ID blueprint.
- Policy can use these [token claims](https://learn.microsoft.com/en-us/entra/agent-id/agent-token-claims): `tid`, `azp`, `oid`, `idtyp`, `roles`, and the agent markers `xms_act_fct` and `xms_sub_fct`.
- The approver signs in to the inbox with Entra OIDC.

### Transport independence

The policy, approval and audit code must not depend on `ExtMcp` types. The extension server is an adapter. A tasks front or a proxy front can then be added without a rewrite.

## Open decisions

| Decision | Options | Lean |
|---|---|---|
| Policy engine, and how "needs approval" is expressed | Cedar (`cedar-go`) with a second action, OPA with a decision document | Decide in Phase 1 |
| State store | Postgres in the cluster on a disk, Azure Flexible Server B1ms | In-cluster first: US$3 against US$19 a month |
| LLM routing and budgets | agentgateway, LiteLLM | agentgateway: one data plane, fewer pods |
| Observability | OTel Collector to Application Insights, self-hosted Langfuse | Application Insights: no cluster memory, 5 GB free |
| Deploy | Helm from GitHub Actions, Flux | Helm first |

Each of these becomes an ADR when it is decided.
