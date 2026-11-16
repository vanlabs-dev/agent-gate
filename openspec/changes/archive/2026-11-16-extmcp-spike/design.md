# Design

## Context

See [proposal.md](proposal.md) for motivation and the [brainstorm](../../../../docs/greenfield/extmcp-spike/01-brainstorm.md) for scope. The repo holds docs only. No Go toolchain is installed on the development machine.

The agentgateway source was read on 2026-11-16 at tag `v1.7.0` (commit `26f5aebe`, released 2026-10-27). Each fact below marked **Read** comes from that source. Each fact marked **Assumed** is not proven until the test passes.

### Answers to the brainstorm's open questions

| # | Question | Answer |
|---|---|---|
| 1 | Version, and install on `kind` | Pin `v1.7.0`. **Read:** charts `agentgateway-crds` and `agentgateway` exist at `oci://ghcr.io/agentgateway/charts` with tag `v1.7.0`. **Assumed:** the charts install on `kind` without the Istio and inference CRDs that the upstream test setup adds. |
| 2 | Config, and claim mapping | **Read:** the feature is `backend.mcp.guardrails` in an `AgentgatewayPolicy`. A `remote` processor names the gate service. Its `metadata` map holds CEL expressions, and each result is one field of `metadata_context`. `jwt.sub` is a valid expression. |
| 3 | Does an `error` skip the upstream call | **Read:** yes. A reject returns before the upstream send (`crates/agentgateway/src/mcp/handler.rs`). The test asserts it. |
| 4 | What the client receives on a deny | **Read:** for `tools/call` with any code other than `UNKNOWN`, an HTTP 200 with a tool result, `isError` true, and one text block that holds `reason`. The `mcp_error` payload is dropped on this path. |
| 5 | Go bindings | **Read:** upstream publishes them as the Go module `github.com/agentgateway/agentgateway/api`. The module has no version tags. |
| 6 | MCP client | The MCP Go SDK. See decision 6. |
| 7 | Location | `test/extmcp/`. See decision 7. |
| 8 | Ignite review | Not a blocker. See decision 11. |

### Further facts from the review

The [review](../../../../docs/greenfield/extmcp-spike/07-review.md) read the same tag on 2026-11-16. Each fact is **Read**. Paths are under `crates/agentgateway/src/mcp/`.

- `mcp_request` is not the client's bytes. It is the gateway's re-encoding of the parsed params, with `_meta` merged in (`handler.rs`, `guardrails/mod.rs`). `_meta` can hold a progress token that changes with each call.
- For `tools/call` the gate receives the upstream's own tool name, not a multiplexed name (`guardrails/mod.rs`).
- With `allowedRequestHeaders` empty, the gate receives every request header (`guardrails/client.rs`).
- The gateway removes the JWT after validation unless the JWT policy sets `preserveToken`.
- A `metadata` expression that fails is skipped with a debug log. The gate then receives no such field (`guardrails/client.rs`, `build_metadata`).
- `FailClosed` is the default. A gate outage gives the client a JSON-RPC internal error with the message `guardrail check failed`, not a tool result (`guardrails/client.rs`, `mod.rs`).
- The deadline facts disagree. The client code sets a 10 s default (`with_default_timeout`). The comment in the upstream test manifest says that no deadline exists without a policy.
- The upstream end-to-end manifest uses only the phases `Request` and `Response`. Upstream does not test `Full` on `tools/call` end to end.

## Goals / Non-Goals

**Goals:**

- Prove or disprove each requirement in [the spec](specs/agentgateway-integration/spec.md) against real agentgateway, not a mock.
- Leave a test that later becomes the agentgateway compatibility test.
- Keep stub code small enough to read in one sitting.

**Non-Goals:**

- Reusable gate code. The stub gate is thrown away in Phase 1. Only the test and manifests carry over.
- The canonical argument encoding for the approval digest. That decision belongs to Phase 1.
- Timeout and performance behaviour. Fail-closed on a gate outage is tested (decision 9). Other failure behaviour is not.

## Decisions

### 1. Pin agentgateway v1.7.0 in Kubernetes controller mode

Install `agentgateway-crds` and `agentgateway` with Helm from `oci://ghcr.io/agentgateway/charts`, version `v1.7.0`. Install the Gateway API CRDs at `v1.7.0-rc.3`, the version agentgateway v1.7.0 builds against. Try the standard channel first. All versions live in one file, `test/extmcp/versions.env`.

Alternative: the `agentgateway-standalone` chart with a static config file. Rejected: Phase 2 deploys controller mode on AKS, and the upstream `ExtMcp` end-to-end test covers controller mode.

### 2. Gateway configuration

