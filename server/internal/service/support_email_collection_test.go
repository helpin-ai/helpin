package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestEmailCollectionSettingsCompatibility(t *testing.T) {
	for _, tt := range []struct {
		raw, timing string
		required    bool
	}{
		{`{}`, "before_handoff", false},
		{`{"require_email_before_chat":true,"force_visitor_identity":true}`, "before_handoff", true},
		{`{"require_email_before_chat":true}`, "before_handoff", false},
		{`{"require_email_before_chat":false}`, "never", false},
		{`{"email_capture_timing":"after_ai_reply","force_visitor_identity":true}`, "after_ai_reply", true},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			settings := parseSettings(tt.raw)
			data, _ := json.Marshal(settings)
			var values map[string]any
			json.Unmarshal(data, &values)
			if values["email_capture_timing"] != tt.timing || settings.ForceVisitorIdentity != tt.required {
				t.Fatalf("timing=%v required=%v", values["email_capture_timing"], settings.ForceVisitorIdentity)
			}
		})
	}
	data, _ := json.Marshal(model.DefaultSupportInboxSettings())
	var defaults map[string]any
	json.Unmarshal(data, &defaults)
	if defaults["email_capture_timing"] != "after_ai_reply" || defaults["force_visitor_identity"] != false {
		t.Fatal("new installations must default to optional capture after AI reply")
	}
}

func TestWidgetRequiredEmailRejectsNextMessageAfterAIReply(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedWorkspace(t, db, "email-ws", "Email", "email-ws", "user-123")
	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	sessions := repository.NewSupportInboxSessionRepository(db)
	installs := repository.NewSupportInboxInstallationRepository(db)
	inst := &model.SupportWidgetInstallation{WorkspaceID: "email-ws", WidgetKey: "email-key", Settings: `{"email_capture_timing":"after_ai_reply","force_visitor_identity":true}`}
	if err := installs.Create(ctx, inst); err != nil {
		t.Fatal(err)
	}
	conv := &model.SupportConversation{WorkspaceID: "email-ws", Subject: "Question", Status: "open", Source: "widget", Channel: "widget"}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	session := &model.SupportWidgetSession{WorkspaceID: "email-ws", SessionToken: "email-token", AnonymousID: "email-anon", IsAnonymous: true, ConversationID: &conv.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := sessions.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	svc := NewSupportInboxService(convRepo, repository.NewSupportMailboxRepository(db), msgRepo, nil, nil, installs, sessions, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "First question", nil); err != nil {
		t.Fatal(err)
	}
	if err := msgRepo.Create(ctx, &model.SupportMessage{WorkspaceID: "email-ws", ConversationID: conv.ID, SenderType: "ai", MessageType: "reply", Content: "Answer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "Second question", nil); err == nil {
		t.Fatal("anonymous visitor bypassed required email")
	}
	if err := db.Model(session).Update("customer_email", "visitor@example.com").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "Second question", nil); err != nil {
		t.Fatalf("identified visitor blocked: %v", err)
	}
}

func TestEmailCollectionPolicyModesAndMilestones(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedWorkspace(t, db, "email-modes", "Email", "email-modes", "user-123")
	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	installs := repository.NewSupportInboxInstallationRepository(db)
	inst := &model.SupportWidgetInstallation{WorkspaceID: "email-modes", WidgetKey: "email-modes-key", Settings: `{}`}
	if err := installs.Create(ctx, inst); err != nil {
		t.Fatal(err)
	}
	conv := &model.SupportConversation{WorkspaceID: inst.WorkspaceID, Subject: "Question", Status: "open", Source: "widget", Channel: "widget"}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	svc := NewSupportInboxService(convRepo, nil, msgRepo, nil, nil, installs, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, tc := range []struct {
		timing                     string
		required, handoff, blocked bool
	}{
		{"before_handoff", true, false, false},
		{"before_handoff", true, true, true},
		{"before_handoff", false, true, false},
		{"after_ai_reply", true, false, false},
		{"never", true, true, false},
	} {
		raw := fmt.Sprintf(`{"email_capture_timing":%q,"force_visitor_identity":%t}`, tc.timing, tc.required)
		if err := db.Model(inst).Update("settings", raw).Error; err != nil {
			t.Fatal(err)
		}
		err := svc.checkWidgetEmailRequirement(ctx, inst.WorkspaceID, conv.ID, "", tc.handoff)
		if (err != nil) != tc.blocked {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	if err := db.Model(inst).Update("settings", `{"email_capture_timing":"after_ai_reply","force_visitor_identity":true}`).Error; err != nil {
		t.Fatal(err)
	}
	for _, msg := range []model.SupportMessage{
		{SenderType: "ai", IsInternal: true, Content: "Private", MessageType: "reply"},
		{SenderType: "ai", Content: "Checking", MessageType: "reply", Metadata: `{"ai_progress_state":"checking"}`},
	} {
		msg.WorkspaceID = inst.WorkspaceID
		msg.ConversationID = conv.ID
		if err := msgRepo.Create(ctx, &msg); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.checkWidgetEmailRequirement(ctx, inst.WorkspaceID, conv.ID, "", false); err != nil {
		t.Fatalf("progress/internal reply gated visitor: %v", err)
	}
	for i := 0; i < 2; i++ {
		publishEmailCollectionStatus(ctx, msgRepo, nil, inst.WorkspaceID, conv.ID, model.SystemEventVisitorEmailRequested)
	}
	if err := db.Model(conv).Update("customer_email", "visitor@example.com").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		publishEmailCollectionStatus(ctx, msgRepo, nil, inst.WorkspaceID, conv.ID, model.SystemEventVisitorEmailProvided)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND message_type = ?", conv.ID, "system").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("wanted two milestones, got %d", count)
	}
}

func TestEmailCollectionSettingsPatch(t *testing.T) {
	current := parseSettings(`{"require_email_before_chat":true,"force_visitor_identity":true}`)
	timing := "after_ai_reply"
	updated := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{EmailCaptureTiming: &timing})
	if updated.EmailCaptureTiming != timing || !updated.ForceVisitorIdentity || !updated.RequireEmailBeforeChat {
		t.Fatal("timing patch lost required setting")
	}
	never := "never"
	updated = mergeSettingsUpdate(updated, model.UpdateInstallationSettingsRequest{EmailCaptureTiming: &never})
	if updated.RequireEmailBeforeChat {
		t.Fatal("legacy clients would still ask for email")
	}
	invalid := model.DefaultSupportInboxSettings()
	invalid.EmailCaptureTiming = "sometimes"
	if err := (&SupportInboxService{}).validateSettings(context.Background(), "ws", invalid); err == nil {
		t.Fatal("unknown timing accepted")
	}
}
