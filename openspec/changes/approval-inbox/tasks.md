# Tasks

Work on the branch `approval-inbox`. Run `make check` before each push.

## 1. Risk checks

- [ ] 1.1 In a scratch `kind` cluster, run Dex v2.46.1 with `skipApprovalScreen`, `passwordConnector: local`, the inbox client and the two static users of design decision 7. From a Go program outside the cluster, run the code flow with PKCE through a port-forward and the host mapping dialer, and get a password-grant access token for `human@agent-gate.test`. Verify: the ID token and the access token carry the same `sub`, the ID token has `email` and `email_verified`, and the login form fields are `login` and `password`. If a check fails, stop and record the fix in the design through `/opsx:update`

## 2. Approver policy

- [ ] 2.1 Add the approver query to `internal/policy` (design decision 4). Verify: unit tests pass for allow, deny with a reason, an undefined rule, a non-object result, a missing `allow`, and an unchanged policy version
- [ ] 2.2 Add the approver rule to `gate/deploy/policy/agentgate.rego`. Verify: the deploy policy test checks `human@agent-gate.test` allowed, `outsider@agent-gate.test` refused, and an unverified email refused

## 3. Approval core

- [ ] 3.1 Add the `authorize` step to `Store.Decide` in the memory store and the Postgres store, after `CheckDecision` and in the same transaction, and pass approver claims through `Gate.Decide`. Verify: unit tests pass for self-approval before policy (the policy is not called), a policy refusal that leaves the request pending and appends `decision_refused` with the reason, and an allowed approval; the Postgres test passes the same cases
- [ ] 3.2 Add `Notifier`, `LogNotifier` and the call in `Gate.Check` (design decision 6). Verify: unit tests show one notification for a new request, none for a repeat, and an unchanged verdict when the notifier panics or is nil
- [ ] 3.3 Add `Pending` and `Get` to both stores. Verify: unit and Postgres tests show that `Pending` returns only pending, unexpired requests in creation order, and `Get` returns `ErrNotFound` for an unknown ref

## 4. Sessions

- [ ] 4.1 Add migration `0002_inbox_sessions.sql` and the session methods in `internal/postgres` (design decision 3). Verify: Postgres tests pass for create, look up by cookie value, rotation at sign-in, delete at sign-out, an expired session treated as absent, and no raw cookie value in the table

## 5. Inbox

- [ ] 5.1 Write the OIDC client in `internal/inbox` (design decision 5). Verify: unit tests against a local test issuer pass for a good sign-in, a wrong `state`, a wrong `nonce`, an ID token for another client, and a missing caller claim
- [ ] 5.2 Write the handlers, CSRF check, response headers and templates of design decision 2. Verify: `httptest` tests cover each scenario in the `approval-inbox` spec that needs no cluster: redirect with no session, 403 for a POST with no session, no token and a wrong token, the list filter, the full arguments, `<script>` escaped, each error status, the headers on every route, and 404 for `/test/requests/x/approve`
- [ ] 5.3 Wire the inbox into `cmd/agent-gate`: new flags, the secret from the environment only, the required inbox config. Remove the test handler, `-test-endpoints`, `-oidc-audiences` and the bearer verifier in `internal/identity`. Verify: `main_test.go` passes for missing inbox config, a secret given as a flag rejected, and defaults; `grep -rn "test-endpoints\|/test/requests" gate` returns nothing outside the e2e removal diff

## 6. End-to-end on `kind`

- [ ] 6.1 Update `gate/deploy/`: Dex config of design decision 7, the gate env and a Secret for the client secret, the readiness probe on `/healthz`. Verify: `make e2e-gate-deploy` brings every pod to ready
- [ ] 6.2 Change `gate/e2e/` to sign in through Dex and decide through the inbox forms, with the host mapping dialer and the cookie jar of design decision 7. Replace the test endpoint helpers. Add scenarios for each brainstorm success signal: approve and replay once, reject then new ref, no session, no CSRF, wrong CSRF, outsider refused, requester refused, expired request refused, `/test/` route 404, and the `Phase 1 exit` subtest of design decision 8. Verify: `make e2e-gate` exits 0 and lists each subtest as passed
- [ ] 6.3 Update the `repo-checks` CI job only if a step changes. Verify: the `gate end-to-end test` check passes on the pull request

## 7. Docs and release

- [ ] 7.1 Update `docs/architecture/options.md` (inbox, approver policy, notifier), `docs/roadmap.md` (Phase 1 done), and `AGENTS.md` if a command changed. Verify: `/lock-in` reports no drift
- [ ] 7.2 After archive and merge, tag `v0.1.0` on the merge commit and push the tag. Verify: `git ls-remote --tags origin v0.1.0` shows the tag
