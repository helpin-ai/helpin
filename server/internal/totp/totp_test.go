package totp

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	first, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	second, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() second call error = %v", err)
	}

	if first == "" || second == "" {
		t.Fatal("expected non-empty secrets")
	}
	if first == second {
		t.Fatal("expected different secrets across calls")
	}
}

func TestBuildProvisioningURI(t *testing.T) {
	got := BuildProvisioningURI("JBSWY3DPEHPK3PXP", "alice@example.com", "Helpin")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if parsed.Scheme != "otpauth" {
		t.Fatalf("expected otpauth scheme, got %q", parsed.Scheme)
	}
	if parsed.Host != "totp" {
		t.Fatalf("expected totp host, got %q", parsed.Host)
	}
	if !strings.Contains(parsed.Path, "Helpin:alice@example.com") {
		t.Fatalf("expected issuer/account label in path, got %q", parsed.Path)
	}
	if parsed.Query().Get("secret") != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("expected secret query param, got %q", parsed.Query().Get("secret"))
	}
	if parsed.Query().Get("issuer") != "Helpin" {
		t.Fatalf("expected issuer query param, got %q", parsed.Query().Get("issuer"))
	}
}

func TestValidateOTP(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1_700_000_000, 0).UTC()

	validCode, err := generateCodeAt(secret, now)
	if err != nil {
		t.Fatalf("generateCodeAt() error = %v", err)
	}
	if !validateOTPAt(secret, validCode, 1, now) {
		t.Fatal("expected current code to validate")
	}

	previousCode, err := generateCodeAt(secret, now.Add(-defaultPeriod))
	if err != nil {
		t.Fatalf("generate previous code: %v", err)
	}
	if !validateOTPAt(secret, previousCode, 1, now) {
		t.Fatal("expected previous window code to validate with skew")
	}
	if validateOTPAt(secret, previousCode, 0, now) {
		t.Fatal("expected previous window code to fail without skew")
	}

	if validateOTPAt(secret, "000000", 1, now) {
		t.Fatal("expected random code to fail")
	}
}
