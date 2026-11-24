//go:build integration

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

func TestSlackSupportThreadIsolationPostgres(t *testing.T) {
	ctx := context.Background()
	db := openSupportOpsPostgres(t)
	ws, _ := seedSupportOpsWorkspace(t, db)
	a := seedChannelAccount(t, db, ws)
	mustExec(t, db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", a.ID)
	a.Provider = "slack"
	inbox := newChannelInbox(db)
	receive := func(sender, ts, root string) *channelReceipt {
		t.Helper()
		e := channels.Event{Kind: channels.EventMessage, ExternalUserID: sender, ExternalChatID: "C1", ExternalMessageKey: "C1:" + ts, ThreadKey: "C1:" + root, ChannelName: "customer-support", DisplayName: sender, Text: "checkout issue"}
		receipt, err := inbox.receiveChannelMessage(ctx, a, e, 1000)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	first := receive("U1", "1", "1")
	second := receive("U1", "2", "2")
	third := receive("U2", "3", "1")
	if first.message.ConversationID == second.message.ConversationID || third.message.ConversationID != first.message.ConversationID || *third.message.SenderDisplayName != "U2" {
		t.Fatal("thread identity/attribution lost")
	}
	if !receive("U2", "3", "1").duplicate {
		t.Fatal("duplicate event was accepted")
	}
	conv, err := repository.NewSlackRepository(db).SupportThread(ctx, a, "C1:1")
	if err != nil || conv == nil || *conv.ProviderChannelName != "customer-support" {
		t.Fatalf("missing Slack destination: %v", err)
	}
	msg := model.SupportMessage{WorkspaceID: ws, ConversationID: conv.ID, SenderType: "user", MessageType: "note", IsInternal: true, Content: "internal only"}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&model.SupportChannelOutbox{}).Where("message_id=?", msg.ID).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("internal note created a delivery: %d %v", n, err)
	}
}

