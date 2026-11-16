# Tasks

Time box: 20 wall-clock hours of apply sessions, from the start of group 1 (design decision 13). Record the start and end time of each apply session on the line below.

Sessions:

- 1: start 2026-11-16 19:07 NZDT, end 19:27 NZDT
- 2: start 2026-11-16 about 19:38 NZDT, end 19:56 NZDT. The maintainer merged pull request 1 at 19:50, and the job passed on `main`

## 1. Checkpoint and toolchain

- [x] 1.1 If today is on or after 2026-11-23 and ADR 0001 does not record the Ignite review, stop and ask the maintainer. Verify: the review is recorded, or the date is earlier
- [x] 1.2 Install Go 1.27.4 to `~/.local/go` and set `PATH`, `GOPATH`, `GOMODCACHE` and `GOCACHE` in `~/.bashrc` (design decision 12). Verify: `go version` prints 1.27.4, `go env GOPATH` prints `~/.local/share/go` expanded, and `~/go` does not exist
- [x] 1.3 Install `kind` v0.33.0, `kubectl` v1.37.2 and `helm` v4.3.2 to `~/.local/bin`, each checked against its published checksum. Verify: each `version` command prints the pinned version

## 2. Public repo

- [x] 2.1 Add `LICENSE` with the unmodified Apache-2.0 text and a short `README.md` (what it is, status, link to `docs/vision.md`). Verify: `LICENSE` matches the text at apache.org byte for byte
- [x] 2.2 Create the public repo `vanlabs-dev/agent-gate` and push `main`. Verify: `gh repo view vanlabs-dev/agent-gate --json visibility` prints `PUBLIC` and GitHub detects the licence as Apache-2.0
- [x] 2.3 Create the branch `extmcp-spike` for all later groups. Verify: `git branch --show-current`

## 3. Cluster and gateway

- [x] 3.1 Add `test/extmcp/versions.env`, the `kind` config and the `Makefile` steps that delete and create the cluster `agent-gate-extmcp`. Verify: `kubectl get nodes` shows one node `Ready`
- [x] 3.2 Add the `Makefile` steps that install the Gateway API CRDs and the `agentgateway-crds` and `agentgateway` charts at `v1.7.0`. Verify: the controller deployment is `Available` and the `GatewayClass` `agentgateway` is `Accepted`. Record any extra CRD the controller needs in `design.md` through `/opsx:update`
- [x] 3.3 Add the namespace and `Gateway` manifest. Verify: the `Gateway` condition `Programmed` is true and a gateway pod is running

## 4. Stubs

- [x] 4.1 Create the Go module in `test/extmcp/` with the bindings pinned to commit `26f5aebe`, gRPC and the MCP Go SDK v1.8.0. Verify: `go list -m github.com/agentgateway/agentgateway/api` prints a pseudo-version that ends in `26f5aebe99a3`
- [x] 4.2 Write `cmd/stub-mcp` with tools `echo` and `delete_thing` and the test-only `GET /calls`. Verify: a Go test calls `echo` through the SDK client in process and `/calls` lists that call
- [x] 4.3 Write `cmd/stub-gate` (design decision 5). Verify: a Go test drives the gRPC server in process through deny, approve, identical replay, second replay and changed arguments, and checks `/requests` and `/responses`. The same test shows that a replay with a different `_meta` passes, and that a call with no `sub` is denied with no pending entry
- [x] 4.4 Add the `Dockerfile` and the `Makefile` steps that build the image and load it into `kind`. Verify: `docker build` passes and the image is listed on the node

## 5. End-to-end test

- [x] 5.1 Add manifests for both stubs, the `AgentgatewayBackend`, the `HTTPRoute` and the guardrails and timeout policies, with `allowedRequestHeaders` set (design decision 2). Verify: both stub deployments are `Available` and each policy reports `Accepted`
- [x] 5.2 Write `TestMain` in `e2e_test.go`: generate the keys, apply the JWT policy, open the port-forwards, reset the stubs. Give the test client the token `http.RoundTripper` (design decision 6). Add the scenario "Ungated tool". Verify: the scenario passes against the cluster
- [x] 5.3 Add the scenarios "First call to a gated tool", "Replay after approval", "Second replay of a used approval", "Replay with changed arguments" and "Replay by a different caller". "Replay after approval" uses a new session and a newly signed token. Verify: all five pass, each asserts the upstream call count from `/calls`, and "Replay after approval" asserts that the upstream arguments equal the approved arguments
- [x] 5.4 Add the scenarios "Call with a valid token", "Call with a token that fails validation" and "Call with no token". The last two send `tools/call` in a session that a valid token opened. Verify: all three pass, and the first asserts from `/requests` that `sub` is in `metadata_context` and that no `authorization` header is present
- [x] 5.5 Add the scenario "Result of a passed call". Verify: it passes and asserts the upstream result in `/responses`
- [x] 5.6 Add the scenario "Gate unavailable" as the last scenario (design decision 9). Verify: it passes, the client error is a JSON-RPC error and not a tool result, `/calls` shows no upstream call, and the stub gate deployment is `Available` again afterwards
- [x] 5.7 Join every step under the `Makefile` target `test` and add `clean`. Verify: `make test` exits 0 two times in a row from a state with no cluster and prints each scenario as passed, `go test` with a wrong build tag makes the target exit non-zero, and `make clean` removes the cluster
- [x] 5.8 Add a "Run the compatibility test" section to `README.md` with the required tools and the command. Verify: the command runs as written

## 6. CI

- [x] 6.1 Add `.github/workflows/extmcp.yml` with `permissions: contents: read` (design decision 10) and open a pull request from `extmcp-spike`. Verify: the job passes on the pull request

## 7. Record the result

- [x] 7.1 If every scenario passes: set ADR 0002 and ADR 0009 to "Accepted, confirmed by the Phase 0 spike" with the date, the pinned version and the findings (config key `mcp.guardrails`, reference as text only, params re-encoded with `_meta`, header forwarding, fail-closed tested). If one fails: leave both decisions unchanged and draft a new ADR with status "Proposed". Verify: each ADR status matches the test result
- [x] 7.2 Update `docs/architecture/options.md` (extension naming, deny shape, known limits, the two deadline facts and their conflict, the rule that a gate denies a call with no identity claim), `docs/conventions.md` (add `test/extmcp/` to the layout), `docs/vision.md` (status) and `docs/roadmap.md` if option A changed. Verify: no doc says that no code exists, and each third-party fact carries a source and the date checked
- [x] 7.3 Merge the pull request after the CI job passes. Verify: the job passes on `main`
