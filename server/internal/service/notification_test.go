package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
	tests := []struct {
		name            string
		priority        string
		digestFrequency string
		expected        string
	}{
		{name: "urgent bypasses digest", priority: "urgent", digestFrequency: "daily", expected: "email"},
		{name: "high bypasses digest", priority: "high", digestFrequency: "weekly", expected: "email"},
		{name: "normal daily uses digest", priority: "normal", digestFrequency: "daily", expected: "digest"},
		{name: "low weekly uses digest", priority: "low", digestFrequency: "weekly", expected: "digest"},
		{name: "normal immediate uses email", priority: "normal", digestFrequency: "immediate", expected: "email"},
		{name: "normal never disables email", priority: "normal", digestFrequency: "never", expected: ""},
		{name: "low none disables email", priority: "low", digestFrequency: "none", expected: ""},
		{name: "urgent still emails when digest disabled", priority: "urgent", digestFrequency: "never", expected: "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectEmailDeliveryChannel(tt.priority, tt.digestFrequency); got != tt.expected {
				t.Fatalf("selectEmailDeliveryChannel(%q, %q) = %q, want %q", tt.priority, tt.digestFrequency, got, tt.expected)
			}
		})
	}
}
