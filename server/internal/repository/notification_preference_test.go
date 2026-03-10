package repository

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveChannelPref_NilPref(t *testing.T) {
	val, found := resolveChannelPref(nil, "assignments", "in_app")
	if found {
		t.Error("expected not found for nil preference")
	}
	if val {
		t.Error("expected false value for nil preference")
	}
}

func TestResolveChannelPref_NilChannelPreferences(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: nil,
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if found {
		t.Error("expected not found for nil channel_preferences")
	}
	if val {
		t.Error("expected false value")
	}
}

func TestResolveChannelPref_EmptyChannelPreferences(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{},
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if found {
		t.Error("expected not found for empty channel_preferences")
	}
	if val {
		t.Error("expected false value")
	}
}

func TestResolveChannelPref_CategoryNotFound(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"comments": map[string]any{"in_app": true, "email": false},
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if found {
		t.Error("expected not found for missing category")
	}
	if val {
		t.Error("expected false value")
	}
}

func TestResolveChannelPref_ChannelNotFound(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"assignments": map[string]any{"in_app": true},
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "email")
	if found {
		t.Error("expected not found for missing channel")
	}
	if val {
		t.Error("expected false value")
	}
}

func TestResolveChannelPref_InAppEnabled(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"assignments": map[string]any{"in_app": true, "email": false},
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if !found {
		t.Fatal("expected found")
	}
	if !val {
		t.Error("expected in_app to be true")
	}
}

func TestResolveChannelPref_EmailDisabled(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"assignments": map[string]any{"in_app": true, "email": false},
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "email")
	if !found {
		t.Fatal("expected found")
	}
	if val {
		t.Error("expected email to be false")
	}
}

func TestResolveChannelPref_InAppDisabled(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"mentions": map[string]any{"in_app": false, "email": true},
		},
	}
	val, found := resolveChannelPref(pref, "mentions", "in_app")
	if !found {
		t.Fatal("expected found")
	}
	if val {
		t.Error("expected in_app to be false")
	}
}

func TestResolveChannelPref_AllCategories(t *testing.T) {
	categories := []string{
		"assignments", "status_changes", "comments",
		"mentions", "subscriptions", "sprints",
	}
	prefs := model.JSONB{}
	for _, cat := range categories {
		prefs[cat] = map[string]any{"in_app": true, "email": false}
	}
	pref := &model.NotificationPreference{ChannelPreferences: prefs}

	for _, cat := range categories {
		inApp, found := resolveChannelPref(pref, cat, "in_app")
		if !found || !inApp {
			t.Errorf("category %q: expected in_app=true found=true, got in_app=%v found=%v", cat, inApp, found)
		}
		email, found := resolveChannelPref(pref, cat, "email")
		if !found || email {
			t.Errorf("category %q: expected email=false found=true, got email=%v found=%v", cat, email, found)
		}
	}
}

func TestResolveChannelPref_InvalidCategoryValueType(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"assignments": "not_a_map",
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if found {
		t.Error("expected not found when category value is not a map")
	}
	if val {
		t.Error("expected false value")
	}
}

func TestResolveChannelPref_InvalidChannelValueType(t *testing.T) {
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"assignments": map[string]any{"in_app": "yes"},
		},
	}
	val, found := resolveChannelPref(pref, "assignments", "in_app")
	if found {
		t.Error("expected not found when channel value is not a bool")
	}
	if val {
		t.Error("expected false value")
	}
}

// TestResolveChannelPref_JSONDeserialized simulates channel preferences that came
// from JSON deserialization (which produces map[string]interface{} instead of map[string]any).
func TestResolveChannelPref_JSONDeserialized(t *testing.T) {
	// When JSONB is scanned from Postgres, nested maps are map[string]interface{}
	pref := &model.NotificationPreference{
		ChannelPreferences: model.JSONB{
			"comments": map[string]interface{}{"in_app": true, "email": false},
		},
	}
	val, found := resolveChannelPref(pref, "comments", "in_app")
	if !found {
		t.Fatal("expected found for JSON-deserialized data")
	}
	if !val {
		t.Error("expected in_app to be true")
	}

	val, found = resolveChannelPref(pref, "comments", "email")
	if !found {
		t.Fatal("expected found for JSON-deserialized data")
	}
	if val {
		t.Error("expected email to be false")
	}
}
