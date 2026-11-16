# 0006: The cluster is off by default

- Status: Accepted
- Date: 2026-11-16

## Context

One D2as_v5 node costs about US$79 a month if it runs 24x7. With fixed costs, that is about US$115 (NZ$195) before model tokens. The ceiling is NZ$150.

## Options

1. Run 24x7
2. On by default, stop after one to two idle days
3. Off by default, start on a schedule and on demand, stop when idle

## Decision

Option 3.

- A scheduled GitHub workflow starts the cluster for the agent's daily run.
- A manual workflow starts it for work sessions and demos.
- The cluster stops after a few idle hours. The ops agent proposes the stop and a human approves it. A scheduled workflow is the backstop.

At about 176 running hours a month, the cost is about US$55 (NZ$94) before tokens.

## Consequences

- A guest cannot use the demo on a stopped cluster. The maintainer starts it before a session.
- State must survive a stop. Persistent volumes do. Standalone pods do not.
- AKS requires a wait of 15 to 30 minutes between a stop and the next start ([docs](https://learn.microsoft.com/en-us/azure/aks/start-stop-cluster)).
- A start can fail when the region is short of capacity. The demo runbook must say what to do.
- It is not yet confirmed which resources bill while the cluster is stopped. Measure in Phase 2.
- If stop and start prove unreliable, the alternative is to destroy and recreate the cluster with Terraform, with state held outside it.
