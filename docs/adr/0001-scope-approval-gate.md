# 0001: Scope is an approval and policy gate, not a gateway

- Status: Accepted
- Date: 2026-11-16
- Review: on or after 2026-11-23, when Microsoft Ignite 2026 announcements are public

## Context

The first idea was a full governed agent platform: identity, policy, audit, budgets, approval, observability and evaluations. A survey on 2026-11-16 found that most of this exists:

- MCP and LLM gateways: agentgateway, IBM ContextForge, LiteLLM, Azure API Management
- Agent identity: Microsoft Entra Agent ID
- Tracing and evaluations: Langfuse, Phoenix

Human approval of tool calls is less covered. Two open-source projects address it:

- [Preloop](https://github.com/preloop/preloop) holds tool calls for approval and keeps an audit hash chain. It is a full control plane in Python.
- [Microsoft Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit) has a `require_approval` policy action. It is a library inside the agent.

Neither codebase has been read yet.

## Options

1. Build a full platform, including a gateway.
2. Build one component: policy decision, human approval and verifiable audit, attached to an existing gateway.
3. Build nothing new: compose existing parts and operate them.

## Decision

Option 2. agent-gate decides tool calls against policy, holds the risky ones for a human, and keeps a verifiable audit record. It adds four things to the prior art:

1. Enforcement at the gateway, not inside the agent
2. Entra Agent ID claims in policy
3. An approver who cannot be the requester
4. An audit chain anchored outside the cluster

## Consequences

- The project adopts a gateway and does not write one. See [ADR 0002](0002-integrate-through-agentgateway-extmcp.md).
- Budgets, routing, tracing and evaluations come from adopted components.
- Phase 0 includes a read of Preloop and the Microsoft toolkit. If the four differences do not hold, this ADR changes.
- The scope is small enough for one maintainer to keep correct.
