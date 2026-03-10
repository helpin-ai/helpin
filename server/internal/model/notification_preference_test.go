package model

import (
	"encoding/json"
	"testing"
)

func TestNotificationPreference_DefaultValues(t *testing.T) {
	pref := NotificationPreference{
		UserID:               "user-1",
		WorkspaceID:          "ws-1",
		EmailEnabled:         true,
		EmailDigestFrequency: "daily",
		BadgeMode:            "all",
		ChannelPreferences:   JSONB{},
	}
	if !pref.EmailEnabled {
		t.Error("expected email_enabled to be true by default")
	}
	if pref.BadgeMode != "all" {
		t.Errorf("expected badge_mode 'all', got %q", pref.BadgeMode)
	}
	if pref.ChannelPreferences == nil {
		t.Error("expected channel_preferences to be non-nil")
	}
}

func TestNotificationPreference_TeamID_Nil(t *testing.T) {
	pref := NotificationPreference{
		UserID:      "user-1",
		WorkspaceID: "ws-1",
	}
	if pref.TeamID != nil {
		t.Error("expected workspace-level preference to have nil team_id")
	}
}

func TestNotificationPreference_TeamID_Set(t *testing.T) {
	teamID := "team-1"
	pref := NotificationPreference{
		UserID:      "user-1",
		WorkspaceID: "ws-1",
		TeamID:      &teamID,
	}
	if pref.TeamID == nil || *pref.TeamID != "team-1" {
		t.Error("expected team-level preference to have team_id 'team-1'")
	}
}

