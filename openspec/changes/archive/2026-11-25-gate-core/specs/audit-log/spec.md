# Spec Delta

## Purpose

Defines the audit record of the gate: what each entry holds, how entries chain by hash, what a checkpoint holds, and how the verifier CLI proves that no entry was changed, removed or reordered.

## ADDED Requirements

### Requirement: Each decision appends an audit entry
The gate SHALL append one entry for each policy decision, each stored request, each approval, rejection and refused decision, each passed approval, and each upstream result that the gate receives. An entry SHALL hold digests of arguments and results, not the values.

#### Scenario: Call that needs approval, then passes
- **WHEN** a call needs approval, is approved and is replayed
- **THEN** the log holds, in order, entries for the stored request, the approval, the passed approval and the result

#### Scenario: Arguments are not in the log
- **WHEN** a call with the argument value `secret-value` is decided
- **THEN** no audit entry contains `secret-value`

### Requirement: The entry format is fixed and versioned
Each entry SHALL hold exactly these fields: `schema`, `seq`, `prev_hash`, `time`, `event`, `tenant`, `agent`, `target`, `tool`, `args_digest`, `call_digest`, `result_digest`, `approval_ref`, `approver`, `policy_version`, `reason`. A field with no value SHALL be `null`. `schema` SHALL be `agent-gate.audit.entry/v1`.

#### Scenario: Entry fields
- **WHEN** an entry is exported
- **THEN** it has each listed field and no other field besides its hash

### Requirement: Entries form one hash chain
Entries SHALL have one order, with `seq` starting at 1 and rising by 1. The hash of an entry SHALL be SHA-256 over the bytes `agent-gate.audit/v1`, a newline, and the RFC 8785 encoding of the entry. `prev_hash` SHALL be the hash of the entry before it, and `sha256:` with 64 zeros for the first entry. Concurrent decisions SHALL NOT produce two entries with the same `seq`.

#### Scenario: Concurrent appends
- **WHEN** many calls are decided at the same time
- **THEN** the exported chain verifies with no gap and no repeated `seq`

### Requirement: The gate writes checkpoints
After a configured number of new entries, the gate SHALL write a checkpoint. A checkpoint SHALL hold `schema`, `log_id`, `seq`, `head_hash`, `prev_seq`, `prev_head_hash`, `time` and a `signatures` list. The `signatures` list SHALL stay outside the hashed bytes of the checkpoint, so that signatures can be added with no format change. In this change the list SHALL be empty. `log_id` SHALL be a configured value, separate from the tenant.

#### Scenario: Checkpoint after N entries
- **WHEN** the checkpoint interval is 5 and 5 entries are appended
- **THEN** one checkpoint exists with `seq` 5 and the hash of entry 5 as `head_hash`

### Requirement: The verifier CLI checks an exported log
`agent-gate-audit export` SHALL write the entries and checkpoints of a log to a file. `agent-gate-audit verify` SHALL read that file, recompute each hash, check `seq` and `prev_hash`, and check each checkpoint against the entry it names. It SHALL exit 0 only when every check passes, and SHALL name the first failed `seq` otherwise. It SHALL report the entries after the last checkpoint as not anchored.

#### Scenario: Untouched log
- **WHEN** the verifier runs on an exported log
- **THEN** it exits 0

#### Scenario: Changed entry
- **WHEN** one field of one entry is changed in the export
- **THEN** the verifier exits non-zero and names that entry's `seq`

#### Scenario: Removed entry
- **WHEN** one entry before the last checkpoint is removed from the export
- **THEN** the verifier exits non-zero

#### Scenario: Reordered entries
- **WHEN** two entries swap places in the export
- **THEN** the verifier exits non-zero

#### Scenario: Removed tail
- **WHEN** the entries after the last checkpoint are removed
- **THEN** the verifier exits 0 and reports the last checkpoint `seq` as the anchored head
