# Vision

## What agent-gate is

agent-gate is an open-source approval and policy gate for AI agent tool calls. It decides each call against policy, holds the risky ones for a human, and keeps an audit record that anyone can verify.

## The problem

Organisations want agents that act on real systems. Three questions stop them:

- What may this agent do?
- Who approved the risky action?
- Can we prove it afterwards?

Gateways route, authenticate and rate-limit agent traffic. Few of them put a human decision in the path of a risky call and leave a record that a third party can check.

## What it does

- Evaluates each MCP tool call against policy. The result is allow, deny or needs approval.
- Holds a call that needs approval until a human decides in an inbox.
- Rejects an approver who is also the requester.
- Passes an approved call once, bound to the exact tool and arguments that were approved.
- Records each decision in a hash-chained audit log, with signed checkpoints stored outside the cluster.

## Where it fits

agent-gate is not a gateway. It plugs into one.

| Concern | Owner |
|---|---|
| MCP and LLM traffic, routing, token budgets | [agentgateway](https://github.com/agentgateway/agentgateway) |
| Agent and human identity | Microsoft Entra ID and Entra Agent ID |
| Policy decision, approval, audit | agent-gate |

## Prior art

Three open-source projects cover parts of this ground. Preloop v1.0.2 and the Microsoft Agent Governance Toolkit v5.0.0 were read from source on 2026-11-16. capsule-emit was checked on 2026-11-24.

- [Preloop](https://github.com/preloop/preloop) (Apache-2.0) is an agent control plane and its own MCP proxy. It holds tool calls for approval under CEL rules, notifies approvers through several channels, and keeps a signed audit hash chain with an offline verifier.
- [Microsoft Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit) (MIT) is mainly a library inside the agent process. It has a `require_approval` policy action and a protocol that binds an approval to a digest of the action.
- [capsule-emit](https://github.com/action-state-group/capsule-emit) (Apache-2.0) seals agent actions into signed, hash-chained records that anyone can verify offline. Its `ExtMcp` processor does this for each `tools/call` through agentgateway ([issue #3485](https://github.com/agentgateway/agentgateway/issues/3485)). It has no policy and no approval.

agent-gate differs in four ways. Each difference is narrow.

| | agent-gate (planned) | Preloop | Microsoft toolkit | capsule-emit |
|---|---|---|---|---|
| Where it enforces | Inside agentgateway, as an extension | In its own MCP proxy | Inside the agent process, plus one Azure API Management sample | Records only, inside agentgateway as an extension |
| Entra Agent ID claims in policy | Yes | No | Verifies Entra tokens, but policy does not use the agent-type claims | No policy. Records the JWT subject |
| Approver cannot be the requester | For every gated call | For one built-in tool | No such rule | No approval |
| Audit anchored outside the system | Automatic, to immutable storage | Signed checkpoints, but the key is stored with the data and export is manual | Not implemented | Checkpoints at an outside witness log |

A team that does not run agentgateway or Entra should look at Preloop first.

[ADR 0001](adr/0001-scope-approval-gate.md) records this comparison and when to review it.

## Audience

Platform and security teams in Microsoft-centric organisations that run agents against real systems.

## Non-goals

- An MCP proxy or gateway
- LLM routing
- An agent framework
- An evaluation product
- A hosted service

## Evidence

This repo is also a public portfolio. Each claim in it must link to evidence:

- A running demo that invited guests can sign in to
- The audit trail of the project's own operations
- The monthly cloud bill, compared with the [cost model](cost-model.md)
- The [decision records](adr/)

## Status

Phase 0 of the [roadmap](roadmap.md) is complete, and Phase 1 started on 2026-11-24. A compatibility test on `kind` proves the integration with agentgateway ([ADR 0002](adr/0002-integrate-through-agentgateway-extmcp.md)). The gate core shipped on 2026-11-25 and runs on `kind`. The approval inbox is next, then the tag `v0.1.0`.
