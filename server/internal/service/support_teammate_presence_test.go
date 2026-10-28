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
	ensureSupportModuleGrantsTable(t, db)

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
	seedSupportModuleGrant(t, db, "grant-online", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-online")
	seedSupportModuleGrant(t, db, "grant-away", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-away")
	seedSupportModuleGrant(t, db, "grant-offline", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-offline")

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
	ensureSupportModuleGrantsTable(t, db)

	ownerID := "owner-idle"
	workspaceID := "ws-idle"
	userID := "user-idle"
	seedUser(t, db, ownerID, "owner-idle@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Idle", "idle", ownerID)
	seedUser(t, db, userID, "idle@example.com", "Idle User", "hash")
	seedWorkspaceMember(t, db, "wm-idle", workspaceID, userID, "idle@example.com", "Idle User", "member")
	seedSupportModuleGrant(t, db, "grant-idle", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-idle")

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

func TestListTeammatePresenceRespectsSupportModuleAccess(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)

	workspaceID := "ws-presence-access"
	ownerID := "user-owner-presence"
	supportID := "user-support-presence"
	marketingID := "user-marketing-presence"

	seedUser(t, db, ownerID, "owner-presence@example.com", "Owner User", "hash")
	seedUser(t, db, supportID, "support-presence@example.com", "Support User", "hash")
	seedUser(t, db, marketingID, "marketing-presence@example.com", "Marketing User", "hash")
	seedWorkspace(t, db, workspaceID, "Presence Access", "presence-access", ownerID)
	seedWorkspaceMember(t, db, "wm-owner-presence", workspaceID, ownerID, "owner-presence@example.com", "Owner User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-support-presence", workspaceID, supportID, "support-presence@example.com", "Support User", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-marketing-presence", workspaceID, marketingID, "marketing-presence@example.com", "Marketing User", model.RoleMember)
	seedSupportModuleGrant(t, db, "grant-support-presence", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-support-presence")

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
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetWorkspaceRepo(repository.NewWorkspaceRepository(db))

	statuses, err := svc.ListTeammatePresence(ctx, workspaceID)
	if err != nil {
		t.Fatalf("ListTeammatePresence: %v", err)
	}

	got := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		got[status.UserID] = struct{}{}
	}
	for _, userID := range []string{ownerID, supportID} {
		if _, ok := got[userID]; !ok {
			t.Fatalf("expected %s in presence statuses, got %#v", userID, got)
		}
	}
	if _, ok := got[marketingID]; ok {
		t.Fatalf("did not expect marketing user in presence statuses, got %#v", got)
	}

	marketingStatus, err := svc.GetTeammatePresence(ctx, workspaceID, marketingID)
	if err != nil {
		t.Fatalf("GetTeammatePresence marketing: %v", err)
	}
	if marketingStatus != nil {
		t.Fatalf("expected nil presence for marketing user, got %#v", marketingStatus)
	}
}

func TestUpdateMyTeammatePresence_ManualOverrideWinsAndCanBeCleared(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)

	ownerID := "owner-2"
	workspaceID := "ws-2"
	userID := "user-override"
	seedUser(t, db, ownerID, "owner2@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Override", "override", ownerID)
	seedUser(t, db, userID, "override@example.com", "Override User", "hash")
	seedWorkspaceMember(t, db, "wm-override", workspaceID, userID, "override@example.com", "Override User", "member")
	seedSupportModuleGrant(t, db, "grant-override", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-override")

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

func TestResolveTeammatePresence_ManualOnlineWearsOffWhenIdle(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		idle      time.Duration
		want      string
	}{
		{name: "using Helpin", connected: true, idle: time.Minute, want: model.SupportTeammateStatusOnline},
		{name: "idle within fifteen minutes", connected: true, idle: 10 * time.Minute, want: model.SupportTeammateStatusOnline},
		{name: "idle past fifteen minutes", connected: true, idle: 20 * time.Minute, want: model.SupportTeammateStatusAway},
		{name: "just closed Helpin", connected: false, idle: time.Minute, want: model.SupportTeammateStatusAway},
		{name: "closed Helpin a while ago", connected: false, idle: 20 * time.Minute, want: model.SupportTeammateStatusOffline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			ensureSupportModuleGrantsTable(t, db)

			workspaceID := "ws-manual-online"
			userID := "user-manual-online"
			seedUser(t, db, "owner-manual-online", "owner-manual@example.com", "Owner", "hash")
			seedWorkspace(t, db, workspaceID, "Manual", "manual-online", "owner-manual-online")
			seedUser(t, db, userID, "manual@example.com", "Manual User", "hash")
			seedWorkspaceMember(t, db, "wm-manual-online", workspaceID, userID, "manual@example.com", "Manual User", "member")
			seedSupportModuleGrant(t, db, "grant-manual-online", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-manual-online")

			presence := websocket.NewPresenceState()
			if _, err := presence.SetAgentOnline(ctx, workspaceID, userID, "conn-1"); err != nil {
				t.Fatalf("SetAgentOnline: %v", err)
			}
			if !tt.connected {
				if _, err := presence.SetAgentOffline(ctx, workspaceID, userID, "conn-1"); err != nil {
					t.Fatalf("SetAgentOffline: %v", err)
				}
			}
			overrides := repository.NewSupportTeammateStatusOverrideRepository(db)
			if _, err := overrides.Upsert(ctx, workspaceID, userID, model.SupportTeammateStatusOnline); err != nil {
				t.Fatalf("set manual online: %v", err)
			}

			statuses, err := resolveSupportTeammatePresenceStatuses(ctx, repository.NewWorkspaceRepository(db), presence, overrides, workspaceID, time.Now().Add(tt.idle))
			if err != nil {
				t.Fatalf("resolveSupportTeammatePresenceStatuses: %v", err)
			}
			if len(statuses) != 1 {
				t.Fatalf("expected 1 status, got %d", len(statuses))
			}
			if statuses[0].Status != tt.want {
				t.Fatalf("status = %q, want %q", statuses[0].Status, tt.want)
			}
			if statuses[0].ManualStatus == nil || *statuses[0].ManualStatus != model.SupportTeammateStatusOnline {
				t.Fatalf("manual_status = %v, want the saved Online choice", statuses[0].ManualStatus)
			}
		})
	}
}
