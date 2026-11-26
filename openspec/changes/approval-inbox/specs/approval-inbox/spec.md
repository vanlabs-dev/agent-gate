# Spec Delta

## Purpose

Defines the web inbox where a human signs in, sees pending approval requests, and approves or rejects them: sign-in, sessions, CSRF protection, the pages, and the protections on each response.

## ADDED Requirements

### Requirement: The inbox has its own listener
The gate SHALL serve the inbox on an HTTP listener separate from the `ExtMcp` gRPC listener. The inbox SHALL answer only the routes it defines. No route under `/test/` SHALL answer.

#### Scenario: Test route
- **WHEN** a client posts to `/test/requests/<ref>/approve`
- **THEN** the inbox returns 404 and the request is unchanged

### Requirement: Sign-in uses the OIDC authorization code flow with PKCE
The inbox SHALL sign a human in with the OIDC authorization code flow and PKCE (S256) against the configured issuer and client. It SHALL check the `state` value and the ID token `nonce`, and SHALL accept only an ID token that the issuer signed for the configured client. Changing the issuer and client SHALL need configuration only.

#### Scenario: Page with no session
- **WHEN** a client opens the pending list with no session
- **THEN** the inbox redirects it to the issuer's authorization endpoint with a PKCE challenge

#### Scenario: Successful sign-in
- **WHEN** a human completes sign-in at the issuer
- **THEN** the inbox sets a session cookie and redirects to the pending list

#### Scenario: Wrong state
- **WHEN** the callback arrives with a `state` that the inbox did not issue for this browser
- **THEN** the inbox returns 400 and sets no session

### Requirement: Sessions are server-side
The inbox SHALL keep each session in the gate's database and SHALL put only a random session id in the cookie. The cookie SHALL be `HttpOnly`, `Secure`, `SameSite=Strict` and `Path=/`. A session SHALL end at a fixed time after sign-in, 8 hours by default, or when the human signs out. The inbox SHALL issue a new session id at sign-in.

#### Scenario: Restart
- **WHEN** the gate restarts while a session is valid
- **THEN** the same cookie still opens the pending list

#### Scenario: Sign-out
- **WHEN** a human signs out
- **THEN** the old cookie no longer opens the pending list

#### Scenario: Expired session
- **WHEN** a session is older than its lifetime
- **THEN** the inbox treats the client as having no session

### Requirement: A state-changing form needs a session and a CSRF token
The inbox SHALL accept a decision or sign-out only as a POST with a valid session and the CSRF token of that session. A missing session, a missing token or a wrong token SHALL return 403 and change nothing. A POST SHALL NOT redirect to sign-in.

#### Scenario: Decision with no session
- **WHEN** a client posts an approval with no session cookie
- **THEN** the inbox returns 403 and the request stays pending

#### Scenario: Decision with no CSRF token
- **WHEN** a signed-in client posts an approval with no CSRF token
- **THEN** the inbox returns 403 and the request stays pending

#### Scenario: Decision with a wrong CSRF token
- **WHEN** a signed-in client posts an approval with the CSRF token of another session
- **THEN** the inbox returns 403 and the request stays pending

### Requirement: The pending list shows what the approver may decide
The pending list SHALL show each pending, unexpired request that the signed-in approver may decide, with its reference, tool, target, caller, creation time and expiry time. It SHALL NOT show a request whose caller is the approver, or one that the approver policy does not allow.

#### Scenario: Allowed approver
- **WHEN** an allowed approver opens the pending list while a request is pending
- **THEN** the list shows that request

#### Scenario: Approver not allowed by policy
- **WHEN** an approver that the policy does not allow opens the pending list
- **THEN** the list does not show the request

### Requirement: A request page shows the full call
A request page SHALL show the reference, tenant, caller, target, tool, status, creation and expiry times, the call digest, and the full canonical arguments as indented JSON. It SHALL NOT truncate the arguments. It SHALL show the approve and reject forms only while the request is pending and unexpired.

#### Scenario: Pending request
- **WHEN** an allowed approver opens the page of a pending request
- **THEN** the page shows every argument and the approve and reject forms

#### Scenario: Unknown reference
- **WHEN** a client with a session opens the page of an unknown reference
- **THEN** the inbox returns 404

### Requirement: Output is escaped
The inbox SHALL escape every value from a request, a claim or a policy reason in HTML output. An argument value SHALL NOT be able to add markup or script to a page.

#### Scenario: Markup in an argument
- **WHEN** a request holds the argument `<script>alert(1)</script>`
- **THEN** the request page shows that text and holds no `<script>` element from it

### Requirement: A decision in the inbox reports its outcome
The inbox SHALL show the outcome of each decision. A refused decision SHALL return an error page with the reason: 404 for an unknown reference, 403 for the requester or an approver the policy does not allow, 409 for a request that is not pending or has expired.

#### Scenario: Approve
- **WHEN** an allowed approver approves a pending request
- **THEN** the inbox redirects to the request page, which shows the status approved

#### Scenario: Decision on an expired request
- **WHEN** an allowed approver approves a request after it expired
- **THEN** the inbox returns 409 with the reason and the request is not approved

### Requirement: Responses carry protective headers
Each inbox response SHALL set `Content-Security-Policy` with `default-src 'none'`, `form-action 'self'` and `frame-ancestors 'none'`, and SHALL set `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and `Cache-Control: no-store`.

#### Scenario: Headers on a page
- **WHEN** a client opens any inbox page
- **THEN** the response has each listed header
