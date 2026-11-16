package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEchoIsRecorded(t *testing.T) {
	srv := httptest.NewServer((&stub{}).handler())
	defer srv.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "hi"}})
	if err != nil {
		t.Fatalf("call echo: %v", err)
	}
	if got := res.Content[0].(*mcp.TextContent).Text; res.IsError || got != "echo: hi" {
		t.Fatalf("result: isError=%v text=%q", res.IsError, got)
	}

	resp, err := http.Get(srv.URL + "/calls")
	if err != nil {
		t.Fatalf("get /calls: %v", err)
	}
	defer resp.Body.Close()
	var calls []call
	if err := json.NewDecoder(resp.Body).Decode(&calls); err != nil {
		t.Fatalf("decode /calls: %v", err)
	}
	if len(calls) != 1 || calls[0].Tool != "echo" || string(calls[0].Arguments) != `{"message":"hi"}` {
		t.Fatalf("calls: %+v", calls)
	}
}