func TestNotificationPreference_ChannelPreferences_CategoryShape(t *testing.T) {
	raw := `{"assignments": {"in_app": true, "email": false}, "comments": {"in_app": true, "email": true}}`
	var cp JSONB
	if err := json.Unmarshal([]byte(raw), &cp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	pref := NotificationPreference{
		ChannelPreferences: cp,
	}

	assignments, ok := pref.ChannelPreferences["assignments"]
	if !ok {
		t.Fatal("expected 'assignments' key in channel_preferences")
	}
	assignMap, ok := assignments.(map[string]any)
	if !ok {
		t.Fatal("expected assignments to be a map")
	}
	if inApp, ok := assignMap["in_app"].(bool); !ok || !inApp {
		t.Error("expected assignments.in_app to be true")
	}
	if email, ok := assignMap["email"].(bool); !ok || email {
		t.Error("expected assignments.email to be false")
	}
}

func TestNotificationPreference_ChannelPreferences_AllCategoriesShape(t *testing.T) {
	raw := `{
		"assignments": {"in_app": true, "email": false},
		"status_changes": {"in_app": false, "email": true},
		"comments": {"in_app": true, "email": true},
		"mentions": {"in_app": true, "email": true},
		"subscriptions": {"in_app": false, "email": false},
		"sprints": {"in_app": true, "email": false}
	}`
	var cp JSONB
	if err := json.Unmarshal([]byte(raw), &cp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	expected := map[string]struct{ inApp, email bool }{
		"assignments":    {true, false},
		"status_changes": {false, true},
		"comments":       {true, true},
		"mentions":       {true, true},
		"subscriptions":  {false, false},
		"sprints":        {true, false},
	}

	for category, want := range expected {
		catVal, ok := cp[category]
		if !ok {
			t.Errorf("missing category %q", category)
			continue
		}
		catMap, ok := catVal.(map[string]any)
		if !ok {
			t.Errorf("category %q is not a map", category)
			continue
		}
		if gotInApp, _ := catMap["in_app"].(bool); gotInApp != want.inApp {
			t.Errorf("category %q in_app: got %v, want %v", category, gotInApp, want.inApp)
		}
		if gotEmail, _ := catMap["email"].(bool); gotEmail != want.email {
			t.Errorf("category %q email: got %v, want %v", category, gotEmail, want.email)
		}
	}
}

func TestUpdateNotificationPreferenceRequest_WithTeamID(t *testing.T) {
	teamID := "team-1"
	req := UpdateNotificationPreferenceRequest{
		TeamID: &teamID,
		ChannelPreferences: map[string]any{
			"mentions": map[string]any{"in_app": false, "email": false},
		},
	}
	if req.TeamID == nil || *req.TeamID != "team-1" {
		t.Error("expected team_id to be set")
	}
	if req.ChannelPreferences == nil {
		t.Error("expected channel_preferences to be set")
	}
}

func TestUpdateNotificationPreferenceRequest_PartialUpdate(t *testing.T) {
	enabled := true
	req := UpdateNotificationPreferenceRequest{
		EmailEnabled: &enabled,
	}
	if req.DoNotDisturb != nil {
		t.Error("expected do_not_disturb to be nil for partial update")
	}
	if req.EmailEnabled == nil || !*req.EmailEnabled {
		t.Error("expected email_enabled to be true")
	}
	if req.TeamID != nil {
		t.Error("expected team_id to be nil for workspace-level update")
	}
}

func TestUpdateNotificationPreferenceRequest_WithoutTeamID(t *testing.T) {
	dnd := true
	req := UpdateNotificationPreferenceRequest{
		DoNotDisturb: &dnd,
	}
	if req.TeamID != nil {
		t.Error("expected team_id to be nil")
	}
	if req.DoNotDisturb == nil || !*req.DoNotDisturb {
		t.Error("expected do_not_disturb to be true")
	}
}

func TestUpdateNotificationPreferenceRequest_ChannelPreferencesJSON(t *testing.T) {
	raw := `{
		"channel_preferences": {
			"assignments": {"in_app": true, "email": false},
			"mentions": {"in_app": false, "email": false}
		},
		"team_id": "team-abc"
	}`
	var req UpdateNotificationPreferenceRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if req.TeamID == nil || *req.TeamID != "team-abc" {
		t.Error("expected team_id to be 'team-abc'")
	}
	if req.ChannelPreferences == nil {
		t.Fatal("expected channel_preferences to be set")
	}
	if len(req.ChannelPreferences) != 2 {
		t.Errorf("expected 2 categories in channel_preferences, got %d", len(req.ChannelPreferences))
	}
}

func TestNotificationPreference_TableName(t *testing.T) {
	pref := NotificationPreference{}
	if pref.TableName() != "notification_preferences" {
		t.Errorf("expected table name 'notification_preferences', got %q", pref.TableName())
	}
}

func TestNotificationPreference_WorkspaceLevelIsDefault(t *testing.T) {
	// A workspace-level preference is the "default" — TeamID is nil
	pref := NotificationPreference{
		UserID:             "u1",
		WorkspaceID:        "w1",
		EmailEnabled:       true,
		ChannelPreferences: JSONB{},
	}
	if pref.TeamID != nil {
		t.Error("workspace-level preference should have nil TeamID")
	}
	if !pref.EmailEnabled {
		t.Error("workspace-level default should have email enabled")
	}
}

func TestNotificationPreference_TeamOverrideOnlyChannelPrefs(t *testing.T) {
	// Team-level overrides only set ChannelPreferences — other fields are inherited from workspace
	teamID := "team-x"
	teamPref := NotificationPreference{
		UserID:      "u1",
		WorkspaceID: "w1",
		TeamID:      &teamID,
		ChannelPreferences: JSONB{
			"comments": map[string]any{"in_app": false, "email": false},
		},
	}
	if teamPref.TeamID == nil {
		t.Fatal("expected team_id to be set")
	}
	if *teamPref.TeamID != "team-x" {
		t.Errorf("expected team_id 'team-x', got %q", *teamPref.TeamID)
	}
	comments, ok := teamPref.ChannelPreferences["comments"]
	if !ok {
		t.Fatal("expected 'comments' in channel_preferences")
	}
	m, ok := comments.(map[string]any)
	if !ok {
		t.Fatal("expected comments to be a map")
	}
	if inApp, _ := m["in_app"].(bool); inApp {
		t.Error("expected team override to have comments.in_app=false")
	}
}
