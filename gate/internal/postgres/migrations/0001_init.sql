CREATE TABLE approval_requests (
    ref         text PRIMARY KEY,
    call_digest text NOT NULL,
    tenant      text NOT NULL,
    agent       text NOT NULL,
    target      text NOT NULL,
    tool        text NOT NULL,
    arguments   text NOT NULL,
    status      text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'used')),
    created_at  timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    approver    text,
    decided_at  timestamptz,
    used_at     timestamptz
);

CREATE INDEX approval_requests_digest ON approval_requests (call_digest, status);

CREATE TABLE audit_entries (
    seq        bigint PRIMARY KEY,
    entry      jsonb NOT NULL,
    entry_hash text NOT NULL
);

CREATE TABLE audit_checkpoints (
    seq             bigint PRIMARY KEY,
    checkpoint      jsonb NOT NULL,
    checkpoint_hash text NOT NULL
);
