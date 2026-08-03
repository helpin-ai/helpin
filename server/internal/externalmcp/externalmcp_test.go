package externalmcp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewClientRejectsUnapprovedAndInsecureEndpoints(t *testing.T) {
	if _, err := NewClient("http://mcp.example.com/mcp", []string{"mcp.example.com"}, false); err == nil {
		t.Fatal("expected insecure endpoint rejection")
	}
	if _, err := NewClient("https://other.example.com/mcp", []string{"mcp.example.com"}, false); err == nil {
		t.Fatal("expected host allowlist rejection")
	}
	if _, err := NewClient("https://user:secret@mcp.example.com/mcp", []string{"mcp.example.com"}, false); err == nil {
		t.Fatal("expected URL userinfo rejection")
	}
}

func TestResolvedHostRejectsPrivateAndSharedAddresses(t *testing.T) {
	if _, err := resolveAllowedHost(context.Background(), "127.0.0.1", false); err == nil {
		t.Fatal("expected loopback to be rejected")
	}
	if addrs, err := resolveAllowedHost(context.Background(), "127.0.0.1", true); err != nil || len(addrs) == 0 {
		t.Fatalf("expected explicitly enabled loopback, addrs=%#v err=%v", addrs, err)
	}
	if !isNonPublicIP(net.ParseIP("100.64.0.1")) {
		t.Fatal("expected shared carrier space to be rejected")
	}
	if isNonPublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public address to be allowed")
	}
}

func TestRelatedOAuthURLAllowsSameSiteOrExplicitHost(t *testing.T) {
	client, err := NewClient("https://mcp.example.com/mcp", []string{"mcp.example.com", "login.identity.test"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ValidateRelatedURL("https://auth.example.com/authorize"); err != nil {
		t.Fatalf("same-site OAuth host rejected: %v", err)
	}
	if err := client.ValidateRelatedURL("https://login.identity.test/authorize"); err != nil {
		t.Fatalf("explicit OAuth host rejected: %v", err)
	}
	if err := client.ValidateRelatedURL("https://untrusted.test/authorize"); err == nil {
		t.Fatal("expected unrelated OAuth host rejection")
	}
}

func TestOAuthDiscoveryRegistrationAndTokenExchange(t *testing.T) {
	var baseURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+baseURL+`/.well-known/oauth-protected-resource/mcp"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource/mcp":
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": baseURL + "/mcp", "authorization_servers": []string{baseURL}, "scopes_supported": []string{"read"}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": baseURL, "authorization_endpoint": baseURL + "/authorize", "token_endpoint": baseURL + "/token",
				"registration_endpoint": baseURL + "/register", "code_challenge_methods_supported": []string{"S256"},
			})
		case "/register":
			_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "helpin-client", "token_endpoint_auth_method": "none"})
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("resource") != baseURL+"/mcp" || len(r.Form.Get("code_verifier")) < 43 {
				t.Errorf("unexpected token request: %v, %#v", err, r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-secret", "refresh_token": "refresh-secret", "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	client, err := NewClient(server.URL+"/mcp", []string{"127.0.0.1"}, true)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	discovery, err := client.DiscoverOAuth(context.Background())
	if err != nil {
		t.Fatalf("discover OAuth: %v", err)
	}
	if discovery.Authorization.TokenEndpoint != server.URL+"/token" {
		t.Fatalf("unexpected token endpoint: %q", discovery.Authorization.TokenEndpoint)
	}
	registration, err := client.RegisterOAuthClient(context.Background(), discovery.Authorization.RegistrationEndpoint, "https://app.example.com/callback")
	if err != nil || registration.ClientID != "helpin-client" {
		t.Fatalf("registration = %#v, %v", registration, err)
	}
	authorization, err := client.NewOAuthAuthorizationRequest(discovery, registration.ClientID, "https://app.example.com/callback", []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.ExchangeCode(context.Background(), discovery.Authorization.TokenEndpoint, registration.ClientID, "", "none", "code", authorization.Verifier, "https://app.example.com/callback", server.URL+"/mcp")
	if err != nil || token.AccessToken != "access-secret" || token.ExpiresAt == nil {
		t.Fatalf("token = %#v, %v", token, err)
	}
}

func TestAuthorizationURLUsesPKCES256AndResourceAudience(t *testing.T) {
	client, err := NewClient("https://mcp.example.com/mcp", []string{"mcp.example.com"}, false)
	if err != nil {
		t.Fatal(err)
	}
	request, err := client.NewOAuthAuthorizationRequest(&OAuthConfiguration{
		Resource: ProtectedResourceMetadata{Resource: "https://mcp.example.com/mcp"},
		Authorization: AuthorizationServerMetadata{
			Issuer: "https://auth.example.com", AuthorizationEndpoint: "https://auth.example.com/authorize",
		},
	}, "client", "https://app.example.com/callback", []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(request.URL)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("resource") != "https://mcp.example.com/mcp" || q.Get("scope") != "read write" {
		t.Fatalf("unexpected authorization query: %s", u.RawQuery)
	}
}

func TestTokenRequestSupportsClientSecretPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret" {
			t.Fatalf("unexpected client authentication: %#v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "token_type": "Bearer"})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, []string{"127.0.0.1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	if _, err := client.ExchangeCode(context.Background(), server.URL, "client", "secret", "client_secret_post", "code", verifier, "https://app.example/callback", server.URL); err != nil {
		t.Fatalf("client_secret_post exchange failed: %v", err)
	}
}

func TestListToolsForwardsCredentialAndClassifiesReadOnlyHint(t *testing.T) {
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "read_customer", Description: "Read a customer", InputSchema: map[string]any{"type": "object"}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		return &mcp.CallToolResult{}, map[string]any{}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	client, err := NewClient(httpServer.URL, []string{"127.0.0.1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(context.Background(), http.Header{"Authorization": []string{"Bearer test-secret"}})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "read_customer" || !tools[0].ReadOnly || tools[0].SchemaHash == "" || !strings.Contains(string(tools[0].InputSchema), "object") {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestListToolsBlocksAndReportsCrossHostRedirect(t *testing.T) {
	var requests atomic.Int32
	var redirectedRequests atomic.Int32
	var redirectTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Host == strings.TrimPrefix(redirectTarget, "http://") {
			redirectedRequests.Add(1)
		}
		http.Redirect(w, r, redirectTarget+"/mcp", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	redirectTarget = server.URL
	endpoint := strings.Replace(server.URL, "127.0.0.1", "localhost", 1) + "/mcp"

	client, err := NewClient(endpoint, []string{"localhost", "127.0.0.1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListTools(context.Background(), http.Header{"Authorization": []string{"Bearer test-secret"}})
	var remoteErr *RemoteError
	if !errors.As(err, &remoteErr) {
		t.Fatalf("ListTools error = %T %v, want RemoteError", err, err)
	}
	if remoteErr.Status != http.StatusTemporaryRedirect || remoteErr.Code != "remote_redirect" ||
		RedirectTarget(err) != redirectTarget+"/mcp" {
		t.Fatalf("unexpected redirect error: %#v", remoteErr)
	}
	if requests.Load() == 0 || redirectedRequests.Load() != 0 {
		t.Fatalf("request count = %d, redirected requests = %d; redirect must remain blocked", requests.Load(), redirectedRequests.Load())
	}
}
