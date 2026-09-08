package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestCRMSituationHTTPCreateReplayAndDetail(t *testing.T) {
	_, routes := situationHTTPFixture(t)
	body := situationHTTPBody(t)
	first := situationHTTPRequest(routes, http.MethodPost, "/situations", body, f.Actor("member"), f.Workspace)
	if first.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", first.Code, first.Body)
	}
	var created model.CRMSituation
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Lifecycle != "open" || created.Attention != "needs_context" {
		t.Fatalf("initial state: %#v", created)
	}
	if strings.Contains(first.Body.String(), "creation_key") || strings.Contains(first.Body.String(), "creation_fingerprint") {
		t.Fatal("private idempotency state leaked")
	}
	again := situationHTTPRequest(routes, http.MethodPost, "/situations", body, f.Actor("member"), f.Workspace)
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() {
		t.Fatalf("replay = %d %s", again.Code, again.Body)
	}
	conflict := situationHTTPRequest(routes, http.MethodPost, "/situations", strings.Replace(body, "Buying intent", "Different work", 1), f.Actor("member"), f.Workspace)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed request = %d %s", conflict.Code, conflict.Body)
	}
	detail := situationHTTPRequest(routes, http.MethodGet, "/situations/"+created.ID, "", f.Actor("viewer"), f.Workspace)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail = %d %s", detail.Code, detail.Body)
	}
	var item model.CRMSituationItem
	if err := json.Unmarshal(detail.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Situation.ID != created.ID || item.Category != "sales" || len(item.References) != 1 || !item.OwnerAvailable {
		t.Fatalf("detail projection = %#v", item)
	}
	foreignActor := f.Actor("member")
	foreignActor.WorkspaceID, foreignActor.WorkspaceMemberID = f.ForeignWorkspace, f.ForeignMember
	foreign := situationHTTPRequest(routes, http.MethodGet, "/situations/"+created.ID, "", foreignActor, f.ForeignWorkspace)
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), "Northstar") {
		t.Fatalf("foreign detail = %d %s", foreign.Code, foreign.Body)
	}
}

func TestCRMSituationHTTPListCounts(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	for _, motion := range []string{"conversion", "prospecting", "retention"} {
		input := f.Situation(motion)
		if err := db.Create(&input).Error; err != nil {
			t.Fatal(err)
		}
	}
	response := situationHTTPRequest(routes, http.MethodGet, "/situations?scope=all&category=sales&page=2&page_size=1", "", f.Actor("viewer"), f.Workspace)
	if response.Code != http.StatusOK {
		t.Fatalf("list = %d %s", response.Code, response.Body)
	}
	var result model.CRMSituationList
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Data) != 1 || result.Page != 2 || result.PageSize != 1 ||
		result.CategoryCounts["sales"] != 2 || result.CategoryCounts["retention"] != 1 || result.CategoryCounts["all"] != 3 {
		t.Fatalf("page-local counts: %#v", result)
	}
}

func TestCRMSituationHTTPPermissionBoundary(t *testing.T) {
	_, routes := situationHTTPFixture(t)
	for _, tt := range []struct {
		name, method, workspace string
		actor                   *authorization.Actor
	}{
		{"missing actor", http.MethodGet, f.Workspace, nil},
		{"forged workspace", http.MethodGet, f.ForeignWorkspace, f.Actor("owner")},
		{"viewer create", http.MethodPost, f.Workspace, f.Actor("viewer")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := situationHTTPRequest(routes, tt.method, "/situations", situationHTTPBody(t), tt.actor, tt.workspace)
			if response.Code != http.StatusForbidden {
				t.Fatalf("permission boundary = %d %s", response.Code, response.Body)
			}
		})
	}
	body := strings.Replace(situationHTTPBody(t), f.Signal, f.ForeignSignal, 1)
	response := situationHTTPRequest(routes, http.MethodPost, "/situations", body, f.Actor("member"), f.Workspace)
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), f.ForeignSignal) {
		t.Fatalf("foreign reference = %d %s", response.Code, response.Body)
	}
}

func TestCRMSituationHTTPInvalidInput(t *testing.T) {
	_, routes := situationHTTPFixture(t)
	for _, path := range []string{
		"/situations?page=0", "/situations?page=-1", "/situations?page_size=101", "/situations?page=banana",
		"/situations?scope=someone_else", "/situations?category=conversion", "/situations?filter=not-json",
		"/situations?filter=" + url.QueryEscape(`{"rules":[{"field":"workspace_id","operator":"is","value":"other"}]}`),
		"/situations/not-a-uuid",
	} {
		response := situationHTTPRequest(routes, http.MethodGet, path, "", f.Actor("member"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d %s", path, response.Code, response.Body)
		}
	}
	body := situationHTTPBody(t)
	for _, invalid := range []string{
		"{", "null", body + "{}", strings.Replace(body, "Buying intent", strings.Repeat("a", 70<<10), 1),
		strings.TrimSuffix(body, "}") + `,"lifecycle":"closed"}`,
		strings.TrimSuffix(body, "}") + `,"execution_status":"succeeded"}`,
		strings.TrimSuffix(body, "}") + `,"workspace_id":"` + f.ForeignWorkspace + `"}`,
	} {
		response := situationHTTPRequest(routes, http.MethodPost, "/situations", invalid, f.Actor("member"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Errorf("invalid POST = %d %s", response.Code, response.Body)
		}
	}
}

func TestCRMSituationHTTPInternalErrorsAreSanitized(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	// Break only this test's isolated schema, never a configured application DB.
	f.Exec(t, db, "ALTER TABLE crm_situations RENAME TO unavailable_situations")
	response := situationHTTPRequest(routes, http.MethodGet, "/situations", "", f.Actor("member"), f.Workspace)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "crm_situations") ||
		strings.Contains(response.Body.String(), "SELECT") || strings.Contains(response.Body.String(), "no such table") {
		t.Fatalf("unsanitized response = %d %s", response.Code, response.Body)
	}
}

func situationHTTPFixture(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()
	db := f.Open(t)
	svc := service.NewCRMSituationService(repository.NewCRMSituationRepository(db), authorization.NewAuthzService(db, nil, nil))
	svc.SetActions(service.NewCRMSuggestionService(repository.NewCRMSuggestionRepository(db), repository.NewCRMDealRepository(db), nil))
	h := NewCRMSituationHandler(svc)
	routes := chi.NewRouter()
	routes.Get("/situations", h.List)
	routes.Get("/situations/{id}", h.Get)
	routes.Post("/situations", h.Create)
	routes.Post("/situations/{id}/commands", h.Command)
	routes.Get("/situations/{id}/history", h.History)
	routes.Post("/situations/{id}/actions/{action_id}/{decision}", h.DecideAction)
	routes.Get("/signal-inbox", h.Inbox)
	routes.Get("/signal-inbox/recommendations/{id}", h.InboxRecommendation)
	routes.Post("/signal-inbox/recommendations/{id}/{decision}", h.DecideInboxRecommendation)
	return db, routes
}

func situationHTTPRequest(routes http.Handler, method, path, body string, actor *authorization.Actor, workspace string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Workspace-ID", workspace)
	if actor != nil {
		req = req.WithContext(authorization.WithActor(req.Context(), actor))
	}
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	return w
}

func situationHTTPBody(t *testing.T) string {
	t.Helper()
	body, err := json.Marshal(model.CreateCRMSituationRequest{
		CreationKey: uuid.NewString(), Title: "Buying intent", Objective: "Arrange a buying discussion",
		CommercialMotion: "conversion", CompanyID: f.Ptr(f.Company),
		References: []model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
