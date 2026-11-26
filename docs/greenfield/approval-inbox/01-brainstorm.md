# Brainstorm: approval-inbox

- Date: 2026-11-27
- Change: the second of two in Phase 1 of the [roadmap](../../roadmap.md). The first, `gate-core`, shipped on 2026-11-25 ([ship note](../gate-core/09-ship.md)).
- Project-level direction is in the [vision](../../vision.md), the [architecture options](../../architecture/options.md) and the [ADRs](../../adr/). [ADR 0007](../../adr/0007-web-inbox-before-teams.md) sets the web inbox first. This file covers this change only.

## Goal

Replace the test approval endpoint with a minimal web inbox served by the gate, behind OIDC sign-in, with approver authorization in policy. Then run the Phase 1 exit test and tag `v0.1.0`.

## Success signals

`make e2e-gate` exits 0 on `kind` and shows:

1. A human signs in to the inbox through Dex, sees a pending request, and approves it. The identical replay passes once.
2. A rejection in the inbox leaves the call blocked. A new call gets a new reference.
3. A form post with no session, or with a missing or wrong CSRF token, changes nothing.
4. An identity that the approver policy does not allow cannot decide. The requester cannot decide.
5. A decision on an expired request is rejected in the inbox.
6. The test approval endpoint is gone. No route under `/test/` answers.
7. The Phase 1 exit test passes: one call allowed, one denied, one approved in the inbox then replayed, and `agent-gate-audit verify` accepts the chain.

Unit tests cover the approver policy outcomes, the session and CSRF checks, and the templates' escaping of tool arguments.

Tag `v0.1.0` after the change is archived.

## Constraints

- The inbox lives in the gate binary, on its own HTTP listener, separate from the `ExtMcp` gRPC port.
- Server-rendered pages with Go `html/template`. Plain HTML forms. No JavaScript build and no front-end dependencies. Chosen for longevity: stdlib only, nothing to keep current.
- Sign-in: OIDC authorization code flow with PKCE against Dex on `kind`. Phase 2 swaps to Entra by configuration only.
- Session in a server-side store, referenced by an `HttpOnly`, `Secure`, `SameSite=Strict` cookie. A CSRF token on every state-changing form.
- Approver authorization is a second OPA decision, for example `data.agentgate.approver`, over the approver claims and the request. A non-allow result is a deny. The requester rule stays in Go and runs first.
- A `Notifier` interface from the start ([ADR 0007](../../adr/0007-web-inbox-before-teams.md)). This change ships one implementation that logs.
- The inbox code does not import `ExtMcp` types. It calls the approval package.
- Each decision in the inbox appends the same audit entry that the test endpoint appended.
- No cloud resources and no cost.

## Non-goals

- Teams cards or any notifier beyond the log
- Entra sign-in, guest invitations, the sandbox role (Phases 2 and 3)
- An audit or trace viewer in the inbox (Phase 3)
- A JSON decision API for other clients
- Styling beyond a readable default

## Options considered

| Question | Options | Chosen |
|---|---|---|
| Approver authorization | OPA rule, config allowlist, any other identity | OPA rule. One policy engine, testable, maps to Entra app roles later |
| Inbox build | Go `html/template`, SPA over a JSON API | `html/template`. No build, no browser token handling, smallest attack surface |
| Test approval endpoint | Remove, keep behind a flag | Remove. The e2e test signs in through Dex with a Go HTTP client and posts the inbox forms. One decision path |
| Where the inbox runs | In the gate binary, separate service | In the gate binary. ADR 0007 says the gate serves the page. No internal API to secure |
| Phase 1 exit and tag | In this change, separate step | In this change |

## Chosen direction

An `inbox` package in `gate/internal/` with an OIDC client, a session store, CSRF checks, templates for the pending list, one request and the decision forms, and a log `Notifier`. The approval package gains approver authorization through a second OPA decision. `cmd/agent-gate` starts the inbox listener and drops the test endpoint. The e2e test drives sign-in and decisions through Dex and the inbox.

## Open questions

1. Session store: Postgres, beside the approvals, or in memory? Postgres survives a restart and more than one replica.
2. The input and output shape of the approver decision, and whether it lives in the same Rego package as the call decision.
3. Which claims the approver identity uses on `kind` (Dex `sub` or `email`) and whether that matches the caller claim for the requester rule.
4. How the e2e test completes the Dex password form without a browser, and how `Secure` cookies work over plain HTTP on `kind` (a dev flag, or TLS on the listener).
5. What a request page shows of the arguments: the full canonical JSON, or truncated with a size limit.
6. Where the `Notifier` is called: on each new pending request only, or also on decisions.
