package service

import (
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportReplyEmailActionIncludesConversationTitle(t *testing.T) {
	event := model.NotificationEventInput{
		Category:       model.NotifCategorySupportReplies,
		EntitySnapshot: model.JSONB{"title": "Billing & invoices"},
	}
	if got := immediateEmailActionText(event); got != "replied to “Billing & invoices”" {
		t.Fatalf("action text = %q", got)
	}
	if got := immediateEmailActionText(model.NotificationEventInput{Category: model.NotifCategorySupportReplies}); got != "replied to a conversation" {
		t.Fatalf("fallback action text = %q", got)
	}

	svc := &NotificationService{}
	_, htmlBody, _ := svc.renderImmediateEmail(nil, event)
	if !strings.Contains(htmlBody, "replied to “Billing &amp; invoices”") {
		t.Fatalf("email actor line missing escaped conversation title")
	}
	if strings.Count(htmlBody, "Billing &amp; invoices") != 1 {
		t.Fatalf("email repeats conversation title")
	}
}

func TestImmediateEmailUsesActualItemTypeForCommentsAndMentions(t *testing.T) {
	for _, tc := range []struct{ eventType, entityType, want string }{
		{"comment.created", "epic", "commented on an epic"},
		{"objective.comment", "objective", "commented on an objective"},
		{"sprint.comment", "sprint", "commented on a sprint"},
		{"comment.mention", "epic", "mentioned you in a comment on an epic"},
		{"checklist.mention", "task", "mentioned you in a checklist item"},
		{"doc.mention", "doc", "mentioned you in a document"},
	} {
		event := model.NotificationEventInput{EventType: tc.eventType, EntityType: tc.entityType, EntitySnapshot: model.JSONB{"title": "Roadmap & goals"}, Body: "Please review this"}
		_, body, _ := (&NotificationService{}).renderImmediateEmail(nil, event)
		if !strings.Contains(body, tc.want) || !strings.Contains(body, "Roadmap &amp; goals") || !strings.Contains(body, "Please review this") {
			t.Fatalf("%s on %s email missing context: %s", tc.eventType, tc.entityType, body)
		}
		if strings.Contains(body, "a task assigned to you") {
			t.Fatalf("%s on %s describes the wrong item", tc.eventType, tc.entityType)
		}
	}
}

func TestImmediateCommentEmailSubjectNamesActorAndItem(t *testing.T) {
	event := model.NotificationEventInput{EventType: "comment.created", EntityType: "epic", Title: "commented on Roadmap", ActorSnapshot: model.JSONB{"name": "Alice"}}
	subject, _, _ := (&NotificationService{}).renderImmediateEmail(nil, event)
	if subject != "Alice commented on Roadmap [Helpin]" {
		t.Fatalf("comment subject = %q", subject)
	}
}

func TestBuildEntityURLForNotificationEmails(t *testing.T) {
	base := "https://app.helpin.ai"
	for _, tc := range []struct {
		entityType string
		entityID   string
		want       string
	}{
		{"task", "task-1", base + "/w/acme/pm/tasks/task-1"},
		{"epic", "epic-1", base + "/w/acme/pm/epics/epic-1"},
		{"objective", "objective-1", base + "/w/acme/pm/objectives/objective-1"},
		{"sprint", "sprint-1", base + "/w/acme/pm/sprints/sprint-1"},
		{"support_conversation", "conv-1", base + "/w/acme/support/conv-1"},
		{"doc", "doc-1", base + "/w/acme/docs/documents/doc-1"},
		{"crm_signal", "signal-1", base + "/w/acme/crm/insights?signal=signal-1"},
		{"unknown", "item-1", base + "/w/acme/notifications"},
	} {
		t.Run(tc.entityType, func(t *testing.T) {
			if got := buildEntityURL(base, "acme", tc.entityType, tc.entityID); got != tc.want {
				t.Fatalf("buildEntityURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name:     "single mention",
			body:     "@azhar.usermaven Assigning this to you",
			expected: []string{"azhar.usermaven"},
		},
		{
			name:     "multiple mentions",
			body:     "@muhammad.azhar and @azhar.usermaven please review",
			expected: []string{"muhammad.azhar", "azhar.usermaven"},
		},
		{
			name:     "no mentions",
			body:     "No mentions here",
			expected: nil,
		},
		{
			name:     "duplicate mentions",
			body:     "@azhar.usermaven check this @azhar.usermaven",
			expected: []string{"azhar.usermaven"},
		},
		{
			name:     "mention with hyphen",
			body:     "@john-doe check this",
			expected: []string{"john-doe"},
		},
		{
			name:     "mention with underscore",
			body:     "@john_doe check this",
			expected: []string{"john_doe"},
		},
		{
			name:     "mention at start",
			body:     "@user test",
			expected: []string{"user"},
		},
		{
			name:     "email should not fully match",
			body:     "Send to user@example.com",
			expected: nil,
		},
		{
			name:     "html attributes do not count as mentions",
			body:     `<p><a href="mailto:user@example.com">Email teammate</a></p>`,
			expected: nil,
		},
		{
			name:     "adjacent rich text block mentions",
			body:     `<p>@alice</p><p>@bob</p>`,
			expected: []string{"alice", "bob"},
		},
		{
			name:     "empty body",
			body:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMentions(tt.body)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d mentions, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, m := range got {
				if m != tt.expected[i] {
					t.Errorf("mention[%d]: expected %q, got %q", i, tt.expected[i], m)
				}
			}
		})
	}
}

func TestApplyStateTransition(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		existing       *model.Notification
		newPriority    string
		expectedStatus string
	}{
		{
			name:           "unread stays unread",
			existing:       &model.Notification{Status: "unread"},
			newPriority:    "normal",
			expectedStatus: "unread",
		},
		{
			name:           "read becomes unread on new event",
			existing:       &model.Notification{Status: "read", ReadAt: &now},
			newPriority:    "normal",
			expectedStatus: "unread",
		},
		{
			name:           "archived stays archived on normal",
			existing:       &model.Notification{Status: "archived", ArchivedAt: &now},
			newPriority:    "normal",
			expectedStatus: "archived",
		},
		{
			name:           "archived becomes unread on urgent",
			existing:       &model.Notification{Status: "archived", ArchivedAt: &now},
			newPriority:    "urgent",
			expectedStatus: "unread",
		},
		{
			name:           "snoozed stays snoozed on normal",
			existing:       &model.Notification{Status: "snoozed", SnoozedUntil: &now},
			newPriority:    "normal",
			expectedStatus: "snoozed",
		},
		{
			name:           "snoozed becomes unread on high",
			existing:       &model.Notification{Status: "snoozed", SnoozedUntil: &now},
			newPriority:    "high",
			expectedStatus: "unread",
		},
		{
			name:           "snoozed becomes unread on urgent",
			existing:       &model.Notification{Status: "snoozed", SnoozedUntil: &now},
			newPriority:    "urgent",
			expectedStatus: "unread",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, _, _ := applyStateTransition(tt.existing, tt.newPriority)
			if status != tt.expectedStatus {
				t.Errorf("expected status %q, got %q", tt.expectedStatus, status)
			}
		})
	}
}

func TestEscalatePriority(t *testing.T) {
	tests := []struct {
		current  string
		incoming string
		expected string
	}{
		{"normal", "high", "high"},
		{"high", "normal", "high"},
		{"normal", "normal", "normal"},
		{"low", "urgent", "urgent"},
		{"urgent", "low", "urgent"},
		{"high", "urgent", "urgent"},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.incoming, func(t *testing.T) {
			got := escalatePriority(tt.current, tt.incoming)
			if got != tt.expected {
				t.Errorf("escalatePriority(%q, %q) = %q, want %q", tt.current, tt.incoming, got, tt.expected)
			}
		})
	}
}

