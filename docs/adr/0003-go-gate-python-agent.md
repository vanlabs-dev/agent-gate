# 0003: Go for the gate, Python for the reference agent

- Status: Accepted
- Date: 2026-11-16

## Context

The gate is a stateful network service on Kubernetes that speaks gRPC to agentgateway. The reference agent is an LLM application that uses MCP and Azure SDKs.

## Options

1. Go for the gate, Python for the agent
2. Python for both
3. C#, Rust or TypeScript for the gate

## Decision

Option 1.

- Go is the norm for Kubernetes-side services. gRPC, OPA and `cedar-go` are native to it.
- Python has the most mature agent and MCP client libraries.

## Consequences

- Two toolchains in CI.
- The gate uses the MCP Go SDK only for test servers. On 2026-11-16 that SDK had no tasks support.
- `cedar-go` is active but small. The policy engine choice stays open until Phase 1.
