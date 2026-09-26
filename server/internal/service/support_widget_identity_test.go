package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestVerifyWidgetIdentity(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	installation := &model.SupportWidgetInstallation{
		WidgetKey:                "widget-key",
		SecretKey:                "signing-secret",
		IdentityVerificationMode: model.IdentityVerificationModeEnforced,
	}
	identity := model.WidgetIdentityPayload{
		Email:          " Buyer@Example.com ",
		ExternalUserID: "user-123",
		Company:        model.JSONB{"id": "company-456"},
	}
	identity.IdentityVerification = signWidgetIdentityForTest(t, installation, identity, now, now.Add(15*time.Minute))

	provenance, err := verifyWidgetIdentity(installation, identity, now)
	if err != nil {
		t.Fatalf("verify signed identity: %v", err)
	}
	if provenance.method != model.IdentityMethodSignedWidget || provenance.trust != model.IdentityTrustVerified {
		t.Fatalf("provenance = %#v", provenance)
	}

	t.Run("tampered company is rejected", func(t *testing.T) {
		tampered := identity
		tampered.Company = model.JSONB{"id": "other-company"}
		if _, err := verifyWidgetIdentity(installation, tampered, now); err == nil {
			t.Fatal("expected tampered company to be rejected")
		}
	})

	t.Run("expired signature is rejected", func(t *testing.T) {
		expired := identity
		expired.IdentityVerification = signWidgetIdentityForTest(
			t, installation, identity, now.Add(-20*time.Minute), now.Add(-5*time.Minute),
		)
		if _, err := verifyWidgetIdentity(installation, expired, now); err == nil {
			t.Fatal("expected expired signature to be rejected")
		}
	})

	t.Run("report only preserves untrusted claim", func(t *testing.T) {
		reportOnly := *installation
		reportOnly.IdentityVerificationMode = model.IdentityVerificationModeReportOnly
		unsigned := identity
		unsigned.IdentityVerification = nil
		got, err := verifyWidgetIdentity(&reportOnly, unsigned, now)
		if err != nil {
			t.Fatalf("report-only verification: %v", err)
		}
		if got.method != model.IdentityMethodBrowserClaim || got.trust != model.IdentityTrustUntrusted {
			t.Fatalf("report-only provenance = %#v", got)
		}
	})
}

func TestListWidgetTokensCarriesCanonicalWorkspaceAndOrigins(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:widget-token-registry?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_widget_installations (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, widget_key TEXT NOT NULL,
		secret_key TEXT NOT NULL, allowed_origins TEXT NOT NULL DEFAULT '{}',
		identity_verification_mode TEXT NOT NULL DEFAULT 'report_only', settings TEXT NOT NULL DEFAULT '{}',
		active BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create installations: %v", err)
	}
	repo := repository.NewSupportInboxInstallationRepository(db)
	installation := &model.SupportWidgetInstallation{
		ID:                       "installation-1",
		WorkspaceID:              "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA",
		WidgetKey:                "widget-key",
		SecretKey:                "server-secret",
		AllowedOrigins:           model.DocsStringArray{"https://app.example.com"},
		IdentityVerificationMode: model.IdentityVerificationModeReportOnly,
		Settings:                 "{}",
		Active:                   true,
	}
	if err := repo.Create(t.Context(), installation); err != nil {
		t.Fatalf("create installation: %v", err)
	}
	service := &SupportInboxService{installationRepo: repo}
	tokens, err := service.ListWidgetTokens(t.Context())
	if err != nil {
		t.Fatalf("list widget tokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].WorkspaceID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if len(tokens[0].Origins) != 1 || tokens[0].Origins[0] != "https://app.example.com" {
		t.Fatalf("origins = %#v", tokens[0].Origins)
	}

	if err := db.Model(&model.SupportWidgetInstallation{}).
		Where("id = ?", installation.ID).Update("workspace_id", "not-a-uuid").Error; err != nil {
		t.Fatalf("corrupt workspace ID: %v", err)
	}
	if _, err := service.ListWidgetTokens(t.Context()); err == nil {
		t.Fatal("expected invalid workspace ID to fail closed")
	}
}

func TestNormalizeAllowedOrigins(t *testing.T) {
	if origins, err := normalizeAllowedOrigins([]string{" * ", "*"}); err != nil || len(origins) != 1 || origins[0] != "*" {
		t.Fatalf("explicit allow-all policy: %v, %v", origins, err)
	}
	origins, err := normalizeAllowedOrigins([]string{
		"HTTPS://APP.EXAMPLE.COM", "https://app.example.com", "http://localhost:3000", "TAURI://LOCALHOST", "tauri://localhost",
	})
	if err != nil {
		t.Fatalf("normalize origins: %v", err)
	}
	if got := strings.Join(origins, ","); got != "http://localhost:3000,https://app.example.com,tauri://localhost" {
		t.Fatalf("origins = %q", got)
	}
	for _, invalid := range []string{"https://*.example.com", "https://example.com/path"} {
		if _, err := normalizeAllowedOrigins([]string{invalid}); err == nil {
			t.Errorf("expected %q to be rejected", invalid)
		}
	}
}

func signWidgetIdentityForTest(
	t *testing.T,
	installation *model.SupportWidgetInstallation,
	identity model.WidgetIdentityPayload,
	issuedAt, expiresAt time.Time,
) *model.WidgetIdentityVerification {
	t.Helper()
	message := strings.Join([]string{
		"helpin-widget-identity:v1",
		installation.WidgetKey,
		strings.ToLower(strings.TrimSpace(identity.Email)),
		identity.ExternalUserID,
		widgetIdentityValue(identity.Company["id"]),
		fmt.Sprintf("%d", issuedAt.Unix()),
		fmt.Sprintf("%d", expiresAt.Unix()),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(installation.SecretKey))
	if _, err := mac.Write([]byte(message)); err != nil {
		t.Fatalf("sign identity: %v", err)
	}
	return &model.WidgetIdentityVerification{
		Version:   "v1",
		IssuedAt:  issuedAt.Unix(),
		ExpiresAt: expiresAt.Unix(),
		Signature: hex.EncodeToString(mac.Sum(nil)),
	}
}
