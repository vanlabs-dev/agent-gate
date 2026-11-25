// Package approval is the gate core. It decides each tool call with policy,
// holds a call that needs approval as a request bound to its call digest, and
// passes it once on identical replay after approval. It has no transport
// types.
package approval

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
	"github.com/vanlabs-dev/agent-gate/gate/internal/canonical"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
)

// DefaultExpiry is the time after which a request expires.
const DefaultExpiry = time.Hour

// Status is the state of a request.
type Status string

const (
	Pending  Status = "pending"
	Approved Status = "approved"
	Rejected Status = "rejected"
	Used     Status = "used"
)

// Request is a stored call that needs approval.
type Request struct {
	Ref        string
	CallDigest string
	Tenant     string
	Agent      string
	Target     string
	Tool       string
	// Arguments is the canonical JSON of the arguments.
	Arguments []byte
	Status    Status
	CreatedAt time.Time
	ExpiresAt time.Time
	Approver  string
	DecidedAt time.Time
	UsedAt    time.Time
}

// Errors of a decision on a request.
var (
	ErrNotFound     = errors.New("unknown approval reference")
	ErrSelfApproval = errors.New("the requester cannot decide its own request")
	ErrNotPending   = errors.New("request is not pending")
	ErrExpired      = errors.New("request has expired")
	ErrNoApprover   = errors.New("no approver identity")
)

// CheckDecision returns the reason a decision on req by approver at now is
// refused, or nil. Each Store calls it inside its transaction.
func CheckDecision(req *Request, approver string, now time.Time) error {
	switch {
	case req == nil:
		return ErrNotFound
	case approver == req.Agent:
		return ErrSelfApproval
	case req.Status != Pending:
		return ErrNotPending
	case !now.Before(req.ExpiresAt):
		return ErrExpired
	}
	return nil
}

// Store keeps requests and the audit log. Each method is one transaction that
// holds the audit lock, so the request change and its audit entry commit
// together.
type Store interface {
	// Append appends one audit entry.
	Append(ctx context.Context, e audit.Entry) error
	// UseOrRequest marks an approved, unused, unexpired request with
	// req.CallDigest as used and appends used(r), returning (r, true). Else it
	// finds a pending unexpired request with the digest, or inserts req,
	// appends required(r), and returns (r, false).
	UseOrRequest(ctx context.Context, req Request, now time.Time, used, required func(Request) audit.Entry) (Request, bool, error)
	// Decide approves or rejects the request ref after CheckDecision. It
	// appends decided(r, nil) on success, or decided(r, err) on a refusal and
	// returns err. r is nil for an unknown ref.
	Decide(ctx context.Context, ref, approver string, approve bool, now time.Time, decided func(r *Request, err error) audit.Entry) (Request, error)
}

// Call is one tool call as the gate sees it.
type Call struct {
	Tenant string
	Agent  string
	Target string
	Tool   string
	// Arguments is the raw JSON arguments, empty when absent.
	Arguments json.RawMessage
	// Claims holds the identity metadata keys that the gateway forwards.
	Claims map[string]any
}

// Verdict is the gate's answer for a call.
type Verdict struct {
	Pass   bool
	Reason string
	// Ref is set when the call needs approval.
	Ref string
}

// Gate is the decision core.
type Gate struct {
	Policy policy.Decider
	Store  Store
	// Expiry defaults to DefaultExpiry.
	Expiry time.Duration
	// Now defaults to time.Now.
	Now func() time.Time
}

func (g *Gate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *Gate) expiry() time.Duration {
	if g.Expiry > 0 {
		return g.Expiry
	}
	return DefaultExpiry
}

// ApprovalRequiredPrefix starts the reason of a call that needs approval.
const ApprovalRequiredPrefix = "approval required: ref="

