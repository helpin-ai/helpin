package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	fakeOIDCClientID     = "helpin-test"
	fakeOIDCClientSecret = "s3cret"
	fakeOIDCRedirectURL  = "http://helpin.test/api/auth/oidc/callback"
	fakeOIDCCode         = "good-code"
)

// fakeOIDCProvider is an httptest identity provider: discovery, JWKS, token
// and userinfo endpoints, with ID tokens signed RS256 via go-jose. The knobs
// are set per test before the flow runs.
type fakeOIDCProvider struct {
	srv *httptest.Server
	key *rsa.PrivateKey

	mu sync.Mutex
	// Knobs.
	down           bool   // discovery answers 503
	audience       string // ID token audience
	nonce          string // ID token nonce; the flow helper fills it from the start redirect
	subject        string
	email          string
	emailVerified  bool
	name           string
	emailInIDToken bool // otherwise the email is only served by userinfo (Authelia 4.39)
	// verifiedAsString sends email_verified in the ID token as "true", as Cognito does.
	verifiedAsString bool
	// Observations.
	challenge   string // code_challenge from the authorize redirect, set by the flow helper
	tokenCalls  int
	userinfoHit int
}

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	p := &fakeOIDCProvider{
		key: key, audience: fakeOIDCClientID, subject: "authelia-user-1",
		email: "sso@example.com", emailVerified: true, name: "Sso Person",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("/jwks", p.jwks)
	mux.HandleFunc("/token", p.token)
	mux.HandleFunc("/userinfo", p.userinfo)
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

// set changes knobs under the provider lock.
func (p *fakeOIDCProvider) set(change func(p *fakeOIDCProvider)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	change(p)
}

func (p *fakeOIDCProvider) discovery(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	down := p.down
	p.mu.Unlock()
	if down {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                p.srv.URL,
		"authorization_endpoint":                p.srv.URL + "/authorize",
		"token_endpoint":                        p.srv.URL + "/token",
		"jwks_uri":                              p.srv.URL + "/jwks",
		"userinfo_endpoint":                     p.srv.URL + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
	})
}

func (p *fakeOIDCProvider) jwks(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &p.key.PublicKey, KeyID: "k1", Algorithm: string(jose.RS256), Use: "sig"},
	}})
}

func (p *fakeOIDCProvider) token(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenCalls++
	invalidGrant := func(why string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": why})
	}
	if id, secret, ok := r.BasicAuth(); !ok || id != fakeOIDCClientID || secret != fakeOIDCClientSecret {
		http.Error(w, "bad client credentials", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code") != fakeOIDCCode {
		invalidGrant("bad code")
		return
	}
	verifier := r.PostForm.Get("code_verifier")
	if verifier == "" {
		invalidGrant("code_verifier is required")
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != p.challenge {
		invalidGrant("code_verifier does not match code_challenge")
		return
	}

	claims := map[string]any{
		"iss": p.srv.URL, "sub": p.subject, "aud": p.audience, "nonce": p.nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	if p.emailInIDToken {
		claims["email"], claims["email_verified"], claims["name"] = p.email, p.emailVerified, p.name
		if p.verifiedAsString {
			claims["email_verified"] = fmt.Sprint(p.emailVerified)
		}
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: p.key, KeyID: "k1"}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idToken, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "access-1", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken,
	})
}

func (p *fakeOIDCProvider) userinfo(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userinfoHit++
	if r.Header.Get("Authorization") != "Bearer access-1" {
		http.Error(w, "bad token", http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sub": p.subject, "email": p.email, "email_verified": p.emailVerified, "name": p.name,
	})
}
