package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentgateway/agentgateway/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// start runs the gate's gRPC server and test HTTP server in process.
func start(t *testing.T) (*gate, api.ExtMcpClient, string) {
	t.Helper()
	g := newGate()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	api.RegisterExtMcpServer(server, g)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	web := httptest.NewServer(g.handler())
	t.Cleanup(web.Close)
	return g, api.NewExtMcpClient(conn), web.URL
}

func request(t *testing.T, sub, params string) *api.McpRequest {
	t.Helper()
	fields := map[string]any{}
	if sub != "" {
		fields["sub"] = sub
	}
	metadata, err := structpb.NewStruct(fields)
	if err != nil {
		t.Fatal(err)
	}
	return &api.McpRequest{
		ServiceNames:    []string{"stub-mcp"},
		Method:          "tools/call",
		MetadataContext: metadata,
		McpRequest:      []byte(params),
		Headers:         []*api.McpHeader{{Key: "mcp-protocol-version", Value: []byte("test")}},
	}
}

// check sends a request and returns the approval reference of a deny, or "" for a pass.
func check(t *testing.T, client api.ExtMcpClient, req *api.McpRequest) string {
	t.Helper()
	res, err := client.CheckRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckRequest: %v", err)
	}
	if res.GetPass() != nil {
		return ""
	}
	reason := res.GetError().GetReason()
	if res.GetError().GetCode() != api.AuthorizationError_PERMISSION_DENIED || reason == "" {
		t.Fatalf("unexpected result: %v", res)
	}
	return reason
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func approve(t *testing.T, base, reason string) {
	t.Helper()
	_, ref, ok := strings.Cut(reason, "ref=")
	if !ok {
		t.Fatalf("no reference in %q", reason)
	}
	resp, err := http.Post(base+"/approve?ref="+ref, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: status %d", resp.StatusCode)
	}
}

const (
	call42     = `{"name":"delete_thing","arguments":{"id":"42","force":true}}`
	call42Meta = `{"_meta":{"progressToken":7},"arguments":{"force":true,"id":"42"},"name":"delete_thing"}`
	call43     = `{"name":"delete_thing","arguments":{"id":"43","force":true}}`
)

func TestDenyApproveReplay(t *testing.T) {
	_, client, base := start(t)

	if got := check(t, client, request(t, "alice", `{"name":"echo","arguments":{"message":"hi"}}`)); got != "" {
		t.Fatalf("ungated tool was denied: %q", got)
	}

	reason := check(t, client, request(t, "alice", call42))
	if !strings.HasPrefix(reason, "approval required: ref=") {
		t.Fatalf("first call: got %q, want a deny with a reference", reason)
	}
	if got := check(t, client, request(t, "alice", call42)); got == "" {
		t.Fatal("replay before approval passed")
	}
	approve(t, base, reason)

	if got := check(t, client, request(t, "alice", call43)); got == "" {
		t.Fatal("changed arguments passed")
	}
	if got := check(t, client, request(t, "bob", call42)); got == "" {
		t.Fatal("different caller passed")
	}
	// The replay has another _meta and another key order. It must still pass.
	if got := check(t, client, request(t, "alice", call42Meta)); got != "" {
		t.Fatalf("identical replay was denied: %q", got)
	}
	if got := check(t, client, request(t, "alice", call42)); got == "" {
		t.Fatal("second replay of a used approval passed")
	}

	var requests []seenRequest
	getJSON(t, base+"/requests", &requests)
	if len(requests) != 7 {
		t.Fatalf("/requests: got %d entries, want 7", len(requests))
	}
	first := requests[1]
	if first.Method != "tools/call" || first.Metadata["sub"] != "alice" ||
		string(first.Params) != call42 || first.Headers["mcp-protocol-version"][0] != "test" {
		t.Fatalf("/requests[1]: %+v", first)
	}
}

func TestResponseIsRecorded(t *testing.T) {
	_, client, base := start(t)
	result := `{"content":[{"type":"text","text":"deleted: 42"}]}`
	res, err := client.CheckResponse(context.Background(), &api.McpResponse{
		ServiceNames: []string{"stub-mcp"},
		Method:       "tools/call",
		McpResponse:  []byte(result),
	})
	if err != nil || res.GetPass() == nil {
		t.Fatalf("CheckResponse: res=%v err=%v", res, err)
	}
	var responses []seenResponse
	getJSON(t, base+"/responses", &responses)
	if len(responses) != 1 || responses[0].Method != "tools/call" || string(responses[0].Result) != result {
		t.Fatalf("/responses: %+v", responses)
	}
}

func TestNoSubIsDeniedWithNoPendingEntry(t *testing.T) {
	g, client, _ := start(t)
	reason := check(t, client, request(t, "", call42))
	if reason != "no identity claim" {
		t.Fatalf("got %q, want a deny with no reference", reason)
	}
	if len(g.approvals) != 0 {
		t.Fatalf("a pending entry was stored: %d", len(g.approvals))
	}
}

func TestResetClearsState(t *testing.T) {
	g, client, base := start(t)
	check(t, client, request(t, "alice", call42))
	resp, err := http.Post(base+"/reset", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(g.approvals) != 0 || len(g.requests) != 0 {
		t.Fatalf("state after reset: %d approvals, %d requests", len(g.approvals), len(g.requests))
	}
}
