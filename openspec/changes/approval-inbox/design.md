# Design

## Context

See [proposal.md](proposal.md) and the [brainstorm](../../../docs/greenfield/approval-inbox/01-brainstorm.md). The gate from `gate-core` decides calls through `approval.Gate`, stores requests and the audit chain in Postgres, and decides requests through `Gate.Decide`, which today only the test handler in `cmd/agent-gate/main.go` calls. `Store.Decide` runs `CheckDecision` inside the transaction that holds the audit lock. Dex v2.46.1 runs on `kind` with in-memory storage, a static password user `human@agent-gate.test` and an issuer URL on the cluster DNS name.

### Answers to the brainstorm's open questions

| # | Question | Answer |
|---|---|---|
| 1 | Session store | Postgres, a new table. See decision 3 |
| 2 | Approver decision shape | Same Rego package, `data.agentgate.approver`, returns `{"allow": bool, "reason": string}`. See decision 4 |
| 3 | Approver claim | The configured caller claim, default `sub`, read from the ID token. Dex writes the same `sub` for a user in the ID token and the access token, so the requester rule compares like with like |
| 4 | Dex without a browser, `Secure` on HTTP | The test client maps cluster host names to port-forwards and keeps its own cookies. See decision 7 |
| 5 | Arguments on a request page | The full canonical JSON, indented. No truncation |
| 6 | Where the notifier is called | On each new pending request only. Decisions are in the audit log |

## Goals / Non-Goals

**Goals:**

- One decision path: the inbox calls `Gate.Decide`, and nothing else decides.
- Every check that guards a decision runs inside the decision transaction.
- Swap Dex for Entra with configuration only.

**Non-Goals:**

- A human using the inbox in a browser on `kind`. The issuer and redirect URLs are cluster names. Phase 2 has real host names and TLS.
- TLS on the inbox listener. Phase 2 terminates TLS in front of the gate.
- Session cleanup jobs. Expired rows are deleted at sign-in.
- Styling. The CSP allows no style sheet, so the pages use browser defaults.

## Decisions

### 1. Package layout

```
gate/internal/
├── inbox/       handlers, templates (embed), OIDC client, sessions, CSRF
├── approval/    + Notifier, LogNotifier, ApproverPolicy hook, Pending/Get
├── policy/      + approver query
├── postgres/    + sessions, Pending/Get, migration 0002
└── identity/    caller and tenant mapping only; bearer verifier removed
```

`inbox` imports `approval`, `policy` types and an interface for sessions. It does not import `extmcp`. `cmd/agent-gate` wires the parts.

### 2. Routes

| Method and path | Needs | Does |
|---|---|---|
| `GET /` | session | Pending list |
| `GET /requests/{ref}` | session | One request |
| `POST /requests/{ref}/decision` | session, CSRF, `action=approve` or `reject` | `Gate.Decide`, then 303 to the request page |
| `GET /login` | none | New pre-sign-in session, redirect to the issuer |
| `GET /callback` | pre-sign-in session | Code exchange, ID token check, new session id, 303 to `/` |
| `POST /logout` | session, CSRF | Delete session, 303 to a signed-out page |
| `GET /healthz` | none | 200, for the readiness probe |

A `GET` with no session redirects to `/login`. A `POST` with no session returns 403. Every other path returns 404.

### 3. Sessions in Postgres

Migration `0002_inbox_sessions.sql`:

```
inbox_sessions(
  id_hash     text PRIMARY KEY,   -- sha256 of the cookie value
  approver    text,               -- null before sign-in
  claims      jsonb,
  csrf        text NOT NULL,
  state       text, nonce text, verifier text,   -- set before sign-in only
  created_at  timestamptz NOT NULL,
  expires_at  timestamptz NOT NULL)
```

- The cookie value is 32 random bytes, base64url. The table holds only its SHA-256, so a database read does not give a usable cookie.
- `/login` creates a row with `state`, `nonce` and the PKCE verifier and a 10 minute expiry. `/callback` looks the row up by cookie, compares `state` in constant time, deletes the row and inserts a signed-in row with a new id. This blocks session fixation and binds the callback to the browser that started it.
- Cookie name `__Host-agent-gate`, attributes as the spec states. Lifetime is a flag, default 8 hours.
- Postgres over memory: the gate already depends on it, a restart does not sign everyone out, and a second replica in Phase 2 works with no change.

