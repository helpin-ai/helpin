package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestListTeammatePresence_UsesLivePresenceAndRecentLastSeen(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ownerID := "owner-1"
	workspaceID := "ws-1"
	seedUser(t, db, ownerID, "owner@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", ownerID)

	userOnline := "user-online"
	userAway := "user-away"
	userOffline := "user-offline"
	seedUser(t, db, userOnline, "online@example.com", "Online User", "hash")
	seedUser(t, db, userAway, "away@example.com", "Away User", "hash")
	seedUser(t, db, userOffline, "offline@example.com", "Offline User", "hash")

	seedWorkspaceMember(t, db, "wm-online", workspaceID, userOnline, "online@example.com", "Online User", "member")
	seedWorkspaceMember(t, db, "wm-away", workspaceID, userAway, "away@example.com", "Away User", "member")
	seedWorkspaceMember(t, db, "wm-offline", workspaceID, userOffline, "offline@example.com", "Offline User", "member")

	presence := websocket.NewPresenceState()
	if _, err := presence.SetAgentOnline(ctx, workspaceID, userOnline, "conn-1"); err != nil {
		t.Fatalf("SetAgentOnline online: %v", err)
	}
	if _, err := presence.SetAgentOnline(ctx, workspaceID, userAway, "conn-2"); err != nil {
		t.Fatalf("SetAgentOnline away: %v", err)
	}
	if _, err := presence.SetAgentOffline(ctx, workspaceID, userAway, "conn-2"); err != nil {
		t.Fatalf("SetAgentOffline away: %v", err)
	}

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

	statuses, err := svc.ListTeammatePresence(ctx, workspaceID)
	if err != nil {
		t.Fatalf("ListTeammatePresence: %v", err)
	}

	got := make(map[string]model.SupportTeammatePresenceStatus, len(statuses))
	for _, status := range statuses {
		got[status.UserID] = status
	}

	if got[userOnline].Status != model.SupportTeammateStatusOnline {
		t.Fatalf("online user status = %q, want %q", got[userOnline].Status, model.SupportTeammateStatusOnline)
	}
	if got[userAway].Status != model.SupportTeammateStatusAway {
		t.Fatalf("away user status = %q, want %q", got[userAway].Status, model.SupportTeammateStatusAway)
	}
	if got[userOffline].Status != model.SupportTeammateStatusOffline {
		t.Fatalf("offline user status = %q, want %q", got[userOffline].Status, model.SupportTeammateStatusOffline)
	}
	if got[userAway].LastSeenAt == nil {
		t.Fatal("expected away user to include last_seen_at")
	}
	if got[userOnline].Source != model.SupportTeammateStatusSourceAuto {
		t.Fatalf("online user source = %q, want %q", got[userOnline].Source, model.SupportTeammateStatusSourceAuto)
	}
}

func TestResolveTeammatePresence_ConnectedIdleUserShowsAway(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ownerID := "owner-idle"
	workspaceID := "ws-idle"
	userID := "user-idle"
	seedUser(t, db, ownerID, "owner-idle@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Idle", "idle", ownerID)
	seedUser(t, db, userID, "idle@example.com", "Idle User", "hash")
	seedWorkspaceMember(t, db, "wm-idle", workspaceID, userID, "idle@example.com", "Idle User", "member")

	presence := websocket.NewPresenceState()
	if _, err := presence.SetAgentOnline(ctx, workspaceID, userID, "conn-1"); err != nil {
		t.Fatalf("SetAgentOnline: %v", err)
	}

	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		repository.NewWorkspaceRepository(db),
		presence,
		nil,
		workspaceID,
		time.Now().UTC().Add(supportTeammateAwayThreshold+time.Second),
	)
	if err != nil {
		t.Fatalf("resolveSupportTeammatePresenceStatuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != model.SupportTeammateStatusAway {
		t.Fatalf("connected idle status = %q, want %q", statuses[0].Status, model.SupportTeammateStatusAway)
	}
}

func TestUpdateMyTeammatePresence_ManualOverrideWinsAndCanBeCleared(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ownerID := "owner-2"
	workspaceID := "ws-2"
	userID := "user-override"
	seedUser(t, db, ownerID, "owner2@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Override", "override", ownerID)
	seedUser(t, db, userID, "override@example.com", "Override User", "hash")
	seedWorkspaceMember(t, db, "wm-override", workspaceID, userID, "override@example.com", "Override User", "member")

	presence := websocket.NewPresenceState()
	if _, err := presence.SetAgentOnline(ctx, workspaceID, userID, "conn-1"); err != nil {
		t.Fatalf("SetAgentOnline: %v", err)
	}

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
	svc.SetStatusOverrideRepo(repository.NewSupportTeammateStatusOverrideRepository(db))

	manualOffline := model.SupportTeammateStatusOffline
	status, err := svc.UpdateMyTeammatePresence(ctx, workspaceID, userID, &manualOffline)
	if err != nil {
		t.Fatalf("UpdateMyTeammatePresence manual: %v", err)
	}
	if status.Status != model.SupportTeammateStatusOffline {
		t.Fatalf("manual status = %q, want %q", status.Status, model.SupportTeammateStatusOffline)
	}
	if status.Source != model.SupportTeammateStatusSourceManual {
		t.Fatalf("manual source = %q, want %q", status.Source, model.SupportTeammateStatusSourceManual)
	}
	if status.ManualStatus == nil || *status.ManualStatus != model.SupportTeammateStatusOffline {
		t.Fatalf("manual_status = %#v, want %q", status.ManualStatus, model.SupportTeammateStatusOffline)
	}

	status, err = svc.UpdateMyTeammatePresence(ctx, workspaceID, userID, nil)
	if err != nil {
		t.Fatalf("UpdateMyTeammatePresence clear: %v", err)
	}
	if status.Status != model.SupportTeammateStatusOnline {
		t.Fatalf("cleared status = %q, want %q", status.Status, model.SupportTeammateStatusOnline)
	}
	if status.Source != model.SupportTeammateStatusSourceAuto {
		t.Fatalf("cleared source = %q, want %q", status.Source, model.SupportTeammateStatusSourceAuto)
	}
	if status.ManualStatus != nil {
		t.Fatalf("manual_status = %#v, want nil after clear", status.ManualStatus)
	}
}
