# tool-call-policy Specification

## Purpose

Defines how the gate decides each MCP tool call: the caller identity it requires, what policy reads, the three outcomes, and how the gate fails closed when it cannot decide.

## Requirements

### Requirement: Each call has a caller identity
The gate SHALL take the caller from one identity claim. Configuration SHALL name that claim as the metadata key that the gateway forwards, with `sub` as the default. The gate SHALL take the tenant from a configured claim, or from a configured fixed value when no tenant claim is set. The gate SHALL deny a call that has no caller value or no tenant value, before policy runs.

#### Scenario: Call with no identity claim
- **WHEN** a `tools/call` reaches the gate with no value for the caller claim
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no call
- **AND** no approval request is stored

#### Scenario: Caller claim set by configuration
- **WHEN** the gate is configured with the caller claim `oid`
- **AND** a call carries `sub` and `oid` with different values
- **THEN** the gate uses the `oid` value as the caller

### Requirement: Policy decides one of three outcomes
For each `tools/call` with a caller, the gate SHALL evaluate policy and get exactly one outcome: allow, deny or needs approval, each with a reason. Policy input SHALL hold the tenant, the caller, the target, the tool name, the arguments and the identity claims that the gateway forwards as metadata.

#### Scenario: Allowed call
- **WHEN** policy returns allow for a call
- **THEN** the upstream server receives the call once
- **AND** the client receives the upstream result

#### Scenario: Denied call
- **WHEN** policy returns deny for a call
- **THEN** the client receives a tool result with `isError` true that contains the policy reason
- **AND** the upstream server receives no call

#### Scenario: Call that needs approval
- **WHEN** policy returns needs approval for a call
- **THEN** the client receives a tool result with `isError` true that contains an approval reference
- **AND** the upstream server receives no call

### Requirement: The gate fails closed when policy cannot decide
The gate SHALL deny a call when policy evaluation fails, returns no outcome, or returns a value that is not one of the three outcomes. The gate SHALL NOT start when a policy file fails to load or compile.

#### Scenario: Policy returns no outcome
- **WHEN** policy defines no outcome for a call
- **THEN** the client receives a tool result with `isError` true
- **AND** the upstream server receives no call

#### Scenario: Policy file with a syntax error
- **WHEN** the gate starts with a policy file that does not compile
- **THEN** the gate exits non-zero and names the file

### Requirement: Policy loads from a directory
At start the gate SHALL load each top-level file with the suffix `.rego` from one configured directory. It SHALL follow symlinks and skip names that start with `.`. Each decision SHALL carry a policy version: a digest of the base names and contents of the loaded files.

#### Scenario: Policy version in the record
- **WHEN** the gate decides a call
- **THEN** the audit entry for that decision holds the policy version

#### Scenario: Changed policy file
- **WHEN** one policy file changes and the gate restarts
- **THEN** later audit entries hold a different policy version

#### Scenario: Directory mounted from a ConfigMap
- **WHEN** the directory is a ConfigMap volume with its `..data` link and timestamped directory
- **THEN** each policy file loads once
- **AND** the policy version equals that of a plain directory with the same files

### Requirement: Calls other than tool calls pass
The gate SHALL pass each MCP method other than `tools/call` without a policy decision.

#### Scenario: Tool list
- **WHEN** a client sends `tools/list`
- **THEN** the client receives the upstream tool list
