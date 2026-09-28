package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAddSupportConversationNote(t *testing.T) {
	for _, target := range []string{"support_conversation", "workspace"} {
		t.Run(target, func(t *testing.T) {
			inbox, db, conv, _ := setupAIControlTest(t)
			mustExec(t, db, `CREATE TABLE agents (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, icon_key TEXT, preset_key TEXT)`)
			mustExec(t, db, `INSERT INTO agents VALUES ('agent','ws','Echo','','support_agent')`)
			mustExec(t, db, `INSERT INTO agents VALUES ('other-agent','other','Echo','','support_agent')`)
			svc := NewInternalCommandService(&AgentService{agentRepo: repository.NewAgentRepository(db)}, nil, nil, nil, nil, nil, nil, nil)
			svc.supportInboxService = inbox
			meta := model.InternalCommandContext{WorkspaceID: "ws", AgentID: "agent", TargetType: target, TargetID: conv.ID}
			input := json.RawMessage(`{"content":"  Confirmed the account is active.  "}`)
			if target == "workspace" {
				meta.TargetID = "ws"
				input = json.RawMessage(`{"conversation_id":"conv","content":"  Confirmed the account is active.  "}`)
			}
			out, err := svc.Execute(context.Background(), meta, "support.add_conversation_note", input)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				MessageID string `json:"message_id"`
			}
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			var note model.SupportMessage
			if err := db.First(&note, "id = ?", result.MessageID).Error; err != nil {
				t.Fatal(err)
			}
			if !note.IsInternal || note.MessageType != "note" || note.WidgetVisible() || note.Content != "Confirmed the account is active." {
				t.Fatalf("incorrect internal note: %+v", note)
			}
			if note.SenderType != "agent" || derefString(note.SenderAgentID) != "agent" || note.SenderUserID != nil || derefString(note.SenderDisplayName) != "Echo" {
				t.Fatalf("incorrect attribution: %+v", note)
			}
			current := readControlConversation(t, db)
			if current.Status != conv.Status || derefString(current.AIActiveRunID) != derefString(conv.AIActiveRunID) || current.AIControlVersion != conv.AIControlVersion || derefString(current.FlowState) != derefString(conv.FlowState) {
				t.Fatal("adding a note changed conversation ownership or status")
			}
			for _, bad := range []struct{ name, ws, input string }{
				{"empty", "ws", `{"conversation_id":"conv","content":"  "}`},
				{"wrong workspace", "other", `{"conversation_id":"conv","content":"private"}`},
				{"wrong conversation", "ws", `{"conversation_id":"missing","content":"private"}`},
			} {
				t.Run(bad.name, func(t *testing.T) {
					badMeta := meta
					badMeta.WorkspaceID = bad.ws
					if bad.ws == "other" {
						badMeta.AgentID = "other-agent"
					}
					if _, err := svc.Execute(context.Background(), badMeta, "support.add_conversation_note", json.RawMessage(bad.input)); err == nil {
						t.Fatal("expected rejection")
					}
				})
			}
			// The actual inbox service enforces mailbox access and deleted-customer read-only state.
			mustExec(t, db, `INSERT INTO support_mailboxes (id,workspace_id,name,handle,created_by_id) VALUES ('private','ws','Private','private','owner')`)
			mustExec(t, db, `UPDATE support_conversations SET mailbox_id='private' WHERE id='conv'`)
			privateCtx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: "teammate", WorkspaceID: "ws", WorkspaceMemberID: "member", Role: model.RoleMember})
			if _, err := svc.Execute(privateCtx, meta, "support.add_conversation_note", input); err == nil || !strings.Contains(err.Error(), "conversation not found") {
				t.Fatalf("mailbox access bypassed: %v", err)
			}
			mustExec(t, db, `UPDATE support_conversations SET mailbox_id=NULL, anonymized_at=CURRENT_TIMESTAMP WHERE id='conv'`)
			if _, err := svc.Execute(context.Background(), meta, "support.add_conversation_note", input); err == nil || !strings.Contains(err.Error(), "read-only") {
				t.Fatalf("deleted customer was writable: %v", err)
			}
			svc.SetAuthorizationService(authorization.NewAuthzService(nil, nil, nil))
			viewer := meta
			viewer.ActorRole = model.RoleViewer
			if _, err := svc.Execute(context.Background(), viewer, "support.add_conversation_note", input); err == nil || !strings.Contains(err.Error(), "permission") {
				t.Fatalf("viewer write was allowed: %v", err)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("rejected request persisted a message: %d", count)
			}
			def, _ := svc.Definition("support.add_conversation_note")
			if !def.Mutating || !slices.Contains(def.SupportedTargetTypes, target) || !slices.Contains(commandPermissionsForDefinition(def), authorization.PermSupportEdit) {
				t.Fatal("missing mutation/target/permission contract")
			}
			if def.RiskLevel() != commandtools.RiskLevelRoutine {
				t.Fatal("internal notes should use ordinary product-write policy")
			}
		})
	}
}

func TestSupportNoteAccessForExistingAgents(t *testing.T) {
	for _, preset := range []string{model.AgentPresetSupportAgent, model.AgentPresetAskAgent} {
		out := runtimeAgentFromHelpinAgent(&model.Agent{ID: "existing", PresetKey: preset, IsSystem: true, RuntimeKind: "native_sdk"}, "helpin")
		if !slices.Contains(out.AllowedTools, "add_support_conversation_note") {
			t.Errorf("existing %s lacks note tool", preset)
		}
	}
}
