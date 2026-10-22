package publicapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type fakeBackend struct {
	principal *model.MCPPrincipal
	authErr   error
	execErr   error
	tool      string
	args      map[string]any
	calls     int
}

func (f *fakeBackend) AuthenticateBearer(_ context.Context, bearer string) (*model.MCPPrincipal, error) {
	if f.authErr != nil {
		return nil, f.authErr
	}
	if bearer != "hmp_good" {
		return nil, service.ErrMCPUnauthorized
	}
	return f.principal, nil
}

func (f *fakeBackend) ExecuteTool(_ context.Context, _ *model.MCPPrincipal, name string, arguments json.RawMessage) (*service.MCPToolResult, error) {
	f.calls++
	f.tool = name
	f.args = nil
	_ = json.Unmarshal(arguments, &f.args)
	if f.execErr != nil {
		return nil, f.execErr
	}
	return &service.MCPToolResult{Summary: "ok", Data: map[string]any{"id": "x"}, Links: map[string]string{"app": "https://app"}}, nil
}

type denyLimiter struct{}

func (denyLimiter) AllowN(context.Context, string, string, bool, int64) bool { return false }

func newTestHandler(t *testing.T, backend *fakeBackend, limiter Limiter) *Handler {
	t.Helper()
	backend.principal = &model.MCPPrincipal{Kind: model.MCPPrincipalKindService, WorkspaceID: "ws", ServicePrincipalID: "sp"}
	handler, err := NewHandler(backend, limiter, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return handler
}

func do(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, BasePath+path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer hmp_good")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRoutesAreConsistentWithToolCatalog(t *testing.T) {
	tools := map[string]service.MCPToolDefinition{}
	for _, tool := range service.PublicToolCatalog() {
		tools[tool.Name] = tool
	}
	if err := validateRoutes(tools); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, route := range Routes {
		id := camelCase(route.Tool)
		if ids[id] {
			t.Errorf("duplicate operationId %s", id)
		}
		ids[id] = true
	}
}

func TestRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeBackend{}, nil)
	req := httptest.NewRequest(http.MethodGet, BasePath+"/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"unauthorized"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	rec = do(handler, http.MethodGet, "/tasks", "", map[string]string{"Authorization": "Bearer nope"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token status %d", rec.Code)
	}
}

