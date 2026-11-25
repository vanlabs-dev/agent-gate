# Ship: gate-core

- Date: 2026-11-25
- Merged: PR #10, `f14d24d` on `main`. The proposal merged before it as PR #9.
- Verify: PASS, in [08-verify.md](08-verify.md).

## What shipped

- The gate, `gate/`: OPA policy with three outcomes, approvals bound to an RFC 8785 call digest, single-use replay, expiry and the requester rule, a hash-chained audit log with checkpoints in Postgres, and the `ExtMcp` adapter.
- `agent-gate-audit export` and `verify`.
- `make e2e-gate`: 14 scenarios on its own `kind` cluster with Dex v2.46.1 and Postgres. It is the required check `gate end-to-end test`. The ruleset on `main` was applied on 2026-11-25.
- The contract test now measures the callout deadline: the gateway fails closed after 5.00 s.

## Where things live

| What | Where |
|---|---|
| Specs | `openspec/specs/tool-call-policy/`, `tool-call-approval/`, `audit-log/`, plus changes in `agentgateway-integration/` and `repo-checks/` |
| Archived change | `openspec/changes/archive/2026-11-25-gate-core/` |
| Code and manifests | `gate/` |
| Workflow | `.github/workflows/gate.yml` |

## Follow-ups

- `approval-inbox`: replaces the test approval endpoint, and adds approver authorization.
- How to tie a `result` entry to its request. The response carries no tool name or request id. This is the open question in the design.
- A repeated call that needs approval appends another `approval_required` entry with the same ref. Revisit this if the log gets noisy.
- Phase 4: sign the checkpoints.