Namespace `agent-gate-test`. Resources, after the upstream test manifest `controller/test/e2e/testdata/extmcp/extmcp.yaml`:

- `Gateway` with class `agentgateway`, one HTTP listener.
- `AgentgatewayBackend` with one static MCP target: the stub MCP server, protocol `StreamableHTTP`.
- `HTTPRoute` from the gateway to that backend.
- `AgentgatewayPolicy` on the route: `traffic.jwtAuthentication`, mode `Strict`, one provider with inline JWKS.
- `AgentgatewayPolicy` on the backend: `backend.mcp.guardrails.processors[0]` with `remote.backendRef` to the stub gate, `failureMode: FailClosed`, `metadata: {sub: jwt.sub, iss: jwt.iss}`, `allowedRequestHeaders: [mcp-protocol-version]`, and `methods: {tools/call: Full}`.
- `AgentgatewayPolicy` on the stub gate `Service`: `backend.http.requestTimeout: 5s`.

`Full` runs both `CheckRequest` and `CheckResponse` for `tools/call`.

An empty `allowedRequestHeaders` forwards every header. The list therefore names one harmless header, so that the gate receives no other. The 5 s deadline is configured and not tested.

### 3. Deny carries the reference in `reason`

The stub gate denies with code `PERMISSION_DENIED` and `reason` set to `approval required: ref=<id>`. The test reads the reference from the text block of the tool result.

Alternative: put structured data in `mcp_error`. Rejected: agentgateway drops it for `tools/call` (question 4). Code `UNKNOWN` keeps it, but the client then receives a JSON-RPC internal error, the same shape as a gate outage.

This is a finding for ADR 0009: the approval reference travels as text only.

### 4. Bindings from the upstream Go module, pinned by commit

`go get github.com/agentgateway/agentgateway/api@26f5aebe99a36a037fe54f8080a608aca5547e15`. Go records a pseudo-version, which is an exact pin.

Alternative: vendor `ext_mcp.proto` and generate code. Rejected: it adds `protoc` or `buf` to the toolchain and CI for no gain.

### 5. Stub gate

One Go binary with two listeners:

- gRPC `ExtMcp` server. `CheckRequest` passes every method except `tools/call` for the hard-coded tool `delete_thing`. For that tool it computes a digest, passes and consumes a matching approved entry, or stores a pending entry and denies. It denies with no pending entry when `sub` is absent from `metadata_context`. `CheckResponse` records the result and passes.
- Test-only HTTP: `POST /approve` with a reference, `GET /requests` and `GET /responses` to read what the gate received, `POST /reset`. `/requests` includes the headers of each request.

The digest is SHA-256 over `sub`, the service name, the tool name and the arguments after a decode and re-encode with Go `encoding/json` (sorted keys). It reads only `name` and `arguments` from `mcp_request`, never the whole params, because `_meta` changes between calls. State is a mutex and a map. This digest is for the spike only.

### 6. Stub MCP server and test client use the MCP Go SDK

`github.com/modelcontextprotocol/go-sdk` v1.8.0. The stub server exposes two tools over streamable HTTP, `echo` (ungated) and `delete_thing` (gated), and a test-only `GET /calls` that returns the calls it received. The test uses the SDK client, so it sees a deny as a real agent does.

The test client sends its token through an `http.RoundTripper` that holds a replaceable token. A scenario can then open a session with a valid token and send a later `tools/call` with a different token, an invalid token or no token.

Alternative: raw JSON-RPC with `net/http`, as the upstream test does with `curl`. Kept as the fallback if the SDK client and agentgateway do not agree on a protocol version. ADR 0003 already limits the SDK to test servers.

### 7. Layout: `test/extmcp/` as its own Go module

```text
test/extmcp/
├── go.mod                 module github.com/vanlabs-dev/agent-gate/test/extmcp
├── versions.env           agentgateway, Gateway API, kind node image, Helm
├── Dockerfile             one image, both stub binaries
├── cmd/stub-gate/
├── cmd/stub-mcp/
├── deploy/                kind config and manifests
└── e2e_test.go            build tag e2e
```

A separate module keeps the SDK and stub dependencies out of the future `gate/` module. `repo-foundations` decides the rest of the layout.

### 8. Token: key generated at run time

The test's `TestMain` generates an RSA key, applies the JWT policy with the public JWKS inline, and signs tokens in process. No issuer service runs, and no private key enters the repo. The invalid-token scenario signs with a second key. The different-caller scenario signs a token with another `sub`. The replay scenario signs a new token for the same `sub`.

Alternative: a committed test key pair. Rejected: conventions forbid secrets in the repo, and secret scanners flag it.

### 9. Harness: `make test`

The root `Makefile` target `test`:

