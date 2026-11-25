# Roadmap

The plan assumes about 10 hours a week from one maintainer. The order puts the largest technical risk first and delays cloud cost until the core works.

Phase 0 was completed on 2026-11-18. Phase 1 started on 2026-11-24. The `gate-core` change built the policy engine, approvals, replay, the hash-chained audit and the verifier CLI, and proved them on `kind` on 2026-11-25. The `approval-inbox` change follows with the minimal inbox, then the tag `v0.1.0`.

| Phase | Weeks | Ships | Exit test |
|---|---|---|---|
| 0 Foundations | 1 to 2 | Public repo, conventions, CI for Go and Python. Build, test and lint commands in `AGENTS.md`. Spike on `kind`: agentgateway calls a stub gate, and a deny followed by a replay works | The spike passes in CI. ADR 0002 and 0009 are confirmed or changed |
| 1 Core on `kind` | 3 to 6 | Policy engine, pending approvals, replay, hash-chained audit, verifier CLI, minimal inbox. One stub MCP server and a script as the agent | One call allowed, one denied, one approved then replayed. The audit chain verifies. Tag `v0.1.0` |
| 2 Azure | 7 to 10 | Terraform, AKS, keyless login from GitHub Actions, Entra token checks, workload identity, Helm deploy, start and stop workflows, budget alerts | The pipeline builds the cluster and deploys the gate. The first real bill is compared with the cost model. Tag `v0.2.0` |
| 3 Agent and demo | 11 to 13 | Python ops agent with one real action: stop the idle cluster or delete an orphan disk. Guest invitations, sandbox role, per-guest token cap, guided tour, README, diagram | A 30-minute walkthrough. A guest signs in and approves a sandboxed action. Tag `v0.3.0` |
| 4 Useful agent | Months 4 to 5 | Spend watcher, Terraform drift check, AKS health check, daily runs, model budgets, signed audit checkpoints in immutable storage, threat model | The agent runs daily. Its approved actions reduce the monthly bill |
| 5 Depth | Months 5 to 6 | Teams approval cards, agent evaluations in CI, signed images, compatibility tests against pinned agentgateway versions, work with upstream on issue #3003 | One exit test for each item, written when the phase starts |
| 6 Beyond | Later | MCP tasks front, Microsoft 365 hygiene agent on a seeded tenant, policy packs, Helm chart for other users | Open |

## Milestone: done enough to demo

The end of Phase 3. A visitor can watch an agent propose an action, see a human approve it, and check the audit record.

## Checkpoints

- **End of Phase 2:** the first real bill decides whether the cost model stands.

## Out of scope until Phase 5

Teams, MCP tasks, evaluations, a second environment.
