package extmcp

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/agentgateway/agentgateway/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
)

type harness struct {
	client api.ExtMcpClient
	gate   *approval.Gate
	store  *approval.MemoryStore
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	p, err := policy.Load(context.Background(), "../../deploy/policy")
	if err != nil {
		t.Fatal(err)
	}
	store := approval.NewMemoryStore()
	gate := &approval.Gate{Policy: p, Store: store}
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer()
	Register(g, &Server{Gate: gate})
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &harness{client: api.NewExtMcpClient(conn), gate: gate, store: store}
}

func request(t *testing.T, sub string, services []string, params string) *api.McpRequest {
	t.Helper()
	md := map[string]any{"iss": "http://dex"}
	if sub != "" {
		md["sub"] = sub
	}
	s, err := structpb.NewStruct(md)
	if err != nil {
		t.Fatal(err)
	}
	return &api.McpRequest{Method: "tools/call", ServiceNames: services, MetadataContext: s, McpRequest: []byte(params)}
}

// check returns "" for pass, or the deny reason.
func (h *harness) check(t *testing.T, req *api.McpRequest) string {
	t.Helper()
	res, err := h.client.CheckRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.GetPass() != nil {
		return ""
	}
	if res.GetError() == nil {
		t.Fatalf("neither pass nor error: %v", res)
	}
	return res.GetError().GetReason()
}

func TestCheckRequest(t *testing.T) {
	h := newHarness(t)
	svc := []string{"agent-gate-e2e/stub-mcp"}

	if r := h.check(t, request(t, "agent-a", svc, `{"name":"echo","arguments":{"text":"hi"}}`)); r != "" {
		t.Errorf("allow: %q", r)
	}
	if r := h.check(t, request(t, "agent-a", svc, `{"name":"delete_thing","arguments":{"id":"prod"}}`)); r != "delete_thing on prod is denied" {
		t.Errorf("deny: %q", r)
	}
	if r := h.check(t, request(t, "", svc, `{"name":"echo"}`)); r == "" {
		t.Error("no identity passed")
	}
	if r := h.check(t, request(t, "agent-a", []string{"a", "b"}, `{"name":"echo"}`)); r == "" {
		t.Error("two service names passed")
	}
	if r := h.check(t, request(t, "agent-a", nil, `{"name":"echo"}`)); r == "" {
		t.Error("no service name passed")
	}

	call := request(t, "agent-a", svc, `{"name":"delete_thing","arguments":{"id":"x"},"_meta":{"progressToken":1}}`)
	r := h.check(t, call)
	ref, ok := strings.CutPrefix(r, approval.ApprovalRequiredPrefix)
	if !ok {
		t.Fatalf("needs approval: %q", r)
	}
	if _, err := h.gate.Decide(context.Background(), ref, "approver", true); err != nil {
		t.Fatal(err)
	}
	replay := request(t, "agent-a", svc, `{"_meta":{"progressToken":2},"name":"delete_thing","arguments":{"id":"x"}}`)
	if r := h.check(t, replay); r != "" {
		t.Fatalf("replay: %q", r)
	}
	if r := h.check(t, replay); r == "" {
		t.Fatal("second replay passed")
	}
}

func TestOtherMethodsPass(t *testing.T) {
	h := newHarness(t)
	res, err := h.client.CheckRequest(context.Background(), &api.McpRequest{Method: "tools/list"})
	if err != nil || res.GetPass() == nil {
		t.Fatalf("tools/list: %v %v", res, err)
	}
}

func TestCheckResponse(t *testing.T) {
	h := newHarness(t)
	md, _ := structpb.NewStruct(map[string]any{"sub": "agent-a"})
	res, err := h.client.CheckResponse(context.Background(), &api.McpResponse{
		Method: "tools/call", ServiceNames: []string{"svc"}, MetadataContext: md, McpResponse: []byte(`{"content":[]}`),
	})
	if err != nil || res.GetPass() == nil {
		t.Fatalf("got %v %v", res, err)
	}
	x := h.store.Export()
	if len(x.Entries) != 1 || x.Entries[0].Event != audit.EventResult || x.Entries[0].ResultDigest == nil {
		t.Fatalf("entries %+v", x.Entries)
	}
}