1. Checks that `go`, `docker`, `kind`, `kubectl` and `helm` exist.
2. Deletes and recreates the `kind` cluster `agent-gate-extmcp` with a pinned node image.
3. Installs the Gateway API CRDs and both charts, then waits for the controller.
4. Builds the stub image and loads it with `kind load docker-image`.
5. Applies the manifests and waits for the gateway and both stubs.
6. Runs `go test -tags e2e -count=1 -v ./...` in `test/extmcp/`.
7. Exits non-zero when the output does not show that `TestE2E` ran and passed. A wrong build tag then fails the target.

The scenario "Gate unavailable" runs last. It scales the stub gate deployment to zero with `kubectl`, waits until no gate pod exists, calls the gated tool, and then scales the deployment back to one.

The test reaches the gateway and the stubs' test-only ports through `kubectl port-forward`. No load balancer is installed. `make clean` deletes the cluster.

The name `test` is deliberate: the greenfield verify script runs `make test` when a `Makefile` has that target.

### 10. CI

One workflow, `.github/workflows/extmcp.yml`, on pull requests and pushes to `main`: checkout, `actions/setup-go` from `test/extmcp/go.mod`, install Helm at the version in `versions.env`, `make test`, and on failure upload pod logs. The workflow sets `permissions: contents: read`.

The `ubuntu-24.04` runner ships Docker, `kind` 0.33.0, `kubectl` 1.37.2 and Helm 3.22.1 ([runner image readme](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md), checked 2026-11-16). `kind` and `kubectl` match decision 12. Helm does not, so the workflow installs Helm 4. The runner image changes weekly, so `kind` and `kubectl` in CI can drift from decision 12. This is accepted: the node image is pinned.

### 11. Ignite checkpoint does not block the spike

The roadmap sets a review of ADR 0001 and 0002 on or after 2026-11-23. The spike costs nothing and its result is input to that review. If the apply session starts on or after 2026-11-23, do the review first, because an announcement could remove the reason for the spike.

### 12. Toolchain on the development machine

| Tool | Version | Location |
|---|---|---|
| Go | 1.27.4 | `~/.local/go`, with `~/.local/go/bin` on `PATH` |
| `kind` | v0.33.0 | `~/.local/bin` |
| `kubectl` | v1.37.2 | `~/.local/bin` |
| `helm` | v4.3.2 | `~/.local/bin` |

`~/.bashrc` sets `GOPATH=~/.local/share/go`, `GOMODCACHE` and `GOCACHE` under `~/data/cache`. Go must never create `~/go`. Docker 29.8.2 is already installed. Security fixes for Helm 3 ended on 2026-11-11 ([Helm 4 release post](https://helm.sh/blog/helm-4-released/), checked 2026-11-16), so Helm 4 is used.

### 13. Time box and exit

20 wall-clock hours of apply sessions, from the start of task group 1. Record the start and end time of each apply session in `tasks.md`. If the test does not pass by then, stop, write what failed in `09-ship.md`, and draft a new ADR with status "Proposed" that chooses between options B and C. Do not edit the decision in ADR 0002.

## Risks / Trade-offs

- [The charts need CRDs or a load balancer that `kind` lacks] → The upstream setup adds Istio CRDs, inference CRDs and MetalLB. Add only what the controller logs demand. Port-forward removes the need for MetalLB.
- [`jwt.sub` does not resolve in `metadata`] → Upstream issue #3485 reports that it resolves only with a JWT policy on the route. Decision 2 attaches one. If it still fails, set `preserveToken: true` on the JWT policy, add `authorization` to `allowedRequestHeaders`, and record the gap in ADR 0002. The requirement that the gate receives no token then fails.
- [The scale to zero is slow, or the gateway keeps a connection to the old pod] → Wait for the pod to be deleted before the call. Give the scenario its own timeout.
- [`CheckResponse` does not run for `tools/call`] → Phase `Full` is in the CRD enum and in the gateway code. If it fails, the spec requirement fails and ADR 0002 records a limit.
- [The SDK client rejects the gateway's protocol version] → Fall back to raw JSON-RPC (decision 6).
- [The pseudo-version of the bindings does not resolve] → Vendor the proto and generate code (decision 4, alternative).
- [`port-forward` is flaky in CI] → Retry the first connection. If it stays flaky, use a NodePort with `extraPortMappings` in the kind config.
- [The repo goes public before the spike passes] → Accepted in the brainstorm. A failed spike is recorded in public as an ADR change.
- [The spike digest is not canonical] → Stated as a non-goal. Phase 1 defines the encoding.

## Migration Plan

Not applicable. Nothing is deployed outside `kind`. Rollback is `make clean` and a revert of the branch.

## Open Questions

None that change the specs or the tasks.
