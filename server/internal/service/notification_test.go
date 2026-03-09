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
			expected: []string{"example.com"},
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
