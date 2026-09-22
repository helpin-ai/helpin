package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func testPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	valid := SignWebhookBody("secret", body)
	tests := []struct {
		name      string
		secret    string
		signature string
		want      bool
	}{
		{name: "valid", secret: "secret", signature: valid, want: true},
		{name: "valid with surrounding space", secret: "secret", signature: " " + valid + " ", want: true},
		{name: "wrong secret", secret: "other", signature: valid},
		{name: "empty secret", secret: "", signature: SignWebhookBody("", body)},
		{name: "missing", secret: "secret", signature: ""},
		{name: "sha1 prefix", secret: "secret", signature: strings.Replace(valid, "sha256=", "sha1=", 1)},
		{name: "not hex", secret: "secret", signature: "sha256=zz"},
		{name: "tampered body", secret: "secret", signature: SignWebhookBody("secret", []byte(`{}`))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyWebhookSignature(tt.secret, body, tt.signature); got != tt.want {
				t.Fatalf("VerifyWebhookSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mutableSource struct {
	mu    sync.Mutex
	creds Credentials
}

func (s *mutableSource) Current(context.Context) (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creds, nil
}

func (s *mutableSource) set(creds Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds = creds
}

func TestClientReadsCredentialsFromSourceOnEachCall(t *testing.T) {
	var issuers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims := jwt.RegisteredClaims{}
		if _, _, err := jwt.NewParser().ParseUnverified(raw, &claims); err != nil {
			t.Errorf("parse app jwt: %v", err)
		}
		issuers = append(issuers, claims.Issuer)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
	}))
	defer server.Close()

	source := &mutableSource{}
	client := NewClientWithSource(source)
	client.apiBaseURL = server.URL

	if client.Configured(context.Background()) {
		t.Fatal("expected unconfigured client with empty source")
	}
	if _, err := client.MintInstallationToken(context.Background(), "1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}

	source.set(Credentials{AppID: "111", PrivateKey: testPEM(t)})
	if _, err := client.MintInstallationToken(context.Background(), "1"); err != nil {
		t.Fatalf("mint with first app: %v", err)
	}
	source.set(Credentials{AppID: "222", PrivateKey: testPEM(t)})
	if _, err := client.MintInstallationToken(context.Background(), "1"); err != nil {
		t.Fatalf("mint with rotated app: %v", err)
	}
	if len(issuers) != 2 || issuers[0] != "111" || issuers[1] != "222" {
		t.Fatalf("expected JWT issuers [111 222], got %v", issuers)
	}
}

func TestManifestClientConvert(t *testing.T) {
	privateKey := testPEM(t)
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if strings.Contains(r.URL.Path, "expired") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 4242, "slug": "helpin-acme", "name": "Helpin (acme)",
			"client_id": "Iv1.abc", "client_secret": "cs", "webhook_secret": "whs",
			"pem": privateKey, "html_url": "https://github.com/apps/helpin-acme",
			"owner": map[string]string{"login": "acme", "type": "Organization"},
		})
	}))
	defer server.Close()

	client := NewManifestClient(server.URL, server.Client())
	conversion, err := client.Convert(context.Background(), "code-123")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/app-manifests/code-123/conversions" {
		t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
	}
	creds := conversion.Credentials()
	if creds.AppID != "4242" || creds.Slug != "helpin-acme" || creds.WebhookSecret != "whs" || creds.ClientSecret != "cs" || !creds.Usable() {
		t.Fatalf("unexpected credentials %+v", creds.AppID)
	}
	if conversion.OwnerLogin != "acme" || creds.InstallURL() != "https://github.com/apps/helpin-acme/installations/new" {
		t.Fatalf("unexpected owner/install url: %q %q", conversion.OwnerLogin, creds.InstallURL())
	}

	if _, err := client.Convert(context.Background(), "expired"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
	if _, err := client.Convert(context.Background(), " "); err == nil {
		t.Fatal("expected empty code error")
	}
}
