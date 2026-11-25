# Review: gate-core

- Date: 2026-11-25
- Session 3, outside voice. Input: the brainstorm, the OpenSpec change `gate-core`, ADR 0009, the architecture options, and the `test/extmcp/` code. No prior chat.

## Critical

| # | Finding | Proposed fix |
|---|---|---|
| C1 | The unsafe number rule says "a number that a 64-bit float cannot represent exactly". Read literally, it denies `0.1` and almost every decimal. | Deny an integer literal (no fraction, no exponent) outside ±(2^53 − 1), and any literal that overflows to infinity. Other numbers are compared after float64 rounding, as RFC 8785 requires. Define `args_digest` as `JCS(null)` when arguments are absent. |
| C2 | A ConfigMap volume holds `..data` and a timestamped `..2026_...` directory. "Every policy file in the directory" can load each file twice (a Rego conflict) and puts the timestamp name into the policy version, so each pod start gets a new version. | Load only top-level `*.rego` files, skip names that start with `.`, follow symlinks. Compute the version over base names and contents. Add a unit test with a ConfigMap-shaped directory. |
| C3 | Dex sets `aud` to the client id for `client_credentials`. Tokens of `agent-a`, `agent-b` and `approver` have three different audiences. The test endpoint validates one audience, so the self-approval test gets 401, not a requester-rule refusal. The gateway JWT policy has the same issue. | Task 1.1 also records `aud`. The gate and the gateway accept a configured list of audiences. If Dex can issue a shared audience (cross-client `trustedPeers`), use that instead and record it. |
| C4 | Task 9.2 sets the gate-wide expiry to "a few seconds". Every other approval scenario then races the clock. | Set the `kind` expiry to 30 s. Run the expiry scenario on its own request and wait 31 s. |
| C5 | The `pgtest` tests run against the same database as the gate. Their 50 appends land in the chain that scenario 9.3 exports, and a reset or migration in a test can break the gate mid-run. | The `pgtest` tests create and use their own database (`agent_gate_pgtest`) on the same server. |

## Medium

| # | Finding | Proposed fix |
|---|---|---|
| M1 | An approval can be used with no execution: the gate commits `used`, then the gateway times out, or the upstream call fails ([issue #4245](https://github.com/agentgateway/agentgateway/issues/4245)). The agent needs a new approval. | Add a risk line to the design. No code change. |
| M2 | Any identity with a valid token from the issuer can approve, including another agent. The scenario "Approval by another identity" fixes that as behaviour. | State in the spec and the design that approver authorization is out of scope here and belongs to `approval-inbox`. |
| M3 | `log_id` is the configured tenant value. In Phase 2 the tenant comes from `tid`, and one chain holds many tenants. | Add `audit.log_id`, default `local`, separate from the tenant. |
| M4 | The caller and tenant claims reach the gate only as gateway `metadata` keys (`sub: jwt.sub`). Configuring `oid` in the gate does nothing unless the route also forwards `oid: jwt.oid`. Policy `claims` holds only the forwarded keys. | Say in design decision 2 that the config names a metadata key, and that the manifests forward each key the gate reads. |
| M5 | Design decision 6 adds `POST /test/delay` to the gate. No task or scenario uses it. The deadline test uses the stub gate. | Remove it from the gate. |

## Accepted with no change

- The Phase 1 verifier proves internal consistency only. Anyone who can edit the export can recompute every hash. Already an accepted risk until Phase 4.
- Policy `reason` is free text from Rego and goes into the audit entry. A policy author can leak an argument value there. Policy authors own that.
- The scenario "Tool list" tests the route config (`methods: tools/call: Full`), not the gate. The gate never sees `tools/list`. Keep the requirement as a guard for a wider config.
- Task 8.1: if the measurement shows no deadline is enforced, stop and ask. This follows from the AGENTS.md limits and needs no edit.

## Resolution

On 2026-11-25 the maintainer accepted every fix for C1 to C5 and M1 to M5. They are folded into the OpenSpec change with `/opsx:update`: the specs `tool-call-approval`, `tool-call-policy` and `audit-log`, `design.md` and `tasks.md`. `proposal.md` is unchanged. The items under "Accepted with no change" stand as written.
