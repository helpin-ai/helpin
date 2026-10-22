package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newCRMMCPTestService(t *testing.T) *MCPService {
	t.Helper()
	db, upserter := setupCRMRecordUpsertTest(t)
	companyRepo := repository.NewCRMCompanyRepository(db)
	service := &MCPService{
		crmContacts: NewCRMContactService(repository.NewCRMContactRepository(db)),
		crmDeals:    NewCRMDealService(repository.NewCRMDealRepository(db), repository.NewCRMAssociationRepository(db)),
	}
	service.SetCRMRecords(NewCRMCompanyService(companyRepo), NewCRMActivityService(repository.NewCRMActivityRepository(db)), upserter)
	return service
}

func crmMCPToolDefinition(t *testing.T, name string) MCPToolDefinition {
	t.Helper()
	for _, definition := range crmMCPToolDefinitions() {
		if definition.Name == name {
			return definition
		}
	}
	t.Fatalf("tool %s is not defined", name)
	return MCPToolDefinition{}
}

func TestCRMMCPToolSchemasAcceptDocumentedPayloads(t *testing.T) {
	tests := []struct {
		tool    string
		payload string
		wantErr bool
	}{
		{tool: "upsert_crm_records", payload: `{"object_type":"company","match_on":"domain","fill_only":true,"idempotency_key":"batch-0001","records":[
			{"ref":"lead-1","fields":{"name":"Acme","domain":"acme.com","employee_count":40},
			 "custom_properties":{"fit":"yes","support_tools":["zendesk"],"confidence":0.8,"skip":false,"old":null},
			 "note":{"subject":"Research","body":"Evidence"}}]}`},
		{tool: "upsert_crm_records", payload: `{"object_type":"company","idempotency_key":"batch-0001","records":[{"fields":{"name":true}}]}`, wantErr: true},
		{tool: "upsert_crm_records", payload: `{"object_type":"company","idempotency_key":"batch-0001","records":[]}`, wantErr: true},
		{tool: "upsert_crm_records", payload: `{"object_type":"company","records":[{"fields":{"name":"A"}}]}`, wantErr: true},
		{tool: "get_crm_company", payload: `{"company_id":"c1","include":["activities","contacts","deals"]}`},
		{tool: "get_crm_contact", payload: `{"contact_id":"c1","include":["deals"]}`, wantErr: true},
		{tool: "get_crm_deal", payload: `{"deal_id":"d1"}`},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			resolved, err := resolveMCPToolSchema(crmMCPToolDefinition(t, tt.tool).InputSchema)
			if err != nil {
				t.Fatalf("resolve schema: %v", err)
			}
			var value any
			if err := json.Unmarshal([]byte(tt.payload), &value); err != nil {
				t.Fatal(err)
			}
			if err := resolved.Validate(value); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCRMMCPUpsertThenGetCompanyWithActivities(t *testing.T) {
	service := newCRMMCPTestService(t)
	principal := &model.MCPPrincipal{WorkspaceID: _upsertTestWorkspace, UserID: "user-1"}
	ctx := context.Background()

	result, handled, err := service.executeCRMMCPTool(ctx, principal, "upsert_crm_records", json.RawMessage(`{
		"object_type":"company","records":[{"fields":{"name":"Acme","domain":"acme.com"},"note":{"body":"Uses Zendesk"}}]
	}`))
	if err != nil || !handled {
		t.Fatalf("upsert handled = %v err = %v", handled, err)
	}
	if result.Summary != "Processed 1 company records: 1 created." {
		t.Fatalf("summary = %q", result.Summary)
	}
	companyID := result.Data.(*model.CRMRecordUpsertResponse).Results[0].ID

	result, _, err = service.executeCRMMCPTool(ctx, principal, "get_crm_company", mustJSON(map[string]any{
		"company_id": companyID, "include": []string{"activities", "contacts"},
	}))
	if err != nil {
		t.Fatalf("get company: %v", err)
	}
	data := result.Data.(map[string]any)
	activities, _ := data["activities"].([]map[string]any)
	if data["id"] != companyID || data["domain"] != "acme.com" || len(activities) != 1 || activities[0]["body"] != "Uses Zendesk" {
		t.Fatalf("company data = %#v", data)
	}
	if _, ok := data["contacts"]; !ok {
		t.Fatalf("contacts include missing: %#v", data)
	}
}

func TestCRMMCPGetCompanyHidesOtherWorkspaces(t *testing.T) {
	service := newCRMMCPTestService(t)
	owner := &model.MCPPrincipal{WorkspaceID: _upsertTestWorkspace}
	result, _, err := service.executeCRMMCPTool(context.Background(), owner, "upsert_crm_records", json.RawMessage(`{
		"object_type":"company","records":[{"fields":{"name":"Acme","domain":"acme.com"}}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	companyID := result.Data.(*model.CRMRecordUpsertResponse).Results[0].ID
	outsider := &model.MCPPrincipal{WorkspaceID: "another-workspace"}
	_, _, err = service.executeCRMMCPTool(context.Background(), outsider, "get_crm_company", mustJSON(map[string]any{"company_id": companyID}))
	if !errors.Is(err, ErrMCPNotFound) {
		t.Fatalf("error = %v, want not found", err)
	}
}

func TestCRMMCPUpsertRequestErrorsAreInvalidInput(t *testing.T) {
	service := newCRMMCPTestService(t)
	principal := &model.MCPPrincipal{WorkspaceID: _upsertTestWorkspace}
	_, _, err := service.executeCRMMCPTool(context.Background(), principal, "upsert_crm_records", json.RawMessage(`{
		"object_type":"contact","match_on":"domain","records":[{"fields":{"first_name":"Ada"}}]
	}`))
	var toolErr *MCPToolError
	if !errors.As(err, &toolErr) || toolErr.Code != MCPErrorCodeInvalidInput {
		t.Fatalf("error = %#v, want invalid input tool error", err)
	}
}
