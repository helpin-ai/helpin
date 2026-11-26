package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPersonalSlackNotifications(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	db := newNotificationServiceTestDB(t)
	mustExecNotificationService(t, db, "CREATE TABLE support_channel_accounts (id TEXT PRIMARY KEY, workspace_id TEXT, provider TEXT, disabled_at DATETIME, handle TEXT, external_account_id TEXT, token_encrypted TEXT)")
	mustExecNotificationService(t, db, "CREATE TABLE slack_members (account_id TEXT, user_id TEXT, slack_user TEXT, notify_assignments BOOLEAN DEFAULT false, notify_mentions BOOLEAN DEFAULT false, notify_categories TEXT DEFAULT '{}', dm_window TEXT NOT NULL DEFAULT '[]', PRIMARY KEY(account_id,user_id))")
	if err := db.AutoMigrate(&model.SupportInboundJob{}); err != nil {
		t.Fatal(err)
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := crypto.EncryptStringWithAAD("test-token", key, channelTokenAAD("install"))
	if err != nil {
		t.Fatal(err)
	}
	mustExecNotificationService(t, db, "INSERT INTO support_channel_accounts VALUES ('install','ws','slack',NULL,'UBOT','T1',?)", token)
	mustExecNotificationService(t, db, "INSERT INTO slack_members(account_id,user_id,slack_user) VALUES ('install','member','U1'),('install','follower','U2'),('install','actor','U3')")
	mustExecNotificationService(t, db, "INSERT INTO entity_followers (entity_type,entity_id,user_id,workspace_id) VALUES ('objective','item','follower','ws')")
	seedNotificationServiceWorkspace(t, db, "ws", "My workspace", now)
	seedNotificationServiceUserSettings(t, db, "member", false, "none", nil, now)
	mustExecNotificationService(t, db, "INSERT INTO notification_preferences(user_id,workspace_id,mute_workspace,channel_preferences) VALUES ('member','ws',false,?)", []byte(`{"assignments":{"in_app":false},"mentions":{"in_app":false}}`))
	repo := repository.NewSlackRepository(db)
	jobs := repository.NewSupportInboundJobRepository(db)
	n := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), repository.NewFollowerRepository(db), repository.NewUserRepository(db), repository.NewWorkspaceRepository(db), nil, nil, "https://helpin.example")
	n.SetSlackNotifications(repo, jobs)
	var sent []map[string]any
	var edits []map[string]any
	var failNextEdit bool
	api := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var message map[string]any
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/chat.postMessage":
			sent = append(sent, message)
		case "/chat.update":
			if failNextEdit {
				failNextEdit = false
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":"internal_error"}`))}, nil
			}
			edits = append(edits, message)
		default:
			t.Errorf("unexpected method %s", r.URL.Path)
		}
		response := fmt.Sprintf(`{"ok":true,"channel":"DM","ts":"%d.1"}`, len(sent))
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	s := &SlackService{repo: repo, notifications: n, channels: &SupportChannelService{cfg: SupportChannelConfig{EncryptionKey: key}, now: func() time.Time { return now }}, api: &slack.Client{BaseURL: "https://slack.example/", HTTP: api}}
	emit := func(kind string) {
		t.Helper()
		err := n.Emit(ctx, model.NotificationEventInput{WorkspaceID: "ws", ActorID: "actor", ActorSnapshot: model.JSONB{"name": "Alex"}, EventType: kind, EntityType: "objective", EntityID: "item", Title: "updated <@U2>", ExplicitRecipients: []string{"member", "actor"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	countJobs := func(want int64) {
		t.Helper()
		var count int64
		if err := db.Model(&model.SupportInboundJob{}).Count(&count).Error; err != nil || count != want {
			t.Fatalf("jobs = %d, want %d, err %v", count, want, err)
		}
	}
	if pref, err := s.NotificationPreferences(ctx, "ws", "unlinked"); err != nil || pref.Linked || pref.TeamID != "T1" {
		t.Fatalf("unlinked settings = %+v, %v", pref, err)
	}
	emit("objective.assigned")
	countJobs(0)
	on := true
	for _, user := range []string{"member", "actor", "follower"} {
		if err := s.SetNotificationPreferences(ctx, "ws", user, model.UpdateSlackNotificationPreferences{Assignments: &on}); err != nil {
			t.Fatal(err)
		}
	}
	emit("objective.mention")
	countJobs(0)
	emit("objective.assigned")
	countJobs(1)
	job, err := jobs.Claim(ctx, SlackNotificationJob, time.Now())
	if err != nil || job == nil {
		t.Fatalf("claim: %v, %v", job, err)
	}
	if err := s.deliverNotification(ctx, job); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0]["channel"] != "U1" || sent[0]["client_msg_id"] != job.ID || sent[0]["unfurl_links"] != false {
		t.Fatalf("unexpected DM: %#v", sent)
	}
	text, _ := sent[0]["text"].(string)
	if sent[0]["mrkdwn"] != false || !strings.Contains(text, "Alex updated <@U2>") || !strings.Contains(text, "https://helpin.example/w/my-workspace/pm/objectives/item") || !strings.Contains(text, "/settings/notifications?slack=projects.assignments") {
		t.Fatalf("unexpected DM content: %q", text)
	}
	mustExecNotificationService(t, db, "UPDATE notification_preferences SET mute_workspace=true WHERE user_id='member'")
	if err := s.deliverNotification(ctx, job); err != nil {
		t.Fatal(err)
	}
	mustExecNotificationService(t, db, "UPDATE notification_preferences SET mute_workspace=false WHERE user_id='member'")
	off := false
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Assignments: &off, Mentions: &on}); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.NotificationPreferences(ctx, "ws", "member")
	if err != nil || stored.Overrides["support.assignments"] != false || stored.Overrides["docs.mentions"] != true {
		t.Fatalf("expected explicit family switches to survive future default changes: %+v, %v", stored.Overrides, err)
	}
	if err := s.deliverNotification(ctx, job); err != nil {
		t.Fatal(err)
	}
	emit("objective.mention")
	countJobs(2)
	mention, err := jobs.Claim(ctx, SlackNotificationJob, time.Now())
	if err != nil || mention == nil {
		t.Fatalf("claim mention: %v, %v", mention, err)
	}
	n.SetAccessChecker(notificationAccessFunc(func(context.Context, string, model.NotificationEventInput) (bool, error) { return false, nil }))
	if err := s.deliverNotification(ctx, mention); err != nil {
		t.Fatal(err)
	}
	n.SetAccessChecker(nil)
	mustExecNotificationService(t, db, "UPDATE user_notification_settings SET do_not_disturb=true WHERE user_id='member'")
	if err := s.deliverNotification(ctx, mention); err != nil {
		t.Fatal(err)
	}
	mustExecNotificationService(t, db, "UPDATE user_notification_settings SET do_not_disturb=false WHERE user_id='member'")
	mustExecNotificationService(t, db, "UPDATE slack_members SET slack_user='Uchanged' WHERE user_id='member'")
	if err := s.deliverNotification(ctx, mention); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 {
		t.Fatalf("suppressed DM delivered: %#v", sent)
	}
	mustExecNotificationService(t, db, "UPDATE slack_members SET slack_user='U1' WHERE user_id='member'")
	if err := s.deliverNotification(ctx, mention); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent[1]["client_msg_id"] != mention.ID {
		t.Fatalf("missing mention: %#v", sent)
	}
	// A row override is independent of other modules and takes effect for
	// pending jobs. A later edit to another row preserves this one.
	for _, patch := range []map[string]bool{{"docs.mentions": false}, {"crm.signals": true}} {
		if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Categories: patch}); err != nil {
			t.Fatal(err)
		}
	}
	pref, err := s.NotificationPreferences(ctx, "ws", "member")
	if err != nil || pref.Categories["docs.mentions"] || !pref.Categories["projects.mentions"] || !pref.Categories["crm.signals"] || pref.Categories["projects.assignments"] {
		t.Fatalf("wrong effective settings: %+v, err %v", pref, err)
	}
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Categories: map[string]bool{"unknown": true}}); err == nil {
		t.Fatal("accepted an unknown row")
	}

	allModules := []model.NotificationEventInput{
		{EntityType: "agent_run", EventType: "task.agent_attention_required", Metadata: model.JSONB{"automation_rule_id": "a", "interaction_id": "i"}},
		{EntityType: "agent_run", EventType: "task.agent_attention_required", Metadata: model.JSONB{"interaction_id": "q"}},
		{EntityType: "crm_signal", EventType: "crm.signal_ready"},
		{EntityType: "external_mcp_server", EventType: "external_mcp.reauthorization_required"},
		{EntityType: "support_conversation", EventType: "support_conversation.mentioned"},
	}
	for i, event := range allModules {
		event.WorkspaceID, event.EntityID, event.Title = "ws", "item", "Personal alert"
		if err := n.queueSlackNotification(ctx, "member", event); err != nil {
			t.Fatal(err)
		}
		countJobs(int64(3 + i))
	}
	automation := allModules[0]
	automation.WorkspaceID, automation.EntityID = "ws", "item"
	if err := n.queueSlackNotification(ctx, "member", automation); err != nil {
		t.Fatal(err)
	}
	countJobs(7)
	mustExecNotificationService(t, db, "CREATE TABLE agent_runs(id TEXT, workspace_id TEXT, status TEXT)")
	mustExecNotificationService(t, db, "CREATE TABLE agent_run_interactions(id TEXT,workspace_id TEXT,run_id TEXT,status TEXT,request_id TEXT)")
	mustExecNotificationService(t, db, "INSERT INTO agent_runs VALUES ('item','ws','paused')")
	mustExecNotificationService(t, db, "INSERT INTO agent_run_interactions VALUES ('row','ws','item','pending','i')")
	checkAction := func(want bool) {
		t.Helper()
		actual, err := repo.NotificationActionPending(ctx, "member", automation)
		if err != nil || actual != want {
			t.Fatalf("action pending = %v (want %v): %v", actual, want, err)
		}
	}
	checkAction(true)
	mustExecNotificationService(t, db, "UPDATE agent_run_interactions SET status='resolved'")
	checkAction(false)
	pending, err := jobs.Claim(ctx, SlackNotificationJob, time.Now())
	if err != nil || pending == nil {
		t.Fatalf("claim automation: %v", err)
	}
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Categories: map[string]bool{"automation.attention": false}}); err != nil {
		t.Fatal(err)
	}
	if err := s.deliverNotification(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 {
		t.Fatal("delivered a queued alert after its row was disabled")
	}
	// A CRM-linked project task uses exactly one row, on initial assignment
	// and on later reassignment from the task screen.

	mustExecNotificationService(t, db, "INSERT INTO crm_associations VALUES ('ws','company','account','task','linked')")
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Categories: map[string]bool{"crm.assignments": true}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id       string
		metadata model.JSONB
		jobs     int64
	}{
		{"ordinary", nil, 7}, {"new", model.JSONB{"crm_follow_up": true}, 8}, {"linked", nil, 9},
	} {
		if err := n.Emit(ctx, model.NotificationEventInput{ExplicitRecipients: []string{"member"}, SkipFollowers: true, WorkspaceID: "ws", EntityType: "task", EntityID: tc.id, EventType: "task.assigned", Metadata: tc.metadata}); err != nil {
			t.Fatal(err)
		}
		countJobs(tc.jobs)
	}
	t.Run("personal DM bursts", func(t *testing.T) {
		try := func(jobID, item string) error {
			t.Helper()
			payload, err := json.Marshal(slackNotification{UserID: "actor", SlackUser: "U3", EntityType: "objective", EntityID: item, EventType: "objective.assigned", Title: "Assignment " + item})
			if err != nil {
				t.Fatal(err)
			}
			j := model.SupportInboundJob{ID: jobID, WorkspaceID: strPtr("ws"), ChannelAccountID: strPtr("install"), Payload: string(payload)}
			return s.deliverNotification(ctx, &j)
		}
		deliver := func(jobID, item string) {
			t.Helper()
			if err := try(jobID, item); err != nil {
				t.Fatal(err)
			}
		}
		before := len(sent)
		for i := 1; i <= 8; i++ {
			item := fmt.Sprintf("item-%d", i)
			deliver(item, item)
		}
		deliver("replay", "item-1")
		if len(sent)-before != 5 || len(edits) == 0 {
			t.Fatalf("burst created %d new DMs and %d edits", len(sent)-before, len(edits))
		}
		last := edits[len(edits)-1]
		content := fmt.Sprint(last["text"])
		for _, item := range []string{"item-5", "item-6", "item-7", "item-8"} {
			if !strings.Contains(content, "/pm/objectives/"+item) {
				t.Errorf("grouped DM omitted the link to %s", item)
			}
		}
		if last["channel"] != "DM" || last["ts"] != fmt.Sprintf("%d.1", before+5) {
			t.Fatalf("edit targeted the wrong DM: %+v", last)
		}
		n.SetAccessChecker(notificationAccessFunc(func(_ context.Context, _ string, event model.NotificationEventInput) (bool, error) {
			return event.EntityID != "item-6", nil
		}))
		failNextEdit = true
		if err := try("retry", "ninth"); err == nil {
			t.Fatal("Slack edit failure was swallowed")
		}
		deliver("retry", "ninth")
		deliver("retry", "ninth")
		content = fmt.Sprint(edits[len(edits)-1]["text"])
		if strings.Contains(content, "/item-6") || !strings.Contains(content, "/ninth") || len(sent)-before != 5 || len(edits) != 4 {
			t.Fatalf("retried edit lost an alert, exposed a revoked item or pinged again: %s", content)
		}
		n.SetAccessChecker(nil)
		now = now.Add(time.Hour)
		deliver("next-hour", "item-1")
		if len(sent)-before != 6 {
			t.Fatal("expected a new DM when the rolling hour expired")
		}
		for i := 1; i < maxItemsPerDM*maxPersonalDMsPerHour; i++ {
			item := fmt.Sprintf("bulk-%d", i)
			deliver(item, item)
		}
		var wait *personalDMDeferred
		if err := try("overflow", "last-item"); !errors.As(err, &wait) || !wait.until.Equal(now.Add(time.Hour)) {
			t.Fatalf("expected the full window to defer the extra item until capacity returns, got %v", err)
		}
		now = wait.until
		deliver("overflow", "last-item")
		if len(sent)-before != 11 || !strings.Contains(fmt.Sprint(sent[len(sent)-1]["text"]), "/last-item") {
			t.Fatal("deferred item did not arrive when capacity returned")
		}
	})
	mustExecNotificationService(t, db, "UPDATE support_channel_accounts SET disabled_at=? WHERE id='install'", now)
	emit("objective.mention")
	countJobs(9)
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Mentions: &off}); err == nil {
		t.Fatal("updated disconnected installation")
	}
}

