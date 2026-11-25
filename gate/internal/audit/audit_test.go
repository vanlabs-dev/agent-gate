package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// build returns an export of n entries with a checkpoint every `every` entries.
func build(t *testing.T, n, every int) Export {
	t.Helper()
	var x Export
	head, cp := Head{}, Head{}
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= n; i++ {
		e := Entry{Time: FormatTime(now), Event: EventAllow, Tenant: Str("local"), Agent: Str("agent-a"),
			Tool: Str("echo"), PolicyVersion: Str("sha256:p"), Reason: Str("ok")}
		h, err := Link(&e, head)
		if err != nil {
			t.Fatal(err)
		}
		x.Entries = append(x.Entries, HashedEntry{e, h})
		head = Head{e.Seq, h}
		if i%every == 0 {
			c, ch, err := NewCheckpoint("local", head, cp, now)
			if err != nil {
				t.Fatal(err)
			}
			x.Checkpoints = append(x.Checkpoints, HashedCheckpoint{c, ch})
			cp = head
		}
	}
	return x
}

func wantFail(t *testing.T, x Export, seq int64) {
	t.Helper()
	_, err := Verify(x)
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("want a VerifyError, got %v", err)
	}
	if seq != 0 && ve.Seq != seq {
		t.Fatalf("named seq %d, want %d (%v)", ve.Seq, seq, err)
	}
}

func TestUntouched(t *testing.T) {
	r, err := Verify(build(t, 12, 5))
	if err != nil {
		t.Fatal(err)
	}
	if r.Entries != 12 || r.Checkpoints != 2 || r.AnchoredHead != 10 || r.Unanchored != 2 {
		t.Fatalf("report %+v", r)
	}
}

func TestEmpty(t *testing.T) {
	if _, err := Verify(Export{}); err != nil {
		t.Fatal(err)
	}
}

func TestChangedEntry(t *testing.T) {
	x := build(t, 12, 5)
	x.Entries[6].Reason = Str("changed")
	wantFail(t, x, 7)
}

func TestRemovedEntry(t *testing.T) {
	x := build(t, 12, 5)
	x.Entries = append(x.Entries[:3], x.Entries[4:]...)
	wantFail(t, x, 4)
}

func TestRemovedCheckpointedEntryAtEnd(t *testing.T) {
	x := build(t, 10, 5)
	x.Entries = x.Entries[:9] // entry 10 is the head of the last checkpoint
	wantFail(t, x, 10)
}

func TestSwappedEntries(t *testing.T) {
	x := build(t, 12, 5)
	x.Entries[2], x.Entries[3] = x.Entries[3], x.Entries[2]
	wantFail(t, x, 3)
}

func TestRemovedTail(t *testing.T) {
	x := build(t, 12, 5)
	x.Entries = x.Entries[:10]
	r, err := Verify(x)
	if err != nil {
		t.Fatal(err)
	}
	if r.AnchoredHead != 10 || r.Unanchored != 0 {
		t.Fatalf("report %+v", r)
	}
}

func TestChangedCheckpoint(t *testing.T) {
	x := build(t, 12, 5)
	x.Checkpoints[1].Time = "2030-01-01T00:00:00.000Z"
	wantFail(t, x, 10)
}

func TestSignaturesOutsideHash(t *testing.T) {
	x := build(t, 5, 5)
	x.Checkpoints[0].Signatures = []Signature{{Alg: "ed25519", KeyID: "k", Sig: "s"}}
	if _, err := Verify(x); err != nil {
		t.Fatal(err)
	}
}

func TestExportFields(t *testing.T) {
	b, err := json.Marshal(build(t, 1, 5).Entries[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := strings.Fields("schema seq prev_hash time event tenant agent target tool args_digest call_digest " +
		"result_digest approval_ref approver policy_version reason hash")
	if len(m) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(m), len(want), m)
	}
	for _, f := range want {
		if _, ok := m[f]; !ok {
			t.Errorf("missing field %s", f)
		}
	}
	if m["target"] != nil {
		t.Errorf("empty field is %v, want null", m["target"])
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	if _, err := Parse([]byte(`{"entries":[{"seq":1,"extra":1}],"checkpoints":[]}`)); err == nil {
		t.Fatal("want an error")
	}
}
