# Brainstorm: gate-core

- Date: 2026-11-24
- Change: the first of two in Phase 1 of the [roadmap](../../roadmap.md). The second is `approval-inbox`.
- Project-level direction is in the [vision](../../vision.md), the [architecture options](../../architecture/options.md) and the [ADRs](../../adr/). This file covers this change only.

## Goal

Build the gate behind `ExtMcp`: policy decision, pending approvals, deny and replay, and a hash-chained audit log with a verifier CLI. Approvals come through a test-only endpoint. The `approval-inbox` change replaces that endpoint with the inbox and then runs the Phase 1 exit test.

## Success signals

One command runs an end-to-end test on `kind` against the pinned agentgateway and exits 0. The test shows:

1. A call that policy allows returns the upstream result.
2. A call that policy denies returns a tool error. The upstream server receives no call.
3. A call that needs approval returns a tool error with an approval reference. After approval, an identical replay passes once.
4. A replay with changed arguments, a replay by another caller, a second replay and a replay after expiry are each denied.
5. An approval by the requester is rejected.
6. A call with no identity claim is denied.
7. The verifier CLI accepts the audit chain. It rejects the chain after one entry is changed, removed or reordered.

Unit tests cover the canonical encoding against RFC 8785 test vectors, the digest, and each policy outcome. The callout deadline is set and measured.

## Constraints

- Policy, approval and audit code do not import `ExtMcp` types. The `ExtMcp` server is an adapter.
- Policy engine: OPA, embedded through its Go library ([ADR 0010](../../adr/0010-policy-engine-opa.md)).
- State in Postgres, in the `kind` cluster. The gate marks an approval used in the same transaction that passes the call.
- The digest covers tenant, agent, target, tool and canonical arguments. It reads only `name` and `arguments` from the params ([ADR 0009](../../adr/0009-suspension-deny-and-replay.md)).
- Canonical arguments use RFC 8785. Compare the Microsoft toolkit's approval protocol first.
- Identity comes from a local OIDC issuer in the cluster. Phase 2 swaps it for Entra by configuration only.
- Audit: hash chain and verifier only. Define the entry and checkpoint formats after comparing capsule-emit and Preloop's sealer, so that Phase 4 adds signing without a format change.
- No cloud resources and no cost.

## Non-goals

- The inbox, inbox sign-in and the notifier. These belong to `approval-inbox`.
- Entra, signed checkpoints, immutable storage
- A Helm chart. Plain manifests on `kind` are enough.
- A result digest for failed upstream calls ([issue #4245](https://github.com/agentgateway/agentgateway/issues/4245))

## Options considered

| Question | Options | Chosen |
|---|---|---|
| How to split Phase 1 | Two changes, or one | Two. `gate-core` first, because it holds the technical risk |
| Policy engine | OPA embedded, or Cedar | OPA. One decision document returns the three outcomes |
| Identity on `kind` | Local OIDC issuer, or static test tokens | Local OIDC issuer. Same code path as Entra |
| Audit anchor in Phase 1 | Chain only, or chain plus a local signing key | Chain only, with a format ready for signing |

## Chosen direction

A Go module in `gate/` with the core packages (policy, approval, audit, canonical encoding), an `ExtMcp` adapter, a test-only approval endpoint and a verifier CLI. Postgres and a local OIDC issuer run on `kind` beside agentgateway and the stub MCP server.

## Open questions

For the proposal session.

1. Does the real gate replace the stub gate in `test/extmcp/`, or does the compatibility test keep its stub and the gate get its own end-to-end test?
2. Which local OIDC issuer: an existing one such as Dex, or a small Go issuer? It must also serve sign-in for `approval-inbox`.
3. Which claim identifies the requester for the self-approval rule: `sub` now, `oid` with Entra? How does configuration map it?
4. The default expiry of an approval.
5. The audit entry fields, after the comparison with capsule-emit, Preloop and the toolkit.
6. How policy files reach the gate: a ConfigMap, or files built into the image.
