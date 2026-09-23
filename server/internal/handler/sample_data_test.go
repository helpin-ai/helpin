package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type handlerSampleSeeder struct{}

func (handlerSampleSeeder) Module() model.ModuleID { return model.ModuleCRM }

func (handlerSampleSeeder) Seed(_ context.Context, env *service.SampleDataEnv) error {
	id := uuid.NewString()
	if err := env.Tx.Exec(`INSERT INTO crm_contacts (id, workspace_id, first_name) VALUES (?, ?, 'Maya')`, id, env.WorkspaceID).Error; err != nil {
		return err
	}
	env.Track(model.SampleEntityCRMContact, id)
	return nil
}

func setupSampleDataHandler(t *testing.T, modules map[model.ModuleID]bool) *SampleDataHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sample_data_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE workspace_members (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT, email TEXT NOT NULL, display_name TEXT NOT NULL, role TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status) VALUES ('member-1', 'ws', 'u1', 'owner@example.com', 'Olivia', 'owner', 'active')`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, first_name TEXT NOT NULL)`,
		`CREATE TABLE sample_data_items (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL UNIQUE, created_by TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	h := &SampleDataHandler{service: service.NewSampleDataServiceWithSeeders(db, handlerSampleSeeder{})}
	h.modules = func(*http.Request) (map[model.ModuleID]bool, error) { return modules, nil }
	return h
}

func sampleDataRequest(h *SampleDataHandler, method, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/workspaces/ws/sample-data", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "ws")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	if userID != "" {
		ctx = middleware.WithUserID(ctx, userID)
	}
	rec := httptest.NewRecorder()
	req = req.WithContext(ctx)
	switch method {
	case http.MethodGet:
		h.Get(rec, req)
	case http.MethodPost:
		h.Load(rec, req)
	case http.MethodDelete:
		h.Remove(rec, req)
	}
	return rec
}

func decodeSampleStatus(t *testing.T, rec *httptest.ResponseRecorder) model.SampleDataStatus {
	t.Helper()
	var status model.SampleDataStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return status
}

func TestSampleDataHandlerLifecycle(t *testing.T) {
	h := setupSampleDataHandler(t, map[model.ModuleID]bool{model.ModuleCRM: true})

	rec := sampleDataRequest(h, http.MethodGet, "u1")
	if rec.Code != http.StatusOK || decodeSampleStatus(t, rec).Loaded {
		t.Fatalf("initial status %d %s", rec.Code, rec.Body.String())
	}

	rec = sampleDataRequest(h, http.MethodPost, "u1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("load status %d %s", rec.Code, rec.Body.String())
	}
	status := decodeSampleStatus(t, rec)
	if !status.Loaded || status.LoadedAt == nil || status.Counts[model.SampleEntityCRMContact] != 1 {
		t.Fatalf("loaded status = %+v", status)
	}

	rec = sampleDataRequest(h, http.MethodPost, "u1")
	if rec.Code != http.StatusConflict {
		t.Fatalf("repeat load status %d, want 409", rec.Code)
	}

	rec = sampleDataRequest(h, http.MethodDelete, "u1")
	if rec.Code != http.StatusOK || decodeSampleStatus(t, rec).Loaded {
		t.Fatalf("remove status %d %s", rec.Code, rec.Body.String())
	}
	rec = sampleDataRequest(h, http.MethodGet, "u1")
	if decodeSampleStatus(t, rec).Loaded {
		t.Fatalf("status after remove still loaded: %s", rec.Body.String())
	}
}

func TestSampleDataHandlerRejectsUnauthenticatedMutations(t *testing.T) {
	h := setupSampleDataHandler(t, map[model.ModuleID]bool{model.ModuleCRM: true})
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		if rec := sampleDataRequest(h, method, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status %d, want 401", method, rec.Code)
		}
	}
}

func TestSampleDataHandlerWithoutSeedableModules(t *testing.T) {
	h := setupSampleDataHandler(t, map[model.ModuleID]bool{model.ModuleAutomation: true})
	if rec := sampleDataRequest(h, http.MethodPost, "u1"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d %s, want 422", rec.Code, rec.Body.String())
	}
}

func TestSampleDataHandlerNonMemberIsForbidden(t *testing.T) {
	h := setupSampleDataHandler(t, map[model.ModuleID]bool{model.ModuleCRM: true})
	if rec := sampleDataRequest(h, http.MethodPost, "someone-else"); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s, want 403", rec.Code, rec.Body.String())
	}
}
