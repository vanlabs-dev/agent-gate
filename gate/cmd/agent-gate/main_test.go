package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/identity"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
)

// fakeAuth accepts "Bearer <name>" and returns name.
type fakeAuth struct{}

func (fakeAuth) Approver(_ context.Context, h string) (string, error) {
	name, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || name == "" {
		return "", identity.ErrNoToken
	}
	return name, nil
}

func newGate(t *testing.T) (*approval.Gate, string) {
	t.Helper()
	p, err := policy.Load(context.Background(), "../../deploy/policy")
	if err != nil {
		t.Fatal(err)
	}
	g := &approval.Gate{Policy: p, Store: approval.NewMemoryStore()}
	v, err := g.Check(context.Background(), approval.Call{Tenant: "local", Agent: "agent-a", Target: "svc", Tool: "delete_thing", Arguments: []byte(`{"id":"x"}`)})
	if err != nil {
		t.Fatal(err)
	}
	return g, v.Ref
}

func post(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestTestEndpoint(t *testing.T) {
	g, ref := newGate(t)
	srv := httptest.NewServer(testHandler(g, fakeAuth{}))
	defer srv.Close()
	url := srv.URL + "/test/requests/" + ref

	if code := post(t, url+"/approve", ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code := post(t, url+"/approve", "agent-a"); code != http.StatusConflict {
		t.Fatalf("self-approval: %d", code)
	}
	if code := post(t, srv.URL+"/test/requests/apr_none/approve", "approver"); code != http.StatusNotFound {
		t.Fatalf("unknown ref: %d", code)
	}
	if code := post(t, url+"/approve", "approver"); code != http.StatusOK {
		t.Fatalf("approve: %d", code)
	}
	if code := post(t, url+"/reject", "approver"); code != http.StatusConflict {
		t.Fatalf("reject after approve: %d", code)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// TestDefaultStart serves with no authenticator, as with no -test-endpoints,
// and checks that nothing answers on the HTTP address.
func TestDefaultStart(t *testing.T) {
	g, _ := newGate(t)
	cfg := config{GRPCAddr: freeAddr(t), HTTPAddr: freeAddr(t)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, g, identity.Claims{}, nil) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c, err := net.Dial("tcp", cfg.GRPCAddr)
		if err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("gRPC listener did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if c, err := net.Dial("tcp", cfg.HTTPAddr); err == nil {
		_ = c.Close()
		t.Fatal("the test endpoint answers without -test-endpoints")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	if _, err := parse([]string{"-dsn", "x", "-test-endpoints"}, io.Discard); err == nil {
		t.Error("-test-endpoints with no issuer passed")
	}
	if _, err := parse(nil, io.Discard); err == nil {
		t.Error("no -dsn passed")
	}
	if _, err := parse([]string{"-help"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-help: %v", err)
	}
	c, err := parse([]string{"-dsn", "x", "-oidc-audiences", "agent-gate, other"}, io.Discard)
	if err != nil || c.Expiry != time.Hour || c.CallerClaim != "sub" || c.Tenant != "local" || len(c.OIDCAudiences) != 2 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
}
