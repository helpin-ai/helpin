package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// TestSupportMessageSystemEventInvariant asserts the repository-level
// guardrail: a message_type='system' row without a valid system_event_type
// is rejected. This is the contract that every future emitter must satisfy.
func TestSupportMessageSystemEventInvariant(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewSupportMessageRepository(db)

	tests := []struct {
		name      string
		msg       *model.SupportMessage
		wantError string
	}{
		{
			name: "reply without system event type is accepted",
			msg: &model.SupportMessage{
				WorkspaceID:    "ws-1",
				ConversationID: "conv-1",
				SenderType:     "customer",
				MessageType:    "reply",
				Content:        "Hello",
			},
		},
		{
			name: "system message with valid event type is accepted",
			msg: &model.SupportMessage{
				WorkspaceID:     "ws-1",
				ConversationID:  "conv-1",
				SenderType:      "user",
				MessageType:     "system",
				SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventAssigned),
				Content:         "Azhar assigned this conversation to Jarek",
			},
		},
		{
			name: "system message missing event type is rejected",
			msg: &model.SupportMessage{
				WorkspaceID:    "ws-1",
				ConversationID: "conv-1",
				SenderType:     "user",
				MessageType:    "system",
				Content:        "legacy row",
			},
			wantError: "system message requires a valid system_event_type",
		},
		{
			name: "system message with bogus event type is rejected",
			msg: &model.SupportMessage{
				WorkspaceID:     "ws-1",
				ConversationID:  "conv-1",
				SenderType:      "user",
				MessageType:     "system",
				SystemEventType: ptrToStr("not_a_real_event"),
				Content:         "legacy row",
			},
			wantError: "system message requires a valid system_event_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.msg)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Create: unexpected error %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Create: expected error containing %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Create: error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

// TestSupportSystemEventTypeConstantsAreValid guards against typos or
// unregistered constants slipping into the taxonomy.
func TestSupportSystemEventTypeConstantsAreValid(t *testing.T) {
	cases := []model.SupportSystemEventType{
		model.SystemEventTeammateJoined,
		model.SystemEventAssigned,
		model.SystemEventUnassigned,
		model.SystemEventTook,
		model.SystemEventAgentAssigned,
		model.SystemEventMailboxMoved,
		model.SystemEventTriageRouted,
		model.SystemEventTriageDismissed,
		model.SystemEventAIEscalated,
		model.SystemEventResolved,
		model.SystemEventReopened,
		model.SystemEventClosed,
	}
	for _, ev := range cases {
		t.Run(string(ev), func(t *testing.T) {
			if !model.IsValidSupportSystemEventType(string(ev)) {
				t.Fatalf("constant %q not recognized by IsValidSupportSystemEventType", ev)
			}
		})
	}
}

func ptrToStr(s string) *string { return &s }
