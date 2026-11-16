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

Two open-source projects already cover part of this ground. Both were reviewed from their READMEs on 2026-11-16, not from their code. A closer read is a Phase 0 task.

- [Preloop](https://github.com/preloop/preloop) (Apache-2.0, Python) is an agent control plane with an MCP firewall that can hold a tool call for approval, and an audit hash chain.
- [Microsoft Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit) (MIT) is a library that runs inside the agent. Its policies include a `require_approval` action.

agent-gate differs in four ways:

1. It enforces at the gateway, not inside the agent. An agent cannot skip a check that it does not run.
2. Policy can use Entra Agent ID claims, so a rule can tell an autonomous agent from one that acts for a user.
3. The approver cannot be the requester.
4. The audit chain is anchored outside the cluster, so a cluster administrator cannot rewrite it without detection.

If the closer read shows that these differences do not hold, [ADR 0001](adr/0001-scope-approval-gate.md) changes.

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

Planning. No code exists yet. See the [roadmap](roadmap.md).
