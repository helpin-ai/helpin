package service

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/externalmcp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestExternalMCPEncryptionKeyFormats(t *testing.T) {
	raw := "01234567890123456789012345678901"
	for _, value := range []string{raw, base64.StdEncoding.EncodeToString([]byte(raw)), "3031323334353637383930313233343536373839303132333435363738393031"} {
		key, err := parseExternalMCPEncryptionKey(value)
		if err != nil || string(key) != raw {
			t.Fatalf("parse key %q = %q, %v", value, key, err)
		}
	}
	if _, err := parseExternalMCPEncryptionKey("too-short"); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestExternalMCPHeaderValidationBlocksHopByHopHeaders(t *testing.T) {
	if _, err := validateAndMarshalExternalMCPHeaders(map[string]string{"Host": "internal"}); err == nil {
		t.Fatal("expected Host header rejection")
	}
	if payload, err := validateAndMarshalExternalMCPHeaders(map[string]string{"X-API-Key": "secret"}); err != nil || len(payload) == 0 {
		t.Fatalf("valid header rejected: %v", err)
	}
	if _, err := validateAndMarshalExternalMCPHeaders(map[string]string{"Bad Header": "secret"}); err == nil {
		t.Fatal("expected invalid HTTP header name rejection")
	}
}

func TestCustomerIOScopeAndToolPolicy(t *testing.T) {
	if _, err := normalizeExternalMCPScopes(model.ExternalMCPProviderCustomerIO, []string{"read", "admin"}); err == nil {
		t.Fatal("expected unsupported scope rejection")
	}
	if !customerIOReadTools["cio_read_api"] || customerIOReadTools["cio_write_api"] || customerIOReadTools["cio_delete_api"] {
		t.Fatal("Customer.io read/write policy is incorrect")
	}
}

func TestExternalMCPProvidersAlwaysIncludeGenericServer(t *testing.T) {
	service := &ExternalMCPService{}
	providers := service.Providers()
	if len(providers) == 0 || providers[0].Provider != model.ExternalMCPProviderCustom {
		t.Fatalf("generic MCP provider should be first, got %#v", providers)
	}
	if len(providers) != 5 {
		t.Fatalf("expected generic server plus four presets, got %#v", providers)
	}
	for _, provider := range providers[1:] {
		if provider.WebsiteURL == "" || provider.EndpointURL == "" {
			t.Fatalf("preset should include website and endpoint metadata, got %#v", provider)
		}
	}
}

func TestExternalMCPServerNameIsRuntimeSafe(t *testing.T) {
	name := externalMCPServerName("Customer.io — EU Production")
	if !validExternalMCPName.MatchString(name) {
		t.Fatalf("unsafe server name %q", name)
	}
}

func TestExternalMCPCrossHostRedirectIsActionable(t *testing.T) {
	err := &externalmcp.RemoteError{
		Operation:      "connection",
		Status:         307,
		Code:           "remote_redirect",
		RedirectTarget: "https://canonical.mcp.example/mcp",
	}
	message, ok := externalMCPCrossHostRedirectMessage(err)
	if !ok || !strings.Contains(message, "canonical.mcp.example") || !strings.Contains(message, "Credentials were not forwarded") {
		t.Fatalf("externalMCPCrossHostRedirectMessage() = %q, %t", message, ok)
	}

	err.RedirectTarget = "not a URL"
	if message, ok := externalMCPCrossHostRedirectMessage(err); ok || message != "" {
		t.Fatalf("unexpected message for invalid redirect: %q, %t", message, ok)
	}
}

func TestExternalMCPReturnPathRejectsOpenRedirects(t *testing.T) {
	for _, value := range []string{"https://evil.example/settings", "//evil.example/settings", "settings/mcp"} {
		if validExternalMCPReturnPath(value) {
			t.Fatalf("expected return path %q to be rejected", value)
		}
	}
	if !validExternalMCPReturnPath("/w/acme/automation/tools/connections") {
		t.Fatal("expected local automation connections return path to be accepted")
	}
}

func TestExternalMCPClientAuthMethodValidation(t *testing.T) {
	for _, value := range []string{"", "none", "client_secret_basic", "client_secret_post"} {
		if !validExternalMCPClientAuthMethod(value) {
			t.Fatalf("expected %q to be supported", value)
		}
	}
	if validExternalMCPClientAuthMethod("private_key_jwt") {
		t.Fatal("expected unsupported client authentication method rejection")
	}
}

func TestExternalMCPOAuthRedirectURLValidation(t *testing.T) {
	for _, value := range []string{"https://app.example.com/api/external-mcp/oauth/callback", "https://app.example.com/callback?tenant=one"} {
		if !validExternalMCPOAuthRedirectURL(value, false) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"http://app.example.com/callback", "https://user:secret@app.example.com/callback", "https://app.example.com/callback#fragment"} {
		if validExternalMCPOAuthRedirectURL(value, false) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	if !validExternalMCPOAuthRedirectURL("http://127.0.0.1:3000/callback", true) {
		t.Fatal("expected explicit local-development callback to be accepted")
	}
}