func TestSpecIsPublicWithoutAuth(t *testing.T) {
	handler := newTestHandler(t, &fakeBackend{}, nil)
	req := httptest.NewRequest(http.MethodGet, BasePath+"/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openapi": "3.1.0"`) {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestQueryParametersAreTyped(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	rec := do(handler, http.MethodGet, "/tasks?limit=5&archived=true&owner_member_ids=a,b&query=bug", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if backend.tool != "list_tasks" || backend.args["limit"] != float64(5) || backend.args["archived"] != true || backend.args["query"] != "bug" {
		t.Fatalf("args = %#v", backend.args)
	}
	if owners, _ := backend.args["owner_member_ids"].([]any); len(owners) != 2 {
		t.Fatalf("owners = %#v", backend.args["owner_member_ids"])
	}
	if _, present := backend.args["idempotency_key"]; present {
		t.Fatal("read must not carry an idempotency key")
	}
	var payload envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload.Summary != "ok" || payload.Links["app"] == "" {
		t.Fatalf("envelope = %#v err %v", payload, err)
	}
}

func TestObjectQueryParametersAreJSON(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	filter := url.QueryEscape(`{"rules":[{"field":"custom_properties.fit","operator":"is","value":"yes"}]}`)
	rec := do(handler, http.MethodGet, "/crm/companies?filter="+filter, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	got, _ := backend.args["filter"].(map[string]any)
	rules, _ := got["rules"].([]any)
	if backend.tool != "list_crm_companies" || len(rules) != 1 {
		t.Fatalf("args = %#v", backend.args)
	}
	if rec := do(handler, http.MethodGet, "/crm/companies?filter=not-json", "", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed filter status %d", rec.Code)
	}
}

func TestCRMRoutesMapPathsBodiesAndMethods(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)

	rec := do(handler, http.MethodPost, "/crm/records/upsert", `{"object_type":"company","records":[{"fields":{"name":"Acme","domain":"acme.com"}}]}`, map[string]string{"Idempotency-Key": "batch-0001"})
	records, _ := backend.args["records"].([]any)
	if rec.Code != http.StatusOK || backend.tool != "upsert_crm_records" || len(records) != 1 || backend.args["idempotency_key"] != "batch-0001" {
		t.Fatalf("upsert status %d tool %q args %#v", rec.Code, backend.tool, backend.args)
	}

	rec = do(handler, http.MethodDelete, "/crm/associations/assoc-1", "", nil)
	if rec.Code != http.StatusOK || backend.tool != "unlink_crm_association" || backend.args["association_id"] != "assoc-1" {
		t.Fatalf("unlink status %d tool %q args %#v", rec.Code, backend.tool, backend.args)
	}

	rec = do(handler, http.MethodPut, "/crm/contacts/contact-1/primary-company", `{"company_id":"company-1"}`, nil)
	if rec.Code != http.StatusOK || backend.tool != "set_primary_contact_company" || backend.args["contact_id"] != "contact-1" || backend.args["company_id"] != "company-1" {
		t.Fatalf("primary company status %d tool %q args %#v", rec.Code, backend.tool, backend.args)
	}

	rec = do(handler, http.MethodGet, "/crm/associations?object_type=crm_company&object_id=company-1", "", nil)
	if rec.Code != http.StatusOK || backend.tool != "list_crm_associations" || backend.args["object_id"] != "company-1" {
		t.Fatalf("list associations status %d tool %q args %#v", rec.Code, backend.tool, backend.args)
	}

	rec = do(handler, http.MethodPost, "/crm/associations", `{"from_object_type":"contact","from_object_id":"contact-1","to_object_type":"company","to_object_id":"company-1"}`, nil)
	if rec.Code != http.StatusOK || backend.tool != "link_crm_objects" || backend.args["to_object_id"] != "company-1" {
		t.Fatalf("link status %d tool %q args %#v body %s", rec.Code, backend.tool, backend.args, rec.Body)
	}
}

type countingLimiter struct{ costs []int64 }

func (l *countingLimiter) AllowN(_ context.Context, _, _ string, _ bool, cost int64) bool {
	l.costs = append(l.costs, cost)
	return true
}

func TestBulkUpsertIsChargedBySize(t *testing.T) {
	limiter := &countingLimiter{}
	handler := newTestHandler(t, &fakeBackend{}, limiter)
	body := `{"object_type":"company","records":[` + strings.TrimSuffix(strings.Repeat(`{"fields":{"name":"x"}},`, 25), ",") + `]}`
	do(handler, http.MethodPost, "/crm/records/upsert", body, nil)
	do(handler, http.MethodPatch, "/crm/companies/company-1", `{"owner_member_id":"m1"}`, nil)
	if len(limiter.costs) != 2 || limiter.costs[0] != 3 || limiter.costs[1] != 1 {
		t.Fatalf("costs = %v, want [3 1]", limiter.costs)
	}
}

func TestBadQueryParametersAreRejected(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	for _, path := range []string{"/tasks?limit=abc", "/tasks?bogus=1", "/tasks?archived=maybe", "/tasks?idempotency_key=abcdefgh"} {
		if rec := do(handler, http.MethodGet, path, "", nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s status %d", path, rec.Code)
		}
	}
	if backend.calls != 0 {
		t.Fatal("invalid requests must not reach the tool layer")
	}
}

func TestMutationMergesPathBodyAndIdempotencyKey(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	rec := do(handler, http.MethodPost, "/tasks/task-1/comments", `{"content":"hello"}`, map[string]string{"Idempotency-Key": "key-12345"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if backend.tool != "add_task_comment" || backend.args["task_id"] != "task-1" || backend.args["content"] != "hello" || backend.args["idempotency_key"] != "key-12345" {
		t.Fatalf("args = %#v", backend.args)
	}

	rec = do(handler, http.MethodPost, "/tasks", `{"name":"x","team_id":"t"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d", rec.Code)
	}
	if key, _ := backend.args["idempotency_key"].(string); !strings.HasPrefix(key, "req-") {
		t.Fatalf("generated key = %v", backend.args["idempotency_key"])
	}

	rec = do(handler, http.MethodPost, "/agent-runs", `{"agent_id":"a","target_type":"workspace","target_id":"w"}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("agent run status %d", rec.Code)
	}
}

func TestPathParametersCannotBeOverriddenByBody(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	if rec := do(handler, http.MethodPatch, "/tasks/task-1", `{"task_id":"other","name":"x"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	if rec := do(handler, http.MethodPost, "/tasks", `{"idempotency_key":"abcdefgh","name":"x"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("body idempotency key status %d", rec.Code)
	}
	if rec := do(handler, http.MethodPost, "/tasks", `[1]`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("non-object body status %d", rec.Code)
	}
	if backend.calls != 0 {
		t.Fatal("invalid requests must not reach the tool layer")
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{service.ErrMCPForbidden, http.StatusForbidden, "forbidden"},
		{service.ErrMCPNotFound, http.StatusNotFound, "not_found"},
		{service.ErrMCPConflict, http.StatusConflict, "idempotency_conflict"},
		{service.ErrMCPInvalidArguments, http.StatusBadRequest, "invalid_arguments"},
		{service.ErrMCPRateLimited, http.StatusTooManyRequests, "rate_limited"},
		{context.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
		{&service.MCPToolError{Code: service.MCPErrorCodeDocumentLocked, Message: "locked"}, http.StatusConflict, "document_locked"},
		{&service.MCPToolError{Code: service.MCPErrorCodeTargetNotFound, Message: "missing"}, http.StatusNotFound, "target_not_found"},
		{&service.MCPToolError{Code: service.MCPErrorCodeRepositoryRequired, Message: "repo"}, http.StatusUnprocessableEntity, "repository_required"},
		{errors.New("database exploded: secret detail"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		backend := &fakeBackend{execErr: tc.err}
		handler := newTestHandler(t, backend, nil)
		rec := do(handler, http.MethodGet, "/me", "", nil)
		var body errorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != tc.status || body.Error.Code != tc.code {
			t.Errorf("%v => %d %q, want %d %q", tc.err, rec.Code, body.Error.Code, tc.status, tc.code)
		}
		if strings.Contains(rec.Body.String(), "secret detail") {
			t.Errorf("internal error text leaked: %s", rec.Body)
		}
	}
}

func TestRateLimitDeniesBeforeExecution(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, denyLimiter{})
	rec := do(handler, http.MethodGet, "/me", "", nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || backend.calls != 0 {
		t.Fatalf("status %d retry %q calls %d", rec.Code, rec.Header().Get("Retry-After"), backend.calls)
	}
}

func TestDisabledAPIIsForbidden(t *testing.T) {
	backend := &fakeBackend{authErr: service.ErrMCPDisabled}
	handler := newTestHandler(t, backend, nil)
	if rec := do(handler, http.MethodGet, "/me", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	handler := newTestHandler(t, &fakeBackend{}, nil)
	if rec := do(handler, http.MethodGet, "/nope", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status %d", rec.Code)
	}
	if rec := do(handler, http.MethodDelete, "/tasks", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status %d", rec.Code)
	}
}

func TestCommittedSpecIsCurrent(t *testing.T) {
	generated, err := BuildSpecJSON("")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "..", "..", "docs", "api", "openapi.json")
	if os.Getenv("UPDATE_OPENAPI") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, generated, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run with UPDATE_OPENAPI=1 to generate)", path, err)
	}
	if !bytes.Equal(committed, generated) {
		t.Fatalf("%s is stale; regenerate with: UPDATE_OPENAPI=1 go test ./internal/publicapi -run TestCommittedSpecIsCurrent", path)
	}
}

func TestSpecShape(t *testing.T) {
	spec, err := BuildSpec("")
	if err != nil {
		t.Fatal(err)
	}
	paths := spec["paths"].(map[string]any)
	operations := 0
	for _, item := range paths {
		for _, raw := range item.(map[string]any) {
			operation := raw.(map[string]any)
			operations++
			if operation["operationId"] == "" || operation["x-helpin-scope"] == "" || operation["summary"] == "" {
				t.Errorf("incomplete operation %v", operation["operationId"])
			}
		}
	}
	if operations != len(Routes) {
		t.Fatalf("operations = %d, routes = %d", operations, len(Routes))
	}
}

func TestSupportSetupRouteUsesAuthorizedToolAndRejectsWorkspaceOverride(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	response := do(handler, http.MethodGet, "/support/setup", "", nil)
	if response.Code != http.StatusOK || backend.tool != "get_support_setup" {
		t.Fatalf("status = %d, tool = %s", response.Code, backend.tool)
	}
	response = do(handler, http.MethodGet, "/support/setup?workspace_id=other", "", nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("workspace override status = %d", response.Code)
	}
	backend.execErr = service.ErrMCPForbidden
	response = do(handler, http.MethodGet, "/support/setup", "", nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d", response.Code)
	}
}

func TestWorkspaceSetupRouteUsesAuthorizedToolAndRejectsWorkspaceOverride(t *testing.T) {
	backend := &fakeBackend{}
	handler := newTestHandler(t, backend, nil)
	response := do(handler, http.MethodGet, "/setup", "", nil)
	if response.Code != http.StatusOK || backend.tool != "get_workspace_setup" {
		t.Fatalf("status = %d, tool = %s", response.Code, backend.tool)
	}
	response = do(handler, http.MethodGet, "/setup?workspace_id=other", "", nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("workspace override status = %d", response.Code)
	}
	backend.execErr = service.ErrMCPForbidden
	response = do(handler, http.MethodGet, "/setup", "", nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d", response.Code)
	}
}
