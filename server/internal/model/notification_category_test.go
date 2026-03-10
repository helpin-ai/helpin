package model

import (
	"testing"
)

func TestEventTypeToCategory_AllEventTypesHaveCategory(t *testing.T) {
	for eventType, category := range EventTypeToCategory {
		if category == "" {
			t.Errorf("event type %q maps to empty category", eventType)
		}
	}
}

func TestEventTypeToCategory_UnknownEventType(t *testing.T) {
	category := EventTypeToCategory["unknown.event"]
	if category != "" {
		t.Errorf("expected empty string for unknown event type, got %q", category)
	}
}

func TestNotifCategoryConstants_NonEmpty(t *testing.T) {
	categories := []string{
		NotifCategoryAssignments,
		NotifCategoryStatusChanges,
		NotifCategoryComments,
		NotifCategoryMentions,
		NotifCategorySubscriptions,
		NotifCategorySprints,
	}
	for _, c := range categories {
		if c == "" {
			t.Error("notification category constant is empty")
		}
	}
}

func TestNotifCategoryConstants_UniqueValues(t *testing.T) {
	categories := []string{
		NotifCategoryAssignments,
		NotifCategoryStatusChanges,
		NotifCategoryComments,
		NotifCategoryMentions,
		NotifCategorySubscriptions,
		NotifCategorySprints,
	}
	seen := map[string]bool{}
	for _, c := range categories {
		if seen[c] {
			t.Errorf("duplicate category constant value: %q", c)
		}
		seen[c] = true
	}
}

func TestEventTypeToCategory_KnownMappings(t *testing.T) {
	tests := []struct {
		eventType string
		want      string
	}{
		{"story.assigned", NotifCategoryAssignments},
		{"story.status_changed", NotifCategoryStatusChanges},
		{"comment.created", NotifCategoryComments},
		{"story.mention", NotifCategoryMentions},
		{"epic.created", NotifCategorySubscriptions},
		{"sprint.created", NotifCategorySprints},
	}
	for _, tt := range tests {
		got := EventTypeToCategory[tt.eventType]
		if got != tt.want {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", tt.eventType, got, tt.want)
		}
	}
}

// TestEventTypeToCategory_AllAssignments verifies all assignment event types map to the assignments category.
func TestEventTypeToCategory_AllAssignments(t *testing.T) {
	assignmentEvents := []string{"story.assigned", "objective.assigned"}
	for _, e := range assignmentEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategoryAssignments {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategoryAssignments)
		}
	}
}

// TestEventTypeToCategory_AllStatusChanges verifies all status change event types.
func TestEventTypeToCategory_AllStatusChanges(t *testing.T) {
	statusEvents := []string{"story.status_changed", "story.blocked", "story.updated"}
	for _, e := range statusEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategoryStatusChanges {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategoryStatusChanges)
		}
	}
}

// TestEventTypeToCategory_AllComments verifies all comment event types.
func TestEventTypeToCategory_AllComments(t *testing.T) {
	commentEvents := []string{
		"comment.created", "story.comment", "objective.comment",
		"epic.comment", "sprint.comment",
	}
	for _, e := range commentEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategoryComments {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategoryComments)
		}
	}
}

// TestEventTypeToCategory_AllMentions verifies all mention event types.
func TestEventTypeToCategory_AllMentions(t *testing.T) {
	mentionEvents := []string{
		"story.mention", "comment.mention", "checklist.mention",
		"objective.mention", "epic.mention",
	}
	for _, e := range mentionEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategoryMentions {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategoryMentions)
		}
	}
}

// TestEventTypeToCategory_AllSubscriptions verifies all subscription event types.
func TestEventTypeToCategory_AllSubscriptions(t *testing.T) {
	subEvents := []string{
		"epic.created", "epic.updated", "epic.deleted",
		"objective.created", "objective.updated", "objective.deleted",
	}
	for _, e := range subEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategorySubscriptions {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategorySubscriptions)
		}
	}
}

// TestEventTypeToCategory_AllSprints verifies all sprint event types.
func TestEventTypeToCategory_AllSprints(t *testing.T) {
	sprintEvents := []string{"sprint.created", "sprint.updated"}
	for _, e := range sprintEvents {
		got := EventTypeToCategory[e]
		if got != NotifCategorySprints {
			t.Errorf("EventTypeToCategory[%q] = %q, want %q", e, got, NotifCategorySprints)
		}
	}
}

// TestEventTypeToCategory_AllMappedToValidCategory ensures every mapped event type uses a known category constant.
func TestEventTypeToCategory_AllMappedToValidCategory(t *testing.T) {
	validCategories := map[string]bool{
		NotifCategoryAssignments:   true,
		NotifCategoryStatusChanges: true,
		NotifCategoryComments:      true,
		NotifCategoryMentions:      true,
		NotifCategorySubscriptions: true,
		NotifCategorySprints:       true,
	}
	for eventType, category := range EventTypeToCategory {
		if !validCategories[category] {
			t.Errorf("event type %q maps to unknown category %q", eventType, category)
		}
	}
}

// TestEventTypeToCategory_MapSize ensures the map has the expected number of entries.
func TestEventTypeToCategory_MapSize(t *testing.T) {
	// 24 event types: 2 assignments + 3 status + 5 comments + 5 mentions + 6 subscriptions + 2 sprints + 1 checklist.mention = 24
	// Adjust if new event types are added.
	if len(EventTypeToCategory) < 20 {
		t.Errorf("expected at least 20 event type mappings, got %d", len(EventTypeToCategory))
	}
}

// TestNotificationEventInput_TeamID verifies TeamID field on NotificationEventInput.
func TestNotificationEventInput_TeamID(t *testing.T) {
	input := NotificationEventInput{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		EventType:   "story.assigned",
		EntityType:  "story",
		EntityID:    "story-1",
		TeamID:      "team-1",
	}
	if input.TeamID != "team-1" {
		t.Errorf("expected TeamID 'team-1', got %q", input.TeamID)
	}
}

// TestNotificationEventInput_EmptyTeamID verifies empty TeamID for workspace-level entities.
func TestNotificationEventInput_EmptyTeamID(t *testing.T) {
	input := NotificationEventInput{
		WorkspaceID: "ws-1",
		EventType:   "epic.created",
		EntityType:  "epic",
		EntityID:    "epic-1",
	}
	if input.TeamID != "" {
		t.Errorf("expected empty TeamID for epic, got %q", input.TeamID)
	}
}
