package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func queueSupportTagJob(t *testing.T, f *supportTriageTestFixture, msg *model.SupportMessage) *SupportTagJobService {
	t.Helper()
	if err := f.db.AutoMigrate(&model.SupportTagJob{}); err != nil {
		t.Fatal(err)
	}
	job := model.SupportTagJob{MessageID: msg.ID, WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, Status: "pending", AvailableAt: time.Now().Add(-time.Second)}
	if err := f.db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	return NewSupportTagJobService(repository.NewSupportTagJobRepository(f.db), f.messageRepo, f.conversationRepo, f.triageSvc.jev, nil)
}

func runSupportTagJob(t *testing.T, f *supportTriageTestFixture, msg *model.SupportMessage) {
	t.Helper()
	s := queueSupportTagJob(t, f, msg)
	if worked, err := s.ProcessNext(f.ctx); err != nil || !worked {
		t.Fatalf("background tagging=%v %v", worked, err)
	}
}

func TestSupportTagJobCompletesOnce(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	tags := attachJev(t, f, p, "off", "primary")
	tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
	s := queueSupportTagJob(t, f, msg)
	if worked, err := s.ProcessNext(f.ctx); err != nil || !worked {
		t.Fatalf("work=%v %v", worked, err)
	}
	if worked, err := s.ProcessNext(f.ctx); err != nil || worked {
		t.Fatalf("duplicate work=%v %v", worked, err)
	}
	linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID)
	if err != nil || !linked || p.calls != 1 {
		t.Fatalf("linked=%v calls=%d err=%v", linked, p.calls, err)
	}
	var notes int64
	if err := f.db.Model(&model.SupportMessage{}).Where("sender_type = 'ai' AND message_type = 'system'").Count(&notes).Error; err != nil || notes != 1 {
		t.Fatalf("notes=%d %v", notes, err)
	}
}

func TestSupportTagJobRejectsChangesDuringProvider(t *testing.T) {
	for _, scenario := range []string{"manual removal", "source edited", "source deleted", "source internal", "newer customer", "renamed tag", "deleted tag", "privacy", "lost lease", "policy"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			p := &fakeJev{probability: .99}
			tags := attachJev(t, f, p, "off", "primary")
			tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
			if err != nil {
				t.Fatal(err)
			}
			conv := f.createConversation(t, "Pricing", "test@example.com", nil)
			msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
			s := queueSupportTagJob(t, f, msg)
			p.during = func() {
				switch scenario {
				case "manual removal":
					if err := tags.AddConversationTag(f.ctx, f.workspaceID, conv.ID, tag.ID, f.actorID); err != nil {
						t.Fatal(err)
					}
					if err := tags.RemoveConversationTag(f.ctx, f.workspaceID, conv.ID, tag.ID, f.actorID); err != nil {
						t.Fatal(err)
					}
				case "source edited":
					mustExec(t, f.db, "UPDATE support_messages SET content = 'Never mind' WHERE id = ?", msg.ID)
				case "source deleted":
					mustExec(t, f.db, "UPDATE support_messages SET deleted_at = ? WHERE id = ?", time.Now(), msg.ID)
				case "source internal":
					mustExec(t, f.db, "UPDATE support_messages SET is_internal = true WHERE id = ?", msg.ID)
				case "newer customer":
					f.createCustomerReply(t, conv.ID, "The issue is about something else")
				case "renamed tag":
					mustExec(t, f.db, "UPDATE support_tags SET name = 'Security' WHERE id = ?", tag.ID)
				case "deleted tag":
					if err := tags.Delete(f.ctx, f.workspaceID, tag.ID); err != nil {
						t.Fatal(err)
					}
				case "privacy":
					mustExec(t, f.db, "UPDATE support_conversations SET anonymized_at = ? WHERE id = ?", time.Now(), conv.ID)
				case "lost lease":
					mustExec(t, f.db, "UPDATE support_tag_jobs SET lease_token = 'replacement' WHERE message_id = ?", msg.ID)
				case "policy":
					s.jev.config.TagsMode = "off"
				}
			}
			if worked, err := s.ProcessNext(f.ctx); err != nil || !worked {
				t.Fatalf("work=%v %v", worked, err)
			}
			var links, notes int64
			if err := f.db.Model(&model.SupportConversationTag{}).Count(&links).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Model(&model.SupportMessage{}).Where("sender_type = 'ai' AND message_type = 'system'").Count(&notes).Error; err != nil {
				t.Fatal(err)
			}
			if links != 0 || notes != 0 {
				t.Fatalf("stale tagging applied: links=%d notes=%d", links, notes)
			}
		})
	}
}

func TestSupportTagJobRetriesProviderFailure(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99, err: errors.New("offline")}
	tags := attachJev(t, f, p, "off", "primary")
	if _, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"}); err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
	s := queueSupportTagJob(t, f, msg)
	if _, err := s.ProcessNext(f.ctx); err == nil {
		t.Fatal("expected provider failure")
	}
	if worked, err := s.ProcessNext(f.ctx); err != nil || worked {
		t.Fatalf("cooldown ignored: %v %v", worked, err)
	}
	var job model.SupportTagJob
	if err := f.db.First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "pending" || job.Attempts != 1 || !job.AvailableAt.After(time.Now()) {
		t.Fatalf("retry=%+v", job)
	}
	p.err = nil
	mustExec(t, f.db, "UPDATE support_tag_jobs SET available_at = ?", time.Now().Add(-time.Second))
	mustExec(t, f.db, "UPDATE support_conversation_triage_events SET created_at = ? WHERE event_type = 'jev_decision'", time.Now().Add(-2*time.Minute))
	if worked, err := s.ProcessNext(f.ctx); err != nil || !worked {
		t.Fatalf("retry=%v %v", worked, err)
	}
	if p.calls != 2 {
		t.Fatalf("calls=%d", p.calls)
	}
}

func TestSupportTagJobStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { defer close(done); (&SupportTagJobService{}).Run(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tagging worker ignored shutdown")
	}
}

type taggingDeniedPolicy struct{}

func (taggingDeniedPolicy) RequireFeature(context.Context, string, EntitlementFeature) error {
	return errors.New("feature unavailable")
}
func (taggingDeniedPolicy) RequireLimitUsage(context.Context, string, EntitlementLimit, int64, int64) error {
	return nil
}

func TestSupportTagJobSkipsIneligibleBeforeProvider(t *testing.T) {
	for _, scenario := range []string{"off", "no provider", "workspace excluded", "entitlement", "deleted", "privacy", "superseded"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			p := &fakeJev{probability: .99}
			tags := attachJev(t, f, p, "off", "primary")
			if _, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"}); err != nil {
				t.Fatal(err)
			}
			conv := f.createConversation(t, "Pricing", "test@example.com", nil)
			msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
			s := queueSupportTagJob(t, f, msg)
			switch scenario {
			case "off":
				s.jev.config.TagsMode = "off"
			case "no provider":
				s.jev = nil
			case "workspace excluded":
				s.jev.workspaces = map[string]bool{"another": true}
			case "entitlement":
				s.entitlements = taggingDeniedPolicy{}
			case "deleted":
				mustExec(t, f.db, "UPDATE support_messages SET deleted_at = ? WHERE id = ?", time.Now(), msg.ID)
			case "privacy":
				mustExec(t, f.db, "UPDATE support_conversations SET anonymized_at = ? WHERE id = ?", time.Now(), conv.ID)
			case "superseded":
				f.createCustomerReply(t, conv.ID, "Never mind")
			}
			if _, err := s.ProcessNext(f.ctx); err != nil {
				t.Fatal(err)
			}
			if p.calls != 0 {
				t.Fatalf("ineligible provider calls=%d", p.calls)
			}
			var job model.SupportTagJob
			if err := f.db.First(&job).Error; err != nil || job.Status != "done" {
				t.Fatalf("job not acknowledged: %+v %v", job, err)
			}
		})
	}
}

func TestSupportTagJobAuditFailureRollsBackAndReusesDecision(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	tags := attachJev(t, f, p, "off", "primary")
	tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
	s := queueSupportTagJob(t, f, msg)
	mustExec(t, f.db, "CREATE TRIGGER reject_tag_note BEFORE INSERT ON support_messages WHEN NEW.message_type='system' BEGIN SELECT RAISE(FAIL, 'audit unavailable'); END")
	if _, err := s.ProcessNext(f.ctx); err == nil {
		t.Fatal("missing audit acknowledged as success")
	}
	if linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID); err != nil || linked {
		t.Fatalf("unaudited tag persisted: %v %v", linked, err)
	}
	mustExec(t, f.db, "DROP TRIGGER reject_tag_note")
	mustExec(t, f.db, "UPDATE support_tag_jobs SET available_at = ?", time.Now().Add(-time.Second))
	// A new worker has only persisted state, including the successful decision.
	s = NewSupportTagJobService(repository.NewSupportTagJobRepository(f.db), f.messageRepo, f.conversationRepo, f.triageSvc.jev, nil)
	if _, err := s.ProcessNext(f.ctx); err != nil {
		t.Fatal(err)
	}
	if linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID); err != nil || !linked || p.calls != 1 {
		t.Fatalf("recovery linked=%v calls=%d err=%v", linked, p.calls, err)
	}
	var notes int64
	if err := f.db.Model(&model.SupportMessage{}).Where("sender_type = 'ai' AND message_type = 'system'").Count(&notes).Error; err != nil || notes != 1 {
		t.Fatalf("duplicate audit notes=%d %v", notes, err)
	}
}

func TestSupportTagManualMutationAndAuditAreAtomic(t *testing.T) {
	for _, add := range []bool{true, false} {
		name := "remove"
		if add {
			name = "add"
		}
		t.Run(name, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			tags := attachJev(t, f, &fakeJev{}, "off", "primary")
			tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
			if err != nil {
				t.Fatal(err)
			}
			conv := f.createConversation(t, "Pricing", "test@example.com", nil)
			if !add {
				if err := tags.AddConversationTag(f.ctx, f.workspaceID, conv.ID, tag.ID, f.actorID); err != nil {
					t.Fatal(err)
				}
			}
			mustExec(t, f.db, "CREATE TRIGGER reject_tag_note BEFORE INSERT ON support_messages WHEN NEW.message_type='system' BEGIN SELECT RAISE(FAIL, 'audit unavailable'); END")
			if err := tags.changeConversationTag(f.ctx, f.workspaceID, conv.ID, tag.ID, f.actorID, add); err == nil {
				t.Fatal("missing audit did not roll back manual change")
			}
			if linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID); err != nil || linked == add {
				t.Fatalf("manual edit escaped rollback: %v %v", linked, err)
			}
		})
	}
}
