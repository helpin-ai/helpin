package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSlackNoteMentionText(t *testing.T) {
	f := newChannelFixture(t)
	s := NewSlackService(f.svc, nil, SlackConfig{})
	calls := map[string]int{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("user")
		calls[id]++
		switch id {
		case "U1":
			fmt.Fprint(w, `{"ok":true,"user":{"id":"U1","profile":{"real_name":"Maya Patel"}}}`)
		default:
			fmt.Fprint(w, `{"ok":false,"error":"user_not_found"}`)
		}
	}))
	defer api.Close()
	s.api.BaseURL = api.URL + "/"
	got := s.feedNoteText(context.Background(), "test", "<@U1> <@U1> <@W2|Jo> <@U3> `<@U4>`\n```\n<@U5>\n``` &lt;@U6&gt;")
	want := "@Maya Patel @Maya Patel @Jo @Slack member `<@U4>`\n```\n<@U5>\n``` &lt;@U6&gt;"
	if got != want || len(calls) != 3 || calls["U1"] != 1 {
		t.Fatalf("text=%q, lookups=%v", got, calls)
	}
}

func TestSlackWaitingNotesExpireAndReleaseBySender(t *testing.T) {
	ctx := context.Background()
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	f.tg.mu.Lock()
	f.tg.botID++
	f.tg.mu.Unlock()
	other := f.connect(t, f.other)
	repo := repository.NewSlackRepository(f.db)
	mustExec(t, f.db, "CREATE TABLE slack_members(account_id text,slack_user text,user_id text,PRIMARY KEY(account_id,slack_user))")
	mustExec(t, f.db, "CREATE TABLE slack_link_grants(token_hash text PRIMARY KEY,account_id text,slack_user text,expires_at datetime)")
	for _, tc := range []struct{ id, account, user string }{
		{"original", account.ID, "U1"}, {"other-member", account.ID, "U2"}, {"other-install", other.ID, "U1"}, {"expired", account.ID, "U1"},
	} {
		subject := model.SupportChannelSubjectKey(tc.account, tc.user)
		job := model.SupportInboundJob{ID: tc.id, Kind: SlackEventJob, ChannelAccountID: &tc.account, SubjectKey: &subject, Payload: "note", Status: model.InboundAwaitingSlackLink}
		if err := repo.GrantLink(ctx, tc.account, tc.user, tc.id, &job); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	mustExec(t, f.db, "UPDATE support_inbound_jobs SET available_at=? WHERE id='expired'", now.Add(-time.Minute))
	if err := repo.Link(ctx, account.ID, "member", "original"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"expired", "original", "other-member", "other-install"} {
		var job model.SupportInboundJob
		if err := f.db.First(&job, "id=?", id).Error; err != nil {
			t.Fatal(err)
		}
		want := model.InboundAwaitingSlackLink
		if id == "original" {
			want = "pending"
		}
		if job.Status != want {
			t.Fatalf("%s: %s, want %s", id, job.Status, want)
		}
	}
	jobs := repository.NewSupportInboundJobRepository(f.db)
	if claimed, err := jobs.Claim(ctx, SlackEventJob, now.Add(time.Second)); err != nil || claimed != nil {
		t.Fatalf("expired note claimed: %v, %v", claimed, err)
	}
	var expired model.SupportInboundJob
	if err := f.db.First(&expired, "id=?", "expired").Error; err != nil || expired.Payload != "" || expired.Status != "completed" {
		t.Fatalf("expired payload retained: %+v, %v", expired, err)
	}
	claimed, err := jobs.Claim(ctx, SlackEventJob, now.Add(time.Second))
	if err != nil || claimed == nil || claimed.ID != "original" {
		t.Fatalf("original not released: %+v, %v", claimed, err)
	}
}
