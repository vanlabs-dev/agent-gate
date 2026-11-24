# Spec Delta

## ADDED Requirements

### Requirement: The gate callout has a deadline
The gateway configuration SHALL set a deadline on each call to the gate. When the gate does not answer within that deadline, the call SHALL fail closed: the client SHALL receive a JSON-RPC error within the deadline plus 2 seconds, and the upstream MCP server SHALL receive no call.

#### Scenario: Gate slower than the deadline
- **WHEN** the deadline is 5 seconds
- **AND** the gate delays its answer by 10 seconds
- **AND** a client with a valid token calls the gated tool
- **THEN** the client receives a JSON-RPC error within 7 seconds
- **AND** the upstream server receives no call
