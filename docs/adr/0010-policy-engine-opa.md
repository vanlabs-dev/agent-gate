# 0010: Policy engine is OPA, embedded

- Status: Accepted
- Date: 2026-11-24

## Context

The gate decides each tool call with one of three outcomes: allow, deny or needs approval. Policy reads the tool name, the arguments and the caller's identity claims ([architecture options](../architecture/options.md)).

## Options

1. **OPA, embedded** through its Go library. A Rego decision document returns the outcome.
2. **Cedar** through `cedar-go`. Cedar decides permit or forbid, so "needs approval" needs a second action, such as `call` and `call_without_approval`.

## Decision

Option 1.

- One decision document expresses all three outcomes, with a reason.
- Claims and arguments map to JSON input with no schema.
- OPA and Rego are mature and widely known.

## Consequences

- Policies are untyped. Unit tests cover each outcome.
- The gate depends on the OPA Go module, a large dependency. `govulncheck` runs on it.
- The policy code wraps OPA behind an interface, so that Cedar remains possible.
