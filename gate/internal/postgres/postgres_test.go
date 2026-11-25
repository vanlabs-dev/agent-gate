//go:build pgtest

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
)

// testDB is the database the tests own. It is never the gate's database.
const testDB = "agent_gate_pgtest"

// open recreates testDB on the server of AGENT_GATE_PGTEST_DSN and opens a
// Store on it.
func open(t *testing.T, every int) *Store {
	t.Helper()
	dsn := os.Getenv("AGENT_GATE_PGTEST_DSN")
	if dsn == "" {
		t.Fatal("AGENT_GATE_PGTEST_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	for _, sql := range []string{"DROP DATABASE IF EXISTS " + testDB + " WITH (FORCE)", "CREATE DATABASE " + testDB} {
		if _, err := admin.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + testDB
	s, err := Open(ctx, u.String(), "pgtest", every)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	var db string
	if err := s.pool.QueryRow(ctx, "SELECT current_database()").Scan(&db); err != nil || db != testDB {
		t.Fatalf("store is on database %q, want %s (%v)", db, testDB, err)
	}
	return s
}

func TestMigrateTwice(t *testing.T) {
	s := open(t, 5)
	if err := s.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func newGate(t *testing.T, s *Store) *approval.Gate {
	t.Helper()
	p, err := policy.Load(context.Background(), "../../deploy/policy")
	if err != nil {
		t.Fatal(err)
	}
	return &approval.Gate{Policy: p, Store: s}
}

func call(agent, args string) approval.Call {
	return approval.Call{Tenant: "local", Agent: agent, Target: "stub-mcp", Tool: "delete_thing", Arguments: json.RawMessage(args)}
}

func TestFlow(t *testing.T) {
	ctx := context.Background()
	s := open(t, 5)
	g := newGate(t, s)
	c := call("agent-a", `{"id":"x"}`)

	v, err := g.Check(ctx, c)
	if err != nil || v.Pass || v.Ref == "" {
		t.Fatalf("first call: %+v %v", v, err)
	}
	again, err := g.Check(ctx, c)
	if err != nil || again.Ref != v.Ref {
		t.Fatalf("repeat: %+v %v", again, err)
	}
	if _, err := g.Decide(ctx, v.Ref, "agent-a", true); !errors.Is(err, approval.ErrSelfApproval) {
		t.Fatalf("self-approval: %v", err)
	}
	r, err := g.Decide(ctx, v.Ref, "approver", true)
	if err != nil || r.Status != approval.Approved {
		t.Fatalf("approve: %+v %v", r, err)
	}
	if v, err := g.Check(ctx, call("agent-b", `{"id":"x"}`)); err != nil || v.Pass {
		t.Fatalf("another caller: %+v %v", v, err)
	}
	if v, err := g.Check(ctx, c); err != nil || !v.Pass {
		t.Fatalf("replay: %+v %v", v, err)
	}
	if v, err := g.Check(ctx, c); err != nil || v.Pass {
		t.Fatalf("second replay: %+v %v", v, err)
	}

	// Rejection, then a new reference.
	c2 := call("agent-a", `{"id":"y"}`)
	v2, _ := g.Check(ctx, c2)
	if _, err := g.Decide(ctx, v2.Ref, "approver", false); err != nil {
		t.Fatal(err)
	}
	if v3, _ := g.Check(ctx, c2); v3.Ref == v2.Ref || v3.Ref == "" {
		t.Fatalf("after rejection: %+v", v3)
	}

	// Expiry.
	g.Expiry = time.Second
	c3 := call("agent-a", `{"id":"z"}`)
	v4, _ := g.Check(ctx, c3)
	g.Now = func() time.Time { return time.Now().Add(2 * time.Second) }
	if _, err := g.Decide(ctx, v4.Ref, "approver", true); !errors.Is(err, approval.ErrExpired) {
		t.Fatalf("approval after expiry: %v", err)
	}

	x, err := s.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := audit.Verify(x); err != nil {
		t.Fatal(err)
	}
	if len(x.Checkpoints) == 0 || x.Checkpoints[0].Seq != 5 || x.Checkpoints[0].HeadHash != x.Entries[4].Hash {
		t.Fatalf("checkpoints %+v", x.Checkpoints)
	}
	if b, _ := json.Marshal(x); strings.Contains(string(b), `"id"`) {
		t.Fatal("arguments in the audit log")
	}
}

func TestConcurrentReplays(t *testing.T) {
	ctx := context.Background()
	s := open(t, 5)
	g := newGate(t, s)
	c := call("agent-a", `{"id":"x"}`)
	v, err := g.Check(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Decide(ctx, v.Ref, "approver", true); err != nil {
		t.Fatal(err)
	}
	var passed, failed atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			v, err := g.Check(ctx, c)
			switch {
			case err != nil:
				failed.Add(1)
			case v.Pass:
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 1 || failed.Load() != 0 {
		t.Fatalf("passed %d, errors %d", passed.Load(), failed.Load())
	}
}

func TestConcurrentAppends(t *testing.T) {
	ctx := context.Background()
	s := open(t, 5)
	var wg sync.WaitGroup
	var failed atomic.Int32
	for range 50 {
		wg.Go(func() {
			e := audit.Entry{Time: audit.FormatTime(time.Now()), Event: audit.EventAllow, Tool: audit.Str("echo")}
			if err := s.Append(ctx, e); err != nil {
				failed.Add(1)
			}
		})
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d appends failed", failed.Load())
	}
	x, err := s.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r, err := audit.Verify(x)
	if err != nil {
		t.Fatal(err)
	}
	if r.Entries != 50 || r.Checkpoints != 10 || r.AnchoredHead != 50 {
		t.Fatalf("report %+v", r)
	}
}
