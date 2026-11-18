//go:build e2e

// Package extmcp_test proves the contract between agentgateway and a gate
// behind its ExtMcp extension. It runs against the kind cluster that
// `make e2e` creates. Each subtest of TestE2E is one scenario of the spec
// openspec/specs/agentgateway-integration.
package extmcp_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	namespace = "agent-gate-test"
	issuer    = "https://issuer.agent-gate.test"
	keyID     = "e2e"
)

var (
	gatewayURL string // the gateway's MCP endpoint
	mcpURL     string // stub MCP server, test-only endpoints
	gateURL    string // stub gate, test-only endpoints

	trustedKey *rsa.PrivateKey // its public half is in the JWT policy
	unknownKey *rsa.PrivateKey
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
	var err error
	if trustedKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		return 0, err
	}
	if unknownKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		return 0, err
	}
	if err := applyJWTPolicy(&trustedKey.PublicKey); err != nil {
		return 0, err
	}

	forwards := []struct {
		target string
		port   int
		base   *string
	}{
		{"service/gateway", 8080, &gatewayURL},
		{"service/stub-mcp", 8080, &mcpURL},
		{"service/stub-gate", 8081, &gateURL},
	}
	for _, f := range forwards {
		base, stop, err := portForward(f.target, f.port)
		if err != nil {
			return 0, fmt.Errorf("port-forward %s: %w", f.target, err)
		}
		defer stop()
		*f.base = base
	}
	gatewayURL += "/mcp"

	if err := waitForGateway(); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// applyJWTPolicy makes the gateway accept only tokens signed by key. The key
// is generated for each run, so no private key is stored in the repo.
func applyJWTPolicy(key *rsa.PublicKey) error {
	b64 := base64.RawURLEncoding.EncodeToString
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID,
		"n": b64(key.N.Bytes()),
		"e": b64(big.NewInt(int64(key.E)).Bytes()),
	}}})
	if err != nil {
		return err
	}
	policy, err := json.Marshal(map[string]any{
		"apiVersion": "agentgateway.dev/v1alpha1",
		"kind":       "AgentgatewayPolicy",
		"metadata":   map[string]string{"name": "jwt", "namespace": namespace},
		"spec": map[string]any{
			"targetRefs": []map[string]string{{
				"group": "gateway.networking.k8s.io", "kind": "HTTPRoute", "name": "mcp",
			}},
			"traffic": map[string]any{"jwtAuthentication": map[string]any{
				"mode": "Strict",
				"providers": []map[string]any{{
					"issuer": issuer,
					"jwks":   map[string]string{"inline": string(jwks)},
				}},
			}},
		},
	})
	if err != nil {
		return err
	}
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(policy)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply JWT policy: %v: %s", err, out)
	}
	return nil
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

// waitForGateway returns when the gateway enforces the JWT policy and a call
// with a valid token passes end to end. The first call also opens the
// gateway's connection to the gate.
func waitForGateway() error {
	deadline := time.Now().Add(90 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		if last = probeGateway(); last == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("gateway not ready: %w", last)
}

func probeGateway() error {
	resp, err := http.Post(gatewayURL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("request with no token: status %d, want 401", resp.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, _, err := connect(ctx, sign(trustedKey, "probe"))
	if err != nil {
		return err
	}
	defer session.Close()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "probe"}})
	if err != nil {
		return err
	}
	if res.IsError {
		return fmt.Errorf("probe call: %s", text(res))
	}
	return nil
}

// sign returns a new RS256 token for sub. Each call returns a different token.
func sign(key *rsa.PrivateKey, sub string) string {
	b64 := base64.RawURLEncoding.EncodeToString
	jti := make([]byte, 8)
	_, _ = rand.Read(jti)
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	claims, _ := json.Marshal(map[string]any{
		"iss": issuer, "sub": sub, "jti": b64(jti),
		"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
	})
	signed := b64(header) + "." + b64(claims)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return signed + "." + b64(sig)
}

// tokenTransport adds a bearer token that a test can replace in an open session.
type tokenTransport struct {
	mu    sync.Mutex
	token string
}

func (t *tokenTransport) set(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = token
}

