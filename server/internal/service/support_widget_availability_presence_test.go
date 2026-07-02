package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// TestWidgetAvailabilityRequiresPresence drives the public widget config
// builder and asserts that IsOnline reflects actual teammate presence rather
// than business hours alone: within hours (here, business hours disabled ->
// always within) with nobody online must report offline, and a teammate coming
// online must flip it to online.
func TestWidgetAvailabilityRequiresPresence(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	mustExec(t, db, `CREATE TABLE IF NOT EXISTS workspace_module_grants (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		module TEXT NOT NULL,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		access_level TEXT NOT NULL DEFAULT 'member',
		created_by_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	const (
		workspaceID = "ws-widget-presence"
		ownerID     = "user-owner"
		supportID   = "user-support"
	)

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hash")
	seedUser(t, db, supportID, "support@example.com", "Support Member", "hash")
	seedWorkspace(t, db, workspaceID, "Widget Presence", "widget-presence", ownerID)
	seedWorkspaceMember(t, db, "wm-owner", workspaceID, ownerID, "owner@example.com", "Owner User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-support", workspaceID, supportID, "support@example.com", "Support Member", model.RoleMember)
	mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level) VALUES (?, ?, ?, ?, ?, ?)`,
		"grant-support", workspaceID, model.ModuleSupport, model.ModuleGrantSubjectWorkspaceMember, "wm-support", model.ModuleGrantAccessLevelMember)

	// Business hours disabled => always "within hours". This isolates the
	// presence gate from the schedule and keeps the test time-independent.
	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = false
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inst := &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		Settings:    string(raw),
		Active:      true,
	}

	presence := websocket.NewPresenceState()

	svc := NewSupportInboxService(
		repository.NewSupportConversationRepository(db),
		repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db),
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)
	svc.SetWorkspaceRepo(repository.NewWorkspaceRepository(db))
	svc.SetPresenceProvider(presence)

	// Within hours, nobody online -> not online.
	resp, err := svc.buildWidgetConfigResponse(ctx, inst)
	if err != nil {
		t.Fatalf("buildWidgetConfigResponse (offline): %v", err)
	}
	if resp.Availability.IsOnline {
		t.Fatalf("expected IsOnline=false when no teammate is online, got true")
	}
	if resp.Availability.StatusText == "Online now" {
		t.Fatalf("expected status text to surface reply expectation, not %q", resp.Availability.StatusText)
	}

	// A support teammate comes online -> widget reports online.
	if _, err := presence.SetAgentOnline(ctx, workspaceID, supportID, "conn-support"); err != nil {
		t.Fatalf("SetAgentOnline support: %v", err)
	}
	resp, err = svc.buildWidgetConfigResponse(ctx, inst)
	if err != nil {
		t.Fatalf("buildWidgetConfigResponse (online): %v", err)
	}
	if !resp.Availability.IsOnline {
		t.Fatalf("expected IsOnline=true when a teammate is online, got false")
	}
	if resp.Availability.StatusText != "Online now" {
		t.Fatalf("expected 'Online now' status text, got %q", resp.Availability.StatusText)
	}
}

// TestBuildWidgetAvailabilityGatesOnPresence checks all four presence/hours
// quadrants deterministically, including presence trumping hours.
func TestBuildWidgetAvailabilityGatesOnPresence(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "America/New_York"

	withinHours := time.Date(2026, 3, 24, 10, 30, 0, 0, loc) // Tue, open
	outsideHours := time.Date(2026, 3, 24, 8, 0, 0, 0, loc)  // Tue, before opening

	t.Run("within hours but nobody online is offline", func(t *testing.T) {
		a := buildWidgetAvailability(settings, withinHours, false)
		if a.IsOnline {
			t.Fatalf("expected IsOnline=false")
		}
		if a.StatusText == "Online now" {
			t.Fatalf("expected reply-expectation status text, got %q", a.StatusText)
		}
	})

	t.Run("within hours with online teammate is online", func(t *testing.T) {
		a := buildWidgetAvailability(settings, withinHours, true)
		if !a.IsOnline {
			t.Fatalf("expected IsOnline=true")
		}
		if a.StatusText != "Online now" {
			t.Fatalf("expected 'Online now', got %q", a.StatusText)
		}
	})

	t.Run("outside hours with online teammate is online (presence trumps hours)", func(t *testing.T) {
		a := buildWidgetAvailability(settings, outsideHours, true)
		if !a.IsOnline {
			t.Fatalf("expected IsOnline=true when a teammate is online outside hours")
		}
		// Offline-branch hours fields remain driven by the schedule.
		if a.NextOnlineAt == nil {
			t.Fatalf("expected NextOnlineAt to remain populated from business hours")
		}
	})

	t.Run("outside hours with nobody online is offline", func(t *testing.T) {
		a := buildWidgetAvailability(settings, outsideHours, false)
		if a.IsOnline {
			t.Fatalf("expected IsOnline=false")
		}
	})
}