func TestNormalizeEmailDigestFrequency(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "default empty to daily", input: "", expected: "daily"},
		{name: "daily", input: "daily", expected: "daily"},
		{name: "weekly", input: "weekly", expected: "weekly"},
		{name: "immediate", input: "immediate", expected: "immediate"},
		{name: "never maps to none", input: "never", expected: "none"},
		{name: "none", input: "none", expected: "none"},
		{name: "unknown defaults to daily", input: "monthly", expected: "daily"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeEmailDigestFrequency(tt.input); got != tt.expected {
				t.Fatalf("normalizeEmailDigestFrequency(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSelectEmailDeliveryChannel(t *testing.T) {
	for _, tc := range []struct{ event, frequency, want string }{
		{"crm.signal_ready", "daily", "digest"},
		{"task.blocked", "weekly", "digest"},
		{"comment.mention", "daily", "email"},
		{"doc.mention", "weekly", "email"},
		{"support_conversation.mentioned", "daily", "email"},
		{"task.agent_attention_required", "never", "email"},
		{"comment.created", "immediate", "email"},
		{"comment.created", "never", ""},
		{"crm.signal_ready", "none", ""},
	} {
		t.Run(tc.event+"/"+tc.frequency, func(t *testing.T) {
			if got := selectEmailDeliveryChannel(tc.event, tc.frequency); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
