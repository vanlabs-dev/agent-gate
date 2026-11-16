// Command stub-gate is a test ExtMcp server. It gates one hard-coded tool
// with deny and replay, and records what agentgateway sends it.
//
// Spike code. State is in memory, and the digest is not canonical.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/agentgateway/agentgateway/api"
	"google.golang.org/grpc"
)

const (
	grpcAddr  = ":9001"
	httpAddr  = ":8081"
	gatedTool = "delete_thing"
)

type approval struct {
	digest   string
	approved bool
	used     bool
}

type seenRequest struct {
	Method       string              `json:"method"`
	ServiceNames []string            `json:"serviceNames"`
	Metadata     map[string]any      `json:"metadata"`
	Params       json.RawMessage     `json:"params"`
	Headers      map[string][]string `json:"headers"`
}

type seenResponse struct {
	Method       string          `json:"method"`
	ServiceNames []string        `json:"serviceNames"`
	Metadata     map[string]any  `json:"metadata"`
	Result       json.RawMessage `json:"result"`
}

type gate struct {
	api.UnimplementedExtMcpServer

	mu        sync.Mutex
	approvals map[string]*approval // by reference
	requests  []seenRequest
	responses []seenResponse
}

func newGate() *gate {
	return &gate{approvals: map[string]*approval{}}
}

func pass() *api.McpRequestResult {
	return &api.McpRequestResult{Result: &api.McpRequestResult_Pass{Pass: &api.Pass{}}}
}

func deny(reason string) *api.McpRequestResult {
	return &api.McpRequestResult{Result: &api.McpRequestResult_Error{Error: &api.AuthorizationError{
		Code:   api.AuthorizationError_PERMISSION_DENIED,
		Reason: reason,
	}}}
}

// raw returns nil for empty input, because an empty json.RawMessage does not marshal.
func raw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return b
}

// digest binds an approval to the caller, the target, the tool and the
// arguments. It reads only name and arguments from params: agentgateway
// merges _meta into params, and _meta changes between calls.
func digest(sub string, services []string, name string, arguments json.RawMessage) (string, error) {
	var args any
	if len(arguments) > 0 {
		dec := json.NewDecoder(bytes.NewReader(arguments))
		dec.UseNumber()
		if err := dec.Decode(&args); err != nil {
			return "", err
		}
	}
	// encoding/json writes map keys in sorted order.
	b, err := json.Marshal([]any{sub, services, name, args})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (g *gate) CheckRequest(_ context.Context, req *api.McpRequest) (*api.McpRequestResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	headers := map[string][]string{}
	for _, h := range req.GetHeaders() {
		headers[h.GetKey()] = append(headers[h.GetKey()], string(h.GetValue()))
	}
	metadata := req.GetMetadataContext().AsMap()
	g.requests = append(g.requests, seenRequest{
		Method:       req.GetMethod(),
		ServiceNames: req.GetServiceNames(),
		Metadata:     metadata,
		Params:       raw(req.GetMcpRequest()),
		Headers:      headers,
	})

	if req.GetMethod() != "tools/call" {
		return pass(), nil
	}
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.GetMcpRequest(), &params); err != nil {
		return deny("invalid params"), nil
	}
	if params.Name != gatedTool {
		return pass(), nil
	}

	// An approval bound to an empty caller would pass for any caller.
	sub, _ := metadata["sub"].(string)
	if sub == "" {
		return deny("no identity claim"), nil
	}
	d, err := digest(sub, req.GetServiceNames(), params.Name, params.Arguments)
	if err != nil {
		return deny("invalid arguments"), nil
	}
	for _, a := range g.approvals {
		if a.digest == d && a.approved && !a.used {
			a.used = true
			return pass(), nil
		}
	}
	ref := make([]byte, 8)
	if _, err := rand.Read(ref); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(ref)
	g.approvals[id] = &approval{digest: d}
	return deny("approval required: ref=" + id), nil
}

func (g *gate) CheckResponse(_ context.Context, resp *api.McpResponse) (*api.McpResponseResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.responses = append(g.responses, seenResponse{
		Method:       resp.GetMethod(),
		ServiceNames: resp.GetServiceNames(),
		Metadata:     resp.GetMetadataContext().AsMap(),
		Result:       raw(resp.GetMcpResponse()),
	})
	return &api.McpResponseResult{Result: &api.McpResponseResult_Pass{Pass: &api.Pass{}}}, nil
}

// handler serves the test-only endpoints.
func (g *gate) handler() http.Handler {
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /approve", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		a, ok := g.approvals[r.URL.Query().Get("ref")]
		if !ok {
			http.Error(w, "unknown reference", http.StatusNotFound)
			return
		}
		a.approved = true
	})
	mux.HandleFunc("GET /requests", func(w http.ResponseWriter, _ *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		writeJSON(w, append([]seenRequest{}, g.requests...))
	})
	mux.HandleFunc("GET /responses", func(w http.ResponseWriter, _ *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		writeJSON(w, append([]seenResponse{}, g.responses...))
	})
	mux.HandleFunc("POST /reset", func(http.ResponseWriter, *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.approvals = map[string]*approval{}
		g.requests = nil
		g.responses = nil
	})
	return mux
}

func main() {
	g := newGate()
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	api.RegisterExtMcpServer(server, g)
	go func() { log.Fatal(server.Serve(lis)) }()
	log.Printf("stub-gate listens on %s (gRPC) and %s (test HTTP)", grpcAddr, httpAddr)
	log.Fatal(http.ListenAndServe(httpAddr, g.handler()))
}