// Exercise the regular OAuth, signed intake, job, reply, and outbox paths against a fake provider.
func TestSlackTwoWaySupportPostgres(t *testing.T) {
	ctx := context.Background()
	f := newOutboxFixture(t)
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{ClientID: "client", ClientSecret: "secret", SigningSecret: "signing", RedirectURL: "https://helpin.example.com/oauth/slack/callback"})
	team := "T" + f.workspace
	var sent []map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			if r.URL.Query().Get("channel") != "C2" {
				t.Error("settings used the old Slack channel ID")
			}
			io.WriteString(w, `{"ok":true,"channel":{"id":"C2","name":"acme","is_member":true}}`)
		case "/oauth.v2.access":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "access_token": "xoxb-fixture", "bot_user_id": "Ubot", "team": map[string]string{"id": team, "name": "Acme"}})
		case "/users.info":
			io.WriteString(w, `{"ok":true,"user":{"team_id":"external","name":"Lena"}}`)
		case "/chat.postMessage":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if r.Header.Get("Authorization") != "Bearer xoxb-fixture" {
				t.Error("wrong bot credential")
			}
			sent = append(sent, body)
			io.WriteString(w, `{"ok":true,"ts":"10.3"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	s.api.BaseURL = fake.URL + "/"
	start, err := s.StartInstall(ctx, f.workspace, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(start)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if err = s.CompleteInstall(ctx, f.workspace, f.userID, state, "code"); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteInstall(ctx, f.workspace, f.userID, state, "code"); err == nil {
		t.Fatal("reused OAuth state")
	}
	account, err := s.repo.Installation(ctx, f.workspace)
	if err != nil || account == nil || account.TokenEncrypted == "xoxb-fixture" {
		t.Fatalf("invalid installation: %v", err)
	}
	if err = s.repo.SetRoute(ctx, model.SlackRoute{AccountID: account.ID, ChannelID: "C1", ChannelName: "acme", Purpose: "support"}); err != nil {
		t.Fatal(err)
	}
	event := fmt.Sprintf(`{"type":"event_callback","team_id":%q,"event_id":"Ev1","event":{"type":"message","channel":"C1","ts":"10.1","user":"Ucustomer","text":"Checkout failed"}}`, team)
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	h := hmac.New(sha256.New, []byte("signing"))
	fmt.Fprintf(h, "v0:%s:%s", stamp, event)
	for i := 0; i < 2; i++ {
		if _, err = s.AcceptEvent(ctx, stamp, "v0="+hex.EncodeToString(h.Sum(nil)), []byte(event)); err != nil {
			t.Fatal(err)
		}
	}
	processed, err := f.svc.processNextJob(ctx, SlackEventJob)
	if err != nil || !processed {
		t.Fatalf("worker: %v %v", processed, err)
	}
	conv, err := s.repo.SupportThread(ctx, account, "C1:10.1")
	if err != nil || conv == nil {
		t.Fatalf("missing inbound thread: %v", err)
	}
	reply := f.reply(t, conv.ID, "user", "Checkout works again.")
	accept := func(body string) {
		t.Helper()
		mac := hmac.New(sha256.New, []byte("signing"))
		fmt.Fprintf(mac, "v0:%s:%s", stamp, body)
		if _, err := s.AcceptEvent(ctx, stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	change := fmt.Sprintf(`{"type":"event_callback","team_id":%q,"event_id":"Ev2","event":{"type":"channel_id_changed","old_channel_id":"C1","new_channel_id":"C2"}}`, team)
	accept(change)
	accept(change)
	s.authz = authorization.NewAuthzService(f.db, slackFixtureMember{f.userID}, nil)
	owner := authorization.WithActor(ctx, &authorization.Actor{Role: "owner", UserID: f.userID, WorkspaceID: f.workspace})
	if err := s.Configure(owner, f.workspace, model.SlackRoute{ChannelID: "C1", Purpose: "support"}); err != nil {
		t.Fatalf("save settings opened before the channel ID change: %v", err)
	}
	routes, err := s.repo.Routes(ctx, account.ID)
	if err != nil || len(routes) != 1 || routes[0].ChannelID != "C1" || routes[0].CurrentChannelID != "C2" {
		t.Fatalf("settings created a new route for the same channel: %+v %v", routes, err)
	}
	accept(fmt.Sprintf(`{"type":"event_callback","team_id":%q,"event_id":"Ev3","event":{"type":"message","channel":"C2","ts":"10.2","thread_ts":"10.1","user":"Ucustomer","text":"Still here"}}`, team))
	if ok, err := f.svc.processNextJob(ctx, SlackEventJob); err != nil || !ok {
		t.Fatalf("reply after channel ID change: %v %v", ok, err)
	}
	var count int64
	if err := f.db.Model(&model.SupportMessage{}).Where("conversation_id=? AND content=?", conv.ID, "Still here").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("lost conversation after channel change: %d / %v", count, err)
	}
	f.due(t)
	if _, err = f.svc.ProcessOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	row := f.row(t, reply.ID)
	if row == nil || row.DispatchStatus != "accepted" || len(sent) != 1 || sent[0]["thread_ts"] != "10.1" || sent[0]["channel"] != "C2" || sent[0]["text"] != "Checkout works again." {
		t.Fatalf("unexpected delivery: %+v / %+v", row, sent)
	}
	if _, err = f.svc.ProcessOutbox(ctx); err != nil || len(sent) != 1 {
		t.Fatal("reply was redelivered", err)
	}
	published, err := s.repo.PublishedReply(ctx, account, conv.ID)
	if err != nil || published.ID != reply.ID {
		t.Fatalf("published update unavailable: %v", err)
	}
	if err = f.svc.accounts.Disconnect(ctx, f.workspace, account.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	start, err = s.StartInstall(ctx, f.workspace, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	u, err = url.Parse(start)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteInstall(ctx, f.workspace, f.userID, u.Query().Get("state"), "fresh-code"); err != nil {
		t.Fatal(err)
	}
	next, err := s.repo.Installation(ctx, f.workspace)
	if err != nil || next == nil || next.ID != account.ID {
		t.Fatalf("same-workspace reconnect lost history: %v", err)
	}
	active, err := s.repo.Routes(ctx, account.ID)
	if err != nil || active == nil || len(active) != 0 {
		t.Fatalf("restored routing without opt-in: %v", err)
	}

}

func TestSlackConnectAttachmentPostgres(t *testing.T) {
	ctx := context.Background()
	f := newOutboxFixture(t)
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", f.account.ID)
	f.account.Provider = "slack"
	const fileData = "Checkout failed at payment"
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil || r.Method != http.MethodPut || string(data) != fileData {
			t.Errorf("unexpected object upload: method=%s bytes=%q err=%v", r.Method, data, err)
		}
	}))
	defer store.Close()
	attachments := repository.NewSupportAttachmentRepository(f.db)
	f.inbox.SetAttachmentService(NewSupportAttachmentService(attachments, storage.NewS3Client("key", "secret", "bucket", "us-east-1", store.URL, "")))
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{})
	s.api.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/api/files.info":
			if r.URL.Query().Get("file") != "F1" {
				t.Fatal("wrong Slack file")
			}
			body = `{"ok":true,"file":{"id":"F1","name":"checkout.txt","mimetype":"text/plain","url_private_download":"https://files.slack.com/private"}}`
		case "/private":
			body = fileData
		default:
			t.Fatalf("unexpected Slack request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	event := channels.Event{Kind: channels.EventMessage, ExternalUserID: "U1", ExternalChatID: "C1", ExternalMessageKey: "C1:1", ThreadKey: "C1:1", Files: []channels.InboundFile{{FileID: "F1"}}}
	receipt, err := f.inbox.receiveChannelMessage(ctx, f.account, event, 25<<20)
	if err != nil {
		t.Fatal(err)
	}
	var job model.SupportInboundJob
	if err := f.db.Where("message_id=? AND kind=?", receipt.message.ID, SupportInboundKindChannelAttachment).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.svc.processAttachmentJob(ctx, &job); err != nil {
		t.Fatal(err)
	}
	messages := []model.SupportMessage{*receipt.message}
	if err := f.inbox.attachmentService.HydrateMessages(ctx, messages); err != nil {
		t.Fatal(err)
	}
	files := messages[0].Attachments
	if len(files) != 1 || files[0].FileName != "checkout.txt" || files[0].FileType != "text/plain" || files[0].FileSize != int64(len(fileData)) || files[0].ProcessingStatus != "" || files[0].FileKey == "" {
		t.Fatalf("inbox file after Slack Connect download: %+v", files)
	}
}

func TestSlackMergedThreadAndTaskHandoffPostgres(t *testing.T) {
	ctx := context.Background()
	f := newOutboxFixture(t)
	a := f.account
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", a.ID)
	a.Provider = "slack"
	receive := func(ts string) string {
		t.Helper()
		r, err := f.inbox.receiveChannelMessage(ctx, a, channels.Event{Kind: channels.EventMessage, ExternalUserID: "Uone", ExternalChatID: "C1", ExternalMessageKey: "C1:" + ts, ThreadKey: "C1:" + ts, ChannelName: "acme", Text: "Help"}, 1000)
		if err != nil {
			t.Fatal(err)
		}
		return r.message.ConversationID
	}
	root, child := receive("1"), receive("2")
	mustExec(t, f.db, "UPDATE support_conversations SET merged_into_id=? WHERE id=?", root, child)
	reply := func(origin string) *model.SupportChannelOutbox {
		t.Helper()
		msg := model.SupportMessage{WorkspaceID: f.workspace, ConversationID: root, MessageType: "reply", SenderType: "user", Content: "Public reply", Metadata: "{}"}
		if origin != "" {
			msg.Metadata = fmt.Sprintf(`{"reply_transport":"slack","delivery_mode":"chat_and_email","provider_origin_id":%q}`, origin)
		}
		if err := f.db.Create(&msg).Error; err != nil {
			t.Fatal(err)
		}
		return f.row(t, msg.ID)
	}
	if row := reply(""); row == nil || row.DispatchStatus != "cancelled" {
		t.Fatalf("ambiguous thread sent: %+v", row)
	}
	row := reply(child)
	if row == nil || row.DispatchStatus != "pending" || derefString(row.ProviderOriginID) != child {
		t.Fatalf("incorrect pinned thread: %+v", row)
	}
	repo := repository.NewSlackRepository(f.db)
	mustExec(t, f.db, "UPDATE support_channel_outbox SET dispatch_status='accepted' WHERE id=?", row.ID)
	public, err := repo.PublishedReply(ctx, a, root)
	if err != nil || public.ID != "" {
		t.Fatalf("another thread's reply was exposed: %+v %v", public, err)
	}
	w := model.PMWorkflow{WorkspaceID: f.workspace, Name: "Engineering"}
	if err = f.db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	state := model.PMWorkflowState{WorkflowID: w.ID, StateType: "unstarted", Name: "Open"}
	if err = f.db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	task := model.PMTask{WorkspaceID: f.workspace, DisplayID: 142, Name: "Fix checkout", WorkflowID: w.ID, WorkflowStateID: state.ID}
	if err = f.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err = repo.SetRoute(ctx, model.SlackRoute{AccountID: a.ID, ChannelID: "C2", ChannelName: "payments", Purpose: "pm"}); err != nil {
		t.Fatal(err)
	}
	if err = repo.LinkTask(ctx, a.ID, "C2", "1", task.ID, f.userID); err != nil {
		t.Fatal(err)
	}
	d, err := repo.Discussion(ctx, a.ID, "C2", "1")
	if err != nil || d.CommentID == "" {
		t.Fatalf("missing linked discussion: %+v %v", d, err)
	}
	if err = repo.ChangeChannelID(ctx, a.ID, "C2", "Cshared"); err != nil {
		t.Fatal(err)
	}
	for _, channel := range []string{"C2", "Cshared"} {
		route, err := repo.Route(ctx, a.ID, channel)
		if err != nil || route == nil || route.ChannelID != "C2" || route.CurrentChannelID != "Cshared" {
			t.Fatalf("lost project channel route for %s: %+v %v", channel, route, err)
		}
		linked, err := repo.Discussion(ctx, a.ID, route.ChannelID, "1")
		if err != nil || linked.TaskID != task.ID || linked.CommentID != d.CommentID {
			t.Fatalf("lost task discussion after channel ID change: %+v %v", linked, err)
		}
	}
	for _, item := range []struct{ parent, key *string }{{nil, nil}, {&d.CommentID, strPtr("from-slack")}, {&d.CommentID, nil}} {
		comment := model.PMComment{AuthorID: f.userID, EntityType: "task", EntityID: task.ID, Body: "Test reply", ParentID: item.parent, SlackSourceKey: item.key}
		if err = f.db.Create(&comment).Error; err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, f.db, "UPDATE support_conversations SET assigned_user_id=?,linked_task_id=? WHERE id=?", f.userID, task.ID, root)
	mustExec(t, f.db, "UPDATE pm_tasks SET completed=true WHERE id=?", task.ID)
	var counts struct{ Comments, Alerts int }
	err = f.db.Raw(`SELECT count(*) FILTER (WHERE jsonb_exists(payload::jsonb,'comment')) AS comments,count(*) FILTER (WHERE conversation_id IS NOT NULL) AS alerts FROM support_inbound_jobs WHERE workspace_id=? AND kind='slack_pm_delivery'`, f.workspace).Scan(&counts).Error
	if err != nil || counts.Comments != 1 || counts.Alerts != 1 {
		t.Fatalf("expected only the explicit reply and one handoff: %+v %v", counts, err)
	}
}

type slackFixtureMember struct{ userID string }

func (f slackFixtureMember) GetMembership(_ context.Context, workspace, user string) (*authorization.MemberInfo, error) {
	if user != f.userID {
		return nil, nil
	}
	return &authorization.MemberInfo{ID: user, Status: "active", Role: "owner"}, nil
}
func (slackFixtureMember) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

func TestSlackPrivateAudiencePostgres(t *testing.T) {
	ctx := context.Background()
	f := newOutboxFixture(t)
	a := f.account
	repo := repository.NewSlackRepository(f.db)
	s := NewSlackService(f.svc, repo, SlackConfig{})
	s.authz = authorization.NewAuthzService(f.db, slackFixtureMember{f.userID}, nil)
	mustExec(t, f.db, "INSERT INTO slack_members(account_id,slack_user,user_id) VALUES (?,'Uagent',?)", a.ID, f.userID)
	for _, tc := range []struct {
		name                                           string
		private, external, allLinked, authorized, want bool
	}{
		{"private authorized channel", true, false, true, true, true},
		{"public history", false, false, true, true, false},
		{"Slack Connect", true, true, true, true, false},
		{"unlinked member", true, false, false, true, false},
		{"task access removed", true, false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/conversations.info" {
					json.NewEncoder(w).Encode(map[string]any{"ok": true, "channel": map[string]bool{"is_member": true, "is_private": tc.private, "is_ext_shared": tc.external}})
				} else {
					members := []string{a.Handle, "Uagent"}
					if !tc.allLinked {
						members = append(members, "Uother")
					}
					json.NewEncoder(w).Encode(map[string]any{"ok": true, "members": members})
				}
			}))
			defer fake.Close()
			s.api.BaseURL = fake.URL + "/"
			ok, err := s.channelAudience(ctx, a, "token", "C1", func(actor *authorization.Actor) (bool, error) { return tc.authorized && actor.UserID == f.userID, nil })
			if err != nil || ok != tc.want {
				t.Fatalf("sharing decision %v, want %v: %v", ok, tc.want, err)
			}
		})
	}
}

func TestSlackTaskReviewOptionsPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ws, user := seedSupportOpsWorkspace(t, db)
	team := model.WorkspaceTeam{WorkspaceID: ws, Name: "Payments", DefaultTaskType: "bug"}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	w := model.PMWorkflow{WorkspaceID: ws, TeamID: &team.ID, Name: "Project"}
	if err := db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	state := model.PMWorkflowState{WorkflowID: w.ID, Name: "In review", StateType: "started"}
	if err := db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	label := model.PMLabel{WorkspaceID: ws, TeamID: &team.ID, Name: "Checkout"}
	if err := db.Create(&label).Error; err != nil {
		t.Fatal(err)
	}
	actor := &authorization.Actor{WorkspaceID: ws, UserID: user, Role: "owner"}
	ctx := authorization.WithActor(context.Background(), actor)
	s := &SlackService{repo: repository.NewSlackRepository(db), api: slack.NewClient(), tasks: &PMTaskService{workspaceRepo: repository.NewWorkspaceRepository(db), workflowRepo: repository.NewPMWorkflowRepository(db), sprintRepo: repository.NewPMSprintRepository(db), labelRepo: repository.NewPMLabelRepository(db), templateRepo: repository.NewPMTaskTemplateRepository(db)}}
	var rendered struct {
		View struct {
			Blocks []struct {
				ID       string `json:"block_id"`
				Type     string
				Elements []struct{ Text string }
				Element  struct {
					Type    string
					Options []struct{ Value string }
				}
			}
		}
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/views.update" {
			t.Error("wrong method", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&rendered); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer fake.Close()
	s.api.BaseURL = fake.URL + "/"
	var action SlackAction
	payload := fmt.Sprintf(`{"actions":[{"action_id":"draft_more"}],"view":{"id":"V1","hash":"H1","state":{"values":{"team":{"value":{"selected_option":{"value":%q}}}}}}}`, team.ID)
	if err := json.Unmarshal([]byte(payload), &action); err != nil {
		t.Fatal(err)
	}
	const notice = "Draft uses part of this thread. Review it for missing details."
	if err := s.openTask(ctx, "token", action, slackDraft{Title: "Checkout fails", TeamID: &team.ID, HistoryNotice: notice}, actor, "draft-id"); err != nil {
		t.Fatal(err)
	}
	var foundLabel, foundState, foundNotice bool
	for _, block := range rendered.View.Blocks {
		if block.Type == "context" && len(block.Elements) == 1 && block.Elements[0].Text == notice {
			foundNotice = true
		}
		for _, opt := range block.Element.Options {
			if block.ID == "labels_"+team.ID && block.Element.Type == "multi_static_select" && opt.Value == label.ID {
				foundLabel = true
			}
			if block.ID == "state_"+team.ID && opt.Value == state.ID {
				foundState = true
			}
		}
	}
	if !foundLabel || !foundState || !foundNotice {
		t.Fatalf("missing draft review content: labels=%v status=%v history notice=%v", foundLabel, foundState, foundNotice)
	}
}

func TestSlackMessageSourcePostgres(t *testing.T) {
	f := newOutboxFixture(t)
	ctx := context.Background()
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", f.account.ID)
	f.account.Provider = "slack"
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{})
	const permalink = "https://acme.slack.com/archives/Cshared/p1700000010000012?thread_ts=1700000000.000001&cid=Cshared"
	calls := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/chat.getPermalink" || r.URL.Query().Get("channel") != "Cshared" || r.URL.Query().Get("message_ts") != "1700000010.000012" || r.Header.Get("Authorization") != "Bearer 424242:token" {
			t.Error("permalink request lost the original message coordinates or installation")
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "permalink": permalink})
	}))
	defer fake.Close()
	s.api.BaseURL = fake.URL + "/"
	event := channels.Event{Kind: channels.EventMessage, ExternalUserID: "Ulena", ExternalChatID: "C1",
		ExternalMessageKey: "C1:1700000000.000001", ThreadKey: "C1:1700000000.000001", ChannelName: "acme", Text: "First example"}
	first, err := f.inbox.receiveChannelMessage(ctx, f.account, event, 1000)
	if err != nil {
		t.Fatal(err)
	}
	origin := first.message.ConversationID
	outbound := f.reply(t, origin, "user", "Customer update")
	_, err = repository.NewSupportMergeRepository(f.db).Merge(ctx, f.workspace, f.conv, []string{origin}, f.userID, uuid.NewString(), true)
	if err != nil {
		t.Fatal(err)
	}
	event.ExternalMessageKey, event.Text = "C1:1700000010.000012", "A second example"
	received, err := f.inbox.receiveChannelMessage(ctx, f.account, event, 1000)
	if err != nil {
		t.Fatal(err)
	}
	msg := received.message
	if msg.ConversationID != f.conv || derefString(msg.OriginConversationID) != origin {
		t.Fatal("reply was not routed through the merged conversation")
	}
	actor := &authorization.Actor{WorkspaceID: f.workspace, WorkspaceMemberID: f.userID, UserID: f.userID, Role: "member"}
	member := authorization.WithActor(ctx, actor)
	check := func(c context.Context, ws, id, want string) {
		t.Helper()
		actual, err := s.MessageSource(c, ws, id)
		if err != nil || actual != want {
			t.Fatalf("got %q, want %q: %v", actual, want, err)
		}
	}
	if err := s.repo.ChangeChannelID(ctx, f.account.ID, "C1", "Cshared"); err != nil {
		t.Fatal(err)
	}
	check(member, f.workspace, msg.ID, permalink)
	check(ctx, f.workspace, msg.ID, "")
	check(member, f.userID, msg.ID, "")
	check(member, f.workspace, outbound.ID, "")
	private := model.SupportMailbox{WorkspaceID: f.workspace, Name: "Private", Handle: "private", CreatedByID: f.userID, Active: true}
	if err := f.db.Create(&private).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, "UPDATE support_conversations SET mailbox_id=? WHERE id=?", private.ID, f.conv)
	check(member, f.workspace, msg.ID, "")
	mustExec(t, f.db, "UPDATE support_conversations SET mailbox_id=NULL WHERE id=?", f.conv)
	mustExec(t, f.db, "UPDATE support_conversations SET anonymized_at=now() WHERE id=?", origin)
	check(member, f.workspace, msg.ID, "")
	if calls != 1 {
		t.Fatalf("looked up inaccessible sources: %d provider calls", calls)
	}
}