func (t *tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.Lock()
	token := t.token
	t.mu.Unlock()
	r = r.Clone(r.Context())
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func connect(ctx context.Context, token string) (*mcp.ClientSession, *tokenTransport, error) {
	transport := &tokenTransport{token: token}
	client := mcp.NewClient(&mcp.Implementation{Name: "extmcp-e2e", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             gatewayURL,
		HTTPClient:           &http.Client{Transport: transport},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	return session, transport, err
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

// The JSON shapes of the stubs' test-only endpoints.
type upstreamCall struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type gateRequest struct {
	Method   string              `json:"method"`
	Metadata map[string]any      `json:"metadata"`
	Params   json.RawMessage     `json:"params"`
	Headers  map[string][]string `json:"headers"`
}

type gateResponse struct {
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
}

// env holds the helpers of one scenario.
type env struct {
	*testing.T
	ctx context.Context
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	e := &env{T: t, ctx: ctx}
	e.post(mcpURL + "/reset")
	e.post(gateURL + "/reset")
	return e
}

func (e *env) post(url string) {
	e.Helper()
	resp, err := http.Post(url, "", nil)
	if err != nil {
		e.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		e.Fatalf("POST %s: status %d", url, resp.StatusCode)
	}
}

func (e *env) get(url string, v any) {
	e.Helper()
	resp, err := http.Get(url)
	if err != nil {
		e.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		e.Fatalf("GET %s: %v", url, err)
	}
}

// session opens a new MCP session through the gateway with a new token for sub.
func (e *env) session(sub string) (*mcp.ClientSession, *tokenTransport) {
	e.Helper()
	session, transport, err := connect(e.ctx, sign(trustedKey, sub))
	if err != nil {
		e.Fatalf("connect as %s: %v", sub, err)
	}
	e.Cleanup(func() { _ = session.Close() })
	return session, transport
}

func (e *env) call(s *mcp.ClientSession, tool string, args map[string]any) *mcp.CallToolResult {
	e.Helper()
	res, err := s.CallTool(e.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		e.Fatalf("call %s: %v", tool, err)
	}
	return res
}

// denied asserts that res is a deny from the gate and returns the approval reference.
func (e *env) denied(res *mcp.CallToolResult) string {
	e.Helper()
	if !res.IsError {
		e.Fatalf("got a result with isError false, want a deny: %s", text(res))
	}
	_, ref, ok := strings.Cut(text(res), "ref=")
	if !ok || ref == "" {
		e.Fatalf("no approval reference in %q", text(res))
	}
	return ref
}

func (e *env) passed(res *mcp.CallToolResult, want string) {
	e.Helper()
	if res.IsError || text(res) != want {
		e.Fatalf("got isError=%v text=%q, want the upstream result %q", res.IsError, text(res), want)
	}
}

func (e *env) upstreamCalls(want int) []upstreamCall {
	e.Helper()
	var calls []upstreamCall
	e.get(mcpURL+"/calls", &calls)
	if len(calls) != want {
		e.Fatalf("upstream received %d calls, want %d: %+v", len(calls), want, calls)
	}
	return calls
}

func (e *env) gateRequests(want int) []gateRequest {
	e.Helper()
	var requests []gateRequest
	e.get(gateURL+"/requests", &requests)
	if len(requests) != want {
		e.Fatalf("gate received %d requests, want %d: %+v", len(requests), want, requests)
	}
	return requests
}

// deniedThenApproved sends the gated call as alice, expects a deny, and records the approval.
func (e *env) deniedThenApproved(args map[string]any) {
	e.Helper()
	s, _ := e.session("alice")
	ref := e.denied(e.call(s, "delete_thing", args))
	e.upstreamCalls(0)
	e.post(gateURL + "/approve?ref=" + ref)
}

func sameJSON(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func kubectl(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("kubectl", append([]string{"-n", namespace}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("kubectl %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func TestE2E(t *testing.T) {
	thing := map[string]any{"id": "42"}

	t.Run("Ungated tool", func(t *testing.T) {
		e := newEnv(t)
		s, _ := e.session("alice")
		e.passed(e.call(s, "echo", map[string]any{"message": "hi"}), "echo: hi")
		if calls := e.upstreamCalls(1); calls[0].Tool != "echo" {
			t.Fatalf("upstream call: %+v", calls[0])
		}
	})

	t.Run("First call to a gated tool", func(t *testing.T) {
		e := newEnv(t)
		s, _ := e.session("alice")
		ref := e.denied(e.call(s, "delete_thing", thing))
		t.Logf("approval reference: %s", ref)
		e.upstreamCalls(0)
	})

	t.Run("Replay after approval", func(t *testing.T) {
		e := newEnv(t)
		e.deniedThenApproved(thing)
		// A new session and a newly signed token for the same sub.
		s, _ := e.session("alice")
		e.passed(e.call(s, "delete_thing", thing), "deleted: 42")
		calls := e.upstreamCalls(1)
		if calls[0].Tool != "delete_thing" || !sameJSON(calls[0].Arguments, []byte(`{"id":"42"}`)) {
			t.Fatalf("upstream call differs from the approved call: %s %s", calls[0].Tool, calls[0].Arguments)
		}
	})

	t.Run("Second replay of a used approval", func(t *testing.T) {
		e := newEnv(t)
		e.deniedThenApproved(thing)
		s, _ := e.session("alice")
		e.passed(e.call(s, "delete_thing", thing), "deleted: 42")
		e.denied(e.call(s, "delete_thing", thing))
		e.upstreamCalls(1)
	})

	t.Run("Replay with changed arguments", func(t *testing.T) {
		e := newEnv(t)
		e.deniedThenApproved(thing)
		s, _ := e.session("alice")
		e.denied(e.call(s, "delete_thing", map[string]any{"id": "43"}))
		e.upstreamCalls(0)
	})

	t.Run("Replay by a different caller", func(t *testing.T) {
		e := newEnv(t)
		e.deniedThenApproved(thing)
		s, _ := e.session("bob")
		e.denied(e.call(s, "delete_thing", thing))
		e.upstreamCalls(0)
	})

	t.Run("Call with a valid token", func(t *testing.T) {
		e := newEnv(t)
		s, _ := e.session("alice")
		e.passed(e.call(s, "echo", map[string]any{"message": "claims"}), "echo: claims")
		req := e.gateRequests(1)[0]
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("params %s: %v", req.Params, err)
		}
		if req.Method != "tools/call" || params.Name != "echo" || !sameJSON(params.Arguments, []byte(`{"message":"claims"}`)) {
			t.Fatalf("gate request: method=%s params=%s", req.Method, req.Params)
		}
		if req.Metadata["sub"] != "alice" || req.Metadata["iss"] != issuer {
			t.Fatalf("metadata_context: %+v, want sub and iss of the token", req.Metadata)
		}
		for name := range req.Headers {
			if !strings.EqualFold(name, "mcp-protocol-version") {
				t.Fatalf("gate received the header %q. Only mcp-protocol-version is allowed", name)
			}
		}
		t.Logf("params as the gate received them: %s", req.Params)
		t.Logf("headers as the gate received them: %v", req.Headers)
	})

	// rejected sends tools/call with a bad token in a session that a valid
	// token opened. The gateway must stop it before the gate and the upstream.
	rejected := func(t *testing.T, token string) {
		e := newEnv(t)
		s, transport := e.session("alice")
		transport.set(token)
		res, err := s.CallTool(e.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "no"}})
		if err == nil {
			t.Fatalf("the call passed: isError=%v text=%q", res.IsError, text(res))
		}
		t.Logf("client error: %v", err)
		e.gateRequests(0)
		e.upstreamCalls(0)
	}

	t.Run("Call with a token that fails validation", func(t *testing.T) {
		rejected(t, sign(unknownKey, "alice"))
	})

	t.Run("Call with no token", func(t *testing.T) {
		rejected(t, "")
	})

	t.Run("Result of a passed call", func(t *testing.T) {
		e := newEnv(t)
		s, _ := e.session("alice")
		e.passed(e.call(s, "echo", map[string]any{"message": "result"}), "echo: result")
		var responses []gateResponse
		e.get(gateURL+"/responses", &responses)
		if len(responses) != 1 || responses[0].Method != "tools/call" ||
			!strings.Contains(string(responses[0].Result), "echo: result") {
			t.Fatalf("gate responses: %+v", responses)
		}
		t.Logf("result as the gate received it: %s", responses[0].Result)
	})

	// Last, because it stops the gate. The gate loses its state.
	t.Run("Gate unavailable", func(t *testing.T) {
		e := newEnv(t)
		t.Cleanup(func() {
			kubectl(t, "scale", "deployment/stub-gate", "--replicas=1")
			kubectl(t, "wait", "deployment/stub-gate", "--for=condition=Available", "--timeout=90s")
		})
		kubectl(t, "scale", "deployment/stub-gate", "--replicas=0")
		kubectl(t, "wait", "pod", "-l", "app=stub-gate", "--for=delete", "--timeout=90s")

		s, _ := e.session("alice")
		res, err := s.CallTool(e.ctx, &mcp.CallToolParams{Name: "delete_thing", Arguments: thing})
		if err == nil {
			t.Fatalf("got a tool result, want a JSON-RPC error: isError=%v text=%q", res.IsError, text(res))
		}
		var rpcErr *jsonrpc.Error
		if !errors.As(err, &rpcErr) {
			t.Fatalf("got %T (%v), want a JSON-RPC error", err, err)
		}
		t.Logf("JSON-RPC error: code=%d message=%q", rpcErr.Code, rpcErr.Message)
		e.upstreamCalls(0)
	})
}
