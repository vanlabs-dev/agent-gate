package approval

import (
	"context"
	"sync"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
)

// MemoryStore is a Store in memory, for tests. One mutex stands in for the
// database transaction and the audit lock.
type MemoryStore struct {
	mu       sync.Mutex
	requests []*Request
	entries  []audit.HashedEntry
	head     audit.Head
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (m *MemoryStore) append(e audit.Entry) error {
	h, err := audit.Link(&e, m.head)
	if err != nil {
		return err
	}
	m.entries = append(m.entries, audit.HashedEntry{Entry: e, Hash: h})
	m.head = audit.Head{Seq: e.Seq, Hash: h}
	return nil
}

// Append implements Store.
func (m *MemoryStore) Append(_ context.Context, e audit.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.append(e)
}

// UseOrRequest implements Store.
func (m *MemoryStore) UseOrRequest(_ context.Context, req Request, now time.Time, used, required func(Request) audit.Entry) (Request, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.requests {
		if r.CallDigest == req.CallDigest && r.Status == Approved && now.Before(r.ExpiresAt) {
			r.Status, r.UsedAt = Used, now
			return *r, true, m.append(used(*r))
		}
	}
	for _, r := range m.requests {
		if r.CallDigest == req.CallDigest && r.Status == Pending && now.Before(r.ExpiresAt) {
			return *r, false, m.append(required(*r))
		}
	}
	r := req
	m.requests = append(m.requests, &r)
	return r, false, m.append(required(r))
}

// Decide implements Store.
func (m *MemoryStore) Decide(_ context.Context, ref, approver string, approve bool, now time.Time, decided func(*Request, error) audit.Entry) (Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var r *Request
	for _, x := range m.requests {
		if x.Ref == ref {
			r = x
		}
	}
	if err := CheckDecision(r, approver, now); err != nil {
		if aerr := m.append(decided(r, err)); aerr != nil {
			return Request{}, aerr
		}
		return Request{}, err
	}
	r.Status = Rejected
	if approve {
		r.Status = Approved
	}
	r.Approver, r.DecidedAt = approver, now
	return *r, m.append(decided(r, nil))
}

// Requests returns a copy of the stored requests.
func (m *MemoryStore) Requests() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Request, len(m.requests))
	for i, r := range m.requests {
		out[i] = *r
	}
	return out
}

// Export returns the audit log.
func (m *MemoryStore) Export() audit.Export {
	m.mu.Lock()
	defer m.mu.Unlock()
	return audit.Export{Entries: append([]audit.HashedEntry{}, m.entries...), Checkpoints: []audit.HashedCheckpoint{}}
}
