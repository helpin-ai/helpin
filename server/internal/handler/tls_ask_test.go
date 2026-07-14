package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestTLSAskHandlerVerify(t *testing.T) {
	h := newTLSAskTestHandler(t, "docs.customer.com")

	tests := []struct {
		name       string
		domain     string
		wantStatus int
	}{
		{name: "registered custom domain is approved", domain: "docs.customer.com", wantStatus: http.StatusOK},
		{name: "registered domain with mixed case and trailing dot", domain: "DOCS.Customer.COM.", wantStatus: http.StatusOK},
		{name: "first-party helpcenter host is approved", domain: "helpcenter.helpin.ai", wantStatus: http.StatusOK},
		{name: "hosted subdomain suffix is approved", domain: "acme.helpin.center", wantStatus: http.StatusOK},
		{name: "unknown domain is denied", domain: "scanner-junk.example.com", wantStatus: http.StatusNotFound},
		{name: "missing domain is rejected", domain: "", wantStatus: http.StatusBadRequest},
		{name: "ip address is rejected without lookup", domain: "203.0.113.10", wantStatus: http.StatusBadRequest},
		{name: "host with port is rejected", domain: "docs.customer.com:443", wantStatus: http.StatusBadRequest},
		{name: "wildcard is rejected", domain: "*.customer.com", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := "/verify-domain"
			if tt.domain != "" {
				target += "?domain=" + url.QueryEscape(tt.domain)
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()

			h.Verify(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Verify(domain=%q) status = %d, want %d", tt.domain, rec.Code, tt.wantStatus)
			}
		})
	}
}

func newTLSAskTestHandler(t *testing.T, customDomain string) *TLSAskHandler {
	t.Helper()

	dbName := fmt.Sprintf("file:tls_ask_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE docs_helpcenter_configs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		subdomain TEXT NOT NULL,
		custom_domain TEXT
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain, custom_domain) VALUES ('c1', 'w1', 'docs', ?)`,
		customDomain,
	).Error; err != nil {
		t.Fatalf("seed config row: %v", err)
	}

	return NewTLSAskHandler(service.NewTLSAskService(repository.NewDocsHelpcenterRepository(db), nil))
}
