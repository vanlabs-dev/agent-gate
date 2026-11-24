# Spec Delta

## Purpose

Defines how a tool call that needs approval is held, decided and passed once: what an approval binds to, how arguments are made canonical, and the rules for single use, expiry and who may approve.

## ADDED Requirements

### Requirement: An approval binds to a call digest
The gate SHALL compute a call digest over the tenant, the caller, the target, the tool name and the canonical arguments, with a schema version inside the hashed bytes. The digest SHALL read only the tool name and the arguments from the call params. The digest SHALL be written as `sha256:` and 64 lowercase hex characters.

#### Scenario: Same call in a new session
- **WHEN** a caller sends the same tool and arguments in two sessions with two tokens
- **THEN** both calls have the same call digest

#### Scenario: Changed `_meta`
- **WHEN** two calls differ only in `_meta`
- **THEN** both calls have the same call digest

#### Scenario: Changed caller, tenant, target, tool or arguments
- **WHEN** two calls differ in one of caller, tenant, target, tool or arguments
- **THEN** the two calls have different call digests

### Requirement: Arguments are made canonical with RFC 8785
The gate SHALL encode the arguments with RFC 8785 (JSON Canonicalization Scheme) before it computes a digest. The gate SHALL deny a call whose arguments hold a number that a 64-bit float cannot represent exactly, so that two different numbers never share a digest.

#### Scenario: RFC 8785 test vectors
- **WHEN** the encoder runs on the test vectors of RFC 8785
- **THEN** each output equals the expected bytes

#### Scenario: Key order and white space
- **WHEN** two argument objects differ only in key order and white space
- **THEN** both encode to the same bytes

#### Scenario: Integer above 2^53
- **WHEN** a call holds the argument `9007199254740993`
- **THEN** the client receives a tool result with `isError` true
- **AND** no approval request is stored

### Requirement: A call that needs approval is stored as a pending request
When policy returns needs approval, the gate SHALL store a pending request with a new approval reference and deny the call. The reference SHALL be unguessable. When a pending request with the same call digest exists and has not expired, the gate SHALL return its reference and store no new request.

#### Scenario: First call
- **WHEN** a call needs approval and no pending request has its digest
- **THEN** the gate stores one pending request
- **AND** the tool result text contains `approval required: ref=` and the reference

#### Scenario: Repeat before a decision
- **WHEN** the caller repeats the identical call before a decision
- **THEN** the tool result contains the same reference
- **AND** the gate stores no second request

### Requirement: An approved call passes once on identical replay
When a call needs approval and an approved, unused, unexpired request has its call digest, the gate SHALL pass the call and mark the request used. The gate SHALL mark the request used and append the audit entry in the same transaction that decides to pass. Two concurrent replays SHALL NOT both pass.

#### Scenario: Replay after approval
- **WHEN** a request is approved
- **AND** the caller replays the identical call in a new session
- **THEN** the upstream server receives the call once with the approved arguments

#### Scenario: Second replay
- **WHEN** an approved call has passed once
- **AND** the caller replays it again
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no further call

#### Scenario: Concurrent replays
- **WHEN** two identical replays of one approved request reach the gate at the same time
- **THEN** exactly one passes

#### Scenario: Replay with changed arguments
- **WHEN** a request is approved
- **AND** the caller sends the same tool with different arguments
- **THEN** the upstream server receives no call

#### Scenario: Replay by another caller
- **WHEN** a request is approved
- **AND** a different caller sends the identical tool and arguments
- **THEN** the upstream server receives no call

### Requirement: Policy is evaluated again on replay
The gate SHALL evaluate policy for each replay. An approval SHALL pass a call only when policy still returns needs approval.

#### Scenario: Policy changed to deny
- **WHEN** a request is approved
- **AND** policy now returns deny for that call
- **THEN** the replay is denied with the policy reason
- **AND** the request stays unused

### Requirement: An approval expires
Each request SHALL expire at a fixed time after it was stored. The default SHALL be 1 hour, and configuration SHALL change it. The gate SHALL NOT pass a call for an expired request, and SHALL NOT accept a decision on one.

#### Scenario: Replay after expiry
- **WHEN** a request is approved
- **AND** the caller replays the call after the request expired
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no call

#### Scenario: Approval after expiry
- **WHEN** an approver approves a request after it expired
- **THEN** the gate rejects the approval

### Requirement: The requester cannot approve
The gate SHALL take the approver from a validated token, using the same configured claim as the caller. The gate SHALL reject a decision by an approver whose value equals the caller of the request.

#### Scenario: Self-approval
- **WHEN** the caller of a request tries to approve it
- **THEN** the gate rejects the approval
- **AND** the request stays pending

#### Scenario: Approval by another identity
- **WHEN** an identity other than the caller approves a pending request
- **THEN** the request becomes approved

### Requirement: An approver can reject a request
An approver SHALL be able to reject a pending request. The gate SHALL NOT pass a call for a rejected request, and SHALL store a new pending request when the caller sends the call again.

#### Scenario: Replay after rejection
- **WHEN** a request is rejected
- **AND** the caller replays the call
- **THEN** the upstream server receives no call
- **AND** the tool result contains a new reference

### Requirement: The test approval endpoint is off by default
The gate SHALL serve the approval endpoint used by tests only when a start flag enables it. The endpoint SHALL accept a decision only with a bearer token that the gate validates against the configured OIDC issuer.

#### Scenario: Default start
- **WHEN** the gate starts with no test flag
- **THEN** no approval endpoint answers

#### Scenario: Decision with no token
- **WHEN** a client posts a decision with no bearer token
- **THEN** the endpoint returns 401 and the request is unchanged
