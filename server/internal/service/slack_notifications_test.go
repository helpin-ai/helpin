package service

import (
	"context"
	"encoding/json"
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
	mustExecNotificationService(t, db, "CREATE TABLE slack_members (account_id TEXT, user_id TEXT, slack_user TEXT, notify_assignments BOOLEAN DEFAULT false, notify_mentions BOOLEAN DEFAULT false, PRIMARY KEY(account_id,user_id))")
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
	api := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("unexpected method %s", r.URL.Path)
		}
		var message map[string]any
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Error(err)
		}
		sent = append(sent, message)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true,"ts":"1.2"}`))}, nil
	})}
	s := &SlackService{repo: repo, notifications: n, channels: &SupportChannelService{cfg: SupportChannelConfig{EncryptionKey: key}}, api: &slack.Client{BaseURL: "https://slack.example/", HTTP: api}}
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
	if sent[0]["mrkdwn"] != false || !strings.Contains(text, "Alex updated <@U2>") || !strings.Contains(text, "https://helpin.example/w/my-workspace/pm/objectives/item") {
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
	mustExecNotificationService(t, db, "UPDATE support_channel_accounts SET disabled_at=? WHERE id='install'", now)
	emit("objective.mention")
	countJobs(2)
	if err := s.SetNotificationPreferences(ctx, "ws", "member", model.UpdateSlackNotificationPreferences{Mentions: &off}); err == nil {
		t.Fatal("updated disconnected installation")
	}
}
