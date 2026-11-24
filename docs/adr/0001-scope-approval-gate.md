# 0001: Scope is an approval and policy gate, not a gateway

- Status: Accepted
- Date: 2026-11-16
- Review: 2026-11-24 against Microsoft Ignite 2026, the decision stands. Next at the end of each roadmap phase

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

## Review 2026-11-24

Ignite 2026 (17 to 20 November) and the prior art were checked on 2026-11-24. No source closes one of the four differences. Three narrow them.

- Work IQ agent policies can allow an action, block it, or ask a person first (public preview). They cover Microsoft 365 and connected sources. The posts do not say who approves, or whether the approval binds to the arguments ([Work IQ](https://techcommunity.microsoft.com/blog/microsoft_365blog/whats-new-in-work-iq-at-ignite-2026-agents-that-get-work-done/4572219), [Microsoft IQ](https://azure.microsoft.com/en-us/blog/introducing-the-new-microsoft-iq-putting-your-organizations-intelligence-to-work-in-every-agent)).
- Conditional Access for agents can require human approval at the point of action (preview) ([Agent 365](https://techcommunity.microsoft.com/blog/agent-365-blog/whats-new-in-microsoft-agent-365-ignite-2026/4571753)).
- [capsule-emit](https://github.com/action-state-group/capsule-emit) (Apache-2.0) seals each agentgateway `tools/call` through `ExtMcp` into signed, hash-chained records with checkpoints at an outside witness ([comment on issue #3485](https://github.com/agentgateway/agentgateway/issues/3485)). It has no policy and no approval. Compare it before building the audit chain.

Unchanged: Azure AI Gateway is GA with MCP federation, per-tool policies and OpenTelemetry logs, but no approval and no tamper-evident audit ([AI Gateway](https://techcommunity.microsoft.com/blog/integrationsonazureblog/azure-ai-gateway-is-now-generally-available-ship-faster-stay-in-control/4572644)). Entra Agent ID changed no token claim. No source names agentgateway or a rule that the approver differs from the requester. Preloop v1.0.2 and the toolkit v5.0.0 are still the latest versions.
