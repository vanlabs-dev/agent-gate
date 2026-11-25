# agentgateway-integration Specification

## Purpose

Defines the contract between agentgateway and a gate behind its `ExtMcp` extension: what the gate receives for an MCP tool call, how a deny reaches the client, how a denied call passes once after approval, and what happens when the gate is unavailable. An end-to-end test on `kind` proves each requirement against one pinned agentgateway version.

## Requirements

### Requirement: Ungated calls pass through
A `tools/call` for a tool that the gate does not gate SHALL reach the upstream MCP server, and the client SHALL receive the upstream result.

#### Scenario: Ungated tool
- **WHEN** a client with a valid token calls a tool that is not gated
- **THEN** the upstream server receives the call once
- **AND** the client receives the upstream result

### Requirement: Gated calls are denied with an approval reference
A `tools/call` for a gated tool with no matching approval SHALL NOT reach the upstream MCP server. The client SHALL receive a tool result with `isError` set to true, and its text SHALL contain an approval reference.

#### Scenario: First call to a gated tool
- **WHEN** a client with a valid token calls the gated tool
- **THEN** the client receives a tool result with `isError` true
- **AND** the result text contains an approval reference
- **AND** the upstream server receives no call

### Requirement: An approved call passes once on identical replay
After the approval for a denied call is recorded, a replay with the same caller, tool and arguments SHALL reach the upstream MCP server and return its result. The replay SHALL NOT depend on the session or on the token instance of the denied call. The approval SHALL then be used.

#### Scenario: Replay after approval
- **WHEN** the approval for a denied call is recorded
- **AND** the client replays the identical call in a new session, with a newly signed token for the same `sub`
- **THEN** the upstream server receives the call once
- **AND** the arguments that the upstream server receives equal the approved arguments
- **AND** the client receives the upstream result

#### Scenario: Second replay of a used approval
- **WHEN** an approved call has passed once
- **AND** the client replays the identical call again
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no further call

### Requirement: An approval is bound to the arguments
An approval SHALL NOT pass a call whose arguments differ from the arguments of the denied call.

#### Scenario: Replay with changed arguments
- **WHEN** the approval for a denied call is recorded
- **AND** the client calls the same tool with different arguments
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no call

### Requirement: An approval is bound to the caller
An approval SHALL NOT pass a call from a caller other than the caller of the denied call.

#### Scenario: Replay by a different caller
- **WHEN** the approval for a denied call is recorded
- **AND** a client with a valid token for a different `sub` calls the same tool with the same arguments
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no call

### Requirement: The gate receives the call and the caller identity
For each `tools/call`, the gate SHALL receive the tool name, the arguments and at least one identity claim. The claim SHALL come from a token that agentgateway has validated for that request. The gate SHALL NOT receive the token.

#### Scenario: Call with a valid token
- **WHEN** a client with a valid token calls a tool
- **THEN** the gate receives the tool name and the arguments
- **AND** the gate receives the `sub` claim of the token
- **AND** the gate receives no `authorization` header

#### Scenario: Call with a token that fails validation
- **WHEN** a client opens a session with a valid token
- **AND** the client then sends a `tools/call` in that session with a token signed by an unknown key
- **THEN** agentgateway rejects the request
- **AND** the gate receives no request for that call
- **AND** the upstream server receives no call

#### Scenario: Call with no token
- **WHEN** a client opens a session with a valid token
- **AND** the client then sends a `tools/call` in that session with no token
- **THEN** agentgateway rejects the request
- **AND** the gate receives no request for that call
- **AND** the upstream server receives no call

### Requirement: The gate receives the result of a passed call
When a `tools/call` passes and the upstream MCP server returns a result, the gate SHALL receive that result.

#### Scenario: Result of a passed call
- **WHEN** a call passes the gate and the upstream server returns a result
- **THEN** the gate receives a response check that contains that result

### Requirement: Calls fail closed when the gate is unavailable
When the gate is unavailable, a `tools/call` SHALL NOT reach the upstream MCP server. The client SHALL receive a JSON-RPC error, not a tool result.

#### Scenario: Gate unavailable
- **WHEN** no gate instance is running
- **AND** a client with a valid token calls the gated tool
- **THEN** the client receives a JSON-RPC error
- **AND** the upstream server receives no call

### Requirement: The contract is tested against one pinned agentgateway version
The repository SHALL pin agentgateway and the `ExtMcp` bindings to one exact version. One command SHALL create a `kind` cluster, deploy that version with the test services, run every scenario in this spec, and exit 0 only when all of them pass. The command SHALL exit non-zero when no scenario ran. The same command SHALL run in CI.

#### Scenario: Local run
- **WHEN** a developer with the toolchain installed runs `make e2e` from the repo root
- **THEN** the command exits 0 and reports each scenario as passed

#### Scenario: CI run
- **WHEN** a pull request is opened or `main` is updated
- **THEN** a GitHub Actions job runs the same command and must pass

### Requirement: The gate callout has a deadline
The gateway configuration SHALL set a deadline on each call to the gate. When the gate does not answer within that deadline, the call SHALL fail closed: the client SHALL receive a JSON-RPC error within the deadline plus 2 seconds, and the upstream MCP server SHALL receive no call.

#### Scenario: Gate slower than the deadline
- **WHEN** the deadline is 5 seconds
- **AND** the gate delays its answer by 10 seconds
- **AND** a client with a valid token calls the gated tool
- **THEN** the client receives a JSON-RPC error within 7 seconds
- **AND** the upstream server receives no call
