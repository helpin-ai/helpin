package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// attachPriorityJev turns on only the priority decision so call counts are priority calls.
func attachPriorityJev(t *testing.T, f *supportTriageTestFixture, provider *fakeJev) *SupportTagService {
	t.Helper()
	tags := attachJev(t, f, provider, "off", "off")
	f.triageSvc.jev.config.PriorityMode = "primary"
	return tags
}

func supportConversationPriority(t *testing.T, f *supportTriageTestFixture, conversationID string) string {
	t.Helper()
	var conv model.SupportConversation
	if err := f.db.Where("id = ?", conversationID).Take(&conv).Error; err != nil {
		t.Fatal(err)
	}
	return conv.Priority
}

func aiPriorityNotes(t *testing.T, f *supportTriageTestFixture) []model.SupportMessage {
	t.Helper()
	var notes []model.SupportMessage
	if err := f.db.Where("sender_type = 'ai' AND system_event_type = ?", model.SystemEventPriorityChanged).Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	return notes
}

func TestSupportTagJobRaisesPriority(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99, priority: model.SupportConversationPriorityUrgent}
	attachPriorityJev(t, f, p)
	conv := f.createConversation(t, "Down", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Our whole account is down and nobody can log in")
	runSupportTagJob(t, f, msg)

	if got := supportConversationPriority(t, f, conv.ID); got != model.SupportConversationPriorityUrgent {
		t.Fatalf("priority = %q, want urgent", got)
	}
	notes := aiPriorityNotes(t, f)
	if len(notes) != 1 || p.calls != 1 {
		t.Fatalf("notes=%d calls=%d", len(notes), p.calls)
	}
	if notes[0].Content != "Helpin AI changed the priority from Medium to Urgent." || !notes[0].IsInternal {
		t.Fatalf("note = %q internal=%v", notes[0].Content, notes[0].IsInternal)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(notes[0].Metadata), &metadata); err != nil || metadata["previous_priority"] != "medium" || metadata["priority"] != "urgent" {
		t.Fatalf("metadata = %v err=%v", metadata, err)
	}
}

func TestSupportTagJobOnlyRaisesPriority(t *testing.T) {
	tests := []struct {
		name, current, answer, want string
	}{
		{name: "medium to high", current: "medium", answer: "high", want: "high"},
		{name: "low to medium", current: "low", answer: "medium", want: "medium"},
		{name: "never lowers high", current: "high", answer: "medium", want: "high"},
		{name: "never lowers to low", current: "medium", answer: "low", want: "medium"},
		{name: "same level", current: "high", answer: "high", want: "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			attachPriorityJev(t, f, &fakeJev{probability: .99, priority: tt.answer})
			conv := f.createConversation(t, "Issue", "test@example.com", nil)
			mustExec(t, f.db, "UPDATE support_conversations SET priority = ? WHERE id = ?", tt.current, conv.ID)
			msg := f.createCustomerReply(t, conv.ID, "Something is wrong")
			runSupportTagJob(t, f, msg)

			if got := supportConversationPriority(t, f, conv.ID); got != tt.want {
				t.Fatalf("priority = %q, want %q", got, tt.want)
			}
			wantNotes := 0
			if tt.want != tt.current {
				wantNotes = 1
			}
			if notes := aiPriorityNotes(t, f); len(notes) != wantNotes {
				t.Fatalf("notes = %d, want %d", len(notes), wantNotes)
			}
		})
	}
}

func TestSupportTagJobSkipsPriorityQuestion(t *testing.T) {
	for _, scenario := range []string{"teammate set priority", "setting off", "resolved", "already urgent", "mode off"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
				settings.AIPriorityEnabled = scenario != "setting off"
			})
			p := &fakeJev{probability: .99, priority: model.SupportConversationPriorityUrgent}
			attachPriorityJev(t, f, p)
			conv := f.createConversation(t, "Down", "test@example.com", nil)
			switch scenario {
			case "teammate set priority":
				if _, err := f.supportSvc.UpdateConversationPriority(f.ctx, f.workspaceID, conv.ID, "low", f.actorID); err != nil {
					t.Fatal(err)
				}
			case "resolved":
				mustExec(t, f.db, "UPDATE support_conversations SET status = 'resolved' WHERE id = ?", conv.ID)
			case "already urgent":
				mustExec(t, f.db, "UPDATE support_conversations SET priority = 'urgent' WHERE id = ?", conv.ID)
			case "mode off":
				f.triageSvc.jev.config.PriorityMode = "off"
			}
			before := supportConversationPriority(t, f, conv.ID)
			msg := f.createCustomerReply(t, conv.ID, "Everything is down")
			runSupportTagJob(t, f, msg)

			if p.calls != 0 {
				t.Fatalf("Jev asked about priority: calls=%d", p.calls)
			}
			if got := supportConversationPriority(t, f, conv.ID); got != before {
				t.Fatalf("priority = %q, want %q", got, before)
			}
			if notes := aiPriorityNotes(t, f); len(notes) != 0 {
				t.Fatalf("AI notes = %d", len(notes))
			}
		})
	}
}

