package approval

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
)

// clock is a settable test clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// switchable is a Decider whose outcome for delete_thing the test can change.
type switchable struct {
	inner policy.Decider
	deny  atomic.Bool
}

func (s *switchable) Decide(ctx context.Context, in policy.Input) policy.Decision {
	if s.deny.Load() && in.Tool == "delete_thing" {
		return policy.Decision{Outcome: policy.Deny, Reason: "now denied"}
	}
	return s.inner.Decide(ctx, in)
}
func (s *switchable) Version() string { return s.inner.Version() }

type fixture struct {
	gate   *Gate
	store  *MemoryStore
	clock  *clock
	policy *switchable
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	p, err := policy.Load(context.Background(), "../../deploy/policy")
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{store: NewMemoryStore(), clock: &clock{t: time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)}, policy: &switchable{inner: p}}
	f.gate = &Gate{Policy: f.policy, Store: f.store, Now: f.clock.Now}
	return f
}

func call(agent, tool, args string) Call {
	return Call{Tenant: "local", Agent: agent, Target: "stub-mcp", Tool: tool, Arguments: json.RawMessage(args),
		Claims: map[string]any{"sub": agent}}
}

func (f *fixture) check(t *testing.T, c Call) Verdict {
	t.Helper()
	v, err := f.gate.Check(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// needsApproval checks the call is held and returns its reference.
func (f *fixture) needsApproval(t *testing.T, c Call) string {
	t.Helper()
	v := f.check(t, c)
	if v.Pass || v.Ref == "" || !strings.Contains(v.Reason, ApprovalRequiredPrefix+v.Ref) {
		t.Fatalf("want approval required, got %+v", v)
	}
	return v.Ref
}

func (f *fixture) approve(t *testing.T, ref string) {
	t.Helper()
	if _, err := f.gate.Decide(context.Background(), ref, "approver", true); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) events() []audit.Event {
	var out []audit.Event
	for _, e := range f.store.Export().Entries {
		out = append(out, e.Event)
	}
	return out
}

func TestOutcomes(t *testing.T) {
	f := newFixture(t)
	if v := f.check(t, call("agent-a", "echo", `{"text":"hi"}`)); !v.Pass {
		t.Errorf("echo: %+v", v)
	}
	if v := f.check(t, call("agent-a", "delete_thing", `{"id":"prod"}`)); v.Pass || v.Reason != "delete_thing on prod is denied" {
		t.Errorf("delete prod: %+v", v)
	}
	if v := f.check(t, call("agent-a", "drop_table", `{}`)); v.Pass {
		t.Errorf("other tool: %+v", v)
	}
	f.needsApproval(t, call("agent-a", "delete_thing", `{"id":"x"}`))
}

func TestNoIdentity(t *testing.T) {
	f := newFixture(t)
	for _, c := range []Call{call("", "delete_thing", `{"id":"x"}`), {Agent: "a", Tool: "delete_thing"}} {
		if v := f.check(t, c); v.Pass {
			t.Errorf("%+v passed", c)
		}
	}
	if n := len(f.store.Requests()); n != 0 {
		t.Fatalf("%d requests stored", n)
	}
	if e := f.store.Export().Entries[0]; e.PolicyVersion != nil {
		t.Fatal("policy ran with no identity")
	}
}

func TestUndefinedPolicy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.rego"), []byte("package agentgate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	f.gate.Policy = p
	if v := f.check(t, call("agent-a", "echo", `{}`)); v.Pass {
		t.Fatalf("passed: %+v", v)
	}
}

func TestUnsafeNumber(t *testing.T) {
	f := newFixture(t)
	if v := f.check(t, call("agent-a", "delete_thing", `{"id":9007199254740993}`)); v.Pass || v.Ref != "" {
		t.Fatalf("got %+v", v)
	}
	if n := len(f.store.Requests()); n != 0 {
		t.Fatalf("%d requests stored", n)
	}
	f.needsApproval(t, call("agent-a", "delete_thing", `{"id":0.1}`))
}

func TestRepeatBeforeDecision(t *testing.T) {
	f := newFixture(t)
	a := f.needsApproval(t, call("agent-a", "delete_thing", `{"id":"x"}`))
	b := f.needsApproval(t, call("agent-a", "delete_thing", `{ "id" : "x" }`))
	if a != b {
		t.Fatalf("refs %s and %s", a, b)
	}
	if n := len(f.store.Requests()); n != 1 {
		t.Fatalf("%d requests stored", n)
	}
}

func TestReplayOnce(t *testing.T) {
	f := newFixture(t)
	c := call("agent-a", "delete_thing", `{"id":"x"}`)
	ref := f.needsApproval(t, c)
	f.approve(t, ref)

	if v := f.check(t, call("agent-a", "delete_thing", `{"id":"y"}`)); v.Pass {
		t.Fatal("changed arguments passed")
	}
	if v := f.check(t, call("agent-b", "delete_thing", `{"id":"x"}`)); v.Pass {
		t.Fatal("another caller passed")
	}
	if v := f.check(t, c); !v.Pass || v.Ref != ref {
		t.Fatalf("replay: %+v", v)
	}
	if v := f.check(t, c); v.Pass {
		t.Fatal("second replay passed")
	}
}

func TestConcurrentReplays(t *testing.T) {
	f := newFixture(t)
	c := call("agent-a", "delete_thing", `{"id":"x"}`)
	f.approve(t, f.needsApproval(t, c))
	var passed atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if v, err := f.gate.Check(context.Background(), c); err == nil && v.Pass {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 1 {
		t.Fatalf("%d replays passed", passed.Load())
	}
}

func TestPolicyChangedToDeny(t *testing.T) {
	f := newFixture(t)
	c := call("agent-a", "delete_thing", `{"id":"x"}`)
	ref := f.needsApproval(t, c)
	f.approve(t, ref)
	f.policy.deny.Store(true)
	if v := f.check(t, c); v.Pass || v.Reason != "now denied" {
		t.Fatalf("got %+v", v)
	}
	if r := f.store.Requests()[0]; r.Status != Approved {
		t.Fatalf("status %s", r.Status)
	}
}

func TestExpiry(t *testing.T) {
	f := newFixture(t)
	f.gate.Expiry = 30 * time.Second
	c := call("agent-a", "delete_thing", `{"id":"x"}`)
	ref := f.needsApproval(t, c)
	f.approve(t, ref)
	f.clock.Add(31 * time.Second)
	if v := f.check(t, c); v.Pass {
		t.Fatal("replay after expiry passed")
	}

	ref = f.needsApproval(t, call("agent-a", "delete_thing", `{"id":"z"}`))
	f.clock.Add(31 * time.Second)
	if _, err := f.gate.Decide(context.Background(), ref, "approver", true); !errors.Is(err, ErrExpired) {
		t.Fatalf("approval after expiry: %v", err)
	}
}

func TestDefaultExpiry(t *testing.T) {
	f := newFixture(t)
	f.needsApproval(t, call("agent-a", "delete_thing", `{"id":"x"}`))
	r := f.store.Requests()[0]
	if got := r.ExpiresAt.Sub(r.CreatedAt); got != time.Hour {
		t.Fatalf("expiry %s", got)
	}
}

func TestSelfApproval(t *testing.T) {
	f := newFixture(t)
	ref := f.needsApproval(t, call("agent-a", "delete_thing", `{"id":"x"}`))
	if _, err := f.gate.Decide(context.Background(), ref, "agent-a", true); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("got %v", err)
	}
	if r := f.store.Requests()[0]; r.Status != Pending {
		t.Fatalf("status %s", r.Status)
	}
	if _, err := f.gate.Decide(context.Background(), ref, "agent-b", true); err != nil {
		t.Fatalf("another identity: %v", err)
	}
	if r := f.store.Requests()[0]; r.Status != Approved || r.Approver != "agent-b" {
		t.Fatalf("got %+v", r)
	}
	if _, err := f.gate.Decide(context.Background(), ref, "approver", false); !errors.Is(err, ErrNotPending) {
		t.Fatalf("second decision: %v", err)
	}
	if _, err := f.gate.Decide(context.Background(), "apr_unknown", "approver", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown ref: %v", err)
	}
}

func TestRejection(t *testing.T) {
	f := newFixture(t)
	c := call("agent-a", "delete_thing", `{"id":"x"}`)
	ref := f.needsApproval(t, c)
	if _, err := f.gate.Decide(context.Background(), ref, "approver", false); err != nil {
		t.Fatal(err)
	}
	if again := f.needsApproval(t, c); again == ref {
		t.Fatal("replay after rejection returned the rejected ref")
	}
}

func TestAuditOrder(t *testing.T) {
	f := newFixture(t)
	c := call("agent-a", "delete_thing", `{"id":"secret-value"}`)
	f.approve(t, f.needsApproval(t, c))
	if v := f.check(t, c); !v.Pass {
		t.Fatal("replay did not pass")
	}
	if err := f.gate.Result(context.Background(), c, []byte(`{"content":[{"type":"text","text":"secret-value"}]}`)); err != nil {
		t.Fatal(err)
	}
	want := []audit.Event{audit.EventApprovalRequired, audit.EventApproved, audit.EventApprovalUsed, audit.EventResult}
	got := f.events()
	if len(got) != len(want) {
		t.Fatalf("events %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v, want %v", got, want)
		}
	}
	x := f.store.Export()
	b, err := json.Marshal(x)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-value") {
		t.Fatal("argument value in the audit log")
	}
	if _, err := audit.Verify(x); err != nil {
		t.Fatal(err)
	}
	if x.Entries[0].PolicyVersion == nil || *x.Entries[0].PolicyVersion != f.policy.Version() {
		t.Fatal("policy version not in the entry")
	}
	if x.Entries[1].Event == audit.EventDecisionRefused {
		t.Fatal("approval refused")
	}
}

func TestRefFormat(t *testing.T) {
	ref, err := NewRef()
	if err != nil {
		t.Fatal(err)
	}
	if len(ref) != 30 || !strings.HasPrefix(ref, "apr_") {
		t.Fatalf("ref %q", ref)
	}
}
