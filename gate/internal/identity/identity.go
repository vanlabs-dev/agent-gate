// Package identity maps gateway metadata to the caller and tenant of a call,
// and validates the bearer tokens of the test approval endpoint.
package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// DefaultCallerClaim is the default metadata key of the caller.
const DefaultCallerClaim = "sub"

// DefaultTenant is the tenant when no tenant claim is configured.
const DefaultTenant = "local"

// Claims names the metadata keys that the gateway forwards. Each key must be
// forwarded by the route, for example `oid: jwt.oid`.
type Claims struct {
	// Caller is the metadata key of the caller. Default "sub".
	Caller string
	// Tenant is the metadata key of the tenant. When empty, StaticTenant is used.
	Tenant string
	// StaticTenant is the tenant when Tenant is empty. Default "local".
	StaticTenant string
}

func (c Claims) callerKey() string {
	if c.Caller == "" {
		return DefaultCallerClaim
	}
	return c.Caller
}

// CallerOf returns the caller value of the claims, or "" when it is missing or
// not a string.
func (c Claims) CallerOf(claims map[string]any) string {
	s, _ := claims[c.callerKey()].(string)
	return s
}

// TenantOf returns the tenant value, or "" when the tenant claim is set and
// missing.
func (c Claims) TenantOf(claims map[string]any) string {
	if c.Tenant != "" {
		s, _ := claims[c.Tenant].(string)
		return s
	}
	if c.StaticTenant == "" {
		return DefaultTenant
	}
	return c.StaticTenant
}

// Verifier validates bearer tokens against one OIDC issuer and a list of
// audiences.
type Verifier struct {
	verifier  *oidc.IDTokenVerifier
	audiences []string
	claims    Claims
}

// NewVerifier discovers the issuer. A token passes when one of its audiences
// is in audiences.
func NewVerifier(ctx context.Context, issuer string, audiences []string, claims Claims) (*Verifier, error) {
	if len(audiences) == 0 {
		return nil, errors.New("identity: no audience configured")
	}
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("identity: %w", err)
	}
	return &Verifier{
		verifier:  p.Verifier(&oidc.Config{SkipClientIDCheck: true}),
		audiences: audiences,
		claims:    claims,
	}, nil
}

// ErrNoToken is returned for a header with no bearer token.
var ErrNoToken = errors.New("no bearer token")

// Approver validates the bearer token in an Authorization header and returns
// the value of the caller claim.
func (v *Verifier) Approver(ctx context.Context, authorization string) (string, error) {
	raw, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || raw == "" {
		return "", ErrNoToken
	}
	tok, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return "", err
	}
	if !slices.ContainsFunc(tok.Audience, func(a string) bool { return slices.Contains(v.audiences, a) }) {
		return "", fmt.Errorf("token audience %v is not in %v", tok.Audience, v.audiences)
	}
	var claims map[string]any
	if err := tok.Claims(&claims); err != nil {
		return "", err
	}
	caller := v.claims.CallerOf(claims)
	if caller == "" {
		return "", fmt.Errorf("token has no %q claim", v.claims.callerKey())
	}
	return caller, nil
}
