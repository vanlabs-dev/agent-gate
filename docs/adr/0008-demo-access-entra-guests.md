# 0008: Demo access through Entra B2B guests

- Status: Accepted
- Date: 2026-11-16

## Context

The demo must be invite-only. The maintainer leads a guided tour. An invited person must also be able to sign in and try the controls. Access must be revocable.

## Options

1. Entra B2B guest invitations
2. IP allowlist
3. Shared credentials
4. Public demo

## Decision

Option 1.

- The maintainer invites each person as a guest in the project's Entra tenant. The first 50,000 monthly active users are free ([pricing](https://learn.microsoft.com/en-us/entra/external-id/external-identities-pricing)).
- A guest gets a sandbox role. In the sandbox, a guest can start a prepared agent task against a sandbox resource group, approve or deny the agent's action, and read the audit trail and the trace of that run.
- Each guest has a token cap at the gateway.

## Consequences

- Removing the guest account revokes access.
- Guest actions appear in the audit log under the guest's own identity, which is part of the demo.
- The sandbox role must not reach the project's real resources. Policy tests must prove it.
- The demo needs a guided tour document: what to click, what to look at.
- The cluster is off by default ([ADR 0006](0006-cluster-off-by-default.md)), so a guest session is arranged in advance.
