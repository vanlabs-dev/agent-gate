# Spec Delta

## Purpose

Defines the contract between agentgateway and a gate behind its `ExtMcp` extension: what the gate receives for an MCP tool call, how a deny reaches the client, and how a denied call passes once after approval. An end-to-end test on `kind` proves each requirement against one pinned agentgateway version.

## ADDED Requirements

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
After the approval for a denied call is recorded, a replay with the same caller, tool and arguments SHALL reach the upstream MCP server and return its result. The approval SHALL then be used.

#### Scenario: Replay after approval
- **WHEN** the approval for a denied call is recorded
- **AND** the client replays the identical call
- **THEN** the upstream server receives the call once
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

### Requirement: The gate receives the call and the caller identity
For each `tools/call`, the gate SHALL receive the tool name, the arguments and at least one identity claim. The claim SHALL come from a token that agentgateway has validated.

#### Scenario: Call with a valid token
- **WHEN** a client with a valid token calls a tool
- **THEN** the gate receives the tool name and the arguments
- **AND** the gate receives the `sub` claim of the token

#### Scenario: Call with a token that fails validation
- **WHEN** a client calls a tool with a token signed by an unknown key
- **THEN** agentgateway rejects the request
- **AND** the gate receives no request for that call

### Requirement: The gate receives the result of a passed call
When a `tools/call` passes and the upstream MCP server returns a result, the gate SHALL receive that result before the client does.

#### Scenario: Result of a passed call
- **WHEN** a call passes the gate and the upstream server returns a result
- **THEN** the gate receives a response check that contains that result

### Requirement: The contract is tested against one pinned agentgateway version
The repository SHALL pin agentgateway and the `ExtMcp` bindings to one exact version. One command SHALL create a `kind` cluster, deploy that version with the test services, run every scenario in this spec, and exit 0 only when all of them pass. The same command SHALL run in CI.

#### Scenario: Local run
- **WHEN** a developer with the toolchain installed runs `make test` from the repo root
- **THEN** the command exits 0 and reports each scenario as passed

#### Scenario: CI run
- **WHEN** a pull request is opened or `main` is updated
- **THEN** a GitHub Actions job runs the same command and must pass
