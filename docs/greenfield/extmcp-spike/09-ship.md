# Ship: extmcp-spike

- Date: 2026-11-16
- Result: the spike passed. Option A stands. [ADR 0002](../../adr/0002-integrate-through-agentgateway-extmcp.md) and [ADR 0009](../../adr/0009-suspension-deny-and-replay.md) are confirmed against agentgateway v1.7.0.
- Time used: about 40 minutes of apply sessions, from a time box of 20 hours.

## What shipped

- The compatibility test in [`test/extmcp/`](../../../test/extmcp/): a stub `ExtMcp` gate, a stub MCP server, `kind` manifests and one end-to-end test with 11 scenarios.
- `make test` at the repo root. It creates the cluster, deploys, runs the test, and exits 0 only when `TestE2E` ran and passed.
- The workflow `.github/workflows/extmcp.yml`. It runs `make test` on each pull request and on `main`.
- The public repo `vanlabs-dev/agent-gate` with `LICENSE` (Apache-2.0) and `README.md`.
- The main spec [`agentgateway-integration`](../../../openspec/specs/agentgateway-integration/spec.md): 9 requirements, 13 scenarios.
- Go 1.27.4, `kind` v0.33.0, `kubectl` v1.37.2 and `helm` v4.3.2 on the development machine.

## Evidence

- Local: [08-verify.md](08-verify.md), result PASS.
- CI: the job passed on pull request 1 and on `main` at commit `3a77f49`.
- A `FailOpen` policy makes the scenario "Gate unavailable" fail. A wrong build tag makes `make e2e` exit non-zero. Both were checked by hand on 2026-11-16.

## Where the artifacts live

| Artifact | Path |
|---|---|
| Brainstorm, review, verify output, this note | `docs/greenfield/extmcp-spike/` |
| Proposal, design, tasks, delta spec | `openspec/changes/archive/2026-11-16-extmcp-spike/` |
| Main spec | `openspec/specs/agentgateway-integration/spec.md` |
| Pinned versions | `test/extmcp/versions.env`, `test/extmcp/go.mod` |

## Differences from the plan

- The charts installed on `kind` with the standard Gateway API channel. No Istio CRDs, inference CRDs or load balancer were needed.
- The maintainer created the public repo and merged pull request 1. The agent's permission rules blocked both actions.
- The Go caches are in `~/data/cache/go/`. The directories `~/data/cache/gobuild` and `~/data/cache/gomod` exist and are root-owned.
- The archive and this note are a second pull request, because task 7.3 could be marked only after the merge.

## Not proven

- The 5 s deadline on the gate callout. It is configured and not tested. The two upstream deadline facts still disagree ([architecture options](../../architecture/options.md)).
- A replay whose `_meta` differs, end to end. The SDK client sent no `_meta`. Only the in-process stub test covers it.
- The order of `CheckResponse` and the client response. The spec does not require it.

## Follow-ups

1. Start the change `repo-foundations`, the second change of Phase 0: lint, the Python scaffold, and the build, test and lint commands in `AGENTS.md`.
2. In `repo-foundations`, decide whether the root `make test` stays the cluster test (review finding F13), make the CI job a required check on `main`, and consider pinning the GitHub actions by commit.
3. On or after 2026-11-23, review ADR 0001 and ADR 0002 against the Microsoft Ignite 2026 announcements ([roadmap](../../roadmap.md)).
4. In Phase 1, define the canonical argument encoding for the approval digest, keep the rule that a call with no identity claim is denied, and measure the callout deadline.
5. When a pinned version changes in `test/extmcp/versions.env`, update the README badges.