func TestSlackModuleRouting(t *testing.T) {
	for _, tc := range []struct {
		kind, entity, row string
		direct            bool
	}{
		{"task.assigned", "task", "projects.assignments", true},
		{"task.status_changed", "task", "projects.status_changes", false},
		{"task.comment", "task", "projects.comments", false},
		{"task.mention", "task", "projects.mentions", true},
		{"task.created", "task", "projects.subscriptions", false},
		{"sprint.updated", "sprint", "projects.sprints", false},
		{"comment.mention", "document", "docs.mentions", true},
		{"doc.updated", "document", "docs.updates", false},
		{"automation_rule.failed", "automation_rule", "automation.failures", true},
		{"crm.assigned", "crm_company", "crm.assignments", true},
		{"crm.mention", "crm_contact", "crm.mentions", true},
		{"crm.signal_ready", "crm_signal", "crm.signals", false},
		{"crm.deal_updated", "crm_deal", "crm.updates", false},
		{"support_conversation.assigned", "support_conversation", "support.assignments", true},
		{"support_conversation.mentioned", "support_conversation", "support.mentions", true},
		{"support_conversation.customer_reply", "support_conversation", "support.replies", false},
		{"support_conversation.linked_task_completed", "support_conversation", "support.replies", false},
		{"task.agent_attention_required", "agent_run", "agents.attention", true},
		{"agent_run.started", "agent_run", "agents.activity", false},
		{"automation_rule.activity", "automation_rule", "automation.activity", false},
		{"external_mcp.reauthorization_required", "external_mcp_server", "integrations.connection", true},
		{"new.event", "task", "", false},
	} {
		event := model.NotificationEventInput{EventType: tc.kind, EntityType: tc.entity}
		pref := model.SlackNotificationPreferences{Linked: true, Assignments: true, Mentions: true}
		if key := slackPreferenceKey(event); key != tc.row || pref.CategoryEnabled(key) != tc.direct {
			t.Errorf("%s: row %q, enabled %t", tc.kind, key, pref.CategoryEnabled(key))
		}
	}
}

func TestSlackCustomerReplyHonorsSavedChoice(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		pref := model.SlackNotificationPreferences{Linked: true, Overrides: model.JSONB{"support.replies": enabled}}
		if pref.CategoryEnabled("support.replies") != enabled {
			t.Fatalf("did not honor saved Slack customer reply setting %t", enabled)
		}
	}
}

func TestSlackTeamMentionAudience(t *testing.T) {
	event := model.NotificationEventInput{EventType: "comment.mention", EntityType: "task", ExplicitRecipients: []string{"person", "teammate"}, SlackMentionRecipients: []string{"person"}}
	if !slackAudienceIncludes(event, "person") || slackAudienceIncludes(event, "teammate") {
		t.Fatal("team expansion changed the personal DM audience")
	}
}
