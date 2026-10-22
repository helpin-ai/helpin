package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestNormalizeTLSAskDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain domain", input: "docs.example.com", want: "docs.example.com"},
		{name: "mixed case is lowercased", input: "Docs.Example.COM", want: "docs.example.com"},
		{name: "trailing dot is stripped", input: "docs.example.com.", want: "docs.example.com"},
		{name: "surrounding whitespace is trimmed", input: "  docs.example.com ", want: "docs.example.com"},
		{name: "empty", input: "", wantErr: true},
		{name: "only a dot", input: ".", wantErr: true},
		{name: "bare label", input: "localhost", wantErr: true},
		{name: "ipv4 address", input: "203.0.113.10", wantErr: true},
		{name: "ipv6 address", input: "2001:db8::1", wantErr: true},
		{name: "host with port", input: "docs.example.com:443", wantErr: true},
		{name: "wildcard", input: "*.example.com", wantErr: true},
		{name: "leading dot", input: ".example.com", wantErr: true},
		{name: "empty label", input: "docs..example.com", wantErr: true},
		{name: "underscore", input: "_dmarc.example.com", wantErr: true},
		{name: "slash", input: "example.com/path", wantErr: true},
		{name: "label starts with hyphen", input: "-docs.example.com", wantErr: true},
		{name: "label ends with hyphen", input: "docs-.example.com", wantErr: true},
		{name: "hyphen inside label is fine", input: "my-docs.example.com", want: "my-docs.example.com"},
		{name: "too long", input: longDomain(260), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTLSAskDomain(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeTLSAskDomain(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeTLSAskDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTLSAskServiceAllowed(t *testing.T) {
	svc, _ := newTLSAskTestService(t, "docs.customer.com", nil)
	ctx := context.Background()

	assertAllowed := func(domain string, want bool) {
		t.Helper()
		got, err := svc.Allowed(ctx, domain)
		if err != nil {
			t.Fatalf("Allowed(%q): %v", domain, err)
		}
		if got != want {
			t.Errorf("Allowed(%q) = %v, want %v", domain, got, want)
		}
	}

	assertAllowed("docs.customer.com", true)
	assertAllowed("evil.example.com", false)
	assertAllowed("helpcenter.helpin.ai", true)
	assertAllowed("helpcenter-stage.helpin.ai", true)
	assertAllowed("acme.helpin.center", true)
	assertAllowed("acme.stage.helpin.center", true)
}

func TestTLSAskServiceExtraAllowedDomains(t *testing.T) {
	svc, _ := newTLSAskTestService(t, "docs.customer.com", []string{"status.helpin.ai", "*.whitelabel.example"})
	ctx := context.Background()

	for domain, want := range map[string]bool{
		"status.helpin.ai":          true,
		"tenant.whitelabel.example": true,
		"whitelabel.example":        false,
	} {
		got, err := svc.Allowed(ctx, domain)
		if err != nil {
			t.Fatalf("Allowed(%q): %v", domain, err)
		}
		if got != want {
			t.Errorf("Allowed(%q) = %v, want %v", domain, got, want)
		}
	}
}

func TestTLSAskServiceCachesBothOutcomes(t *testing.T) {
	svc, db := newTLSAskTestService(t, "docs.customer.com", nil)
	ctx := context.Background()

	// Prime the caches.
	if allowed, err := svc.Allowed(ctx, "docs.customer.com"); err != nil || !allowed {
		t.Fatalf("prime positive: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := svc.Allowed(ctx, "unknown.example.com"); err != nil || allowed {
		t.Fatalf("prime negative: allowed=%v err=%v", allowed, err)
	}

	// Flip the database state; cached answers must survive within the TTL.
	if err := db.Exec(`DELETE FROM docs_helpcenter_configs`).Error; err != nil {
		t.Fatalf("delete rows: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain, custom_domain) VALUES ('c2', 'w2', 'unknown', 'unknown.example.com')`,
	).Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if allowed, _ := svc.Allowed(ctx, "docs.customer.com"); !allowed {
		t.Error("positive result was not served from cache")
	}
	if allowed, _ := svc.Allowed(ctx, "unknown.example.com"); allowed {
		t.Error("negative result was not served from cache")
	}

	// After the TTL both answers must follow the new database state.
	svc.now = func() time.Time { return time.Now().Add(2 * _tlsAskCacheTTL) }
	if allowed, _ := svc.Allowed(ctx, "docs.customer.com"); allowed {
		t.Error("expired positive entry still allowed")
	}
	if allowed, _ := svc.Allowed(ctx, "unknown.example.com"); !allowed {
		t.Error("expired negative entry still denied")
	}
}

func newTLSAskTestService(t *testing.T, customDomain string, extraAllowed []string) (*TLSAskService, *gorm.DB) {
	t.Helper()
	db := setupTLSAskTestDB(t)
	if err := db.Exec(
		`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain, custom_domain) VALUES ('c1', 'w1', 'docs', ?)`,
		customDomain,
	).Error; err != nil {
		t.Fatalf("seed config row: %v", err)
	}
	return NewTLSAskService(repository.NewDocsHelpcenterRepository(db), extraAllowed), db
}

func setupTLSAskTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:tls_ask_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.Exec(`CREATE TABLE docs_helpcenter_configs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		subdomain TEXT NOT NULL,
		custom_domain TEXT,
		custom_domain_status TEXT DEFAULT 'verified'
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	return db
}

func longDomain(length int) string {
	labels := make([]byte, 0, length)
	for len(labels) < length {
		labels = append(labels, "abcdefgh."...)
	}
	return string(labels[:length])
}
