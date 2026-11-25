# Tasks

Work on the branch `gate-core`. Run `make check` before each push.

## 1. Risk checks

- [x] 1.1 In a scratch `kind` cluster, run Dex v2.46.1 with one `client_credentials` client and get a token. Verify: the token's `iss` is the configured issuer even through a port-forward, its `sub` equals the value the test helper computes, and its `aud` is recorded, with whether Dex can issue a shared audience. If `iss` or `sub` fails, stop and ask the maintainer
- [x] 1.2 Read the agentgateway v1.7.0 `jwtAuthentication` source for remote JWKS. Verify: design decision 9 records which path applies, through `/opsx:update`

## 2. Module and canonical encoding

- [x] 2.1 Create the Go module `gate/` with the layout of design decision 1, the bindings pinned to the commit in `test/extmcp/go.mod`, and `gate/versions.env`. Add `gate` to `GO_MODULES` and a `depguard` rule that allows the bindings only in `internal/extmcp`. Add `gate/` to `.github/dependabot.yml` with the bindings ignored. Verify: `make check` passes, and an import of the bindings from `internal/policy` fails `make lint-go`
- [x] 2.2 Write `internal/canonical`: JCS encoding, the unsafe number check, `CallDigest` and `ArgsDigest` (design decision 4). Verify: unit tests pass for the RFC 8785 test vectors, key order and white space, `_meta` ignored, each changed field giving a new digest, `9007199254740993` and `1e400` refused, `0.1` accepted, and absent arguments hashed as `null`

## 3. Policy

- [x] 3.1 Write `internal/policy`: the `Decider` interface and the OPA implementation that loads a directory and computes the policy version (design decision 3). Verify: unit tests pass for allow, deny, approve, an undefined decision, an invalid outcome, a file that does not compile, a version change after a file change, and a ConfigMap-shaped directory that loads each file once with the same version as a plain directory
- [x] 3.2 Add `gate/deploy/policy/agentgate.rego` with the `kind` policy. Verify: a unit test runs it for each rule

## 4. Audit

- [x] 4.1 Write `internal/audit`: the entry and checkpoint types, the hash functions and `Verify` (design decisions 7 and 8). Verify: unit tests pass for an untouched chain, a changed entry, a removed entry, swapped entries, a removed tail, and a changed checkpoint
- [x] 4.2 Write `cmd/agent-gate-audit` with `verify <file>`. Verify: a test runs it on a fixture and on each tampered fixture and checks the exit code and the named `seq`

## 5. Approval core

- [x] 5.1 Write `internal/approval`: the `Store` interface, the decision flow of design decision 2, approve, reject, expiry and the requester rule, with an in-memory store for unit tests. Verify: unit tests cover each scenario in the `tool-call-approval` and `tool-call-policy` specs that needs no network, including policy changed to deny on replay
- [x] 5.2 Write `internal/identity`: caller and tenant from claims, and OIDC token checks with `go-oidc`. Verify: unit tests pass for the default claim, a configured `oid` claim, a missing claim, a token from an unknown issuer, and a token with an audience outside the configured list

## 6. Postgres

- [x] 6.1 Write `internal/postgres`: embedded migrations, the `Store`, the audit appender with the advisory lock, and checkpoints (design decisions 5 and 7). Add `export --dsn` to `agent-gate-audit`. The tests create and use their own database `agent_gate_pgtest`. Verify: tests with the build tag `pgtest` pass against a local Postgres container in scratch, including concurrent replays (exactly one passes) and 50 concurrent appends (the export verifies)

## 7. Server

- [x] 7.1 Write `internal/extmcp`, the adapter (design decision 2), and `cmd/agent-gate` with config, the 3 s bound and `--test-endpoints` (design decision 6). Verify: an in-process gRPC test drives allow, deny, needs approval, approve, replay, and a call with two service names; the test endpoint returns 401 with no token and does not answer without the flag
- [ ] 7.2 Add `gate/Dockerfile`. Verify: `docker build` passes and the image runs `agent-gate --help`

## 8. Callout deadline in the contract test

- [ ] 8.1 Add `POST /delay` to `test/extmcp/cmd/stub-gate` and the scenario "Gate slower than the deadline" to `test/extmcp/e2e_test.go` (design decision 11). Verify: `make e2e` passes and logs the measured time. Record the time in `docs/architecture/options.md`

## 9. Gate end-to-end test

- [ ] 9.1 Add `gate/deploy/` manifests: Dex, Postgres, the gate with the policy ConfigMap, the stub MCP server, both routes with the metadata keys the gate reads, the guardrails and timeout policies, and an approval expiry of 30 s (design decisions 2 and 10). Refactor the `Makefile` so that `e2e` and `e2e-gate` share the cluster, gateway and image steps. Verify: `make e2e-gate` reaches a cluster where every deployment is `Available` and every policy is `Accepted`, and `make e2e` still passes
- [ ] 9.2 Write `gate/e2e/e2e_test.go` with Dex tokens and scenarios for allow, policy deny, needs approval, approval by `approver`, identical replay, changed arguments, another caller (`agent-b`), second replay, replay after expiry (its own request, waiting 31 s), self-approval, rejection, and a call on `/mcp-noauth`. Run the `pgtest` tests in the same target. Verify: `make e2e-gate` exits 0, and exits non-zero with `E2E_TAGS=none`
- [ ] 9.3 Add the scenario that exports the audit log, runs `agent-gate-audit verify` on it, then on copies with one entry changed, one removed and two swapped. Verify: exit 0 on the untouched export and non-zero on each copy
- [ ] 9.4 Add `.github/workflows/gate.yml` with the job `gate end-to-end test`, and add the check to `.github/rulesets/main.json`. Verify: the job passes on the pull request, `make lint-actions` passes, and the ruleset is applied with the `gh api` command in `docs/conventions.md` after merge

## 10. Docs

- [ ] 10.1 Update `AGENTS.md` (commands: `make e2e-gate`, `make clean`), `docs/conventions.md` (layout: `gate/` exists; checks table), `docs/architecture/options.md` (measured deadline, Dex on `kind`, policy contract link) and `docs/roadmap.md` (Phase 1 status). Verify: each command in `AGENTS.md` runs as written, and no doc names the stub gate as the gate
