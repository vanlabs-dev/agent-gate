# Review: extmcp-spike

- Date: 2026-11-16
- Session: 3, outside voice. Inputs: the [brainstorm](01-brainstorm.md), the [OpenSpec change](../../../openspec/changes/extmcp-spike/) and the project docs. No chat history.
- Source check: agentgateway tag `v1.7.0` (commit `26f5aebe`), read on 2026-11-16. File paths in the findings are in that tree under `crates/agentgateway/src/mcp/`.

## Verdict

The plan is sound and the scope fits a spike. No finding blocks apply. Findings F1 to F3 let the spike pass while a security property stays unproven. Fix them before apply, or record each as unproven in ADR 0002.

## Design claims checked against the source

Confirmed:

- Tag `v1.7.0` is commit `26f5aebe`, dated 2026-10-27.
- `api/go.mod` declares the module `github.com/agentgateway/agentgateway/api`.
- The root `go.mod` requires `sigs.k8s.io/gateway-api v1.7.0-rc.3`.
- The policy shape `backend.mcp.guardrails.processors[].remote` with `backendRef`, `failureMode` and `metadata`. Phase `Full` exists (`guardrails/phase.rs`). The upstream end-to-end manifest uses only `Request` and `Response`, so upstream does not test `Full` on `tools/call` end to end.
- A reject returns from the request hook before the upstream send (`handler.rs`).
- The deny shape: for `tools/call`, any code except internal error becomes a tool result with `isError` and the `reason` text. The `mcp_error` payload is dropped (`mod.rs`, `jsonrpc_error_body`).

Not checked: the charts on `kind`, the charts with Helm 4, `jwt.sub` in the `metadata` expressions, the runner image versions, the MCP Go SDK version.

## Findings

| ID | Level | Finding | Proposed change |
|---|---|---|---|
| F1 | High | The scenario "Call with a token that fails validation" passes without proof. The SDK client fails at `initialize`. The gate is called only for `tools/call`, so "the gate receives no request" is true with or without validation. | Open the session with a valid token. Then send `tools/call` with a token signed by an unknown key, by a token swap in the client's `http.RoundTripper`. Add a no-token case the same way. |
| F2 | High | `failureMode: FailClosed` is configured and not tested. The design lists failure mode as a non-goal. A gate that fails open is not a gate, and a fail-open result would change the choice of option A. | Add a requirement and a scenario "Gate unavailable": scale the stub gate to zero, call the gated tool, expect a JSON-RPC error and no upstream call. Run it last. |
| F3 | High | The spec binds an approval to "the same caller". No scenario sends the replay from a different caller. | Add a scenario "Replay by a different caller": a token with another `sub`, the same tool and arguments. Expect a deny and no upstream call. |
| F4 | Medium | The replay runs in the same session with the same token. A real replay comes minutes or hours later, in a new session with a new token. | "Replay after approval" uses a new client session and a newly signed token with the same `sub`. |
| F5 | Medium | The requirement says the gate receives the result "before the client does". No scenario asserts the order. | Remove "before the client does" from the requirement. |
| F6 | Medium | `make test` can exit 0 when no scenario ran. The scenarios sit behind a build tag, and `./...` also runs the in-process stub tests. The spec says the run "reports each scenario as passed", but design step 6 has no `-v`. | Run with `-v`. Make the `Makefile` fail when the end-to-end test did not run. |
| F7 | Medium | The time box has no unit. "20 hours of work" cannot be measured when an agent does the apply. | State the unit: wall-clock hours of apply sessions. Record the start and end of each session in `tasks.md`. |
| F8 | Medium | The gate does not receive the client's bytes. `mcp_request` is the gateway's re-encoding of the parsed params, with `_meta` merged in (`handler.rs`, `guardrails/mod.rs`). `_meta` can hold a progress token that changes with each call. | In design decision 5, state that the digest reads `name` and `arguments` only. In "Replay after approval", assert that the upstream arguments equal the approved arguments. Record the fact for ADR 0009. |
| F9 | Medium | The gate receives every request header by default (`allowedRequestHeaders` empty). The gateway removes the JWT after validation unless `preserveToken` is set, so the fallback "forward the `authorization` header" in Risks does not work as written. | Set `allowedRequestHeaders` to a short list in design decision 2. Assert that no `authorization` header reaches the gate. Correct the fallback. |
| F10 | Low | A `metadata` expression that fails is skipped with a debug log (`guardrails/client.rs`, `build_metadata`). A gate that hashes a missing `sub` as empty binds the approval to no caller. | The stub gate denies when `sub` is absent. Record the rule for Phase 1. |
| F11 | Low | The deadline facts disagree. The client code sets a 10 s default (`with_default_timeout`). The upstream manifest comment and `options.md` say no deadline exists without a policy. The 5 s policy is not tested. | Keep the policy. In task 7.2, state what was read and name the conflict. |
| F12 | Low | The workflow declares no `permissions`. `kind` and `kubectl` come from the runner image, which changes weekly. | Add `permissions: contents: read`. Accept the drift: the node image is pinned. |
| F13 | Low | Root `make test` is a cluster test that takes minutes. The name serves the verify script. `repo-foundations` must keep it or split it. | Accept. Note it in `09-ship.md`. |
| F14 | Low | Task 2.2 pushes `LICENSE` and `README.md` to `main` with no pull request, against the conventions. | Accept. No remote exists before that push. |

## Checked and fine

- For `tools/call` the gate receives the upstream's own tool name (`delete_thing`), not a multiplexed name (`guardrails/mod.rs`).
- A gate outage and an approval deny differ at the client: a JSON-RPC internal error against a tool result with `isError`.
- The tracked files hold one author identity and no secrets.
- Over-engineering: nothing to cut. Each stub endpoint serves one assertion.

## Resolution

Decided by the maintainer on 2026-11-16.

- F1 to F12: the proposed change is folded into the OpenSpec change through `/opsx:update`. For F2, fail-closed is tested and failure mode is no longer a non-goal.
- F10: an in-process stub test covers the rule. The spec has no scenario for it.
- F13 and F14: accepted with no change.

The change now has 25 tasks and passes `openspec validate extmcp-spike --strict`.
