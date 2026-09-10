package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func setupPipelineHandlerTest(t *testing.T) *CRMDealHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pipeline_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces VALUES ('owned')`,
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY,workspace_id TEXT,name TEXT,is_default BOOLEAN,default_commercial_motion TEXT,position INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`INSERT INTO crm_pipelines (id,workspace_id,name,is_default,updated_at) VALUES ('p','owned','Sales',false,'2026-01-01 00:00:00+00:00')`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY,pipeline_id TEXT,position INTEGER)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY,pipeline_id TEXT,stage_id TEXT)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewCRMDealHandler(service.NewCRMDealService(repository.NewCRMDealRepository(db), nil))
}

func TestPipelineHandlersRejectOtherWorkspace(t *testing.T) {
	tests := []struct {
		name, method string
		handle       func(*CRMDealHandler) http.HandlerFunc
	}{
		{"get", http.MethodGet, func(h *CRMDealHandler) http.HandlerFunc { return h.GetPipeline }},
		{"update", http.MethodPut, func(h *CRMDealHandler) http.HandlerFunc { return h.UpdatePipeline }},
		{"delete", http.MethodDelete, func(h *CRMDealHandler) http.HandlerFunc { return h.DeletePipeline }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := setupPipelineHandlerTest(t)
			req := httptest.NewRequest(tt.method, "/api/crm/pipelines/p", strings.NewReader(`{"name":"Changed"}`))
			req.Header.Set("X-Workspace-ID", "other")
			route := chi.NewRouteContext()
			route.URLParams.Add("id", "p")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			tt.handle(h)(response, req)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			got, err := h.dealService.GetPipeline(context.Background(), "p")
			if err != nil || got.Name != "Sales" {
				t.Errorf("cross-workspace write: %v, %v", got, err)
			}
		})
	}
}

func TestPipelineUpdateRejectsMalformedProbability(t *testing.T) {
	for _, probability := range []string{`1.5`, `"50"`, `101`, `-1`} {
		t.Run(probability, func(t *testing.T) {
			h := setupPipelineHandlerTest(t)
			body := fmt.Sprintf(`{"stages":[{"name":"Lead","stage_type":"open","probability":%s}]}`, probability)
			req := httptest.NewRequest(http.MethodPut, "/api/crm/pipelines/p", strings.NewReader(body))
			req.Header.Set("X-Workspace-ID", "owned")
			route := chi.NewRouteContext()
			route.URLParams.Add("id", "p")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			h.UpdatePipeline(response, req)
			if response.Code != http.StatusBadRequest {
				t.Errorf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestPipelineUpdateReturnsConflictForStaleEditor(t *testing.T) {
	h := setupPipelineHandlerTest(t)
	req := httptest.NewRequest(http.MethodPut, "/api/crm/pipelines/p", strings.NewReader(`{"name":"Changed","expected_updated_at":"2025-01-01T00:00:00Z"}`))
	req.Header.Set("X-Workspace-ID", "owned")
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "p")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	response := httptest.NewRecorder()
	h.UpdatePipeline(response, req)
	if response.Code != http.StatusConflict {
		t.Errorf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPipelineInternalErrorsAreSanitized(t *testing.T) {
	response := httptest.NewRecorder()
	writeCRMPipelineError(response, httptest.NewRequest(http.MethodGet, "/", nil), fmt.Errorf("private database details"))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private") {
		t.Errorf("status=%d body=%s", response.Code, response.Body.String())
	}
}
