// Package postgres is the Postgres Store of the gate: approval requests, the
// audit hash chain and checkpoints. Each write takes one advisory lock in its
// transaction, which gives the audit log one order.
package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
)

//go:embed migrations/*.sql
var migrations embed.FS

// auditLock is the advisory lock key of the audit chain.
const auditLock int64 = 0x6167656e74676174 // "agentgat"

// DefaultCheckpointEvery is the default checkpoint interval.
const DefaultCheckpointEvery = 100

// Store implements approval.Store on Postgres.
type Store struct {
	pool            *pgxpool.Pool
	logID           string
	checkpointEvery int64
}

var _ approval.Store = (*Store)(nil)

// Open connects, runs the migrations, and returns the Store.
func Open(ctx context.Context, dsn, logID string, checkpointEvery int) (*Store, error) {
	if checkpointEvery <= 0 {
		checkpointEvery = DefaultCheckpointEvery
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	s := &Store{pool: pool, logID: logID, checkpointEvery: int64(checkpointEvery)}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the pool.
func (s *Store) Close() { s.pool.Close() }

func (s *Store) migrate(ctx context.Context) error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, auditLock); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY)`); err != nil {
			return err
		}
		for _, name := range names {
			var done bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&done); err != nil {
				return err
			}
			if done {
				continue
			}
			sql, err := migrations.ReadFile(name)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("postgres: migration %s: %w", name, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
				return err
			}
		}
		return nil
	})
}

// locked runs fn in a transaction that holds the audit lock.
func (s *Store) locked(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, auditLock); err != nil {
			return err
		}
		return fn(tx)
	})
}

// appendEntry links e to the head, inserts it, and writes a checkpoint when
// its seq is a multiple of the interval. The caller holds the audit lock.
func (s *Store) appendEntry(ctx context.Context, tx pgx.Tx, e audit.Entry) error {
	head := audit.Head{}
	err := tx.QueryRow(ctx, `SELECT seq, entry_hash FROM audit_entries ORDER BY seq DESC LIMIT 1`).Scan(&head.Seq, &head.Hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	h, err := audit.Link(&e, head)
	if err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_entries (seq, entry, entry_hash) VALUES ($1, $2, $3)`, e.Seq, b, h); err != nil {
		return err
	}
	if e.Seq%s.checkpointEvery != 0 {
		return nil
	}
	prev := audit.Head{}
	err = tx.QueryRow(ctx, `SELECT seq, checkpoint->>'head_hash' FROM audit_checkpoints ORDER BY seq DESC LIMIT 1`).Scan(&prev.Seq, &prev.Hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	c, ch, err := audit.NewCheckpoint(s.logID, audit.Head{Seq: e.Seq, Hash: h}, prev, time.Now())
	if err != nil {
		return err
	}
	cb, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_checkpoints (seq, checkpoint, checkpoint_hash) VALUES ($1, $2, $3)`, c.Seq, cb, ch)
	return err
}

// Append implements approval.Store.
func (s *Store) Append(ctx context.Context, e audit.Entry) error {
	return s.locked(ctx, func(tx pgx.Tx) error { return s.appendEntry(ctx, tx, e) })
}

const requestColumns = `ref, call_digest, tenant, agent, target, tool, arguments, status,
	created_at, expires_at, coalesce(approver, ''), decided_at, used_at`

func scanRequest(row pgx.Row) (approval.Request, error) {
	var r approval.Request
	var args, status string
	var decided, used *time.Time
	err := row.Scan(&r.Ref, &r.CallDigest, &r.Tenant, &r.Agent, &r.Target, &r.Tool, &args, &status,
		&r.CreatedAt, &r.ExpiresAt, &r.Approver, &decided, &used)
	if err != nil {
		return r, err
	}
	r.Arguments, r.Status = []byte(args), approval.Status(status)
	if decided != nil {
		r.DecidedAt = *decided
	}
	if used != nil {
		r.UsedAt = *used
	}
	return r, nil
}

// UseOrRequest implements approval.Store.
func (s *Store) UseOrRequest(ctx context.Context, req approval.Request, now time.Time, used, required func(approval.Request) audit.Entry) (approval.Request, bool, error) {
	var out approval.Request
	var passed bool
	err := s.locked(ctx, func(tx pgx.Tx) error {
		// Consume once: one row affected means pass.
		r, err := scanRequest(tx.QueryRow(ctx, `UPDATE approval_requests SET status = 'used', used_at = $2
			WHERE ref = (SELECT ref FROM approval_requests
				WHERE call_digest = $1 AND status = 'approved' AND expires_at > $2
				ORDER BY created_at LIMIT 1)
			AND status = 'approved' AND expires_at > $2
			RETURNING `+requestColumns, req.CallDigest, now))
		if err == nil {
			out, passed = r, true
			return s.appendEntry(ctx, tx, used(r))
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		r, err = scanRequest(tx.QueryRow(ctx, `SELECT `+requestColumns+` FROM approval_requests
			WHERE call_digest = $1 AND status = 'pending' AND expires_at > $2
			ORDER BY created_at LIMIT 1`, req.CallDigest, now))
		if err == nil {
			out = r
			return s.appendEntry(ctx, tx, required(r))
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO approval_requests
			(ref, call_digest, tenant, agent, target, tool, arguments, status, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $9)`,
			req.Ref, req.CallDigest, req.Tenant, req.Agent, req.Target, req.Tool, string(req.Arguments),
			req.CreatedAt, req.ExpiresAt); err != nil {
			return err
		}
		out = req
		out.Status = approval.Pending
		return s.appendEntry(ctx, tx, required(out))
	})
	if err != nil {
		return approval.Request{}, false, fmt.Errorf("postgres: %w", err)
	}
	return out, passed, nil
}

