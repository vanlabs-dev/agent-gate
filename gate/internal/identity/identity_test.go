package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestClaims(t *testing.T) {
	m := map[string]any{"sub": "s-1", "oid": "o-1", "tid": "t-1"}
	if got := (Claims{}).CallerOf(m); got != "s-1" {
		t.Errorf("default claim: %q", got)
	}
	if got := (Claims{Caller: "oid"}).CallerOf(m); got != "o-1" {
		t.Errorf("oid claim: %q", got)
	}
	if got := (Claims{}).CallerOf(map[string]any{"oid": "o-1"}); got != "" {
		t.Errorf("missing claim: %q", got)
	}
	if got := (Claims{}).CallerOf(map[string]any{"sub": 7}); got != "" {
		t.Errorf("non-string claim: %q", got)
	}
	if got := (Claims{}).TenantOf(m); got != "local" {
		t.Errorf("default tenant: %q", got)
	}
	if got := (Claims{StaticTenant: "acme"}).TenantOf(m); got != "acme" {
		t.Errorf("static tenant: %q", got)
	}
	if got := (Claims{Tenant: "tid"}).TenantOf(m); got != "t-1" {
		t.Errorf("tenant claim: %q", got)
	}
	if got := (Claims{Tenant: "tid"}).TenantOf(map[string]any{}); got != "" {
		t.Errorf("missing tenant claim: %q", got)
	}
}

// issuer is a fake OIDC issuer: discovery, JWKS, and a signer.
type issuer struct {
	srv    *httptest.Server
	signer jose.Signer
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	is := &issuer{signer: signer}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": is.srv.URL, "jwks_uri": is.srv.URL + "/keys",
			"authorization_endpoint": is.srv.URL + "/auth", "token_endpoint": is.srv.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	is.srv = httptest.NewServer(mux)
	t.Cleanup(is.srv.Close)
	return is
}

func (is *issuer) token(t *testing.T, iss string, aud []string, extra map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := jwt.Claims{Issuer: iss, Audience: aud, Expiry: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now)}
	s, err := jwt.Signed(is.signer).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifier(t *testing.T) {
	is := newIssuer(t)
	ctx := context.Background()
	v, err := NewVerifier(ctx, is.srv.URL, []string{"agent-gate"}, Claims{})
	if err != nil {
		t.Fatal(err)
	}
	sub := map[string]any{"sub": "approver-sub", "oid": "approver-oid"}

	got, err := v.Approver(ctx, "Bearer "+is.token(t, is.srv.URL, []string{"agent-gate", "approver"}, sub))
	if err != nil || got != "approver-sub" {
		t.Fatalf("valid token: %q %v", got, err)
	}
	if _, err := v.Approver(ctx, ""); !errors.Is(err, ErrNoToken) {
		t.Errorf("no token: %v", err)
	}
	if _, err := v.Approver(ctx, "Bearer "+is.token(t, is.srv.URL, []string{"other"}, sub)); err == nil {
		t.Error("token with an audience outside the list passed")
	}
	if _, err := v.Approver(ctx, "Bearer "+is.token(t, "https://unknown.example", []string{"agent-gate"}, sub)); err == nil {
		t.Error("token from an unknown issuer passed")
	}
	if _, err := v.Approver(ctx, "Bearer "+is.token(t, is.srv.URL, []string{"agent-gate"}, map[string]any{})); err == nil {
		t.Error("token with no caller claim passed")
	}

	oid, err := NewVerifier(ctx, is.srv.URL, []string{"agent-gate"}, Claims{Caller: "oid"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := oid.Approver(ctx, "Bearer "+is.token(t, is.srv.URL, []string{"agent-gate"}, sub)); err != nil || got != "approver-oid" {
		t.Fatalf("oid claim: %q %v", got, err)
	}
}

func TestNoAudience(t *testing.T) {
	if _, err := NewVerifier(context.Background(), "http://unused", nil, Claims{}); err == nil {
		t.Fatal("want an error")
	}
}