### 4. Approver policy

- `policy.OPA` prepares a second query, `data.agentgate.approver`, from the same files. The policy version stays one digest over the files.
- Input:

  ```json
  {"approver": {"id": "<caller claim value>", "claims": {...ID token claims}},
   "request": {"ref": "...", "tenant": "...", "agent": "...", "target": "...", "tool": "...", "arguments": {...}}}
  ```

- Output: an object with boolean `allow` and string `reason`. Anything else is a refusal with a reason such as `approver policy defines no decision`.
- `Store.Decide` takes a new `authorize func(approval.Request) error`. It runs after `CheckDecision`, in the same transaction, so the requester rule runs first and a refusal appends `decision_refused` with the reason, as today.
- The pending list and request page call the same check, outside a transaction, to filter what they show. The decision still checks again.
- The `kind` policy allows an approver whose `email` claim is in a fixed set holding `human@agent-gate.test`, and requires `email_verified`.

### 5. OIDC client

- `golang.org/x/oauth2` for the code exchange with `oauth2.S256ChallengeOption` and `oauth2.VerifierOption`. `go-oidc` for discovery and the ID token check with the client id as audience. Both are already in `go.sum`.
- Flags (each with an `AGENT_GATE_` variable): `-inbox-addr` (default `:8081`), `-inbox-url` (the external base URL, for the redirect URL and the notifier link), `-oidc-issuer`, `-oidc-client-id`, `-session-lifetime`. The client secret comes from `AGENT_GATE_OIDC_CLIENT_SECRET` only, never a flag.
- The inbox is required. The gate does not start without the issuer, client id, secret and inbox URL. A gate that can hold a call but not decide it is not useful.

### 6. Notifier

```go
type Notifier interface { Notify(ctx context.Context, r Request) }
```

`Gate.Check` calls it after `UseOrRequest` returns a request whose ref is the new ref it generated. `LogNotifier` writes one `slog` line with ref, tool, target, agent, expiry and `<inbox-url>/requests/<ref>`. `Notify` returns nothing, so a failing notifier cannot change a verdict. Teams in Phase 5 is a second implementation.

### 7. End-to-end test without a browser

- The test port-forwards Dex and the inbox as today. Its HTTP client has a `DialContext` that maps `dex.agent-gate-e2e.svc.cluster.local:5556` and `agent-gate.agent-gate-e2e.svc.cluster.local:8081` to the local forwarded ports, so the redirect chain works with the real issuer and redirect URLs.
- Dex config: `oauth2.skipApprovalScreen: true`, an inbox client `agent-gate-inbox` with the redirect URL `http://agent-gate.agent-gate-e2e.svc.cluster.local:8081/callback`, and a second static user `outsider@agent-gate.test` that the approver policy does not allow.
- The test follows redirects to Dex's local login form, posts `login` and `password`, and follows the redirect to `/callback`.
- Go's `cookiejar` does not send `Secure` cookies over `http`. The test uses a small jar that does. The gate always sets `Secure`. No dev flag.
- The self-approval scenario needs a human requester. The test gets a password-grant access token for `human@agent-gate.test` with the `agent-gate` audience, makes the call, then tries to approve it in the inbox as the same user. Dex needs `oauth2.passwordConnector: local` and a client with the `password` grant.
- The CSRF scenarios post the form with no token and with the token of a second session.

### 8. Phase 1 exit test

A subtest named `Phase 1 exit` runs, in one test: an allowed call, a denied call, a held call approved in the inbox then replayed, and `agent-gate-audit verify` on a fresh export. The existing `Audit log` subtest keeps the tamper cases. The tag `v0.1.0` goes on the merge commit of the archive pull request, after `make e2e-gate` passes in CI.

## Risks / Trade-offs

- **Dex form markup changes** break the test's form post. Mitigation: Dex is pinned. The test fails loudly with the page body.
- **Policy runs inside the audit lock.** OPA evaluation is in-process and fast for the small approver rule. A large policy would hold the lock longer.
- **The list filter evaluates policy once per pending request.** Fine for a handful of requests. Revisit with paging if the list grows.
- **No browser on `kind`** means no manual click-through before Phase 2. The e2e test is the proof.
- **CSP with no styles** gives plain pages. Accepted by the brainstorm non-goal.

## Open Questions

- How Entra maps app roles to the approver input. Phase 2 decides. The input already passes all ID token claims.
