//go:build e2e

// Package e2e tests the gate end to end on the kind cluster of
// `make e2e-gate`: agentgateway, Dex, Postgres, the gate and the stub MCP
// server. Each subtest of TestE2E is one scenario of the gate-core specs.
//
// The gate keeps its state between scenarios, so each scenario uses its own
// argument values.
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	namespace = "agent-gate-e2e"
	// The client secrets are the test values in gate/deploy/dex.yaml.
	audienceScope = "openid audience:server:client_id:agent-gate"
	// The kind expiry, AGENT_GATE_EXPIRY in gate/deploy/gate.yaml.
	expiry = 30 * time.Second
)

var secrets = map[string]string{"agent-a": "e2e-agent-a", "agent-b": "e2e-agent-b", "approver": "e2e-approver"}

var (
	gatewayURL string // the gateway, no path
	mcpURL     string // stub MCP server, test-only endpoints
	dexURL     string
	gateURL    string // the gate's test approval endpoint
	pgPort     string // Postgres, for the audit export
)

func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	forwards := []struct {
		target string
		port   int
		base   *string
	}{
		{"service/gateway", 8080, &gatewayURL},
		{"service/stub-mcp", 8080, &mcpURL},
		{"service/dex", 5556, &dexURL},
		{"service/agent-gate", 8081, &gateURL},
		{"service/postgres", 5432, &pgPort},
	}
	for _, f := range forwards {
		base, stop, err := portForward(f.target, f.port)
		if err != nil {
			return 0, fmt.Errorf("port-forward %s: %w", f.target, err)
		}
		defer stop()
		*f.base = base
	}
	pgPort = strings.TrimPrefix(pgPort, "http://127.0.0.1:")
	if err := waitForGateway(); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

var forwardLine = regexp.MustCompile(`Forwarding from 127\.0\.0\.1:(\d+)`)

// portForward opens a local port to a service and returns its base URL.
func portForward(target string, port int) (string, func(), error) {
	cmd := exec.Command("kubectl", "-n", namespace, "port-forward", target, fmt.Sprintf(":%d", port))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	stop := func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if m := forwardLine.FindStringSubmatch(scanner.Text()); m != nil {
			go func() { // drain, or kubectl blocks on a full pipe
				for scanner.Scan() {
				}
			}()
			return "http://127.0.0.1:" + m[1], stop, nil
		}
	}
	stop()
	return "", nil, fmt.Errorf("no forwarded port: %s", stderr.String())
}

// waitForGateway returns when a call with a Dex token passes end to end. The
// gateway fetches the Dex JWKS in the background, so the first calls may fail.
func waitForGateway() error {
	deadline := time.Now().Add(120 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		if last = probe(); last == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("gateway not ready: %w", last)
}

func probe() error {
	token, err := dexToken("agent-a")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := connect(ctx, "/mcp", token)
	if err != nil {
		return err
	}
	defer s.Close()
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "probe"}})
	if err != nil {
		return err
	}
	if res.IsError {
		return fmt.Errorf("probe: %s", text(res))
	}
	return nil
}

// dexToken returns a new client_credentials access token for client, with
// the shared audience agent-gate.
func dexToken(client string) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {audienceScope}}
	req, err := http.NewRequest(http.MethodPost, dexURL+"/dex/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(client, secrets[client])
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.AccessToken == "" {
		return "", fmt.Errorf("dex token for %s: status %d: %v", client, resp.StatusCode, err)
	}
	return body.AccessToken, nil
}

