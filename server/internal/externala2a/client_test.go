package externala2a

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseCardPrefersJSONRPCInterface(t *testing.T) {
	card, err := ParseCard([]byte(`{
		"name": "Hermes", "description": "Ops agent", "version": "0.21.5",
		"provider": {"organization": "Nous Research"},
		"supportedInterfaces": [
			{"url": "https://hermes.example.com/rest", "protocolBinding": "HTTP+JSON", "protocolVersion": "1.0"},
			{"url": "https://hermes.example.com/a2a", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"}
		],
		"capabilities": {"streaming": true, "pushNotifications": false},
		"skills": [{"id": "triage", "name": "Triage", "description": "Sorts tickets"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if card.InterfaceURL != "https://hermes.example.com/a2a" || card.ProtocolBinding != "JSONRPC" || card.ProtocolVersion != "1.0" {
		t.Fatalf("interface = %#v", card)
	}
	if card.ProviderName != "Nous Research" || !card.Streaming || card.PushNotifications || len(card.Skills) != 1 {
		t.Fatalf("card = %#v", card)
	}
}

func TestParseCardAcceptsV03Card(t *testing.T) {
	card, err := ParseCard([]byte(`{"name": "Legacy", "url": "https://legacy.example.com/a2a", "preferredTransport": "JSONRPC", "protocolVersion": "0.3.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	if card.InterfaceURL != "https://legacy.example.com/a2a" || card.ProtocolVersion != "0.3.0" || card.ProtocolBinding != "JSONRPC" {
		t.Fatalf("card = %#v", card)
	}
	if _, err := ParseCard([]byte(`{"name": "No interface"}`)); err == nil {
		t.Fatal("expected a card without interfaces to be rejected")
	}
}

func TestNormalizeCardURLAppendsWellKnownPath(t *testing.T) {
	client := NewClient(Options{})
	got, err := client.NormalizeCardURL("https://hermes.example.com/agents/")
	if err != nil || got != "https://hermes.example.com/agents/.well-known/agent-card.json" {
		t.Fatalf("normalize base = %q, %v", got, err)
	}
	got, err = client.NormalizeCardURL("https://hermes.example.com/.well-known/agent-card.json")
	if err != nil || got != "https://hermes.example.com/.well-known/agent-card.json" {
		t.Fatalf("normalize full = %q, %v", got, err)
	}
	for _, bad := range []string{"http://hermes.example.com", "ftp://x", "https://user:pw@hermes.example.com", "https://127.0.0.1/", "https://localhost/"} {
		if _, err := client.NormalizeCardURL(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestClientBlocksLoopbackUnlessAllowlisted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"name":"Local","url":"http://127.0.0.1/a2a"}`)
	}))
	defer server.Close()
	cardURL := server.URL + WellKnownCardPath

	if _, _, err := NewClient(Options{}).FetchCard(context.Background(), cardURL, ""); err == nil {
		t.Fatal("expected loopback card URL to be rejected without an allowlist")
	}
	// A public-looking hostname that resolves to loopback is blocked at dial time.
	blocked := NewClient(Options{})
	if _, err := blocked.Download(context.Background(), strings.Replace(server.URL, "127.0.0.1", "localhost", 1), "", "", 10); err == nil {
		t.Fatal("expected localhost download to be rejected")
	}

	host := mustHost(t, server.URL)
	allowed := NewClient(Options{AllowedPrivateHosts: []string{host}})
	card, raw, err := allowed.FetchCard(context.Background(), cardURL, "")
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "Local" || len(raw) == 0 {
		t.Fatalf("card = %#v", card)
	}
}

func TestDownloadEnforcesSizeLimitAndBearerHost(t *testing.T) {
	var sawAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, strings.Repeat("x", 64))
	}))
	defer server.Close()
	host := mustHost(t, server.URL)
	client := NewClient(Options{AllowedPrivateHosts: []string{host}})

	download, err := client.Download(context.Background(), server.URL+"/f.txt", "other.example.com", "secret", 16)
	if err == nil {
		_, err = io.ReadAll(download.Body)
		_ = download.Body.Close()
	}
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("read error = %v, want ErrTooLarge", err)
	}
	if sawAuth != "" {
		t.Fatal("token must not be sent to another host")
	}
	download, err = client.Download(context.Background(), server.URL+"/f.txt", host, "secret", 1024)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(download.Body)
	_ = download.Body.Close()
	if sawAuth != "Bearer secret" {
		t.Fatalf("authorization = %q", sawAuth)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Hostname()
}
