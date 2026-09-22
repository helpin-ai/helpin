package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type aiConnectionTestProvider struct {
	err error
}

func (p aiConnectionTestProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: "OK"}, nil
}

func setupAIConnectionHandlerTest(t *testing.T, provider llm.Provider) (*AIConnectionHandler, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "connections.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.AIConnection{}, &model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspace_members (workspace_id TEXT,user_id TEXT,status TEXT);
		INSERT INTO workspace_members VALUES ('workspace','owner','active'),('workspace','teammate','active')`).Error; err != nil {
		t.Fatal(err)
	}
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := service.NewAgentRuntimeClient("https://runtime.example", "helpin", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewAIConnectionService(repository.NewAIConnectionRepository(db), catalog, runtime, service.AIConnectionConfig{EncryptionKey: strings.Repeat("k", 32), AppID: "helpin"})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetTestProviderFactory(func(string, sdk.ModelCredential) (llm.Provider, error) { return provider, nil })
	login, err := svc.Create(context.Background(), "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "anthropic", APIKey: "sk-ant-private"})
	if err != nil {
		t.Fatal(err)
	}
	return NewAIConnectionHandler(svc), login.Connection.ID
}

func serveAIConnectionTest(h *AIConnectionHandler, user, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/ai-connections/"+id+"/test", strings.NewReader(body))
	ctx := middleware.WithWorkspaceID(middleware.WithUserID(req.Context(), user), "workspace")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("connectionID", id)
	req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()
	h.Test(rec, req)
	return rec
}

func TestAIConnectionHandlerTestReportsSuccess(t *testing.T) {
	h, id := setupAIConnectionHandlerTest(t, aiConnectionTestProvider{})
	rec := serveAIConnectionTest(h, "owner", id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var result model.AIConnectionTestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "claude-haiku-4-5" || result.Error != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAIConnectionHandlerTestAcceptsModelAndHidesProviderErrors(t *testing.T) {
	h, id := setupAIConnectionHandlerTest(t, aiConnectionTestProvider{err: &llm.ProviderError{StatusCode: 401, Message: "bad key sk-ant-private"}})
	rec := serveAIConnectionTest(h, "owner", id, `{"model":"claude-sonnet-5"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-ant-private") {
		t.Fatal("response echoed the credential")
	}
	var result model.AIConnectionTestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Model != "claude-sonnet-5" || !strings.Contains(result.Error, "HTTP 401") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAIConnectionHandlerTestRejectsInvalidRequests(t *testing.T) {
	h, id := setupAIConnectionHandlerTest(t, aiConnectionTestProvider{})
	tests := []struct {
		name, user, body string
		want             int
	}{
		{name: "unknown field", user: "owner", body: `{"api_key":"x"}`, want: http.StatusBadRequest},
		{name: "another member's personal connection", user: "teammate", body: "", want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := serveAIConnectionTest(h, tt.user, id, tt.body); rec.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
