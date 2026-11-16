# 0005: AKS Free tier, one node, Australia East

- Status: Accepted
- Date: 2026-11-16

## Context

The platform must run on a personal budget of NZ$80 to NZ$150 a month. The maintainer is in New Zealand. Prices are in the [cost model](../cost-model.md).

## Options

**Runtime**

1. AKS, Free tier, one node
2. AKS Automatic
3. Azure Container Apps, consumption plan

**Region**

1. Australia East
2. New Zealand North

## Decision

AKS Free tier with one D2as_v5 node, in Australia East.

- Container Apps is cheaper: about US$10 a month idle against US$36. AKS is chosen anyway. The project exists partly to show Kubernetes platform work, and agentgateway's Kubernetes mode uses the Gateway API.
- AKS Automatic cannot be stopped and always carries the Standard tier fee.
- New Zealand North listed no Azure AI Foundry models on 2026-11-16 and costs 4 to 6% more for VMs.

## Consequences

- B-series VMs cannot run the AKS system node pool, so the node is a D-series.
- The Free tier has no uptime SLA. That is acceptable for a demo.
- Model deployments use the regional Standard type, so prompts are processed in Australia.
- A single-node system pool must be confirmed in Phase 2. The Microsoft docs disagree on the minimum.
- An Arm node (D2pls_v6) is a later saving, once images are multi-arch.
