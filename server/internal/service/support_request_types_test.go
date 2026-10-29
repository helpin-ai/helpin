package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// setupRequestTypePortal wires request types into the portal test service
// and defines a severity select (customer-visible) and an account id text
// field (internal) for conversations.
func setupRequestTypePortal(t *testing.T) (*CustomerPortalService, *gorm.DB, *PortalWorkspace, *SupportRequestTypeService) {
	t.Helper()
	svc, db, _, ws := setupCustomerPortalService(t)
	fields := NewCustomFieldService(repository.NewCustomFieldRepository(db))
	types := NewSupportRequestTypeService(repository.NewSupportRequestTypeRepository(db), fields, repository.NewSupportMailboxRepository(db))
	svc.inbox.SetRequestTypes(types, fields)
	ctx := context.Background()
	for _, req := range []model.CustomFieldDefinitionRequest{
		{Key: "severity", Label: "Severity", Description: "How bad is it?", FieldType: "select", Options: model.CustomFieldOptions{{Value: "high", Label: "High"}, {Value: "low", Label: "Low"}}},
		{Key: "account_id", FieldType: "text"},
		{Key: "steps", FieldType: "long_text"},
	} {
		if _, err := fields.Create(ctx, ws.ID, "", model.CustomFieldObjectConversation, req); err != nil {
			t.Fatalf("define %s: %v", req.Key, err)
		}
	}
	return svc, db, ws, types
}

func bugReportRequest() model.SupportRequestTypeRequest {
	high := "high"
	return model.SupportRequestTypeRequest{
		Name: "Bug report", Description: "Something is broken", Enabled: true, DefaultPriority: &high,
		Fields: []model.SupportRequestTypeField{
			{Key: "severity", Required: true, CustomerVisible: true},
			{Key: "steps", CustomerVisible: true, HelpText: "What did you do?"},
			{Key: "account_id", Required: true},
		},
	}
}

