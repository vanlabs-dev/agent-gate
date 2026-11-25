// Package audit defines the audit entry and checkpoint formats, their hashes,
// and the verifier for an exported log.
package audit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/canonical"
)

const (
	EntrySchema      = "agent-gate.audit.entry/v1"
	CheckpointSchema = "agent-gate.audit.checkpoint/v1"

	entryDomain      = "agent-gate.audit/v1\n"
	checkpointDomain = "agent-gate.checkpoint/v1\n"
)

// ZeroHash is the prev_hash of the first entry and the prev_head_hash of the
// first checkpoint.
var ZeroHash = "sha256:" + strings.Repeat("0", 64)

// Event is the kind of an audit entry.
type Event string

const (
	EventAllow            Event = "allow"
	EventDeny             Event = "deny"
	EventApprovalRequired Event = "approval_required"
	EventApproved         Event = "approved"
	EventRejected         Event = "rejected"
	EventDecisionRefused  Event = "decision_refused"
	EventApprovalUsed     Event = "approval_used"
	EventResult           Event = "result"
)

// Entry is one audit record. Its JSON form is exactly the field list of the
// audit-log spec. A field with no value is null.
type Entry struct {
	Schema        string  `json:"schema"`
	Seq           int64   `json:"seq"`
	PrevHash      string  `json:"prev_hash"`
	Time          string  `json:"time"`
	Event         Event   `json:"event"`
	Tenant        *string `json:"tenant"`
	Agent         *string `json:"agent"`
	Target        *string `json:"target"`
	Tool          *string `json:"tool"`
	ArgsDigest    *string `json:"args_digest"`
	CallDigest    *string `json:"call_digest"`
	ResultDigest  *string `json:"result_digest"`
	ApprovalRef   *string `json:"approval_ref"`
	Approver      *string `json:"approver"`
	PolicyVersion *string `json:"policy_version"`
	Reason        *string `json:"reason"`
}

// Checkpoint anchors the chain head at one seq. Signatures stay outside the
// hashed bytes.
type Checkpoint struct {
	Schema       string      `json:"schema"`
	LogID        string      `json:"log_id"`
	Seq          int64       `json:"seq"`
	HeadHash     string      `json:"head_hash"`
	PrevSeq      int64       `json:"prev_seq"`
	PrevHeadHash string      `json:"prev_head_hash"`
	Time         string      `json:"time"`
	Signatures   []Signature `json:"signatures"`
}

// Signature is reserved for Phase 4. The list is empty in Phase 1.
type Signature struct {
	Alg   string `json:"alg"`
	KeyID string `json:"key_id"`
	Sig   string `json:"sig"`
}

// Str returns a pointer to s, or nil when s is empty.
func Str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// FormatTime writes t as RFC 3339 UTC with milliseconds.
func FormatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// Hash returns the entry hash: SHA-256 over the domain string and the RFC 8785
// encoding of the entry.
func Hash(e Entry) (string, error) {
	b, err := canonical.JCS(e)
	if err != nil {
		return "", err
	}
	return canonical.Digest(append([]byte(entryDomain), b...)), nil
}

// CheckpointHash returns the checkpoint hash, computed without signatures.
func CheckpointHash(c Checkpoint) (string, error) {
	b, err := canonical.JCS(struct {
		Schema       string `json:"schema"`
		LogID        string `json:"log_id"`
		Seq          int64  `json:"seq"`
		HeadHash     string `json:"head_hash"`
		PrevSeq      int64  `json:"prev_seq"`
		PrevHeadHash string `json:"prev_head_hash"`
		Time         string `json:"time"`
	}{c.Schema, c.LogID, c.Seq, c.HeadHash, c.PrevSeq, c.PrevHeadHash, c.Time})
	if err != nil {
		return "", err
	}
	return canonical.Digest(append([]byte(checkpointDomain), b...)), nil
}

// Head is the last entry of a chain. The zero value is an empty chain.
type Head struct {
	Seq  int64
	Hash string
}

// Link sets the chain fields of e after head and returns its hash.
func Link(e *Entry, head Head) (string, error) {
	e.Schema = EntrySchema
	e.Seq = head.Seq + 1
	e.PrevHash = head.Hash
	if head.Seq == 0 {
		e.PrevHash = ZeroHash
	}
	return Hash(*e)
}

