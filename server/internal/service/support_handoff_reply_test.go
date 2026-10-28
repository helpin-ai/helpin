package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestComposeHandoffEmail(t *testing.T) {
	tests := []struct {
		name, customer, update, want string
	}{
		{name: "greets by first name", customer: "Sarah Lee", want: "Hi Sarah,\n\nWe have it.\n\nBest regards,"},
		{name: "greets without a name", want: "Hi,\n\nWe have it.\n\nBest regards,"},
		{name: "never greets by email address", customer: "sarah@example.com", want: "Hi,\n\nWe have it.\n\nBest regards,"},
		{name: "puts the checked update first", customer: "Sarah", update: "Your posts failed on Monday.", want: "Hi Sarah,\n\nYour posts failed on Monday.\n\nWe have it.\n\nBest regards,"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeHandoffEmail(tt.customer, tt.update, " We have it. "); got != tt.want {
				t.Fatalf("composeHandoffEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeNoticeTranslator stands in for translation and records what it was asked.
type fakeNoticeTranslator struct {
	result string
	asked  []string
}

func (f *fakeNoticeTranslator) TranslateCustomerNotice(_ context.Context, _, _, text, _ string) string {
	f.asked = append(f.asked, text)
	return f.result
}

// escalateAndReadHandoffReply hands a fresh conversation to the team and returns
// the message the customer got.
func escalateAndReadHandoffReply(t *testing.T, conv model.SupportConversation, mutate func(*model.SupportInboxSettings), translator supportCustomerNoticeTranslator) *model.SupportMessage {
	t.Helper()
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)
	workspaceID := "ws-handoff-reply"
	seedUser(t, db, "user-owner", "owner@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Handoff Reply", "handoff-reply", "user-owner")
	seedSupportInstallationSettings(t, db, workspaceID, mutate)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	conv.WorkspaceID, conv.Subject, conv.Status = workspaceID, "Posts failed", "open"
	conv.AIState = strPtr("pending")
	conv.FlowState = strPtr(model.SupportConversationFlowStateAIHandling)
	if err := convRepo.Create(ctx, &conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	svc := &SupportAIService{
		conversationRepo: convRepo,
		messageRepo:      messageRepo,
		handoffRepo:      repository.NewAgentHandoffRepository(db),
		installationRepo: repository.NewSupportInboxInstallationRepository(db),
	}
	svc.SetSupportRoutingDependencies(repository.NewWorkspaceRepository(db), nil, repository.NewSupportTeammateStatusOverrideRepository(db))
	svc.SetSupportAvailabilitySources(NewSupportResponderDirectory(messageRepo), repository.NewSupportInboxSessionRepository(db))
	if translator != nil {
		svc.SetCustomerNoticeTranslator(translator)
	}
	if err := svc.EscalateToHuman(ctx, workspaceID, conv.ID, "customer_requested"); err != nil {
		t.Fatalf("EscalateToHuman: %v", err)
	}
	messages, err := messageRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
	if err != nil {
		t.Fatalf("ListByConversation: %v", err)
	}
	for i := range messages {
		if messages[i].SenderType == "ai" && messages[i].MessageType == "reply" {
			return &messages[i]
		}
	}
	t.Fatal("no handoff message was sent to the customer")
	return nil
}

func TestSupportAIServiceEscalateToHuman_EmailHandoffReadsAsEmail(t *testing.T) {
	emailConv := model.SupportConversation{Channel: "email", Source: "email", CustomerName: strPtr("Sarah Lee")}
	bothChannels := func(settings *model.SupportInboxSettings) { settings.AIReplyChannels = "both" }

	t.Run("during business hours it says the team will reply, whoever is online", func(t *testing.T) {
		// Nobody is around, so a chat would get the busy message.
		reply := escalateAndReadHandoffReply(t, emailConv, bothChannels, nil)
		want := "Hi Sarah,\n\n" + model.DefaultEmailHandoffMessage + "\n\nBest regards,"
		if reply.Content != want {
			t.Fatalf("email handoff = %q, want %q", reply.Content, want)
		}
		if derefString(reply.ViaChannel) != "email" {
			t.Fatalf("via_channel = %q, want email", derefString(reply.ViaChannel))
		}
	})

	t.Run("after hours it says when the team is back", func(t *testing.T) {
		// Open only on the day after tomorrow, so the team is away now.
		openDay := strings.ToLower(time.Now().UTC().AddDate(0, 0, 2).Weekday().String()[:3])
		reply := escalateAndReadHandoffReply(t, emailConv, func(settings *model.SupportInboxSettings) {
			bothChannels(settings)
			settings.BusinessHoursEnabled = true
			settings.BusinessHoursTimezone = "UTC"
			for day, hours := range settings.BusinessHoursSchedule {
				hours.Enabled = day == openDay
				hours.Start, hours.End = "09:00", "17:00"
				settings.BusinessHoursSchedule[day] = hours
			}
		}, nil)
		if !strings.HasPrefix(reply.Content, "Hi Sarah,\n\nThanks for getting in touch.") || !strings.HasSuffix(reply.Content, "\n\nBest regards,") {
			t.Fatalf("after-hours email is not laid out as an email: %q", reply.Content)
		}
		if !strings.Contains(reply.Content, "The team is away right now and back on ") || !strings.Contains(reply.Content, "at 9:00 AM UTC.") {
			t.Fatalf("after-hours email does not say when the team is back: %q", reply.Content)
		}
	})

	t.Run("a workspace's own email message is used", func(t *testing.T) {
		reply := escalateAndReadHandoffReply(t, emailConv, func(settings *model.SupportInboxSettings) {
			bothChannels(settings)
			settings.EmailHandoffMessage = "Our team has your email."
		}, nil)
		if reply.Content != "Hi Sarah,\n\nOur team has your email.\n\nBest regards," {
			t.Fatalf("email handoff = %q", reply.Content)
		}
	})
}

func TestSupportAIServiceEscalateToHuman_ChatHandoffKeepsStateMessage(t *testing.T) {
	// Business hours are off and nobody is around: the chat busy message.
	reply := escalateAndReadHandoffReply(t, model.SupportConversation{Channel: "widget"}, nil, nil)
	if !strings.HasPrefix(reply.Content, "I've notified the team.") {
		t.Fatalf("chat handoff = %q, want the team busy message", reply.Content)
	}
}

func TestSupportAIServiceEscalateToHuman_TranslatesHandoffMessage(t *testing.T) {
	t.Run("sends the translation", func(t *testing.T) {
		translator := &fakeNoticeTranslator{result: "He avisado al equipo."}
		reply := escalateAndReadHandoffReply(t, model.SupportConversation{Channel: "widget"}, nil, translator)
		if reply.Content != "He avisado al equipo." {
			t.Fatalf("handoff = %q, want the translation", reply.Content)
		}
		if len(translator.asked) != 1 || !strings.HasPrefix(translator.asked[0], "I've notified the team.") {
			t.Fatalf("translator was asked %q, want the handoff message", translator.asked)
		}
	})

	t.Run("sends the original when translation comes back empty", func(t *testing.T) {
		reply := escalateAndReadHandoffReply(t, model.SupportConversation{Channel: "widget"}, nil, &fakeNoticeTranslator{})
		if !strings.HasPrefix(reply.Content, "I've notified the team.") {
			t.Fatalf("handoff = %q, want the original message", reply.Content)
		}
	})
}

func TestTranslateCustomerNotice(t *testing.T) {
	const notice = "Hello, our team has your message."
	t.Run("translates into the customer's language", func(t *testing.T) {
		env, conv, provider := translationFixture(t)
		got := env.service.supportInboxService.TranslateCustomerNotice(context.Background(), conv.WorkspaceID, conv.ID, notice, "en")
		if got != "Hallo, our team has your message." || provider.calls != 1 {
			t.Fatalf("notice = %q after %d calls, want the German translation", got, provider.calls)
		}
	})
	t.Run("keeps the text when the customer shares the team's language", func(t *testing.T) {
		env, conv, provider := translationFixture(t)
		if got := env.service.supportInboxService.TranslateCustomerNotice(context.Background(), conv.WorkspaceID, conv.ID, notice, "de"); got != notice || provider.calls != 0 {
			t.Fatalf("notice = %q after %d calls, want it unchanged without a call", got, provider.calls)
		}
	})
	t.Run("keeps the text when translation fails", func(t *testing.T) {
		env, conv, provider := translationFixture(t)
		provider.fail = true
		if got := env.service.supportInboxService.TranslateCustomerNotice(context.Background(), conv.WorkspaceID, conv.ID, notice, "en"); got != notice {
			t.Fatalf("notice = %q, want it unchanged", got)
		}
	})
	t.Run("keeps the text without translation set up", func(t *testing.T) {
		if got := (&SupportInboxService{}).TranslateCustomerNotice(context.Background(), "ws", "conv", notice, "en"); got != notice {
			t.Fatalf("notice = %q, want it unchanged", got)
		}
	})
}

func TestSupportTeamHoursLine(t *testing.T) {
	tests := []struct {
		name          string
		open          bool
		nextOpen      string
		customerClock bool
		replyTime     string
		want          string
	}{
		{name: "open", open: true, replyTime: "a few minutes", want: "Team availability: open now, inside business hours. During business hours the team usually replies within a few minutes."},
		{name: "closed on the business clock", nextOpen: "tomorrow at 9:00 AM EDT", want: "Team availability: closed now; back tomorrow at 9:00 AM EDT."},
		{name: "closed on the customer's clock", nextOpen: "tomorrow at 6:00 PM", customerClock: true, want: "Team availability: closed now; back tomorrow at 6:00 PM on the customer's clock."},
		{name: "closed with no opening", want: "Team availability: closed now, outside business hours."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportTeamHoursLine(tt.open, tt.nextOpen, tt.customerClock, tt.replyTime); got != tt.want {
				t.Fatalf("supportTeamHoursLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeSupportConversationSummary_IncludesTeamAvailability(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv-1", Subject: "Posts failed", Channel: "widget"}
	summary := runtimeSupportConversationSummary(conv, nil, "Team availability: closed now; back tomorrow at 9:00 AM EDT.")
	if !strings.Contains(summary, "Conversation channel: chat.\nTeam availability: closed now; back tomorrow at 9:00 AM EDT.") {
		t.Fatalf("summary does not carry the team availability after the channel:\n%s", summary)
	}
	if strings.Contains(runtimeSupportConversationSummary(conv, nil, ""), "Team availability") {
		t.Fatal("summary should not mention availability when none is known")
	}
}

func TestUpdateInstallationSettings_EmailHandoffMessages(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	svc := env.service.supportInboxService
	workspaceID := "11111111-1111-1111-1111-111111111111"

	custom, empty := "  Our team has your email.  ", " "
	_, settings, err := svc.UpdateInstallationSettings(ctx, workspaceID, model.UpdateInstallationSettingsRequest{EmailHandoffMessage: &custom, EmailHandoffMessageAfterHours: &empty})
	if err != nil {
		t.Fatalf("UpdateInstallationSettings: %v", err)
	}
	if settings.EmailHandoffMessage != "Our team has your email." {
		t.Errorf("email_handoff_message = %q, want it trimmed", settings.EmailHandoffMessage)
	}
	if settings.EmailHandoffMessageAfterHours != model.DefaultEmailHandoffMessageAfterHours {
		t.Errorf("an empty after-hours message should fall back to the default, got %q", settings.EmailHandoffMessageAfterHours)
	}

	tooLong := strings.Repeat("a", 2001)
	if _, _, err := svc.UpdateInstallationSettings(ctx, workspaceID, model.UpdateInstallationSettingsRequest{EmailHandoffMessage: &tooLong}); err == nil {
		t.Fatal("a message over 2000 characters should be rejected")
	}
}
