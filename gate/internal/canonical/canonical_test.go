package canonical

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The test vectors are from github.com/cyberphone/json-canonicalization
// (testdata/), the reference implementation of RFC 8785.
func TestRFC8785Vectors(t *testing.T) {
	files, err := filepath.Glob("testdata/input/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no test vectors: %v", err)
	}
	for _, in := range files {
		t.Run(filepath.Base(in), func(t *testing.T) {
			input, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata/output", filepath.Base(in)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Arguments(input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, bytes.TrimRight(want, "\n")) {
				t.Errorf("got  %s\nwant %s", got, want)
			}
		})
	}
}

func TestKeyOrderAndWhiteSpace(t *testing.T) {
	a, err := Arguments([]byte(`{"b": 1, "a": {"y": [1, 2], "x": "v"}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Arguments([]byte("{\n\t\"a\":{\"x\":\"v\",\"y\":[1,2]},\"b\":1}"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("%s != %s", a, b)
	}
}

func TestNumbers(t *testing.T) {
	for _, tc := range []struct {
		in     string
		unsafe bool
	}{
		{`{"n": 9007199254740993}`, true},
		{`{"n": -9007199254740993}`, true},
		{`{"n": 9007199254740991}`, false},
		{`{"n": -9007199254740991}`, false},
		{`[1e400]`, true},
		{`[-1e400]`, true},
		{`{"n": 0.1}`, false},
		{`{"n": 1e30}`, false},
		{`{"n": 1.5e-300}`, false},
	} {
		_, err := Arguments([]byte(tc.in))
		if got := errors.Is(err, ErrUnsafeNumber); got != tc.unsafe {
			t.Errorf("%s: unsafe = %v, want %v (err %v)", tc.in, got, tc.unsafe, err)
		}
		if !tc.unsafe && err != nil {
			t.Errorf("%s: %v", tc.in, err)
		}
	}
}

func TestAbsentArguments(t *testing.T) {
	got, err := Arguments(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "null" {
		t.Fatalf("got %s", got)
	}
	if ArgsDigest(got) != Digest([]byte("null")) {
		t.Fatal("absent arguments do not hash as null")
	}
	_, args, err := Params([]byte(`{"name":"echo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Arguments(args); string(got) != "null" {
		t.Fatalf("got %s", got)
	}
}

func digestOf(t *testing.T, params string, tenant, agent, target string) string {
	t.Helper()
	tool, raw, err := Params([]byte(params))
	if err != nil {
		t.Fatal(err)
	}
	args, err := Arguments(raw)
	if err != nil {
		t.Fatal(err)
	}
	d, err := CallDigest(Call{tenant, agent, target, tool, args})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCallDigest(t *testing.T) {
	base := digestOf(t, `{"name":"delete_thing","arguments":{"id":"x"}}`, "t", "a", "svc")
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(base) {
		t.Fatalf("bad format %q", base)
	}
	same := map[string]string{
		"_meta changed": digestOf(t, `{"name":"delete_thing","arguments":{"id":"x"},"_meta":{"progressToken":7}}`, "t", "a", "svc"),
		"white space":   digestOf(t, `{ "arguments" : { "id" : "x" }, "name" : "delete_thing" }`, "t", "a", "svc"),
	}
	for name, d := range same {
		if d != base {
			t.Errorf("%s: digest changed", name)
		}
	}
	differ := map[string]string{
		"tenant":    digestOf(t, `{"name":"delete_thing","arguments":{"id":"x"}}`, "t2", "a", "svc"),
		"agent":     digestOf(t, `{"name":"delete_thing","arguments":{"id":"x"}}`, "t", "b", "svc"),
		"target":    digestOf(t, `{"name":"delete_thing","arguments":{"id":"x"}}`, "t", "a", "svc2"),
		"tool":      digestOf(t, `{"name":"delete_other","arguments":{"id":"x"}}`, "t", "a", "svc"),
		"arguments": digestOf(t, `{"name":"delete_thing","arguments":{"id":"y"}}`, "t", "a", "svc"),
		"absent":    digestOf(t, `{"name":"delete_thing"}`, "t", "a", "svc"),
	}
	for name, d := range differ {
		if d == base {
			t.Errorf("changed %s: same digest", name)
		}
	}
}
