# Design

## Context

See [proposal.md](proposal.md) for motivation and the [brainstorm](../../../../docs/greenfield/gate-core/01-brainstorm.md) for scope. The repo has one Go module, `test/extmcp/`: a stub gate with in-memory state and a non-canonical digest, a stub MCP server, manifests and an end-to-end test that signs its own tokens. Its facts about agentgateway v1.7.0 are in [ADR 0009](../../../../docs/adr/0009-suspension-deny-and-replay.md) and the [architecture options](../../../../docs/architecture/options.md#known-limits-of-a). This design relies on them and does not repeat them.

Research on 2026-11-25 read three projects: the Microsoft Agent Governance Toolkit (commit `3233be2`, ADR 0030 "action-bound approval protocol"), Preloop (commit `24d6624`, `services/audit_chain.py`) and capsule-emit (commit `e744134`). Decisions 4, 5 and 8 cite what each one gave.

### Answers to the brainstorm's open questions

| # | Question | Answer |
|---|---|---|
| 1 | Replace the stub gate? | No. `test/extmcp/` stays the pinned-version contract test. `gate/` gets its own end-to-end test on its own cluster |
| 2 | Local OIDC issuer | Dex v2.46.1. See decision 9 |
| 3 | Requester claim | Configurable, default `sub`. Phase 2 sets `oid` |
| 4 | Default expiry | 1 hour, configurable |
| 5 | Audit fields | See decision 8 |
| 6 | Policy delivery | Files in a directory, mounted from a ConfigMap |

## Goals / Non-Goals

**Goals:**

- Keep policy, approval and audit free of transport types, so that a tasks front or a proxy front can reuse them.
- Make every guarantee in the specs testable without a cluster, except the ones that need Postgres or agentgateway.

**Non-Goals:**

- High availability. One gate replica. The Postgres lock makes a second replica safe, but no test covers it.
- Policy reload without a restart.
- Correlating a result entry with its request entry. See Open Questions.
- Approver authorization. Any identity with a valid token other than the requester can decide. `approval-inbox` adds who may approve.

## Decisions

### 1. Module layout

```
gate/
├── cmd/agent-gate/          server: ExtMcp gRPC, test HTTP endpoint
├── cmd/agent-gate-audit/    export, verify
├── internal/canonical/      RFC 8785, digests
├── internal/policy/         Decider interface, OPA implementation
├── internal/approval/       request state machine, Store interface
├── internal/audit/          entry, chain, checkpoint, verify
├── internal/postgres/       Store and audit appender, embedded migrations
├── internal/identity/       claim mapping, OIDC token check for the test endpoint
├── internal/extmcp/         adapter: ExtMcp types to core types
├── deploy/                  kind manifests
├── e2e/                     end-to-end test, build tag e2e
├── versions.env             Dex and Postgres image pins
└── Dockerfile
```

Only `internal/extmcp` imports the agentgateway bindings. A lint rule (`depguard` in `.golangci.yml`) enforces it. The module pins the bindings to the same commit as `test/extmcp/go.mod`.

Alternative: one module for the gate and the contract test. Rejected: the contract test must stay small and must not move with gate code.

### 2. Decision flow

For each `tools/call`, the adapter builds a `Call{Tenant, Agent, Target, Tool, Arguments, Claims}` and calls the core. The core:

1. Denies if `Agent` or `Tenant` is empty.
2. Canonicalizes the arguments. Denies on an unsafe number.
3. Evaluates policy. Deny and allow return at once, each with an audit entry.
4. For needs approval, in one transaction under the audit lock: find an approved, unused, unexpired request with the call digest. If found, mark it used, append `approval_used`, commit, pass. If not, find a pending unexpired request with the digest, or insert one and append `approval_required`. Commit, deny with the reference.

`Target` is the single entry of the `ExtMcp` service names. The adapter denies a call with zero or several service names. `Tenant` comes from `tenant.claim`, or from `tenant.static` (default `local`) when the claim is not set. Phase 2 sets `tenant.claim: tid`.

The caller and tenant claims reach the gate only as gateway `metadata` keys, for example `sub: jwt.sub`. The config names a metadata key, not a token claim. The route manifests forward each key the gate reads, so a config of `oid` also needs `oid: jwt.oid` on the route. Policy `claims` holds only the forwarded keys.

The gate bounds its own work for one call with a 3 s context, below the 5 s gateway deadline. On a timeout it returns a gRPC error, which fails closed as a JSON-RPC error.

### 3. Policy contract

Rego package `agentgate`. The gate evaluates `data.agentgate.decision` with input `{tenant, agent, target, tool, arguments, claims}` and expects `{"outcome": "allow" | "deny" | "approve", "reason": string}`. Anything else is deny. The gate loads each top-level `*.rego` file in the directory, follows symlinks and skips names that start with `.`. A ConfigMap volume holds a `..data` link and a timestamped directory, and a plain walk loads each file twice and puts the timestamp into the version. The policy version is `sha256:` over the sorted base names and contents. The query is prepared once at start.

The `kind` policy: `echo` allows; `delete_thing` with `id` equal to `prod` denies; any other `delete_thing` needs approval; any other tool denies. The deny rule reads the arguments, so the test proves that policy sees them.

### 4. Call digest and canonical encoding

```
call_digest = "sha256:" + hex(SHA-256(JCS({
  "schema": "agent-gate.call/v1",
  "tenant": ..., "agent": ..., "target": ..., "tool": ...,
  "arguments": <arguments, or null when absent>
})))
args_digest = "sha256:" + hex(SHA-256(JCS(arguments, or null when absent)))
```

The gate denies an integer literal (no fraction, no exponent) outside ±(2^53 − 1) and any number that overflows to infinity. Other numbers are compared after float64 rounding, as RFC 8785 requires, so `0.1` passes.

- From the Microsoft toolkit: the `sha256:` prefix, a schema version inside the hashed object, and the rule that an integer outside the exact range of a 64-bit float breaks equality. The gate denies such an integer rather than hashing it lossily.
- Different from the toolkit: `tenant` is part of the digest, and the requester rule is explicit.
- Library: `github.com/gowebpki/jcs` v1.0.2. A unit test runs the RFC 8785 test vectors. If the library fails one, replace it with the reference Go code from `cyberphone/json-canonicalization`.

Alternative: sorted-key `encoding/json`, as in the spike. Rejected: it does not fix number formatting or string escaping.

### 5. Approval requests

Table `approval_requests`: `ref`, `call_digest`, `tenant`, `agent`, `target`, `tool`, `arguments` (canonical JSON, for the inbox), `status` (`pending`, `approved`, `rejected`, `used`), `created_at`, `expires_at`, `approver`, `decided_at`, `used_at`.

- Reference: `apr_` and 26 characters of base32 over 128 random bits.
- Single use: `UPDATE ... SET status = 'used' WHERE ref = $1 AND status = 'approved' AND expires_at > now() RETURNING ref`. One row affected means pass. This is the toolkit's consume-once compare-and-swap, with the audit append in the same transaction.
- Expiry is checked at decision time and at replay time, as in the toolkit. No background job.
- An approval or rejection reads the approver from the test endpoint's validated token. It fails when the approver equals `agent`, the request is not pending, or it has expired. Each refusal appends `decision_refused`. Any other identity with a valid token can decide. Approver authorization is a non-goal here.

### 6. Test approval endpoint

`--test-endpoints` enables an HTTP listener with `POST /test/requests/{ref}/approve` and `POST /test/requests/{ref}/reject`. Each needs a bearer token that `go-oidc` validates against the configured issuer and a configured list of audiences. The callout deadline test uses the stub gate, so the gate has no delay route. The approver is the configured caller claim of that token. The flag is set only in `gate/deploy/`. `approval-inbox` removes the decision routes.

### 7. Audit append and chain

Table `audit_entries`: `seq` (primary key), `entry` (JSONB), `entry_hash`. Table `audit_checkpoints`: `seq`, `checkpoint` (JSONB), `checkpoint_hash`.

- Each append takes `pg_advisory_xact_lock` on one fixed key, reads the head, writes `seq + 1`. The lock gives one order, and it releases with the transaction.
- The hash is computed in Go from the struct, not from the JSONB, because JSONB does not keep bytes. Export writes the struct form.
- After each append, when `seq` is a multiple of `audit.checkpoint_every` (default 100, 5 on `kind`), the same transaction writes a checkpoint.

### 8. Entry and checkpoint format

Entry fields are listed in the [audit-log spec](specs/audit-log/spec.md). `event` is one of `allow`, `deny`, `approval_required`, `approved`, `rejected`, `decision_refused`, `approval_used`, `result`. `time` is RFC 3339 UTC with milliseconds.

```
entry_hash      = "sha256:" + hex(SHA-256("agent-gate.audit/v1\n" || JCS(entry)))
checkpoint_hash = "sha256:" + hex(SHA-256("agent-gate.checkpoint/v1\n" || JCS(checkpoint without signatures)))
```

`log_id` comes from `audit.log_id`, default `local`, separate from the tenant. In Phase 2 the tenant comes from `tid` and one chain holds many tenants. Phase 4 adds items `{alg, key_id, sig}` to `signatures`, signed over `checkpoint_hash`.

- From Preloop: an explicit field list, so a new column cannot slip in unhashed; domain separation strings; a nullable signature with a key id.
- From capsule-emit: signatures outside the hashed bytes, a list so that witnesses can be added, `prev_seq` and `prev_head_hash` in each checkpoint.
- Rejected: Preloop's non-JCS JSON and capsule-emit's Merkle tree. No Phase 1 need asks for inclusion proofs.

### 9. Identity on `kind`: Dex v2.46.1

Dex runs with in-memory storage and a ConfigMap config. Static clients `agent-a`, `agent-b` and `approver` use the `client_credentials` grant, which Dex added in v2.46.0. One static password user exists for `approval-inbox`.

- Dex writes `sub` as base64url, without padding, of a protobuf whose field 1 is the user id. For `client_credentials` the user id is the client id and there is no connector field: `agent-a` gives `CgdhZ2VudC1h`. The test computes the expected value and does not parse it. Task 1.1 confirmed this on 2026-11-25. `iss` stays the configured issuer through a port-forward.
- agentgateway v1.7.0 takes `jwks.remote` with a `backendRef` to the Dex `Service` and `jwksPath: dex/keys`. The controller fetches it over plain HTTP when no backend TLS policy is set, and caches it for at least 5 minutes. Task 1.2 confirmed this. The inline fallback is not needed.
- Dex sets `aud` to the client id by default. A static client `agent-gate` lists `agent-a`, `agent-b` and `approver` in `trustedPeers`, and each token request asks for the scope `audience:server:client_id:agent-gate`. The access and ID tokens then carry `aud: ["agent-gate", "<client>"]`. The gate and the gateway accept the configured list of audiences, which on `kind` is `[agent-gate]`.
- The gate validates test endpoint tokens with `go-oidc` against the in-cluster issuer URL.

Alternative: navikt mock-oauth2-server. Rejected: JVM image, and `iss` follows the Host header, which breaks behind a port-forward.

### 10. End-to-end layout

Cluster `agent-gate-e2e`, namespace `agent-gate-e2e`. agentgateway installs from the same pinned charts as `make e2e`. Two `HTTPRoute`s reach the stub MCP backend: `/mcp` has the JWT policy; `/mcp-noauth` has none, so its calls reach the gate with no claims. The guardrails policy targets the backend and covers both. The stub MCP server is the image built from `test/extmcp/`. Postgres runs as a single pod on an `emptyDir`.

The Postgres store tests (`internal/postgres`, build tag `pgtest`) run inside `make e2e-gate` against a port-forward. They cover the concurrent replay and concurrent append scenarios, which `make test` cannot reach without a database. They create and use their own database, `agent_gate_pgtest`, on the same server, so their appends do not enter the gate's chain.

Approval expiry on `kind` is 30 s. The expiry scenario uses its own request and waits 31 s. A shorter gate-wide expiry would make every other approval scenario race the clock.

The Makefile shares the cluster, gateway and image steps between `e2e` and `e2e-gate` through a `CLUSTER` variable. CI runs `make e2e-gate` in a new job `gate end-to-end test` in `.github/workflows/gate.yml`. The ruleset adds it as a required check.

### 11. Callout deadline

The stub gate in `test/extmcp/` gets `POST /delay`. A new scenario sets 10 s, calls the gated tool, and asserts a JSON-RPC error within 7 s and no upstream call. The test logs the measured time. Docs record it in the architecture options. This settles the conflict in the upstream sources.

## Risks / Trade-offs

- [The gate rejects large integers that some tools may send] → The tool error names the reason. Revisit if a real tool needs them.
- [An approval can be used with no execution: the gate commits `used`, then the gateway times out or the upstream call fails ([issue #4245](https://github.com/agentgateway/agentgateway/issues/4245))] → Accepted. The agent needs a new approval. The 3 s bound keeps the first case rare.
- [One advisory lock serializes every append] → Fine for Phase 1 volume. Shard per tenant later if needed.
- [Without signatures, anyone with database write can rewrite the whole chain] → Accepted for Phase 1. Phase 4 signs checkpoints into immutable storage.
- [The tail after the last checkpoint is not anchored] → The verifier reports it.
- [OPA is a large dependency] → `govulncheck` runs on `gate/`.

## Migration Plan

New module and new cluster. Nothing to migrate. Schema migrations are embedded SQL that run at start.

## Open Questions

- How to tie a `result` entry to the request it answers. `McpResponse` may carry no request id. The entry holds agent, target and tool for now. Settle it when the inbox shows results, without a format change: `call_digest` is already a field.
