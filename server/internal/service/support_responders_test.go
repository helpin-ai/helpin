package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestNewSupportResponders(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)
	longAgo := now.Add(-90 * 24 * time.Hour)
	daysAgo := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }
	reply := func(at, joined time.Time) repository.SupportTeammateReply {
		return repository.SupportTeammateReply{UserID: "user-1", RepliedAt: at, MemberCreatedAt: joined}
	}

	tests := []struct {
		name        string
		replies     []repository.SupportTeammateReply
		wantCounted bool
		wantHistory bool
	}{
		{
			name:        "replies on three separate days count",
			replies:     []repository.SupportTeammateReply{reply(daysAgo(1), longAgo), reply(daysAgo(5), longAgo), reply(daysAgo(12), longAgo)},
			wantCounted: true,
			wantHistory: true,
		},
		{
			name:        "replies on two days do not count",
			replies:     []repository.SupportTeammateReply{reply(daysAgo(1), longAgo), reply(daysAgo(5), longAgo)},
			wantHistory: true,
		},
		{
			name: "many replies on one day are one day",
			replies: []repository.SupportTeammateReply{
				reply(daysAgo(2), longAgo), reply(daysAgo(2).Add(time.Hour), longAgo), reply(daysAgo(2).Add(2*time.Hour), longAgo),
			},
			wantHistory: true,
		},
		{
			name: "days are counted in the business timezone",
			// 10:00 and 20:00 New York time on October 19 fall on two UTC dates.
			replies: []repository.SupportTeammateReply{
				reply(time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC), longAgo),
				reply(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), longAgo),
				reply(time.Date(2026, 10, 22, 15, 0, 0, 0, time.UTC), longAgo),
			},
			wantHistory: true,
		},
		{
			name:        "replies older than the window are ignored",
			replies:     []repository.SupportTeammateReply{reply(daysAgo(1), longAgo), reply(daysAgo(2), longAgo), reply(daysAgo(20), longAgo)},
			wantHistory: true,
		},
		{
			name:        "a new member counts after one reply",
			replies:     []repository.SupportTeammateReply{reply(daysAgo(1), daysAgo(3))},
			wantCounted: true,
			wantHistory: true,
		},
		{
			name:        "a member who joined over a week ago needs three days",
			replies:     []repository.SupportTeammateReply{reply(daysAgo(1), daysAgo(10))},
			wantHistory: true,
		},
		{
			name:        "no replies means no history, so everyone counts",
			wantCounted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responders := newSupportResponders(tt.replies, newYork, now)
			if responders.hasReplyHistory != tt.wantHistory {
				t.Fatalf("hasReplyHistory = %v, want %v", responders.hasReplyHistory, tt.wantHistory)
			}
			if got := responders.includes("user-1"); got != tt.wantCounted {
				t.Fatalf("includes(user-1) = %v, want %v", got, tt.wantCounted)
			}
		})
	}
}

func TestNewSupportResponders_UsesAcceptedAtAsJoinDate(t *testing.T) {
	now := time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)
	accepted := now.Add(-2 * 24 * time.Hour)
	replies := []repository.SupportTeammateReply{{
		UserID:          "user-invited-long-ago",
		RepliedAt:       now.Add(-time.Hour),
		AcceptedAt:      &accepted,
		MemberCreatedAt: now.Add(-60 * 24 * time.Hour),
	}}

	if !newSupportResponders(replies, time.UTC, now).includes("user-invited-long-ago") {
		t.Fatal("a member who accepted their invite two days ago should count after one reply")
	}
}

