# Spec Delta

## MODIFIED Requirements

### Requirement: The requester cannot approve
The gate SHALL take the approver from the ID token of the inbox sign-in, using the same configured claim as the caller. The gate SHALL reject a decision by an approver whose value equals the caller of the request. The gate SHALL check this rule before the approver policy.

#### Scenario: Self-approval
- **WHEN** the caller of a request tries to approve it
- **THEN** the gate rejects the approval
- **AND** the request stays pending

#### Scenario: Approval by another identity
- **WHEN** an identity other than the caller, allowed by the approver policy, approves a pending request
- **THEN** the request becomes approved

## ADDED Requirements

### Requirement: An approver needs an allow from policy
The gate SHALL evaluate the approver decision `data.agentgate.approver` from the loaded policy files for each decision. Its input SHALL hold the approver identity, the approver claims and the request without its status. The gate SHALL accept a decision only when the result is an object with `allow` true. Any other result, an undefined result or an error SHALL refuse the decision with a reason.

#### Scenario: Approver allowed
- **WHEN** the approver policy returns `allow` true for an approver
- **THEN** that approver can approve the request

#### Scenario: Approver not allowed
- **WHEN** the approver policy returns `allow` false for an approver
- **THEN** the gate refuses the decision with the policy reason
- **AND** the request stays pending
- **AND** the log holds a refused decision entry

#### Scenario: No approver rule
- **WHEN** the policy files define no `data.agentgate.approver`
- **THEN** the gate refuses every decision

### Requirement: The notifier hears of each new request
The gate SHALL pass each new pending request to its notifier after the request is stored. A repeat of a call that returns an existing pending reference SHALL NOT notify again. A notifier failure SHALL NOT change the verdict of the call. The gate SHALL ship a notifier that writes one log line with the reference and the inbox link.

#### Scenario: New request
- **WHEN** a call needs approval and no pending request has its digest
- **THEN** the gate logs one notification with the reference

#### Scenario: Repeat before a decision
- **WHEN** the caller repeats the identical call before a decision
- **THEN** the gate logs no second notification

## REMOVED Requirements

### Requirement: The test approval endpoint is off by default
**Reason**: The inbox is now the only way to decide a request.
**Migration**: Sign in to the inbox and use its forms. The flags `-test-endpoints` and `-oidc-audiences` are removed.