// NewCheckpoint returns the checkpoint for head after prev, and its hash.
// prev is the zero Head when there is no earlier checkpoint.
func NewCheckpoint(logID string, head, prev Head, now time.Time) (Checkpoint, string, error) {
	c := Checkpoint{
		Schema:       CheckpointSchema,
		LogID:        logID,
		Seq:          head.Seq,
		HeadHash:     head.Hash,
		PrevSeq:      prev.Seq,
		PrevHeadHash: prev.Hash,
		Time:         FormatTime(now),
		Signatures:   []Signature{},
	}
	if prev.Seq == 0 {
		c.PrevHeadHash = ZeroHash
	}
	h, err := CheckpointHash(c)
	return c, h, err
}

// HashedEntry is an entry with its hash, as exported.
type HashedEntry struct {
	Entry
	Hash string `json:"hash"`
}

// HashedCheckpoint is a checkpoint with its hash, as exported.
type HashedCheckpoint struct {
	Checkpoint
	Hash string `json:"hash"`
}

// Export is the file that agent-gate-audit export writes.
type Export struct {
	Entries     []HashedEntry      `json:"entries"`
	Checkpoints []HashedCheckpoint `json:"checkpoints"`
}

// Report is the result of a passed verification.
type Report struct {
	Entries     int
	Checkpoints int
	// AnchoredHead is the seq of the last checkpoint, or 0 with no checkpoint.
	AnchoredHead int64
	// Unanchored is the number of entries after the last checkpoint.
	Unanchored int
}

// VerifyError names the first entry or checkpoint that failed.
type VerifyError struct {
	Checkpoint bool
	Seq        int64
	Msg        string
}

func (e *VerifyError) Error() string {
	kind := "entry"
	if e.Checkpoint {
		kind = "checkpoint"
	}
	return fmt.Sprintf("%s seq %d: %s", kind, e.Seq, e.Msg)
}

// Verify recomputes each hash, checks seq and prev_hash, and checks each
// checkpoint against the entry it names.
func Verify(x Export) (Report, error) {
	prev := Head{}
	hashes := map[int64]string{}
	for i, he := range x.Entries {
		want := prev.Seq + 1
		if he.Seq != want {
			return Report{}, &VerifyError{Seq: want, Msg: fmt.Sprintf("found seq %d at position %d", he.Seq, i+1)}
		}
		if he.Schema != EntrySchema {
			return Report{}, &VerifyError{Seq: he.Seq, Msg: fmt.Sprintf("schema %q", he.Schema)}
		}
		wantPrev := prev.Hash
		if prev.Seq == 0 {
			wantPrev = ZeroHash
		}
		if he.PrevHash != wantPrev {
			return Report{}, &VerifyError{Seq: he.Seq, Msg: "prev_hash does not match the entry before it"}
		}
		h, err := Hash(he.Entry)
		if err != nil {
			return Report{}, &VerifyError{Seq: he.Seq, Msg: err.Error()}
		}
		if h != he.Hash {
			return Report{}, &VerifyError{Seq: he.Seq, Msg: "hash does not match the entry"}
		}
		hashes[he.Seq] = h
		prev = Head{he.Seq, h}
	}

	cp := Head{}
	for _, hc := range x.Checkpoints {
		fail := func(msg string) (Report, error) {
			return Report{}, &VerifyError{Checkpoint: true, Seq: hc.Seq, Msg: msg}
		}
		if hc.Schema != CheckpointSchema {
			return fail(fmt.Sprintf("schema %q", hc.Schema))
		}
		h, err := CheckpointHash(hc.Checkpoint)
		if err != nil {
			return fail(err.Error())
		}
		if h != hc.Hash {
			return fail("hash does not match the checkpoint")
		}
		wantPrevHead := cp.Hash
		if cp.Seq == 0 {
			wantPrevHead = ZeroHash
		}
		if hc.PrevSeq != cp.Seq || hc.PrevHeadHash != wantPrevHead {
			return fail("prev_seq or prev_head_hash does not match the checkpoint before it")
		}
		if hc.Seq <= cp.Seq {
			return fail("seq does not rise")
		}
		eh, ok := hashes[hc.Seq]
		if !ok {
			return fail("names an entry that is not in the log")
		}
		if eh != hc.HeadHash {
			return fail("head_hash does not match the entry")
		}
		cp = Head{hc.Seq, hc.HeadHash}
	}
	return Report{
		Entries:      len(x.Entries),
		Checkpoints:  len(x.Checkpoints),
		AnchoredHead: cp.Seq,
		Unanchored:   int(prev.Seq - cp.Seq),
	}, nil
}

// Parse reads an export file. Unknown fields are an error, so that a field
// added to an entry cannot pass unhashed.
func Parse(b []byte) (Export, error) {
	var x Export
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&x); err != nil {
		return Export{}, fmt.Errorf("parse export: %w", err)
	}
	return x, nil
}