func TestSupportCustomerTeammates_Available(t *testing.T) {
	now := time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)
	responders := supportResponders{userIDs: map[string]struct{}{"responder": {}}, hasReplyHistory: true}
	presenceOf := func(userID string, connected bool, idle time.Duration, manual string) supportTeammatePresence {
		seen := now.Add(-idle)
		teammate := supportTeammatePresence{
			status:    model.SupportTeammatePresenceStatus{UserID: userID, LastSeenAt: &seen},
			connected: connected,
		}
		if manual != "" {
			teammate.status.ManualStatus = &manual
		}
		return teammate
	}

	tests := []struct {
		name     string
		teammate supportTeammatePresence
		want     bool
	}{
		{name: "responder using Helpin", teammate: presenceOf("responder", true, time.Minute, ""), want: true},
		{name: "responder idle for ten minutes", teammate: presenceOf("responder", true, 10*time.Minute, ""), want: true},
		{name: "responder idle past the window", teammate: presenceOf("responder", true, 16*time.Minute, "")},
		{name: "responder who closed Helpin", teammate: presenceOf("responder", false, time.Minute, "")},
		{name: "responder set to away", teammate: presenceOf("responder", true, time.Minute, model.SupportTeammateStatusAway)},
		{name: "responder set to offline", teammate: presenceOf("responder", true, time.Minute, model.SupportTeammateStatusOffline)},
		{name: "responder set to online", teammate: presenceOf("responder", true, time.Minute, model.SupportTeammateStatusOnline), want: true},
		{name: "teammate who does not answer customers", teammate: presenceOf("viewer", true, time.Minute, "")},
		{name: "teammate who does not answer customers set to online", teammate: presenceOf("viewer", true, time.Minute, model.SupportTeammateStatusOnline)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customers := supportCustomerTeammates{teammates: []supportTeammatePresence{tt.teammate}, responders: responders, now: now}
			if got := customers.available(tt.teammate); got != tt.want {
				t.Fatalf("available() = %v, want %v", got, tt.want)
			}
			if got := customers.anyAvailable(); got != tt.want {
				t.Fatalf("anyAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportCustomerTeammates_WithoutReplyHistoryAnyoneAroundCounts(t *testing.T) {
	now := time.Now().UTC()
	seen := now.Add(-time.Minute)
	teammate := supportTeammatePresence{status: model.SupportTeammatePresenceStatus{UserID: "owner", LastSeenAt: &seen}, connected: true}
	customers := supportCustomerTeammates{teammates: []supportTeammatePresence{teammate}, now: now}

	if !customers.anyAvailable() {
		t.Fatal("with no reply history, a teammate who is around should be available")
	}
	if customers.responderUserIDs() != nil {
		t.Fatal("with no reply history, responderUserIDs should be nil")
	}
}

func TestSupportCustomerTeammates_StatusByUserID(t *testing.T) {
	now := time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)
	recent := now.Add(-10 * time.Minute)
	away := model.SupportTeammateStatusAway
	customers := supportCustomerTeammates{
		teammates: []supportTeammatePresence{
			// Idle for ten minutes: Away inside Helpin, still available to customers.
			{status: model.SupportTeammatePresenceStatus{UserID: "responder", Status: model.SupportTeammateStatusAway, LastSeenAt: &recent}, connected: true},
			{status: model.SupportTeammatePresenceStatus{UserID: "viewer", Status: model.SupportTeammateStatusOnline, LastSeenAt: &recent}, connected: true},
			{status: model.SupportTeammatePresenceStatus{UserID: "responder-away", Status: away, ManualStatus: &away, LastSeenAt: &recent}, connected: true},
			{status: model.SupportTeammatePresenceStatus{UserID: "gone", Status: model.SupportTeammateStatusOffline}},
		},
		responders: supportResponders{
			userIDs:         map[string]struct{}{"responder": {}, "responder-away": {}, "gone": {}},
			hasReplyHistory: true,
		},
		now: now,
	}

	want := map[string]string{
		"responder":      model.SupportTeammateStatusOnline,
		"viewer":         model.SupportTeammateStatusAway,
		"responder-away": model.SupportTeammateStatusAway,
		"gone":           model.SupportTeammateStatusOffline,
	}
	got := customers.statusByUserID()
	for userID, status := range want {
		if got[userID] != status {
			t.Errorf("status[%s] = %q, want %q", userID, got[userID], status)
		}
	}
}

func TestSupportResponderDirectory_CountsOnlyPublicTeammateReplies(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-responders"
	seedUser(t, db, "owner", "owner@example.com", "Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Responders", "responders", "owner")
	for _, member := range []struct{ id, userID, status string }{
		{id: "wm-replier", userID: "replier", status: model.WorkspaceMemberStatusActive},
		{id: "wm-noter", userID: "noter", status: model.WorkspaceMemberStatusActive},
		{id: "wm-removed", userID: "removed", status: "removed"},
	} {
		seedUser(t, db, member.userID, member.userID+"@example.com", member.userID, "hash")
		mustExec(t, db, `INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			member.id, workspaceID, member.userID, member.userID+"@example.com", member.userID, model.RoleMember, member.status, time.Now().Add(-60*24*time.Hour), time.Now())
	}

	now := time.Now().UTC()
	seedMessage := func(id, userID, messageType string, internal bool, daysAgo int) {
		t.Helper()
		mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, sender_user_id, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, workspaceID, "conv-1", "user", messageType, userID, "hello", internal, now.Add(-time.Duration(daysAgo)*24*time.Hour), now)
	}
	for day := 1; day <= 3; day++ {
		seedMessage(fmt.Sprintf("reply-%d", day), "replier", "reply", false, day)
		seedMessage(fmt.Sprintf("note-%d", day), "noter", "note", true, day)
		seedMessage(fmt.Sprintf("internal-reply-%d", day), "noter", "reply", true, day)
		seedMessage(fmt.Sprintf("removed-%d", day), "removed", "reply", false, day)
	}

	directory := NewSupportResponderDirectory(repository.NewSupportMessageRepository(db))
	responders, err := directory.responders(ctx, workspaceID, "America/New_York", now)
	if err != nil {
		t.Fatalf("responders: %v", err)
	}
	if !responders.includes("replier") {
		t.Error("a member with public replies on three days should count")
	}
	if responders.includes("noter") {
		t.Error("internal notes and internal replies should not count")
	}
	if responders.includes("removed") {
		t.Error("removed members should not count")
	}

	// A newly qualifying teammate shows up once the cached answer expires.
	for day := 1; day <= 3; day++ {
		seedMessage(fmt.Sprintf("late-%d", day), "noter", "reply", false, day)
	}
	cached, err := directory.responders(ctx, workspaceID, "America/New_York", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("cached responders: %v", err)
	}
	if cached.includes("noter") {
		t.Error("responders should be served from the cache within its lifetime")
	}
	refreshed, err := directory.responders(ctx, workspaceID, "America/New_York", now.Add(supportResponderCacheTTL+time.Minute))
	if err != nil {
		t.Fatalf("refreshed responders: %v", err)
	}
	if !refreshed.includes("noter") {
		t.Error("responders should reload after the cache expires")
	}
}

func TestSupportResponderDirectory_NilCountsEveryone(t *testing.T) {
	var directory *SupportResponderDirectory
	responders, err := directory.responders(context.Background(), "ws-1", "UTC", time.Now())
	if err != nil {
		t.Fatalf("responders: %v", err)
	}
	if !responders.includes("anyone") {
		t.Fatal("a nil directory should count every teammate")
	}
}

func TestSupportAIServiceEscalateToHuman_AfterHoursWhenOnlyNonRespondersAreAround(t *testing.T) {
	tests := []struct {
		name      string
		aroundIDs []string
		want      string
	}{
		{name: "teammates who do not answer customers", aroundIDs: []string{"user-viewer"}, want: model.HandoffStateAfterHours},
		{name: "a support responder", aroundIDs: []string{"user-viewer", "user-responder"}, want: model.HandoffStateLive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			ensureSupportModuleGrantsTable(t, db)

			workspaceID := "ws-after-hours"
			seedUser(t, db, "user-owner", "owner@example.com", "Owner", "hash")
			seedWorkspace(t, db, workspaceID, "After Hours", "after-hours", "user-owner")
			for _, userID := range []string{"user-viewer", "user-responder"} {
				seedUser(t, db, userID, userID+"@example.com", userID, "hash")
				mustExec(t, db, `INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					"wm-"+userID, workspaceID, userID, userID+"@example.com", userID, model.RoleAdmin, model.WorkspaceMemberStatusActive, time.Now().Add(-60*24*time.Hour), time.Now())
			}
			now := time.Now().UTC()
			for day := 1; day <= 3; day++ {
				mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, sender_user_id, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("history-%d", day), workspaceID, "conv-history", "user", "reply", "user-responder", "hello", false, now.Add(-time.Duration(day)*24*time.Hour), now)
			}
			// Closed every day: the workspace is always outside business hours.
			seedSupportInstallationSettings(t, db, workspaceID, func(settings *model.SupportInboxSettings) {
				settings.BusinessHoursEnabled = true
				for day, hours := range settings.BusinessHoursSchedule {
					hours.Enabled = false
					settings.BusinessHoursSchedule[day] = hours
				}
			})

			presence := websocket.NewPresenceState()
			for _, userID := range tt.aroundIDs {
				if _, err := presence.SetAgentOnline(ctx, workspaceID, userID, "conn-"+userID); err != nil {
					t.Fatalf("SetAgentOnline: %v", err)
				}
				if err := presence.TouchAgentActivity(ctx, workspaceID, userID, "conn-"+userID); err != nil {
					t.Fatalf("TouchAgentActivity: %v", err)
				}
			}
			overrides := repository.NewSupportTeammateStatusOverrideRepository(db)
			if _, err := overrides.Upsert(ctx, workspaceID, "user-viewer", model.SupportTeammateStatusOnline); err != nil {
				t.Fatalf("set manual online: %v", err)
			}

			convRepo := repository.NewSupportConversationRepository(db)
			messageRepo := repository.NewSupportMessageRepository(db)
			aiPending := "pending"
			conv := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Need a human", Status: "open", AIState: &aiPending, FlowState: strPtr(model.SupportConversationFlowStateAIHandling)}
			if err := convRepo.Create(ctx, conv); err != nil {
				t.Fatalf("create conversation: %v", err)
			}
			svc := &SupportAIService{
				conversationRepo: convRepo,
				messageRepo:      messageRepo,
				handoffRepo:      repository.NewAgentHandoffRepository(db),
				installationRepo: repository.NewSupportInboxInstallationRepository(db),
			}
			svc.SetSupportRoutingDependencies(repository.NewWorkspaceRepository(db), presence, overrides)
			svc.SetSupportAvailabilitySources(NewSupportResponderDirectory(messageRepo), repository.NewSupportInboxSessionRepository(db))

			if err := svc.EscalateToHuman(ctx, workspaceID, conv.ID, "customer_requested"); err != nil {
				t.Fatalf("EscalateToHuman: %v", err)
			}
			updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if updated.HandoffState == nil || *updated.HandoffState != tt.want {
				t.Fatalf("handoff_state = %v, want %q", derefString(updated.HandoffState), tt.want)
			}
		})
	}
}
