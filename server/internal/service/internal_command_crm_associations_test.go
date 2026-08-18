package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCRMAssociationCommandsLinkListSetPrimaryAndUnlink(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	linked, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.link_objects", json.RawMessage(`{
		"from_object_type":"contact","from_object_id":"contact-1","to_object_type":"company","to_object_id":"company-1"
	}`))
	if err != nil {
		t.Fatalf("link objects: %v", err)
	}
	var association map[string]any
	if err := json.Unmarshal(linked, &association); err != nil {
		t.Fatal(err)
	}
	associationID, _ := association["association_id"].(string)
	if associationID == "" {
		t.Fatalf("missing association ID: %#v", association)
	}

	listed, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.list_associations", json.RawMessage(`{"object_type":"contact","object_id":"contact-1","limit":10}`))
	if err != nil {
		t.Fatalf("list associations: %v", err)
	}
	var listResult map[string]any
	if err := json.Unmarshal(listed, &listResult); err != nil {
		t.Fatal(err)
	}
	if items, ok := listResult["associations"].([]any); !ok || len(items) != 1 {
		t.Fatalf("unexpected associations: %#v", listResult)
	}

	if _, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.set_primary_contact_company", json.RawMessage(`{"contact_id":"contact-1","company_id":"company-1"}`)); err != nil {
		t.Fatalf("set primary company: %v", err)
	}
	if _, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.unlink_association", mustJSON(map[string]any{"association_id": associationID})); err != nil {
		t.Fatalf("unlink association: %v", err)
	}
}

func TestCRMAssociationCommandRejectsMissingAndCrossWorkspaceObjects(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	_, err := env.commands.Execute(context.Background(), env.meta("workspace", "ws-crm-1"), "crm.link_objects", json.RawMessage(`{
		"from_object_type":"contact","from_object_id":"contact-1","to_object_type":"company","to_object_id":"company-other"
	}`))
	if err == nil || !strings.Contains(err.Error(), "company not found") {
		t.Fatalf("expected cross-workspace company rejection, got %v", err)
	}
}
