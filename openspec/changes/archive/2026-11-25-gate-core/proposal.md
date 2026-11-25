# Proposal

## Why

Phase 0 proved the deny and replay contract with a stub gate. Phase 1 needs the real gate: a policy decision for each tool call, approvals that bind to one exact call, and an audit log that a third party can check. This change builds that core and proves it on `kind`. The [brainstorm](../../../../docs/greenfield/gate-core/01-brainstorm.md) holds scope and success signals. The `approval-inbox` change follows it and replaces the test-only approval endpoint with the inbox.

## What Changes

- Add a Go module at `gate/`: policy, approval, audit and canonical encoding packages with no `ExtMcp` types, an `ExtMcp` adapter, a Postgres store, and a test-only approval endpoint that is off unless a flag enables it.
- Embed OPA ([ADR 0010](../../../../docs/adr/0010-policy-engine-opa.md)). Rego files load from a directory, mounted from a ConfigMap on `kind`.
- Bind each approval to a digest of tenant, agent, target, tool and RFC 8785 canonical arguments ([ADR 0009](../../../../docs/adr/0009-suspension-deny-and-replay.md)). An approval is single use, expires after 1 hour by default, and cannot be given by the requester.
- Append each decision to a SHA-256 hash chain in Postgres. Write unsigned checkpoints in a format that takes signatures in Phase 4 with no change.
- Add the CLI `agent-gate-audit` with `export` and `verify`.
- Add an end-to-end test on its own `kind` cluster: agentgateway at the pinned version, Dex v2.46.1 as the local OIDC issuer, Postgres, the gate and the stub MCP server from `test/extmcp/`. One command runs it: `make e2e-gate`. CI runs it as a new required check.
- Keep the stub gate in `test/extmcp/` as the pinned-version contract test. Add one scenario to it that sets and measures the callout deadline.

## Capabilities

### New Capabilities

- `tool-call-policy`: how the gate decides a tool call: the caller identity it requires, the policy input, the three outcomes, and fail-closed behaviour.
- `tool-call-approval`: how a call that needs approval is held, approved or rejected, and passed once: the call digest, canonical arguments, single use, expiry and the requester rule.
- `audit-log`: what the gate records for each decision, the hash chain and checkpoint formats, and the verifier CLI.

### Modified Capabilities

- `agentgateway-integration`: add a requirement that the gate callout has a deadline, and that a gate slower than the deadline fails closed.
- `repo-checks`: add the gate end-to-end test as a command and as a required pull request check.

## Impact

- **New code:** `gate/` (Go module, manifests, Dockerfile, end-to-end test). One delay knob in `test/extmcp/cmd/stub-gate`.
- **Changed files:** `Makefile`, `.github/workflows/`, `.github/rulesets/main.json`, `.github/dependabot.yml`, `AGENTS.md`.
- **New dependencies:** the OPA Go module, `pgx`, `go-oidc`, an RFC 8785 library, the Dex and Postgres images.
- **Docs:** `docs/conventions.md` (repo layout, checks), `docs/architecture/options.md` (deadline, identity on `kind`), `docs/roadmap.md` (Phase 1 status).
- **Cost:** none. No cloud resources. CI on a public repo has no cost.

Approved: 2026-11-25 (vanlabs-dev)