func TestSupportRequestTypeCreateValidates(t *testing.T) {
	_, _, ws, types := setupRequestTypePortal(t)
	ctx := context.Background()
	created, err := types.Create(ctx, ws.ID, bugReportRequest())
	if err != nil {
		t.Fatal(err)
	}
	if created.Fields[2].Required {
		t.Fatal("an internal field must not be required of customers")
	}
	bad := "critical"
	for _, tc := range []struct {
		name string
		edit func(*model.SupportRequestTypeRequest)
		want error
	}{
		{"empty name", func(r *model.SupportRequestTypeRequest) { r.Name = "  " }, ErrSupportRequestTypeInvalid},
		{"same name in other case", func(r *model.SupportRequestTypeRequest) { r.Name = "BUG  report" }, repository.ErrSupportRequestTypeNameTaken},
		{"unknown field", func(r *model.SupportRequestTypeRequest) {
			r.Name, r.Fields = "Other", []model.SupportRequestTypeField{{Key: "nope"}}
		}, ErrSupportRequestTypeInvalid},
		{"field twice", func(r *model.SupportRequestTypeRequest) {
			r.Name, r.Fields = "Other", []model.SupportRequestTypeField{{Key: "steps"}, {Key: "steps"}}
		}, ErrSupportRequestTypeInvalid},
		{"bad priority", func(r *model.SupportRequestTypeRequest) { r.Name, r.DefaultPriority = "Other", &bad }, ErrSupportInvalidPriority},
		{"unknown inbox", func(r *model.SupportRequestTypeRequest) {
			id := uuid.NewString()
			r.Name, r.MailboxID = "Other", &id
		}, ErrSupportRequestTypeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := bugReportRequest()
			tc.edit(&req)
			if _, err := types.Create(ctx, ws.ID, req); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSupportRequestTypePortalViewShowsCustomerFields(t *testing.T) {
	_, _, ws, types := setupRequestTypePortal(t)
	ctx := context.Background()
	if _, err := types.Create(ctx, ws.ID, bugReportRequest()); err != nil {
		t.Fatal(err)
	}
	off := model.SupportRequestTypeRequest{Name: "Billing", Enabled: false}
	if _, err := types.Create(ctx, ws.ID, off); err != nil {
		t.Fatal(err)
	}
	portal, err := types.ForPortal(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(portal) != 1 || portal[0].Name != "Bug report" {
		t.Fatalf("portal types = %#v, want only the enabled one", portal)
	}
	fields := portal[0].Fields
	if len(fields) != 2 || fields[0].Key != "severity" || fields[0].HelpText != "" || fields[1].HelpText != "What did you do?" {
		t.Fatalf("fields = %#v, want severity (no teammate description) and steps, no internal field", fields)
	}
}

func TestSupportRequestTypeValidateSubmission(t *testing.T) {
	_, _, ws, types := setupRequestTypePortal(t)
	ctx := context.Background()
	// Without enabled types the plain form needs no type.
	if requestType, values, err := types.ValidateSubmission(ctx, ws.ID, "", nil); err != nil || requestType != nil || values != nil {
		t.Fatalf("untyped submission = %v %v %v", requestType, values, err)
	}
	bug, err := types.Create(ctx, ws.ID, bugReportRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		typeID string
		values map[string]any
	}{
		{"type now required", "", nil},
		{"internal field", bug.ID, map[string]any{"severity": "high", "account_id": "A-1"}},
		{"unknown field", bug.ID, map[string]any{"severity": "high", "nope": "x"}},
		{"required missing", bug.ID, map[string]any{"steps": "Click"}},
		{"bad option", bug.ID, map[string]any{"severity": "critical"}},
		{"unknown type", uuid.NewString(), map[string]any{"severity": "high"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := types.ValidateSubmission(ctx, ws.ID, tc.typeID, tc.values); !errors.Is(err, ErrSupportRequestTypeInvalid) {
				t.Fatalf("error = %v, want invalid", err)
			}
		})
	}
	requestType, values, err := types.ValidateSubmission(ctx, ws.ID, bug.ID, map[string]any{"severity": "high", "steps": nil})
	if err != nil || requestType.ID != bug.ID || len(values) != 1 || values["severity"] != "high" {
		t.Fatalf("valid submission = %v %#v %v", requestType, values, err)
	}
	req := bugReportRequest()
	req.Enabled = false
	if _, err := types.Update(ctx, ws.ID, bug.ID, req); err != nil {
		t.Fatal(err)
	}
	if _, _, err := types.ValidateSubmission(ctx, ws.ID, bug.ID, map[string]any{"severity": "high"}); !errors.Is(err, ErrSupportRequestTypeInvalid) {
		t.Fatalf("disabled type error = %v", err)
	}
}

func TestCustomerPortalCreateRequestWithType(t *testing.T) {
	svc, db, ws, types := setupRequestTypePortal(t)
	ctx := context.Background()
	bug, err := types.Create(ctx, ws.ID, bugReportRequest())
	if err != nil {
		t.Fatal(err)
	}
	ws, err = svc.ResolveWorkspace(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if config := svc.Configuration(ctx, ws); len(config.RequestTypes) != 1 || config.RequestTypes[0].ID != bug.ID {
		t.Fatalf("configuration request types = %#v", config.RequestTypes)
	}
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if _, err := svc.CreateRequest(ctx, access, "Crash", "It crashes", nil, nil, PortalRequestFields{}); !errors.Is(err, ErrSupportRequestTypeInvalid) {
		t.Fatalf("untyped request with types enabled = %v", err)
	}
	request, err := svc.CreateRequest(ctx, access, "Crash", "It crashes", nil, nil, PortalRequestFields{RequestTypeID: bug.ID, Fields: map[string]any{"severity": "high", "steps": "Open the app"}})
	if err != nil {
		t.Fatal(err)
	}
	var conversation model.SupportConversation
	if err := db.Where("workspace_id = ? AND subject = ?", ws.ID, "Crash").Take(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.RequestTypeID == nil || *conversation.RequestTypeID != bug.ID || conversation.Priority != "high" {
		t.Fatalf("conversation type %v priority %s", conversation.RequestTypeID, conversation.Priority)
	}
	if conversation.CustomFields["severity"] != "high" || conversation.CustomFields["steps"] != "Open the app" {
		t.Fatalf("custom fields = %#v", conversation.CustomFields)
	}
	if err := types.Delete(ctx, ws.ID, bug.ID); !errors.Is(err, repository.ErrSupportRequestTypeInUse) {
		t.Fatalf("delete used type = %v, want in use", err)
	}
	// Internal fields never reach the customer.
	if _, err := svc.inbox.SetConversationFields(ctx, ws.ID, conversation.ID, map[string]any{"account_id": "A-1"}, ""); err != nil {
		t.Fatal(err)
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if detail.RequestType == nil || *detail.RequestType != "Bug report" || len(detail.Fields) != 2 || detail.Fields[0] != (PortalFieldValue{Label: "Severity", Value: "High"}) {
		t.Fatalf("portal detail type %v fields %#v", detail.RequestType, detail.Fields)
	}
}

func TestSupportConversationFieldsAndType(t *testing.T) {
	svc, db, ws, types := setupRequestTypePortal(t)
	ctx := context.Background()
	bug, err := types.Create(ctx, ws.ID, bugReportRequest())
	if err != nil {
		t.Fatal(err)
	}
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws.ID, DisplayID: 7, Subject: "Slow", Status: "open", Channel: "email"}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	inbox := svc.inbox
	updated, err := inbox.SetConversationRequestType(ctx, ws.ID, conversation.ID, &bug.ID, "")
	if err != nil || updated.RequestTypeID == nil || *updated.RequestTypeID != bug.ID {
		t.Fatalf("set type = %v %v", updated, err)
	}
	if _, err := inbox.SetConversationFields(ctx, ws.ID, conversation.ID, map[string]any{"severity": "critical"}, ""); !errors.Is(err, ErrCustomFieldInvalid) {
		t.Fatalf("bad option = %v", err)
	}
	if _, err := inbox.SetConversationFields(ctx, ws.ID, conversation.ID, map[string]any{"severity": "high", "account_id": "A-1"}, ""); err != nil {
		t.Fatal(err)
	}
	// Unchanged values write no new events.
	if _, err := inbox.SetConversationFields(ctx, ws.ID, conversation.ID, map[string]any{"severity": "high"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.SetConversationFields(ctx, ws.ID, conversation.ID, map[string]any{"account_id": nil}, ""); err != nil {
		t.Fatal(err)
	}
	var stored model.SupportConversation
	if err := db.Where("id = ?", conversation.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CustomFields["severity"] != "high" || stored.CustomFields["account_id"] != nil {
		t.Fatalf("stored fields = %#v", stored.CustomFields)
	}
	var events []string
	if err := db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND message_type = 'system'", conversation.ID).
		Order("created_at").Pluck("content", &events).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{"Helpin set the request type to Bug report.", "Helpin set Account id to A-1.", "Helpin set Severity to High.", "Helpin cleared Account id."}
	if strings.Join(events, "|") != strings.Join(want, "|") {
		t.Fatalf("events = %q, want %q", events, want)
	}
}

func TestSupportSLAPolicyMatchesRequestType(t *testing.T) {
	bug, billing := "type-bug", "type-billing"
	policy := model.SupportSLAPolicy{Enabled: true, RequestTypeIDs: model.DocsStringArray{bug}}
	for _, tc := range []struct {
		name string
		typ  *string
		want bool
	}{
		{"same type", &bug, true},
		{"other type", &billing, false},
		{"no type", nil, false},
	} {
		conversation := model.SupportConversation{Priority: "medium", Channel: "portal", RequestTypeID: tc.typ}
		if got := supportSLAPolicyMatches(policy, conversation, nil); got != tc.want {
			t.Errorf("%s: matches = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSupportFieldCommands(t *testing.T) {
	f := newSupportActionsFixture(t)
	fields := NewCustomFieldService(repository.NewCustomFieldRepository(f.db))
	types := NewSupportRequestTypeService(repository.NewSupportRequestTypeRepository(f.db), fields, repository.NewSupportMailboxRepository(f.db))
	f.inbox.SetRequestTypes(types, fields)
	if _, err := fields.Create(f.ctx, f.workspaceID, "", model.CustomFieldObjectConversation, model.CustomFieldDefinitionRequest{
		Key: "severity", Label: "Severity", FieldType: "select", Options: model.CustomFieldOptions{{Value: "high", Label: "High"}},
	}); err != nil {
		t.Fatal(err)
	}
	bug, err := types.Create(f.ctx, f.workspaceID, model.SupportRequestTypeRequest{Name: "Bug report", Enabled: true, Fields: []model.SupportRequestTypeField{{Key: "severity", CustomerVisible: true}}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.supportInboxService = f.inbox
	meta := model.InternalCommandContext{WorkspaceID: f.workspaceID, ActorID: f.userID, ActorRole: model.RoleOwner}
	conversationID := f.conversation(t, 1)
	run := func(command, input string) (map[string]any, error) {
		t.Helper()
		raw, err := svc.Execute(f.ctx, meta, command, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result, nil
	}
	listed, err := run("support.list_request_types", `{}`)
	if err != nil || len(listed["request_types"].([]any)) != 1 || len(listed["fields"].([]any)) != 1 {
		t.Fatalf("list request types = %v %v", listed, err)
	}
	set, err := run("support.set_conversation_fields", `{"conversation_id":"`+conversationID+`","request_type_id":"`+bug.ID+`","fields":{"severity":"high"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if set["request_type"].(map[string]any)["name"] != "Bug report" || set["fields"].([]any)[0].(map[string]any)["display"] != "High" {
		t.Fatalf("set fields result = %v", set)
	}
	got, err := run("support.get_conversation", `{"conversation_id":"`+conversationID+`"}`)
	if err != nil || got["fields"].([]any)[0].(map[string]any)["customer_visible"] != true {
		t.Fatalf("get conversation fields = %v %v", got, err)
	}
	var commandErr *CommandError
	if _, err := run("support.set_conversation_fields", `{"conversation_id":"`+conversationID+`","fields":{"severity":"low"}}`); !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
		t.Fatalf("bad option = %v, want invalid input", err)
	}
	if _, err := run("support.set_conversation_fields", `{"conversation_id":"`+conversationID+`"}`); !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
		t.Fatalf("nothing to set = %v, want invalid input", err)
	}
	cleared, err := run("support.set_conversation_fields", `{"conversation_id":"`+conversationID+`","request_type_id":null}`)
	if err != nil || cleared["request_type"] != nil {
		t.Fatalf("clear type = %v %v", cleared, err)
	}
}
