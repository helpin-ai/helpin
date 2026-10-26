package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportWriteCommands(t *testing.T) {
	f := newSupportActionsFixture(t)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.supportInboxService = f.inbox
	meta := model.InternalCommandContext{WorkspaceID: f.workspaceID, ActorID: f.userID, ActorRole: model.RoleOwner}
	conversationID := f.conversation(t, 1)
	run := func(command string, input string) (map[string]any, error) {
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
	expectInput := func(err error, label string) {
		t.Helper()
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
			t.Fatalf("%s: want invalid input, got %v", label, err)
		}
	}

	if result, err := run("support.set_conversation_priority", `{"conversation_id":"`+conversationID+`","priority":"urgent"}`); err != nil || result["priority"] != "urgent" {
		t.Fatalf("priority: %v %v", result, err)
	}
	_, err := run("support.set_conversation_priority", `{"conversation_id":"`+conversationID+`","priority":"critical"}`)
	expectInput(err, "unknown priority")

	until := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if result, err := run("support.snooze_conversation", `{"conversation_id":"`+conversationID+`","until":"`+until+`"}`); err != nil || result["snoozed_until"] == nil {
		t.Fatalf("snooze: %v %v", result, err)
	}
	_, err = run("support.snooze_conversation", `{"conversation_id":"`+conversationID+`","until":"tomorrow"}`)
	expectInput(err, "unparseable snooze time")
	_, err = run("support.snooze_conversation", `{"conversation_id":"`+conversationID+`","until":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)+`"}`)
	expectInput(err, "snooze in the past")
	if result, err := run("support.unsnooze_conversation", `{"conversation_id":"`+conversationID+`"}`); err != nil || result["snoozed_until"] != nil {
		t.Fatalf("unsnooze: %v %v", result, err)
	}

	if _, err := run("support.add_conversation_note", `{"conversation_id":"`+conversationID+`","content":"Checked the invoice."}`); err != nil {
		t.Fatalf("teammate note: %v", err)
	}
	var note model.SupportMessage
	if err := f.db.Where("conversation_id = ? AND message_type = 'note'", conversationID).Take(&note).Error; err != nil ||
		!note.IsInternal || note.SenderType != "user" || note.SenderUserID == nil || *note.SenderUserID != f.userID {
		t.Fatalf("note: %+v %v", note, err)
	}

	if result, err := run("support.set_conversation_status", `{"conversation_id":"`+conversationID+`","status":"resolved"}`); err != nil || result["status"] != "resolved" {
		t.Fatalf("status: %v %v", result, err)
	}
	_, err = run("support.set_conversation_status", `{"conversation_id":"`+conversationID+`","status":"archived"}`)
	expectInput(err, "unknown status")
	_, err = run("support.snooze_conversation", `{"conversation_id":"`+conversationID+`","until":"`+until+`"}`)
	expectInput(err, "snooze a resolved conversation")

	_, err = run("support.reply_to_conversation", `{"conversation_id":"`+conversationID+`","content":"Hi","delivery_mode":"fax"}`)
	expectInput(err, "unknown delivery mode")
	_, err = run("support.create_conversation", `{"customer_email":"Ada <ada@example.com>","subject":"Hi","content":"Hello"}`)
	expectInput(err, "display name in the email")

	var commandErr *CommandError
	for label, id := range map[string]string{"missing": uuid.NewString(), "malformed": "not-a-uuid"} {
		_, err = run("support.set_conversation_priority", `{"conversation_id":"`+id+`","priority":"low"}`)
		if !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorNotFound {
			t.Fatalf("%s conversation: %v", label, err)
		}
	}

	open := f.conversation(t, 2)
	_, err = run("support.reply_to_conversation", `{"conversation_id":"`+open+`","content":"Hi","delivery_mode":"chat_only"}`)
	expectInput(err, "chat reply to an email-only conversation")
	mustExec(t, f.db, `UPDATE support_conversations SET anonymous_id = 'visitor-1' WHERE id = ?`, open)
	reply, err := run("support.reply_to_conversation", `{"conversation_id":"`+open+`","content":"We are on it.","delivery_mode":"chat_only"}`)
	if err != nil || reply["status"] != "sent" || reply["message_id"] == "" {
		t.Fatalf("reply: %v %v", reply, err)
	}
	var sent model.SupportMessage
	if err := f.db.Where("id = ?", reply["message_id"]).Take(&sent).Error; err != nil || sent.IsInternal || sent.SenderType != "user" ||
		sent.SenderUserID == nil || *sent.SenderUserID != f.userID || sent.Content != "We are on it." {
		t.Fatalf("sent reply: %+v %v", sent, err)
	}

	// Without a teammate there is nobody to send a reply as.
	system := model.InternalCommandContext{WorkspaceID: f.workspaceID}
	_, err = svc.Execute(f.ctx, system, "support.reply_to_conversation", json.RawMessage(`{"conversation_id":"`+conversationID+`","content":"Hi"}`))
	if err == nil || !strings.Contains(err.Error(), "teammate") {
		t.Fatalf("reply without an actor: %v", err)
	}
}

func TestSupportWriteCommandsAreRegistered(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	for _, name := range []string{"support.set_conversation_status", "support.set_conversation_priority", "support.snooze_conversation",
		"support.unsnooze_conversation", "support.reply_to_conversation", "support.create_conversation"} {
		if def, ok := svc.Definition(name); !ok || def.Tool == nil || !def.Mutating || def.Execute == nil {
			t.Errorf("support write command %q is not registered", name)
		}
	}
	requirements := parityMCPCommandRequirements()
	for _, alias := range []string{"reply_to_support_conversation", "create_support_conversation"} {
		if requirements[alias].Scope != MCPScopeSupportReply {
			t.Errorf("%s needs the reply scope, has %q", alias, requirements[alias].Scope)
		}
	}
	for _, alias := range []string{"set_support_conversation_status", "set_support_conversation_priority", "snooze_support_conversation", "unsnooze_support_conversation", "add_support_conversation_note"} {
		if requirements[alias].Scope != MCPScopeSupportWrite {
			t.Errorf("%s scope = %q", alias, requirements[alias].Scope)
		}
	}
	if !isMCPWriteScope(MCPScopeSupportReply) {
		t.Error("the reply scope must count as a write scope, so read-only tokens refuse it")
	}
}
