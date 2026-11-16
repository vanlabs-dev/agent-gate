# 0001: Scope is an approval and policy gate, not a gateway

- Status: Accepted
- Date: 2026-11-16
- Review: on or after 2026-11-23 (Microsoft Ignite 2026), then at the end of each roadmap phase

## Context

The first idea was a full governed agent platform. A survey on 2026-11-16 found that most of it exists:

- MCP and LLM gateways: agentgateway, IBM ContextForge, LiteLLM, Azure API Management
- Agent identity: Microsoft Entra Agent ID
- Tracing and evaluations: Langfuse, Phoenix

Human approval of tool calls also exists in open source. Both projects below were read from source on 2026-11-16.

- [Preloop](https://github.com/preloop/preloop) v1.0.2 is its own MCP proxy with CEL approval rules, several approval channels, a signed audit hash chain, an offline verifier and a Helm chart. It has no gateway extension, no Entra claims in policy, and no general rule that the approver differs from the requester. Its checkpoint key is stored with the data.
- [Microsoft Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit) v5.0.0 enforces inside the agent process. Its approval protocol binds an approval to an RFC 8785 digest of the action, with single use and expiry. Its approval stores are in memory, and it has no external audit anchoring.

## Options

1. Build a full platform, including a gateway.
2. Build one component: policy decision, human approval and verifiable audit, attached to an existing gateway.
3. Build nothing new: operate Preloop or the Microsoft toolkit.

## Decision

Option 2. agent-gate adds four things to the prior art:

1. Enforcement inside agentgateway, for teams that already run it
2. Entra Agent ID claims in policy
3. An approver who cannot be the requester, for every gated call
4. Audit checkpoints written automatically to immutable storage outside the system

## Consequences

- The project adopts a gateway and does not write one. See [ADR 0002](0002-integrate-through-agentgateway-extmcp.md).
- The four differences are narrow. Either project could close one in a release. The review dates above exist for that reason.
- Reuse before invention: compare the toolkit's action-bound approval protocol before defining the digest binding ([ADR 0009](0009-suspension-deny-and-replay.md)), and Preloop's sealer design before building the audit chain.
- If a review finds no difference left, option 3 or a contribution to one of those projects replaces this decision.
