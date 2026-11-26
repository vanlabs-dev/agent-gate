# Proposal

## Why

The gate can hold a call for approval, but only a test endpoint with a bearer token can decide it, and any identity other than the requester may approve. Phase 1 ends with a human approving in a real inbox, so this change adds that inbox, puts approver authorization in policy, removes the test endpoint, and runs the Phase 1 exit test. The [brainstorm](../../../docs/greenfield/approval-inbox/01-brainstorm.md) holds scope and success signals. [ADR 0007](../../../docs/adr/0007-web-inbox-before-teams.md) sets the web inbox first.

## What Changes

- Add a web inbox to the gate binary, on its own HTTP listener: a list of pending requests, one page for each request, and approve and reject forms. Server-rendered with Go `html/template`. No JavaScript.
- Sign in with the OIDC authorization code flow and PKCE. Dex on `kind`, Entra in Phase 2 by configuration only.
- Keep sessions in Postgres. The cookie is `HttpOnly`, `Secure` and `SameSite=Strict`. Each form carries a CSRF token.
- Add approver authorization: a second OPA decision, `data.agentgate.approver`, in the same policy files. The requester rule stays in Go and runs first.
- Add a `Notifier` interface. The gate calls it once for each new pending request. This change ships one implementation that logs.
- **BREAKING:** Remove the test approval endpoint and the flags `-test-endpoints`, `-oidc-audiences`. Add inbox flags.
- Change `make e2e-gate` to sign in through Dex and decide in the inbox. Add the Phase 1 exit test.
- Tag `v0.1.0` after the change is archived.

## Capabilities

### New Capabilities

- `approval-inbox`: the web inbox: sign-in, sessions, CSRF protection, the pages and forms, what a request page shows, and the response headers.

### Modified Capabilities

- `tool-call-approval`: the requester rule takes the approver from the inbox sign-in, not a bearer token. A decision needs an allow from the approver policy. A new pending request is passed to the notifier. The test approval endpoint is removed.

## Impact

- **New code:** `gate/internal/inbox/` (handlers, templates, OIDC client, sessions, CSRF), a session migration in `gate/internal/postgres/migrations/`, an approver query in `gate/internal/policy/`, a `Notifier` in `gate/internal/approval/`.
- **Changed code:** `gate/cmd/agent-gate` (flags, listener, no test handler), `gate/internal/identity` (bearer verifier removed), `gate/e2e/`, `gate/deploy/` (Dex users and inbox client, gate env, approver rule).
- **New dependencies:** `golang.org/x/oauth2` for the code flow and PKCE. `go-oidc` stays for ID token checks.
- **Docs:** `docs/architecture/options.md` (inbox, approver policy), `docs/roadmap.md` (Phase 1 done), `AGENTS.md` if a command changes.
- **Cost:** none. No cloud resources.

Approved: 2026-11-27 (vanlabs-dev)