// dexSub is the sub that Dex writes for a client_credentials token: base64url,
// no padding, of a protobuf whose field 1 is the client id.
func dexSub(client string) string {
	b := append([]byte{0x0a, byte(len(client))}, client...)
	return base64.RawURLEncoding.EncodeToString(b)
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func connect(ctx context.Context, path, token string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "gate-e2e", Version: "0.0.0"}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             gatewayURL + path,
		HTTPClient:           &http.Client{Transport: bearer{token}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

type upstreamCall struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

// env holds the helpers of one scenario.
type env struct {
	*testing.T
	ctx context.Context
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	e := &env{T: t, ctx: ctx}
	resp, err := http.Post(mcpURL+"/reset", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return e
}

// unique returns a new argument value, so that no scenario sees the
// requests of another.
func unique() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// session opens a new MCP session on /mcp with a new Dex token for client.
func (e *env) session(client string) *mcp.ClientSession {
	e.Helper()
	token, err := dexToken(client)
	if err != nil {
		e.Fatal(err)
	}
	s, err := connect(e.ctx, "/mcp", token)
	if err != nil {
		e.Fatalf("connect as %s: %v", client, err)
	}
	e.Cleanup(func() { _ = s.Close() })
	return s
}

func (e *env) call(s *mcp.ClientSession, tool string, args any) *mcp.CallToolResult {
	e.Helper()
	res, err := s.CallTool(e.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		e.Fatalf("call %s: %v", tool, err)
	}
	return res
}

// held asserts that res is held for approval and returns the reference.
func (e *env) held(res *mcp.CallToolResult) string {
	e.Helper()
	ref, ok := strings.CutPrefix(text(res), "approval required: ref=")
	if !res.IsError || !ok || ref == "" {
		e.Fatalf("want approval required, got isError=%v text=%q", res.IsError, text(res))
	}
	return ref
}

// denied asserts that res is a deny with no approval reference.
func (e *env) denied(res *mcp.CallToolResult, reason string) {
	e.Helper()
	if !res.IsError || strings.Contains(text(res), "ref=") || !strings.Contains(text(res), reason) {
		e.Fatalf("want a deny with %q, got isError=%v text=%q", reason, res.IsError, text(res))
	}
}

func (e *env) passed(res *mcp.CallToolResult, want string) {
	e.Helper()
	if res.IsError || text(res) != want {
		e.Fatalf("got isError=%v text=%q, want the upstream result %q", res.IsError, text(res), want)
	}
}

func (e *env) upstreamCalls(want int) []upstreamCall {
	e.Helper()
	resp, err := http.Get(mcpURL + "/calls")
	if err != nil {
		e.Fatal(err)
	}
	defer resp.Body.Close()
	var calls []upstreamCall
	if err := json.NewDecoder(resp.Body).Decode(&calls); err != nil {
		e.Fatal(err)
	}
	if len(calls) != want {
		e.Fatalf("upstream received %d calls, want %d: %+v", len(calls), want, calls)
	}
	return calls
}

// decide posts a decision on ref with a token of client ("" for no token)
// and returns the status code.
func (e *env) decide(ref, action, client string) int {
	e.Helper()
	req, _ := http.NewRequest(http.MethodPost, gateURL+"/test/requests/"+ref+"/"+action, nil)
	if client != "" {
		token, err := dexToken(client)
		if err != nil {
			e.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func (e *env) approve(ref string) {
	e.Helper()
	if code := e.decide(ref, "approve", "approver"); code != http.StatusOK {
		e.Fatalf("approve %s: status %d", ref, code)
	}
}

// heldThenApproved sends delete_thing as agent-a, expects it held, and approves it.
func (e *env) heldThenApproved(id string) string {
	e.Helper()
	ref := e.held(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": id}))
	e.upstreamCalls(0)
	e.approve(ref)
	return ref
}

func sameJSON(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func TestE2E(t *testing.T) {
	// flowRef is the request of "Replay after approval", which the audit
	// scenario looks up.
	var flowRef string
	const secret = "secret-value"

	t.Run("Allowed call", func(t *testing.T) {
		e := newEnv(t)
		e.passed(e.call(e.session("agent-a"), "echo", map[string]any{"message": "hi"}), "echo: hi")
		e.upstreamCalls(1)
	})

	t.Run("Denied call", func(t *testing.T) {
		e := newEnv(t)
		e.denied(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": "prod"}), "delete_thing on prod is denied")
		e.upstreamCalls(0)
	})

	t.Run("Call that needs approval", func(t *testing.T) {
		e := newEnv(t)
		s := e.session("agent-a")
		args := map[string]any{"id": unique()}
		ref := e.held(e.call(s, "delete_thing", args))
		if again := e.held(e.call(s, "delete_thing", args)); again != ref {
			t.Fatalf("repeat before a decision: ref %s, want %s", again, ref)
		}
		e.upstreamCalls(0)
	})

	t.Run("Integer above 2^53", func(t *testing.T) {
		e := newEnv(t)
		res := e.call(e.session("agent-a"), "delete_thing", json.RawMessage(`{"id":9007199254740993}`))
		e.denied(res, "unsafe number")
		e.upstreamCalls(0)
	})

	t.Run("Replay after approval", func(t *testing.T) {
		e := newEnv(t)
		id := secret + "-" + unique()
		flowRef = e.heldThenApproved(id)
		// A new session and a new token for the same client.
		e.passed(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": id}), "deleted: "+id)
		calls := e.upstreamCalls(1)
		if calls[0].Tool != "delete_thing" || !sameJSON(calls[0].Arguments, []byte(`{"id":"`+id+`"}`)) {
			t.Fatalf("upstream call differs from the approved call: %s %s", calls[0].Tool, calls[0].Arguments)
		}
	})

	t.Run("Replay with changed arguments", func(t *testing.T) {
		e := newEnv(t)
		e.heldThenApproved(unique())
		e.held(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": unique()}))
		e.upstreamCalls(0)
	})

	t.Run("Replay by another caller", func(t *testing.T) {
		e := newEnv(t)
		id := unique()
		e.heldThenApproved(id)
		e.held(e.call(e.session("agent-b"), "delete_thing", map[string]any{"id": id}))
		e.upstreamCalls(0)
	})

	t.Run("Second replay", func(t *testing.T) {
		e := newEnv(t)
		id := unique()
		e.heldThenApproved(id)
		s := e.session("agent-a")
		e.passed(e.call(s, "delete_thing", map[string]any{"id": id}), "deleted: "+id)
		e.held(e.call(s, "delete_thing", map[string]any{"id": id}))
		e.upstreamCalls(1)
	})

	t.Run("Self-approval", func(t *testing.T) {
		e := newEnv(t)
		ref := e.held(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": unique()}))
		if code := e.decide(ref, "approve", "agent-a"); code != http.StatusConflict {
			t.Fatalf("self-approval: status %d, want 409", code)
		}
		// Still pending: another identity can approve it.
		e.approve(ref)
	})

	t.Run("Decision with no token", func(t *testing.T) {
		e := newEnv(t)
		ref := e.held(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": unique()}))
		if code := e.decide(ref, "approve", ""); code != http.StatusUnauthorized {
			t.Fatalf("no token: status %d, want 401", code)
		}
		e.approve(ref)
	})

	t.Run("Replay after rejection", func(t *testing.T) {
		e := newEnv(t)
		args := map[string]any{"id": unique()}
		s := e.session("agent-a")
		ref := e.held(e.call(s, "delete_thing", args))
		if code := e.decide(ref, "reject", "approver"); code != http.StatusOK {
			t.Fatalf("reject: status %d", code)
		}
		if again := e.held(e.call(s, "delete_thing", args)); again == ref {
			t.Fatal("replay after rejection returned the rejected reference")
		}
		e.upstreamCalls(0)
	})

	t.Run("Call with no identity claim", func(t *testing.T) {
		e := newEnv(t)
		s, err := connect(e.ctx, "/mcp-noauth", "")
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		res, err := s.CallTool(e.ctx, &mcp.CallToolParams{Name: "delete_thing", Arguments: map[string]any{"id": unique()}})
		if err != nil {
			t.Fatalf("got a JSON-RPC error, want a tool result: %v", err)
		}
		e.denied(res, "no caller or tenant identity")
		e.upstreamCalls(0)
	})

	t.Run("Replay after expiry", func(t *testing.T) {
		e := newEnv(t)
		id := unique()
		e.heldThenApproved(id)
		time.Sleep(expiry + time.Second)
		e.held(e.call(e.session("agent-a"), "delete_thing", map[string]any{"id": id}))
		e.upstreamCalls(0)
	})

	t.Run("Audit log", func(t *testing.T) {
		e := newEnv(t)
		dir := t.TempDir()
		bin := filepath.Join(dir, "agent-gate-audit")
		if out, err := exec.Command("go", "build", "-o", bin, "../cmd/agent-gate-audit").CombinedOutput(); err != nil {
			t.Fatalf("build: %v: %s", err, out)
		}
		export := filepath.Join(dir, "export.json")
		dsn := "postgres://postgres@127.0.0.1:" + pgPort + "/agent_gate?sslmode=disable"
		if out, err := exec.Command(bin, "export", "--dsn", dsn, "--out", export).CombinedOutput(); err != nil {
			t.Fatalf("export: %v: %s", err, out)
		}
		raw, err := os.ReadFile(export)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("an argument value is in the audit log")
		}
		var x struct {
			Entries     []map[string]any `json:"entries"`
			Checkpoints []map[string]any `json:"checkpoints"`
		}
		if err := json.Unmarshal(raw, &x); err != nil {
			t.Fatal(err)
		}
		if len(x.Checkpoints) == 0 {
			t.Fatal("no checkpoint")
		}
		e.checkFlow(x.Entries, flowRef)

		verify := func(name string, entries []map[string]any) error {
			b, err := json.Marshal(map[string]any{"entries": entries, "checkpoints": x.Checkpoints})
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, name+".json")
			if err := os.WriteFile(p, b, 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(bin, "verify", p).CombinedOutput()
			t.Logf("verify %s: %s", name, bytes.TrimSpace(out))
			return err
		}
		if err := verify("untouched", x.Entries); err != nil {
			t.Fatalf("untouched export: %v", err)
		}
		clone := func() []map[string]any {
			out := make([]map[string]any, len(x.Entries))
			for i, m := range x.Entries {
				c := map[string]any{}
				for k, v := range m {
					c[k] = v
				}
				out[i] = c
			}
			return out
		}
		changed := clone()
		changed[1]["reason"] = "changed"
		removed := clone()
		removed = append(removed[:1], removed[2:]...)
		swapped := clone()
		swapped[1], swapped[2] = swapped[2], swapped[1]
		for name, entries := range map[string][]map[string]any{"changed": changed, "removed": removed, "swapped": swapped} {
			var exit *exec.ExitError
			if err := verify(name, entries); !errors.As(err, &exit) || exit.ExitCode() == 0 {
				t.Errorf("verify %s: %v, want a non-zero exit", name, err)
			}
		}
	})
}

// checkFlow asserts that the request ref has, in order, the entries
// approval_required, approved, approval_used, and then a result of agent-a.
func (e *env) checkFlow(entries []map[string]any, ref string) {
	e.Helper()
	if ref == "" {
		e.Fatal("no request from the scenario Replay after approval")
	}
	want := []string{"approval_required", "approved", "approval_used"}
	i := 0
	for _, en := range entries {
		if i < len(want) && en["approval_ref"] == ref && en["event"] == want[i] {
			if en["event"] == "approval_used" && en["agent"] != dexSub("agent-a") {
				e.Fatalf("approval_used agent %v, want %s", en["agent"], dexSub("agent-a"))
			}
			if en["event"] != "approved" && en["policy_version"] == nil {
				e.Fatalf("no policy version in %v", en)
			}
			i++
			continue
		}
		if i == len(want) && en["event"] == "result" && en["agent"] == dexSub("agent-a") {
			return
		}
	}
	e.Fatalf("request %s: found %d of %v and no result after them", ref, i, want)
}
