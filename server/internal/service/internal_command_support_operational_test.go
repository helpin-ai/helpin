package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportOperationalCommandsAreRegistered(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	expected := map[string]bool{
		"support.list_conversations": false, "support.get_conversation": false, "support.list_tags": false,
		"support.list_inboxes": false, "support.list_assignees": false,
		"support.assign_conversation": true, "support.move_conversation": true,
		"support.add_conversation_tag": true, "support.remove_conversation_tag": true,
		"support.link_conversation_task": true, "support.link_conversation_contact": true,
		"support.update_conversation_subject": true,
	}
	for name, mutating := range expected {
		def, ok := svc.Definition(name)
		if !ok || def.Tool == nil {
			t.Errorf("missing support operational command %q", name)
			continue
		}
		if def.Mutating != mutating {
			t.Errorf("%s mutating = %v, want %v", name, def.Mutating, mutating)
		}
	}
}

func TestSupportOperationalCommandRejectsConflictingConversationTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", TargetType: "support_conversation", TargetID: "conv-1"}, "support.update_conversation_subject", json.RawMessage(`{"conversation_id":"conv-2","subject":"No"}`))
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected target conflict, got %v", err)
	}
}

func TestSupportOperationalCommandRequiresConfiguredServices(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", TargetType: "support_conversation", TargetID: "conv-1"}, "support.get_conversation", json.RawMessage(`{"conversation_id":"conv-1"}`))
	if err == nil || !strings.Contains(err.Error(), "support operational services") {
		t.Fatalf("expected unavailable service error, got %v", err)
	}
}