func TestSupportTagJobKeepsPriorityWhenUnsure(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	attachPriorityJev(t, f, &fakeJev{probability: .9, priority: model.SupportConversationPriorityUrgent})
	conv := f.createConversation(t, "Down", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "It might be down")
	runSupportTagJob(t, f, msg)

	if got := supportConversationPriority(t, f, conv.ID); got != model.SupportConversationPriorityMedium {
		t.Fatalf("priority = %q, want medium below the threshold", got)
	}
}

func TestSupportTagJobRechecksPriorityAfterProvider(t *testing.T) {
	for _, scenario := range []string{"teammate set priority", "setting turned off", "resolved", "merged"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			p := &fakeJev{probability: .99, priority: model.SupportConversationPriorityUrgent}
			attachPriorityJev(t, f, p)
			conv := f.createConversation(t, "Down", "test@example.com", nil)
			msg := f.createCustomerReply(t, conv.ID, "Everything is down")
			s := queueSupportTagJob(t, f, msg)
			want := model.SupportConversationPriorityMedium
			p.during = func() {
				switch scenario {
				case "teammate set priority":
					want = model.SupportConversationPriorityLow
					if _, err := f.supportSvc.UpdateConversationPriority(f.ctx, f.workspaceID, conv.ID, want, f.actorID); err != nil {
						t.Fatal(err)
					}
				case "setting turned off":
					mustExec(t, f.db, "DELETE FROM support_widget_installations WHERE workspace_id = ?", f.workspaceID)
					seedSupportInstallationSettings(t, f.db, f.workspaceID, func(settings *model.SupportInboxSettings) {
						settings.AIPriorityEnabled = false
					})
				case "resolved":
					mustExec(t, f.db, "UPDATE support_conversations SET status = 'resolved' WHERE id = ?", conv.ID)
				case "merged":
					other := f.createConversation(t, "Other", "test@example.com", nil)
					mustExec(t, f.db, "UPDATE support_conversations SET merged_into_id = ? WHERE id = ?", other.ID, conv.ID)
				}
			}
			if worked, err := s.ProcessNext(f.ctx); err != nil || !worked {
				t.Fatalf("work=%v %v", worked, err)
			}
			if got := supportConversationPriority(t, f, conv.ID); got != want {
				t.Fatalf("priority = %q, want %q", got, want)
			}
			if notes := aiPriorityNotes(t, f); len(notes) != 0 {
				t.Fatalf("AI notes = %d", len(notes))
			}
		})
	}
}

func TestSupportTagJobAsksTagsAndPriorityTogether(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99, priority: model.SupportConversationPriorityHigh}
	tags := attachJev(t, f, p, "off", "primary")
	f.triageSvc.jev.config.PriorityMode = "primary"
	tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Billing"})
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Billing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "The invoice page crashes and I need it for month end today")
	runSupportTagJob(t, f, msg)

	linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID)
	if err != nil || !linked {
		t.Fatalf("tag linked=%v err=%v", linked, err)
	}
	if got := supportConversationPriority(t, f, conv.ID); got != model.SupportConversationPriorityHigh {
		t.Fatalf("priority = %q, want high", got)
	}
	if p.calls != 1 {
		t.Fatalf("calls = %d, want one call for tags and priority", p.calls)
	}
}

func TestSupportTagJobPriorityShadowModeChangesNothing(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99, priority: model.SupportConversationPriorityUrgent}
	attachPriorityJev(t, f, p)
	f.triageSvc.jev.config.PriorityMode = "shadow"
	conv := f.createConversation(t, "Down", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Everything is down")
	runSupportTagJob(t, f, msg)

	if p.calls != 1 {
		t.Fatalf("shadow mode should still ask: calls=%d", p.calls)
	}
	if got := supportConversationPriority(t, f, conv.ID); got != model.SupportConversationPriorityMedium {
		t.Fatalf("priority = %q, want medium", got)
	}
}

func TestAIPrioritySettingDefaultsOnForSavedSettings(t *testing.T) {
	if !parseSettings(`{"triage_enabled":true}`).AIPriorityEnabled {
		t.Fatal("saved settings without the key should keep AI priority on")
	}
	if parseSettings(`{"ai_priority_enabled":false}`).AIPriorityEnabled {
		t.Fatal("an explicit off should stay off")
	}
	off := false
	merged := mergeSettingsUpdate(model.DefaultSupportInboxSettings(), model.UpdateInstallationSettingsRequest{AIPriorityEnabled: &off})
	if merged.AIPriorityEnabled {
		t.Fatal("update did not turn AI priority off")
	}
}
