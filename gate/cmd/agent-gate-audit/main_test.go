package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
)

// fixture returns a 12-entry export with checkpoints at seq 5 and 10.
func fixture(t *testing.T) audit.Export {
	t.Helper()
	var x audit.Export
	head, cp := audit.Head{}, audit.Head{}
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= 12; i++ {
		e := audit.Entry{Time: audit.FormatTime(now), Event: audit.EventDeny, Tenant: audit.Str("local"),
			Agent: audit.Str("agent-a"), Tool: audit.Str("rm"), Reason: audit.Str("no")}
		h, err := audit.Link(&e, head)
		if err != nil {
			t.Fatal(err)
		}
		x.Entries = append(x.Entries, audit.HashedEntry{Entry: e, Hash: h})
		head = audit.Head{Seq: e.Seq, Hash: h}
		if i%5 == 0 {
			c, ch, err := audit.NewCheckpoint("local", head, cp, now)
			if err != nil {
				t.Fatal(err)
			}
			x.Checkpoints = append(x.Checkpoints, audit.HashedCheckpoint{Checkpoint: c, Hash: ch})
			cp = head
		}
	}
	return x
}

func write(t *testing.T, x audit.Export) string {
	t.Helper()
	b, err := json.Marshal(x)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerify(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tamper func(*audit.Export)
		code   int
		out    string
	}{
		{"untouched", func(*audit.Export) {}, 0, "anchored head seq 10"},
		{"changed", func(x *audit.Export) { x.Entries[6].Tool = audit.Str("echo") }, 1, "entry seq 7"},
		{"removed", func(x *audit.Export) { x.Entries = append(x.Entries[:3], x.Entries[4:]...) }, 1, "entry seq 4"},
		{"swapped", func(x *audit.Export) { x.Entries[1], x.Entries[2] = x.Entries[2], x.Entries[1] }, 1, "entry seq 2"},
		{"removed tail", func(x *audit.Export) { x.Entries = x.Entries[:10] }, 0, "anchored head seq 10"},
		{"changed checkpoint", func(x *audit.Export) { x.Checkpoints[0].HeadHash = audit.ZeroHash }, 1, "checkpoint seq 5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := fixture(t)
			tc.tamper(&x)
			var stdout, stderr bytes.Buffer
			code := run([]string{"verify", write(t, x)}, &stdout, &stderr)
			if code != tc.code {
				t.Fatalf("exit %d, want %d: %s%s", code, tc.code, stdout.String(), stderr.String())
			}
			if out := stdout.String() + stderr.String(); !strings.Contains(out, tc.out) {
				t.Fatalf("output %q does not contain %q", out, tc.out)
			}
		})
	}
}

func TestUnanchoredReported(t *testing.T) {
	var stdout bytes.Buffer
	if code := run([]string{"verify", write(t, fixture(t))}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "not anchored: 2 entries after seq 10") {
		t.Fatalf("output %q", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	if code := run(nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
		t.Fatalf("exit %d", code)
	}
}
