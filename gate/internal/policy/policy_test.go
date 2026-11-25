package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPolicy = `package agentgate

decision := {"outcome": "allow", "reason": "echo is safe"} if input.tool == "echo"

decision := {"outcome": "deny", "reason": "no"} if input.tool == "rm"

decision := {"outcome": "approve", "reason": concat("", ["ask ", input.claims.sub])} if input.tool == "delete_thing"

decision := {"outcome": "maybe", "reason": "bad"} if input.tool == "invalid"

decision := "a string" if input.tool == "not_object"
`

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func load(t *testing.T, files map[string]string) *OPA {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	o, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOutcomes(t *testing.T) {
	o := load(t, map[string]string{"p.rego": testPolicy})
	ctx := context.Background()
	for _, tc := range []struct {
		tool   string
		want   Outcome
		reason string
	}{
		{"echo", Allow, "echo is safe"},
		{"rm", Deny, "no"},
		{"delete_thing", Approve, "ask agent-a"},
		{"unknown", Deny, "policy defines no decision"},
		{"invalid", Deny, `policy returned invalid outcome "maybe"`},
		{"not_object", Deny, "policy decision is not an object"},
	} {
		got := o.Decide(ctx, Input{Tool: tc.tool, Claims: map[string]any{"sub": "agent-a"}})
		if got.Outcome != tc.want || got.Reason != tc.reason {
			t.Errorf("%s: got %+v, want %s %q", tc.tool, got, tc.want, tc.reason)
		}
	}
}

func TestCompileError(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"good.rego": testPolicy, "broken.rego": "package agentgate\n\ndecision := {"})
	_, err := Load(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "broken.rego") {
		t.Fatalf("want an error naming broken.rego, got %v", err)
	}
}

func TestNoFiles(t *testing.T) {
	if _, err := Load(context.Background(), t.TempDir()); err == nil {
		t.Fatal("want an error for an empty directory")
	}
}

func TestVersionChanges(t *testing.T) {
	a := load(t, map[string]string{"p.rego": testPolicy})
	b := load(t, map[string]string{"p.rego": testPolicy + "\n# changed\n"})
	c := load(t, map[string]string{"q.rego": testPolicy})
	if a.Version() == b.Version() || a.Version() == c.Version() {
		t.Fatal("version does not change with content or name")
	}
	if !strings.HasPrefix(a.Version(), "sha256:") {
		t.Fatalf("version %q", a.Version())
	}
}

// TestConfigMapDirectory builds the layout of a ConfigMap volume:
//
//	..2026_11_25_10_00_00.123/p.rego
//	..data -> ..2026_11_25_10_00_00.123
//	p.rego -> ..data/p.rego
func TestConfigMapDirectory(t *testing.T) {
	plain := load(t, map[string]string{"p.rego": testPolicy, "README.md": "x"})

	dir := t.TempDir()
	ts := "..2026_11_25_10_00_00.123"
	if err := os.Mkdir(filepath.Join(dir, ts), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, filepath.Join(dir, ts), map[string]string{"p.rego": testPolicy})
	if err := os.Symlink(ts, filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/p.rego", filepath.Join(dir, "p.rego")); err != nil {
		t.Fatal(err)
	}
	// A loaded twice file would be a Rego conflict: two complete definitions.
	cm, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if cm.Version() != plain.Version() {
		t.Fatalf("ConfigMap version %s != plain %s", cm.Version(), plain.Version())
	}
	if got := cm.Decide(context.Background(), Input{Tool: "echo"}); got.Outcome != Allow {
		t.Fatalf("got %+v", got)
	}
}
