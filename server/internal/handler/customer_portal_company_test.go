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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func openCompanyPortalHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, slug TEXT)`,
		`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT, settings TEXT, active BOOLEAN)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, display_name TEXT, crm_contact_id TEXT, UNIQUE(workspace_id, email))`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, first_name TEXT, last_name TEXT, email TEXT, portal_access TEXT, portal_role TEXT, updated_at DATETIME)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, domain TEXT, portal_default_role TEXT, updated_at DATETIME)`,
		`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
		`CREATE TABLE support_portal_company_members (workspace_id TEXT, crm_company_id TEXT, crm_contact_id TEXT, added_by_user_id TEXT, created_at DATETIME, PRIMARY KEY (crm_company_id, crm_contact_id))`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, crm_contact_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME, reconciled_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_conversations (snoozed_until DATETIME, snoozed_by_user_id TEXT, id TEXT PRIMARY KEY, workspace_id TEXT, display_id INTEGER, last_public_message_id TEXT, last_public_sender_type TEXT, customer_awaiting_response BOOLEAN, customer_email TEXT, customer_name TEXT, crm_contact_id TEXT, crm_company_id TEXT, subject TEXT, status TEXT, channel TEXT, source TEXT, portal_visible BOOLEAN, portal_visibility_changed_at DATETIME, primary_recipient_state TEXT, created_at DATETIME, last_public_message_at DATETIME, resolved_at DATETIME, anonymized_at DATETIME)`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT, customer_last_read_at DATETIME, UNIQUE(workspace_id, conversation_id))`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, customer_email TEXT, identity_trust TEXT, identity_verified_at DATETIME)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, origin_conversation_id TEXT, content TEXT, message_type TEXT, sender_type TEXT, is_internal BOOLEAN, metadata TEXT, deleted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_merge_messages (workspace_id TEXT, conversation_id TEXT, message_id TEXT, merge_id TEXT)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func execAll(t *testing.T, db *gorm.DB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("%v\n%s", err, statement)
		}
	}
}

// The portal API exposes a member's company and its requests, attributed to
// the colleague who asked, without changing the personal request list.
func TestPortalHTTPCompanyScope(t *testing.T) {
	db := openCompanyPortalHandlerDB(t)
	secret := strings.Repeat("b", 64)
	settings, _ := json.Marshal(map[string]bool{"portal_enabled": true})
	execAll(t, db,
		`INSERT INTO workspaces VALUES ('ws', 'acme-support')`,
		fmt.Sprintf(`INSERT INTO support_widget_installations VALUES ('install', 'ws', '%s', 1)`, settings),
		`INSERT INTO crm_companies (id, workspace_id, name, portal_default_role) VALUES ('acme', 'ws', 'Acme', 'member')`,
		`INSERT INTO crm_contacts (id, workspace_id, first_name, last_name, email, portal_access) VALUES
			('alice', 'ws', 'Alice', 'Doe', 'alice@acme.test', 'allowed'), ('bob', 'ws', 'Bob', 'Roe', 'bob@acme.test', 'allowed')`,
		`INSERT INTO crm_associations VALUES ('l1', 'ws', 'contact', 'alice', 'company', 'acme'), ('l2', 'ws', 'contact', 'bob', 'company', 'acme')`,
		`INSERT INTO support_portal_company_members (workspace_id, crm_company_id, crm_contact_id) VALUES ('ws', 'acme', 'alice'), ('ws', 'acme', 'bob')`,
		`INSERT INTO support_portal_identities (id, workspace_id, email, crm_contact_id) VALUES ('alice-id', 'ws', 'alice@acme.test', 'alice'), ('bob-id', 'ws', 'bob@acme.test', 'bob')`,
		`INSERT INTO support_conversations (id, workspace_id, display_id, customer_email, crm_contact_id, crm_company_id, subject, status, channel, source, portal_visible, created_at) VALUES
			('c-alice', 'ws', 1, 'alice@acme.test', 'alice', 'acme', 'subject-alice', 'open', 'email', 'email', 1, CURRENT_TIMESTAMP),
			('c-bob', 'ws', 2, 'bob@acme.test', 'bob', 'acme', 'subject-bob', 'open', 'email', 'email', 1, CURRENT_TIMESTAMP)`,
		`INSERT INTO support_portal_request_references (id, workspace_id, conversation_id, portal_identity_id, reference) VALUES
			('r1', 'ws', 'c-alice', 'alice-id', 'req-alice'), ('r2', 'ws', 'c-bob', 'bob-id', 'req-bob')`,
	)
	if err := db.Exec(`INSERT INTO support_portal_sessions (id, workspace_id, identity_id, crm_contact_id, token_hash, expires_at, reconciled_at) VALUES ('s1', 'ws', 'alice-id', 'alice', ?, ?, ?)`,
		fmt.Sprintf("%x", sha256.Sum256([]byte(secret))), time.Now().Add(time.Hour), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	inbox := service.NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewCustomerPortalHandler(service.NewCustomerPortalService(repository.NewCustomerPortalRepository(db), inbox, nil, ""))
	router := chi.NewRouter()
	router.Route("/api/public/portal/{slug}", func(r chi.Router) {
		r.Get("/session", h.Session)
		r.Get("/requests", h.Requests)
	})
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/public/portal/acme-support"+path, nil)
		r.AddCookie(&http.Cookie{Name: portalCookie, Value: secret})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	var session struct {
		Companies []repository.PortalCompanyMembership `json:"companies"`
	}
	w := get("/session")
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &session) != nil {
		t.Fatalf("session: %d %s", w.Code, w.Body.String())
	}
	if len(session.Companies) != 1 || session.Companies[0] != (repository.PortalCompanyMembership{ID: "acme", Name: "Acme", Role: "member"}) {
		t.Fatalf("session companies = %+v", session.Companies)
	}

	w = get("/requests")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "subject-alice") || strings.Contains(w.Body.String(), "subject-bob") {
		t.Fatalf("personal list: %d %s", w.Code, w.Body.String())
	}

	var company []struct {
		Reference   string            `json:"reference"`
		Own         *bool             `json:"own"`
		RequestedBy string            `json:"requested_by"`
		Company     map[string]string `json:"company"`
	}
	w = get("/requests?scope=company&company_id=acme")
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &company) != nil {
		t.Fatalf("company list: %d %s", w.Code, w.Body.String())
	}
	byReference := map[string]int{}
	for i, row := range company {
		byReference[row.Reference] = i
	}
	bob, alice := company[byReference["req-bob"]], company[byReference["req-alice"]]
	if len(company) != 2 || bob.Own == nil || *bob.Own || bob.RequestedBy != "Bob Roe" || bob.Company["name"] != "Acme" || alice.Own == nil || !*alice.Own {
		t.Fatalf("company rows = %+v", company)
	}
	if strings.Contains(w.Body.String(), "c-bob") {
		t.Fatal("company list exposed a conversation id")
	}

	w = get("/requests?scope=everyone")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown scope: %d %s", w.Code, w.Body.String())
	}
	w = get("/requests?scope=company&company_id=" + uuid.NewString())
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("foreign company filter: %d %s", w.Code, w.Body.String())
	}
}

func TestCustomerPortalAdminCompanyErrors(t *testing.T) {
	db := openCompanyPortalHandlerDB(t)
	acme, alice := uuid.NewString(), uuid.NewString()
	execAll(t, db,
		fmt.Sprintf(`INSERT INTO crm_companies (id, workspace_id, name) VALUES ('%s', 'ws', 'Acme')`, acme),
		fmt.Sprintf(`INSERT INTO crm_contacts (id, workspace_id, first_name, email) VALUES ('%s', 'ws', 'Alice', 'alice@acme.test')`, alice),
	)
	h := NewCustomerPortalAdminHandler(service.NewCustomerPortalService(repository.NewCustomerPortalRepository(db), nil, nil, ""))
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(middleware.WithWorkspaceID(r.Context(), "ws")))
		})
	})
	router.Get("/portal/companies/{companyId}", h.CompanyAccess)
	router.Put("/portal/companies/{companyId}", h.UpdateCompanyAccess)
	router.Post("/portal/companies/{companyId}/members/from-domain", h.AddCompanyDomainMembers)
	router.Put("/portal/companies/{companyId}/members/{contactId}", h.UpdateCompanyMember)
	router.Put("/portal/contacts/{contactId}/role", h.UpdateContactRole)
	for _, tt := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"malformed company id", http.MethodGet, "/portal/companies/missing", "", http.StatusNotFound},
		{"unknown company", http.MethodGet, "/portal/companies/" + uuid.NewString(), "", http.StatusNotFound},
		{"update unknown company", http.MethodPut, "/portal/companies/missing", `{"default_role":"member"}`, http.StatusNotFound},
		{"add members to unknown company", http.MethodPost, "/portal/companies/missing/members/from-domain", "", http.StatusNotFound},
		{"member of unknown company", http.MethodPut, "/portal/companies/" + uuid.NewString() + "/members/" + uuid.NewString(), `{"member":true}`, http.StatusNotFound},
		{"member without a decision", http.MethodPut, "/portal/companies/" + acme + "/members/" + alice, `{}`, http.StatusBadRequest},
		{"invalid company role", http.MethodPut, "/portal/companies/" + acme, `{"default_role":"owner"}`, http.StatusBadRequest},
		{"malformed company body", http.MethodPut, "/portal/companies/" + acme, `{`, http.StatusBadRequest},
		{"invalid contact role", http.MethodPut, "/portal/contacts/" + alice + "/role", `{"portal_role":"owner"}`, http.StatusBadRequest},
		{"unknown contact", http.MethodPut, "/portal/contacts/missing/role", `{"portal_role":"member"}`, http.StatusNotFound},
		{"malformed contact body", http.MethodPut, "/portal/contacts/" + alice + "/role", `[]`, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d, want %d: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

// The suggestions route is public: it serves published articles only and
// answers short or unsearchable text with an empty list.
func TestPortalHTTPSuggestionsShape(t *testing.T) {
	db := openCompanyPortalHandlerDB(t)
	settings, _ := json.Marshal(map[string]bool{"portal_enabled": true, "portal_intake_enabled": true})
	execAll(t, db,
		`INSERT INTO workspaces VALUES ('ws', 'acme-support')`,
		fmt.Sprintf(`INSERT INTO support_widget_installations VALUES ('install', 'ws', '%s', 1)`, settings),
	)
	inbox := service.NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewCustomerPortalHandler(service.NewCustomerPortalService(repository.NewCustomerPortalRepository(db), inbox, nil, ""))
	router := chi.NewRouter()
	router.Get("/api/public/portal/{slug}/suggestions", h.Suggestions)
	for _, tt := range []struct {
		path string
		want int
	}{
		{"/api/public/portal/acme-support/suggestions?q=help", http.StatusOK},
		{"/api/public/portal/missing/suggestions?q=export+fails+on+safari", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if w.Code != tt.want {
			t.Fatalf("%s: status %d %s", tt.path, w.Code, w.Body.String())
		}
		if tt.want == http.StatusOK && strings.TrimSpace(w.Body.String()) != `{"articles":[],"search_completed":false}` {
			t.Fatalf("%s: body %s", tt.path, w.Body.String())
		}
	}
}
