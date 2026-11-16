// Command stub-mcp is a test MCP server. It serves two tools over streamable
// HTTP and records each call it receives, so that a test can prove whether a
// call reached the upstream server.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const listenAddr = ":8080"

type call struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type stub struct {
	mu    sync.Mutex
	calls []call
}

type echoInput struct {
	Message string `json:"message"`
}

type deleteInput struct {
	ID string `json:"id"`
}

func (s *stub) record(req *mcp.CallToolRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call{Tool: req.Params.Name, Arguments: req.Params.Arguments})
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func (s *stub) handler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "stub-mcp", Version: "0.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "Returns the message."},
		func(_ context.Context, req *mcp.CallToolRequest, in echoInput) (*mcp.CallToolResult, any, error) {
			s.record(req)
			return text("echo: " + in.Message), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "delete_thing", Description: "Pretends to delete a thing."},
		func(_ context.Context, req *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
			s.record(req)
			return text("deleted: " + in.ID), nil, nil
		})

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	// Test-only endpoints.
	mux.HandleFunc("GET /calls", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		calls := s.calls
		if calls == nil {
			calls = []call{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(calls)
	})
	mux.HandleFunc("POST /reset", func(http.ResponseWriter, *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls = nil
	})
	return mux
}

func main() {
	log.Printf("stub-mcp listens on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, (&stub{}).handler()))
}
