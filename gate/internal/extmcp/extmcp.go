// Package extmcp adapts the agentgateway ExtMcp gRPC service to the gate core.
// It is the only package in this module that imports the agentgateway bindings.
package extmcp

import "github.com/agentgateway/agentgateway/api"

// Server implements the ExtMcp gRPC service.
type Server struct {
	api.UnimplementedExtMcpServer
}

var _ api.ExtMcpServer = (*Server)(nil)
