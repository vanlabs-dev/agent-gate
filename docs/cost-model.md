# Cost model

- Budget ceiling: NZ$150 a month. Target: NZ$95 to NZ$110.
- Prices: USD list prices, Australia East, checked 2026-11-16.
- Source, unless a row says otherwise: [Azure Retail Prices API](https://prices.azure.com/api/retail/prices).
- NZD figures assume NZ$1.70 for US$1. The rate was not checked.
- A month is 730 hours.
- Cloud cost starts in Phase 2 of the [roadmap](roadmap.md).

## Fixed and running costs

| Item | Always-on US$/month | Running US$/hour | Note |
|---|---|---|---|
| AKS control plane, Free tier | 0 | 0 | [Tiers](https://learn.microsoft.com/en-us/azure/aks/free-standard-pricing-tiers) |
| One D2as_v5 node (2 vCPU, 8 GiB) | 0 | 0.108 | US$78.84 at 24x7 |
| OS disk, P4 32 GiB | 5.28 | | Assumed to bill when stopped |
| Postgres disk, E4 32 GiB | 3.26 | | |
| Standard public IP | 3.65 | | |
| Standard Load Balancer | 18.25 | | Assumed to bill when stopped |
| Container registry, Basic | 5.07 | | |
| DNS zone | 0.50 | | |
| Key Vault | About 0 | | Billed per operation |
| **Total** | **36.01** | **0.108** | |

## Monthly total by usage pattern

| Pattern | Running hours | US$/month | NZ$/month |
|---|---|---|---|
| Stopped all month | 0 | 36 | 61 |
| Chosen: 8 hours on 22 days | 176 | 55 | 94 |
| Running 24x7 (rejected) | 730 | 115 | 195 |

Add model tokens: capped at US$10 (NZ$17) a month at the gateway.

Expected total: NZ$95 to NZ$110 a month, depending on token use.

## Comparison: Azure Container Apps

The consumption plan has no fixed fee and scales to zero ([billing](https://learn.microsoft.com/en-us/azure/container-apps/billing)). With a Postgres Flexible Server B1ms:

| Pattern | US$/month |
|---|---|
| Stopped | About 10 |
| 20 running hours | About 10.50 |
| 24x7 | 65 to 160 |

AKS is chosen for other reasons. See [ADR 0005](adr/0005-aks-free-tier-australia-east.md).

## Costs to avoid

| Item | US$/month | Use in its place |
|---|---|---|
| AKS Standard tier | 73 | Free tier |
| AKS Automatic | Standard tier fee, and it cannot be stopped ([docs](https://learn.microsoft.com/en-us/azure/aks/intro-aks-automatic)) | AKS with a manual node pool |
| Azure Managed Grafana, Standard | About 36, plus 6 for each active user | Application Insights |
| Application Gateway for Containers | About 157 | agentgateway as the in-cluster Gateway API controller |
| Postgres Flexible Server B1ms | 18.98 | Postgres in the cluster on a disk |
| Log Analytics above 5 GB | 3.34 for each GB | Sample traces and keep ingestion under the free 5 GB |

## Controls

- Budget alerts at NZ$100 and NZ$150, created in Terraform (`azurerm_consumption_budget_subscription`).
- Azure cost data arrives 8 to 24 hours late and budgets are evaluated daily ([docs](https://learn.microsoft.com/en-us/azure/cost-management-billing/costs/tutorial-acm-create-budgets)). The alert is a backstop, not a control.
- The real controls are the stop workflow, the token cap at the gateway, and the ops agent's daily spend check.

## Region and data residency

- Australia East. New Zealand North costs 4 to 6% more for VMs and listed no Azure AI Foundry models on 2026-11-16 ([availability](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/models-sold-directly-by-azure-region-availability)).
- Use the regional Standard deployment type, so prompts are processed in Australia ([deployment types](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/deployment-types)).

## Later savings

- An Arm node, D2pls_v6: US$56.50 at 24x7 against US$78.84. Needs multi-arch images and a check that 4 GiB is enough.
- A spot user pool for the agent's job. The system pool cannot use spot.
- Destroy and recreate the cluster in place of stop and start: about US$5.60 a month fixed. Needs all state outside the cluster.

## Not yet verified

Measure these in Phase 2 and update this page.

- Which resources bill while the cluster is stopped. The table assumes the Load Balancer, the public IP and the disks do.
- Cluster start time.
- Whether a single-node system pool meets current AKS minimums. The Microsoft docs disagree ([system pools](https://learn.microsoft.com/en-us/azure/aks/use-system-pools), [quotas](https://learn.microsoft.com/en-us/azure/aks/quotas-skus-regions)).
- Whether one 8 GiB node holds the full workload.
- The NZD exchange rate.
