package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func newCompanyScopeHarness(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_companies (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		display_id TEXT NOT NULL,
		external_id TEXT,
		name TEXT NOT NULL,
		domain TEXT,
		industry TEXT,
		employee_count INTEGER,
		annual_revenue REAL,
		description TEXT,
		logo_url TEXT,
		linkedin_url TEXT,
		headquarters TEXT,
		owner_member_id TEXT,
		customer_success_owner_member_id TEXT,
		custom_properties TEXT NOT NULL DEFAULT '{}',
		portal_default_role TEXT,
		portal_admin_invites BOOLEAN NOT NULL DEFAULT 0,
		portal_invite_any_domain BOOLEAN NOT NULL DEFAULT 0,
		support_sla_tier_id TEXT,
		support_sla_tier_changed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create companies table: %v", err)
	}
	h := NewCRMCompanyHandler(service.NewCRMCompanyService(repository.NewCRMCompanyRepository(db)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithWorkspaceID(req.Context(), req.Header.Get("X-Test-Workspace"))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/companies", h.Create)
	r.Get("/companies/{id}", h.Get)
	r.Put("/companies/{id}", h.Update)
	r.Delete("/companies/{id}", h.Delete)
	return db, r
}

func companyScopeRequest(h http.Handler, method, path, workspaceID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Workspace", workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCRMCompanyEndpointsRejectOtherWorkspaces(t *testing.T) {
	db, h := newCompanyScopeHarness(t)
	company := model.CRMCompany{ID: uuid.NewString(), WorkspaceID: "workspace-a", DisplayID: "COM-1", Name: "Acme"}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}
	path := "/companies/" + company.ID

	for _, tc := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPut, `{"name":"Hijacked"}`},
		{http.MethodDelete, ""},
	} {
		if rec := companyScopeRequest(h, tc.method, path, "workspace-b", tc.body); rec.Code != http.StatusNotFound {
			t.Fatalf("%s from another workspace = %d, want 404 (%s)", tc.method, rec.Code, rec.Body.String())
		}
	}
	var stored model.CRMCompany
	if err := db.WithContext(context.Background()).First(&stored, "id = ?", company.ID).Error; err != nil {
		t.Fatalf("company was deleted by another workspace: %v", err)
	}
	if stored.Name != "Acme" {
		t.Fatalf("company name = %q, want unchanged", stored.Name)
	}

	if rec := companyScopeRequest(h, http.MethodGet, path, "workspace-a", ""); rec.Code != http.StatusOK {
		t.Fatalf("GET from own workspace = %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := companyScopeRequest(h, http.MethodDelete, path, "workspace-a", ""); rec.Code != http.StatusOK {
		t.Fatalf("DELETE from own workspace = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCRMCompanyCreateIgnoresForeignWorkspaceInBody(t *testing.T) {
	db, h := newCompanyScopeHarness(t)

	rec := companyScopeRequest(h, http.MethodPost, "/companies", "workspace-a", `{"workspace_id":"workspace-b","name":"Acme"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create with foreign workspace_id = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.CRMCompany{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("companies created = %d, %v; want 0", count, err)
	}
}
