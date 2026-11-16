# 0007: Web approval inbox first, Teams later

- Status: Accepted
- Date: 2026-11-16

## Context

A human needs a place to see a pending tool call and approve or deny it.

## Options

1. **Web inbox.** A small page served by the gate, behind Entra sign-in.
2. **Teams adaptive cards.** The strongest fit for Microsoft-centric organisations. It needs a Microsoft 365 tenant with a Teams licence and a bot registration. The project has no such tenant.

## Decision

Option 1 first. Teams follows in Phase 5 as a second channel.

## Consequences

- Any invited Entra guest can approve, with no Microsoft 365 licence.
- The gate needs a notifier interface from the start, so that Teams is an addition and not a rewrite.
- The inbox is part of the gate's attack surface. It needs Entra OIDC, CSRF protection and the rule that an approver cannot be the requester.
- Until Teams exists, approvers must open the inbox to see requests.
