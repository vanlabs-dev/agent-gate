// Command agent-gate is the gate: an agentgateway ExtMcp server that decides
// each MCP tool call with policy, holds calls that need approval, and writes
// a hash-chained audit log to Postgres.
//
// Each flag also reads an environment variable, named in its help text. The
// flag wins.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/vanlabs-dev/agent-gate/gate/internal/approval"
	"github.com/vanlabs-dev/agent-gate/gate/internal/extmcp"
	"github.com/vanlabs-dev/agent-gate/gate/internal/identity"
	"github.com/vanlabs-dev/agent-gate/gate/internal/policy"
	"github.com/vanlabs-dev/agent-gate/gate/internal/postgres"
)

type config struct {
	GRPCAddr        string
	HTTPAddr        string
	PolicyDir       string
	DSN             string
	CallerClaim     string
	TenantClaim     string
	Tenant          string
	Expiry          time.Duration
	LogID           string
	CheckpointEvery int
	TestEndpoints   bool
	OIDCIssuer      string
	OIDCAudiences   []string
}

func env(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func parse(args []string, stderr io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet("agent-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.GRPCAddr, "grpc-addr", env("AGENT_GATE_GRPC_ADDR", ":9001"), "ExtMcp gRPC listen address (AGENT_GATE_GRPC_ADDR)")
	fs.StringVar(&c.HTTPAddr, "http-addr", env("AGENT_GATE_HTTP_ADDR", ":8081"), "test endpoint listen address (AGENT_GATE_HTTP_ADDR)")
	fs.StringVar(&c.PolicyDir, "policy-dir", env("AGENT_GATE_POLICY_DIR", "/etc/agent-gate/policy"), "directory of .rego files (AGENT_GATE_POLICY_DIR)")
	fs.StringVar(&c.DSN, "dsn", env("AGENT_GATE_DSN", ""), "Postgres connection string (AGENT_GATE_DSN)")
	fs.StringVar(&c.CallerClaim, "caller-claim", env("AGENT_GATE_CALLER_CLAIM", identity.DefaultCallerClaim), "metadata key of the caller (AGENT_GATE_CALLER_CLAIM)")
	fs.StringVar(&c.TenantClaim, "tenant-claim", env("AGENT_GATE_TENANT_CLAIM", ""), "metadata key of the tenant; empty uses -tenant (AGENT_GATE_TENANT_CLAIM)")
	fs.StringVar(&c.Tenant, "tenant", env("AGENT_GATE_TENANT", identity.DefaultTenant), "fixed tenant when no tenant claim is set (AGENT_GATE_TENANT)")
	expiry := fs.String("expiry", env("AGENT_GATE_EXPIRY", approval.DefaultExpiry.String()), "approval expiry (AGENT_GATE_EXPIRY)")
	fs.StringVar(&c.LogID, "log-id", env("AGENT_GATE_LOG_ID", "local"), "audit log id (AGENT_GATE_LOG_ID)")
	every := fs.String("checkpoint-every", env("AGENT_GATE_CHECKPOINT_EVERY", fmt.Sprint(postgres.DefaultCheckpointEvery)), "entries between checkpoints (AGENT_GATE_CHECKPOINT_EVERY)")
	fs.BoolVar(&c.TestEndpoints, "test-endpoints", env("AGENT_GATE_TEST_ENDPOINTS", "") == "true", "serve the test approval endpoint (AGENT_GATE_TEST_ENDPOINTS=true)")
	fs.StringVar(&c.OIDCIssuer, "oidc-issuer", env("AGENT_GATE_OIDC_ISSUER", ""), "issuer of test endpoint tokens (AGENT_GATE_OIDC_ISSUER)")
	audiences := fs.String("oidc-audiences", env("AGENT_GATE_OIDC_AUDIENCES", ""), "comma-separated accepted audiences (AGENT_GATE_OIDC_AUDIENCES)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	var err error
	if c.Expiry, err = time.ParseDuration(*expiry); err != nil || c.Expiry <= 0 {
		return c, fmt.Errorf("invalid -expiry %q", *expiry)
	}
	if _, err := fmt.Sscan(*every, &c.CheckpointEvery); err != nil || c.CheckpointEvery <= 0 {
		return c, fmt.Errorf("invalid -checkpoint-every %q", *every)
	}
	for _, a := range strings.Split(*audiences, ",") {
		if a = strings.TrimSpace(a); a != "" {
			c.OIDCAudiences = append(c.OIDCAudiences, a)
		}
	}
	if c.DSN == "" {
		return c, errors.New("-dsn is required")
	}
	if c.TestEndpoints && (c.OIDCIssuer == "" || len(c.OIDCAudiences) == 0) {
		return c, errors.New("-test-endpoints needs -oidc-issuer and -oidc-audiences")
	}
	return c, nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		slog.Error("agent-gate", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parse(args, os.Stderr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	p, err := policy.Load(ctx, cfg.PolicyDir)
	if err != nil {
		return err
	}
	store, err := postgres.Open(ctx, cfg.DSN, cfg.LogID, cfg.CheckpointEvery)
	if err != nil {
		return err
	}
	defer store.Close()
	claims := identity.Claims{Caller: cfg.CallerClaim, Tenant: cfg.TenantClaim, StaticTenant: cfg.Tenant}
	var auth authenticator
	if cfg.TestEndpoints {
		if auth, err = identity.NewVerifier(ctx, cfg.OIDCIssuer, cfg.OIDCAudiences, claims); err != nil {
			return err
		}
	}
	gate := &approval.Gate{Policy: p, Store: store, Expiry: cfg.Expiry}
	slog.Info("agent-gate starts", "policy_version", p.Version(), "test_endpoints", cfg.TestEndpoints)
	return serve(ctx, cfg, gate, claims, auth)
}

// authenticator returns the approver of an Authorization header.
type authenticator interface {
	Approver(ctx context.Context, authorization string) (string, error)
}

// serve runs the gRPC listener, and the test HTTP listener when auth is not
// nil, until ctx ends or a listener fails.
func serve(ctx context.Context, cfg config, gate *approval.Gate, claims identity.Claims, auth authenticator) error {
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	g := grpc.NewServer()
	extmcp.Register(g, &extmcp.Server{Gate: gate, Claims: claims})
	errc := make(chan error, 2)
	go func() { errc <- g.Serve(lis) }()
	defer g.Stop()

	if auth != nil {
		hl, err := net.Listen("tcp", cfg.HTTPAddr)
		if err != nil {
			return err
		}
		h := &http.Server{Handler: testHandler(gate, auth), ReadHeaderTimeout: 5 * time.Second}
		go func() { errc <- h.Serve(hl) }()
		defer h.Close()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errc:
		return err
	}
}

// testHandler serves the test approval endpoint. approval-inbox replaces it.
func testHandler(gate *approval.Gate, auth authenticator) http.Handler {
	decide := func(approve bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			approver, err := auth.Approver(r.Context(), r.Header.Get("Authorization"))
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			req, err := gate.Decide(r.Context(), r.PathValue("ref"), approver, approve)
			switch {
			case errors.Is(err, approval.ErrNotFound):
				http.Error(w, err.Error(), http.StatusNotFound)
			case errors.Is(err, approval.ErrSelfApproval), errors.Is(err, approval.ErrNotPending), errors.Is(err, approval.ErrExpired):
				http.Error(w, err.Error(), http.StatusConflict)
			case err != nil:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			default:
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"ref": req.Ref, "status": string(req.Status), "approver": req.Approver})
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /test/requests/{ref}/approve", decide(true))
	mux.HandleFunc("POST /test/requests/{ref}/reject", decide(false))
	return mux
}
