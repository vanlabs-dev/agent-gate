# 0004: The anchor agent is an Azure ops agent

- Status: Accepted
- Date: 2026-11-16

## Context

A gate needs an agent that does real work and takes actions that need real approval. A demo agent with invented tasks proves nothing.

## Options

1. **Microsoft 365 tenant hygiene agent.** Finds stale guests, risky app consents and licence waste. It needs a licensed tenant. The maintainer has none in real use, so the problems would be seeded.
2. **Azure ops agent on the project's own subscription.** Watches spend, Terraform drift and AKS health. Proposes changes such as "stop the idle cluster" or "delete this orphan disk".

## Decision

Option 2. The agent operates the platform that hosts the gate. Each write action goes through the gate and needs approval.

## Consequences

- The agent has real work from the first deployment, with no licence cost.
- The audit trail of the project's own operations is real data for the demo.
- The agent can stop the cluster that it runs on. That action is the last in a run. A GitHub workflow starts the cluster. See [ADR 0006](0006-cluster-off-by-default.md).
- The agent's Azure role must be narrow: read on the subscription, write only on named resource groups.
- The Microsoft 365 agent can follow as a second agent on a seeded tenant.
