package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Exercise the HTTP handlers and real session validation, not just cookie parsing.
func TestPortalHTTPAuthorizationAndVisibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, slug TEXT)`,
		`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT, settings TEXT, active BOOLEAN)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, display_name TEXT)`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, customer_email TEXT, subject TEXT, status TEXT, channel TEXT, source TEXT, portal_visible BOOLEAN, created_at DATETIME, last_public_message_at DATETIME, resolved_at DATETIME, deleted_at DATETIME, anonymized_at DATETIME)`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT)`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, customer_email TEXT, identity_trust TEXT, identity_verified_at DATETIME)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, content TEXT, message_type TEXT, sender_type TEXT, is_internal BOOLEAN, created_at DATETIME)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	workspace, otherWorkspace := uuid.NewString(), uuid.NewString()
	identity, otherIdentity := uuid.NewString(), uuid.NewString()
	secret := strings.Repeat("a", 64)
	settings, _ := json.Marshal(map[string]bool{"portal_enabled": true})
	for index, ws := range []string{workspace, otherWorkspace} {
		if err := db.Exec(`INSERT INTO workspaces (id, slug) VALUES (?, ?)`, ws, fmt.Sprintf("space-%d", index)).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO support_widget_installations (id, workspace_id, settings, active) VALUES (?, ?, ?, 1)`, uuid.NewString(), ws, string(settings)).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, email string }{{identity, "customer@example.com"}, {otherIdentity, "other@example.com"}} {
		if err := db.Exec(`INSERT INTO support_portal_identities (id, workspace_id, email) VALUES (?, ?, ?)`, row.id, workspace, row.email).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO support_portal_sessions (id, workspace_id, identity_id, token_hash, expires_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), workspace, identity, fmt.Sprintf("%x", sha256.Sum256([]byte(secret))), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		reference, email, owner, status, channel string
		visible                                  bool
	}{
		{"visible", "customer@example.com", identity, "open", "email", true},
		{"other", "other@example.com", otherIdentity, "open", "email", true},
		{"hidden", "customer@example.com", identity, "open", "email", false},
		{"spam", "customer@example.com", identity, "spam", "email", true},
		{"internal", "customer@example.com", identity, "open", "internal", true},
		{"deleted", "customer@example.com", identity, "open", "email", true},
	} {
		convID := uuid.NewString()
		var deleted any
		if row.reference == "deleted" {
			deleted = time.Now()
		}
		if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, customer_email, subject, status, channel, source, portal_visible, created_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, convID, workspace, row.email, "subject-"+row.reference, row.status, row.channel, row.channel, row.visible, time.Now(), deleted).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO support_portal_request_references (id, workspace_id, conversation_id, portal_identity_id, reference) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), workspace, convID, row.owner, row.reference).Error; err != nil {
			t.Fatal(err)
		}
	}
	inbox := service.NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewCustomerPortalHandler(service.NewPortalAuthService(repository.NewPortalAuthRepository(db), inbox, nil, ""))
	router := chi.NewRouter()
	router.Route("/api/public/portal/{slug}", func(r chi.Router) {
		r.Get("/session", h.Session)
		r.Get("/requests", h.Requests)
		r.Get("/requests/{reference}", h.RequestDetail)
	})
	for _, credential := range []struct {
		name     string
		decorate func(*http.Request)
		allowed  bool
	}{
		{"anonymous", func(*http.Request) {}, false},
		{"workspace bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+secret) }, false},
		{"portal bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+secret) }, false},
		{"widget cookie", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "widget_session", Value: secret}) }, false},
		{"portal cookie", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: portalCookie, Value: secret}) }, true},
	} {
		t.Run(credential.name, func(t *testing.T) {
			paths := []string{"/session", "/requests", "/requests/visible"}
			if credential.allowed {
				paths = paths[:2]
			} // Detail success requires the full inbox projection schema.
			for _, path := range paths {
				r := httptest.NewRequest(http.MethodGet, "/api/public/portal/space-0"+path, nil)
				credential.decorate(r)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				if credential.allowed && w.Code != http.StatusOK || !credential.allowed && w.Code != http.StatusUnauthorized {
					t.Fatalf("%s: status %d body %s", path, w.Code, w.Body.String())
				}
			}
		})
	}
	for _, path := range []string{"/api/public/portal/space-1/session", "/api/public/portal/space-0/requests/other", "/api/public/portal/space-0/requests/hidden", "/api/public/portal/space-0/requests/spam", "/api/public/portal/space-0/requests/internal", "/api/public/portal/space-0/requests/deleted"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(&http.Cookie{Name: portalCookie, Value: secret})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		want := http.StatusNotFound
		if strings.HasSuffix(path, "/session") {
			want = http.StatusUnauthorized
		}
		if w.Code != want || strings.Contains(w.Body.String(), "subject-") {
			t.Errorf("%s: status %d body %s", path, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/public/portal/space-0/requests", nil)
	r.AddCookie(&http.Cookie{Name: portalCookie, Value: secret})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "subject-visible") {
		t.Fatalf("list status %d body %s", w.Code, w.Body.String())
	}
	for _, hidden := range []string{"subject-other", "subject-hidden", "subject-spam", "subject-internal", "subject-deleted"} {
		if strings.Contains(w.Body.String(), hidden) {
			t.Errorf("list leaked %s", hidden)
		}
	}
}