// Decide implements approval.Store.
func (s *Store) Decide(ctx context.Context, ref, approver string, approve bool, now time.Time, decided func(*approval.Request, error) audit.Entry) (approval.Request, error) {
	var out approval.Request
	var refused error
	err := s.locked(ctx, func(tx pgx.Tx) error {
		var rp *approval.Request
		r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestColumns+` FROM approval_requests WHERE ref = $1 FOR UPDATE`, ref))
		switch {
		case err == nil:
			rp = &r
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		if refused = approval.CheckDecision(rp, approver, now); refused != nil {
			return s.appendEntry(ctx, tx, decided(rp, refused))
		}
		status := approval.Rejected
		if approve {
			status = approval.Approved
		}
		if _, err := tx.Exec(ctx, `UPDATE approval_requests SET status = $2, approver = $3, decided_at = $4 WHERE ref = $1`,
			ref, string(status), approver, now); err != nil {
			return err
		}
		r.Status, r.Approver, r.DecidedAt = status, approver, now
		out = r
		return s.appendEntry(ctx, tx, decided(&r, nil))
	})
	if err != nil {
		return approval.Request{}, fmt.Errorf("postgres: %w", err)
	}
	if refused != nil {
		return approval.Request{}, refused
	}
	return out, nil
}

// Export reads the audit log in seq order.
func (s *Store) Export(ctx context.Context) (audit.Export, error) {
	return Export(ctx, s.pool)
}

// Querier is a pgx connection or pool.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Export reads the audit log in seq order from q.
func Export(ctx context.Context, q Querier) (audit.Export, error) {
	x := audit.Export{Entries: []audit.HashedEntry{}, Checkpoints: []audit.HashedCheckpoint{}}
	rows, err := q.Query(ctx, `SELECT entry, entry_hash FROM audit_entries ORDER BY seq`)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		var b []byte
		var he audit.HashedEntry
		if err := rows.Scan(&b, &he.Hash); err != nil {
			return x, err
		}
		if err := json.Unmarshal(b, &he.Entry); err != nil {
			return x, err
		}
		x.Entries = append(x.Entries, he)
	}
	if err := rows.Err(); err != nil {
		return x, err
	}
	rows, err = q.Query(ctx, `SELECT checkpoint, checkpoint_hash FROM audit_checkpoints ORDER BY seq`)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		var b []byte
		var hc audit.HashedCheckpoint
		if err := rows.Scan(&b, &hc.Hash); err != nil {
			return x, err
		}
		if err := json.Unmarshal(b, &hc.Checkpoint); err != nil {
			return x, err
		}
		x.Checkpoints = append(x.Checkpoints, hc)
	}
	return x, rows.Err()
}