// Check decides one tool call. An error means the gate could not decide; the
// caller must fail closed.
func (g *Gate) Check(ctx context.Context, c Call) (Verdict, error) {
	now := g.now()
	base := audit.Entry{
		Time:   audit.FormatTime(now),
		Tenant: audit.Str(c.Tenant),
		Agent:  audit.Str(c.Agent),
		Target: audit.Str(c.Target),
		Tool:   audit.Str(c.Tool),
	}
	deny := func(e audit.Entry, reason string) (Verdict, error) {
		e.Event = audit.EventDeny
		e.Reason = audit.Str(reason)
		if err := g.Store.Append(ctx, e); err != nil {
			return Verdict{}, err
		}
		return Verdict{Reason: reason}, nil
	}

	if c.Agent == "" || c.Tenant == "" {
		return deny(base, "no caller or tenant identity")
	}
	args, err := canonical.Arguments(c.Arguments)
	if err != nil {
		return deny(base, err.Error())
	}
	digest, err := canonical.CallDigest(canonical.Call{Tenant: c.Tenant, Agent: c.Agent, Target: c.Target, Tool: c.Tool, Arguments: args})
	if err != nil {
		return Verdict{}, err
	}
	base.ArgsDigest = audit.Str(canonical.ArgsDigest(args))
	base.CallDigest = audit.Str(digest)

	var decoded any
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return Verdict{}, err
	}
	d := g.Policy.Decide(ctx, policy.Input{
		Tenant: c.Tenant, Agent: c.Agent, Target: c.Target, Tool: c.Tool,
		Arguments: decoded, Claims: c.Claims,
	})
	base.PolicyVersion = audit.Str(g.Policy.Version())
	base.Reason = audit.Str(d.Reason)

	switch d.Outcome {
	case policy.Allow:
		e := base
		e.Event = audit.EventAllow
		if err := g.Store.Append(ctx, e); err != nil {
			return Verdict{}, err
		}
		return Verdict{Pass: true, Reason: d.Reason}, nil
	case policy.Approve:
	default:
		return deny(base, d.Reason)
	}

	ref, err := NewRef()
	if err != nil {
		return Verdict{}, err
	}
	withRef := func(event audit.Event) func(Request) audit.Entry {
		return func(r Request) audit.Entry {
			e := base
			e.Event = event
			e.ApprovalRef = audit.Str(r.Ref)
			e.Approver = audit.Str(r.Approver)
			return e
		}
	}
	r, used, err := g.Store.UseOrRequest(ctx, Request{
		Ref: ref, CallDigest: digest,
		Tenant: c.Tenant, Agent: c.Agent, Target: c.Target, Tool: c.Tool,
		Arguments: args, Status: Pending, CreatedAt: now, ExpiresAt: now.Add(g.expiry()),
	}, now, withRef(audit.EventApprovalUsed), withRef(audit.EventApprovalRequired))
	if err != nil {
		return Verdict{}, err
	}
	if used {
		return Verdict{Pass: true, Reason: d.Reason, Ref: r.Ref}, nil
	}
	return Verdict{Reason: ApprovalRequiredPrefix + r.Ref, Ref: r.Ref}, nil
}

// Result appends the audit entry for an upstream result. result is the raw
// result JSON; its digest is SHA-256 over those bytes.
func (g *Gate) Result(ctx context.Context, c Call, result []byte) error {
	return g.Store.Append(ctx, audit.Entry{
		Time:         audit.FormatTime(g.now()),
		Event:        audit.EventResult,
		Tenant:       audit.Str(c.Tenant),
		Agent:        audit.Str(c.Agent),
		Target:       audit.Str(c.Target),
		Tool:         audit.Str(c.Tool),
		ResultDigest: audit.Str(canonical.Digest(result)),
	})
}

// Decide approves or rejects a request. A refusal is recorded and returned.
func (g *Gate) Decide(ctx context.Context, ref, approver string, approve bool) (Request, error) {
	if approver == "" {
		return Request{}, ErrNoApprover
	}
	now := g.now()
	return g.Store.Decide(ctx, ref, approver, approve, now, func(r *Request, err error) audit.Entry {
		e := audit.Entry{
			Time:        audit.FormatTime(now),
			ApprovalRef: audit.Str(ref),
			Approver:    audit.Str(approver),
		}
		if r != nil {
			e.Tenant, e.Agent, e.Target, e.Tool = audit.Str(r.Tenant), audit.Str(r.Agent), audit.Str(r.Target), audit.Str(r.Tool)
			e.CallDigest = audit.Str(r.CallDigest)
			e.ArgsDigest = audit.Str(canonical.ArgsDigest(r.Arguments))
		}
		switch {
		case err != nil:
			e.Event = audit.EventDecisionRefused
			e.Reason = audit.Str(err.Error())
		case approve:
			e.Event = audit.EventApproved
		default:
			e.Event = audit.EventRejected
		}
		return e
	})
}

var refEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewRef returns "apr_" and 26 base32 characters over 128 random bits.
func NewRef() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("approval reference: %w", err)
	}
	return "apr_" + strings.ToLower(refEncoding.EncodeToString(b)), nil
}
