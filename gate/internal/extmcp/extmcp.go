// Package extmcp adapts the agentgateway ExtMcp gRPC service to the gate core.
// It is the only package in this module that imports the agentgateway bindings.
package extmcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/agentgateway/agentgateway/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/canonical"
	"github.com/vanlabs-dev/agent-gate/gate/internal/identity"
)

// Bound is the time the gate gives itself for one call, below the 5 s gateway
// deadline.
const Bound = 3 * time.Second

// Server implements the ExtMcp gRPC service.
type Server struct {
	api.UnimplementedExtMcpServer

	Gate   *approval.Gate
	Claims identity.Claims
	Log    *slog.Logger
}

var _ api.ExtMcpServer = (*Server)(nil)

// Register registers s on g.
func Register(g *grpc.Server, s *Server) { api.RegisterExtMcpServer(g, s) }

func pass() *api.McpRequestResult {
	return &api.McpRequestResult{Result: &api.McpRequestResult_Pass{Pass: &api.Pass{}}}
}

func deny(reason string) *api.McpRequestResult {
	return &api.McpRequestResult{Result: &api.McpRequestResult_Error{Error: &api.AuthorizationError{
		Code:   api.AuthorizationError_PERMISSION_DENIED,
		Reason: reason,
	}}}
}

func (s *Server) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// call builds the core call from the request. It returns a reason when
// the request cannot be a gated call.
func (s *Server) call(services []string, claims map[string]any) (approval.Call, string) {
	if len(services) != 1 {
		return approval.Call{}, "the call must name exactly one target"
	}
	return approval.Call{
		Tenant: s.Claims.TenantOf(claims),
		Agent:  s.Claims.CallerOf(claims),
		Target: services[0],
		Claims: claims,
	}, ""
}

// CheckRequest decides one MCP request. Methods other than tools/call pass.
// An error from the core is a gRPC error, which the gateway fails closed.
func (s *Server) CheckRequest(ctx context.Context, req *api.McpRequest) (*api.McpRequestResult, error) {
	if req.GetMethod() != "tools/call" {
		return pass(), nil
	}
	c, reason := s.call(req.GetServiceNames(), req.GetMetadataContext().AsMap())
	if reason != "" {
		return deny(reason), nil
	}
	tool, args, err := canonical.Params(req.GetMcpRequest())
	if err != nil {
		return deny(err.Error()), nil
	}
	c.Tool, c.Arguments = tool, args

	ctx, cancel := context.WithTimeout(ctx, Bound)
	defer cancel()
	v, err := s.Gate.Check(ctx, c)
	if err != nil {
		s.log().Error("check failed", "tool", tool, "err", err)
		return nil, status.Error(codes.Unavailable, "gate could not decide")
	}
	s.log().Info("decision", "tool", tool, "agent", c.Agent, "pass", v.Pass, "reason", v.Reason)
	if v.Pass {
		return pass(), nil
	}
	return deny(v.Reason), nil
}

// CheckResponse records the result of a tools/call and passes it.
func (s *Server) CheckResponse(ctx context.Context, resp *api.McpResponse) (*api.McpResponseResult, error) {
	ok := &api.McpResponseResult{Result: &api.McpResponseResult_Pass{Pass: &api.Pass{}}}
	if resp.GetMethod() != "tools/call" {
		return ok, nil
	}
	c, _ := s.call(resp.GetServiceNames(), resp.GetMetadataContext().AsMap())
	if len(resp.GetServiceNames()) > 0 {
		c.Target = resp.GetServiceNames()[0]
	}
	ctx, cancel := context.WithTimeout(ctx, Bound)
	defer cancel()
	if err := s.Gate.Result(ctx, c, resp.GetMcpResponse()); err != nil {
		s.log().Error("result not recorded", "err", err)
		return nil, status.Error(codes.Unavailable, "gate could not record the result")
	}
	return ok, nil
}
