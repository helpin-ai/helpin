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

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSlackInboxCreatePostgres(t *testing.T) {
	ctx := context.Background()
	f := newOutboxFixture(t)
	a := f.account
	ws := repository.NewWorkspaceRepository(f.db)
	f.inbox.workspaceRepo = ws
	f.inbox.assocRepo = repository.NewCRMAssociationRepository(f.db)
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack',ai_replies_disabled=true WHERE id=?", a.ID)
	mustExec(t, f.db, "INSERT INTO slack_members(account_id,slack_user,user_id) VALUES (?,'Uagent',?)", a.ID, f.userID)
	seedWorkspaceMember(t, f.db, f.userID, f.workspace, f.userID, f.userID+"@example.com", "Agent", model.RoleOwner)
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{SigningSecret: "test-signing-key"})
	tasks := NewPMTaskService(repository.NewPMTaskRepository(f.db), ws, repository.NewPMWorkflowRepository(f.db), nil, nil, repository.NewPMLabelRepository(f.db), nil, nil, nil, NewPMActivityService(repository.NewPMActivityRepository(f.db)), nil, nil, nil, nil)
	f.inbox.SetTaskService(tasks)
	s.SetPM(tasks, nil, authorization.NewAuthzService(f.db, slackFixtureMember{f.userID}, nil), nil, nil, "https://helpin.example")
	route := model.SlackRoute{AccountID: a.ID, ChannelID: "Cdesk", ChannelName: "desk", Purpose: "inbox", MailboxIDs: []string{"shared"}}
	if err := s.repo.SetRoute(ctx, route, false); err != nil {
		t.Fatal(err)
	}
	var view, prompt, confirmation map[string]any
	private, external := false, false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			fmt.Fprintf(w, `{"ok":true,"channel":{"id":"Cdesk","is_member":true,"is_private":%t,"is_ext_shared":%t}}`, private, external)
		case "/conversations.members":
			io.WriteString(w, `{"ok":true,"members":["Uagent"]}`)
		case "/chat.getPermalink":
			fmt.Fprintf(w, `{"ok":true,"permalink":"https://example.slack.com/archives/Cdesk/p%s"}`, strings.ReplaceAll(r.URL.Query().Get("message_ts"), ".", ""))
		case "/views.open":
			io.WriteString(w, `{"ok":true,"view":{"id":"Vform"}}`)
		case "/views.update", "/chat.postEphemeral", "/chat.postMessage":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			switch r.URL.Path {
			case "/views.update":
				view = payload["view"].(map[string]any)
			case "/chat.postMessage":
				confirmation = payload
			default:
				prompt = payload
			}
			io.WriteString(w, `{"ok":true}`)
		case "/agents.sessions.setStatus", "/users.info":
			io.WriteString(w, `{"ok":true}`)
		default:
			t.Errorf("unexpected API method %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	s.api.BaseURL = fake.URL + "/"
	event := func(text, ts, root string) {
		t.Helper()
		b, err := json.Marshal(slack.Message{Type: "message", Channel: "Cdesk", User: "Uagent", Text: text, TS: ts, ThreadTS: root})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.process(ctx, &model.SupportInboundJob{ChannelAccountID: &a.ID, Payload: string(b)}); err != nil {
			t.Fatal(err)
		}
	}
	click := func(payload map[string]any, user string) any {
		t.Helper()
		payload["team"] = map[string]string{"id": a.ExternalAccountID}
		payload["user"] = map[string]string{"id": user}
		payload["trigger_id"] = uuid.NewString()
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body := []byte(url.Values{"payload": {string(data)}}.Encode())
		stamp := strconv.FormatInt(s.channels.now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(s.cfg.SigningSecret))
		fmt.Fprintf(mac, "v0:%s:", stamp)
		mac.Write(body)
		result, err := s.Action(ctx, stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), body)
		if err != nil {
			t.Fatal(err)
		}
		if payload["type"] == "view_submission" && payload["view"].(map[string]any)["callback_id"] == "helpin_task" {
			return result
		}
		var job model.SupportInboundJob
		if err = f.db.Where("kind='slack_shortcut' AND status='pending'").First(&job).Error; err != nil {
			t.Fatal(err)
		}
		if err = s.shortcutJob(ctx, &job); err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.db, "UPDATE support_inbound_jobs SET status='completed' WHERE id=?", job.ID)
		return result
	}
	action := func(key, id, user string) {
		t.Helper()
		click(map[string]any{"type": "block_actions", "channel": map[string]string{"id": "Cdesk"}, "actions": []any{map[string]string{"action_id": key, "value": id}}}, user)
	}
	shortcut := func(ts, root, body string) {
		t.Helper()
		click(map[string]any{"type": "message_action", "channel": map[string]string{"id": "Cdesk"}, "callback_id": "create_support", "message": map[string]string{"user": "Uauthor", "ts": ts, "thread_ts": root, "text": body}}, "Uagent")
	}
	field := func(text string) any { return map[string]any{"value": map[string]string{"value": text}} }
	selected := func(value string) any {
		return map[string]any{"value": map[string]any{"selected_option": map[string]string{"value": value}}}
	}
	submit := func(id, form, inbox string) {
		t.Helper()
		click(map[string]any{"type": "view_submission", "view": map[string]any{"id": form, "callback_id": "inbox_ticket_save", "private_metadata": id, "state": map[string]any{"values": map[string]any{"subject": field("Export timed out"), "description": field("Internal follow-up"), "inbox": selected(inbox)}}}}, "Uagent")
	}
	shortcut("100.1", "", "Export fails\nOn CSV downloads")
	if view["callback_id"] != "inbox_ticket_save" || !strings.Contains(fmt.Sprint(view["blocks"]), "initial_option") || !strings.Contains(fmt.Sprint(view["blocks"]), "On CSV downloads") {
		t.Fatalf("ticket review: %v", view)
	}
	id := view["private_metadata"].(string)
	action("inbox_ticket_review", id, "Uother")
	if strings.Contains(fmt.Sprint(view), "CSV downloads") || view["submit"] != nil {
		t.Fatal("another user opened the draft")
	}
	external = true
	submit(id, "Vexternal", "shared")
	if confirmation != nil {
		t.Fatal("ticket created after channel became external")
	}
	external = false
	submit(id, "Vwrong", uuid.NewString())
	if confirmation != nil {
		t.Fatal("accepted an unfollowed inbox")
	}
	submit(id, "Vsave", "shared")
	convID := inboundStableID("slack-staff-ticket:" + a.ID + ":Cdesk:100.1")
	if confirmation["thread_ts"] != "100.1" || !strings.Contains(fmt.Sprint(confirmation["blocks"]), "/support/"+convID) {
		t.Fatalf("missing linked confirmation: %v", confirmation)
	}
	confirmation = nil
	submit(id, "Vreplay", "shared")
	if confirmation != nil {
		t.Fatal("duplicate confirmation on replay")
	}
	var notes []model.SupportMessage
	if err := f.db.Where("conversation_id=?", convID).Find(&notes).Error; err != nil || len(notes) != 1 || !notes[0].IsInternal || notes[0].SenderType != "user" || notes[0].Content != "Internal follow-up" || !strings.Contains(notes[0].Metadata, "/p1001") || f.row(t, notes[0].ID) != nil {
		t.Fatalf("ticket note/replay/outbound: %v %v", notes, err)
	}
	mention := "<@" + a.Handle + "> "
	event("ordinary top-level chat", "200.1", "")
	if prompt != nil {
		t.Fatal("ordinary chat got an action")
	}
	event(mention+"create a ticket for nightly exports", "201.1", "")
	if !strings.Contains(fmt.Sprint(prompt["blocks"]), "Review support ticket") {
		t.Fatalf("support mention: %v", prompt)
	}
	mustExec(t, f.db, "UPDATE slack_feed_threads SET root_ts='300.1' WHERE account_id=? AND channel_id='Cdesk' AND conversation_id=?", a.ID, convID)
	event(mention+"create a ticket", "301.1", "300.1")
	if !strings.Contains(fmt.Sprint(prompt["text"]), "Export timed out") || !strings.Contains(fmt.Sprint(prompt["blocks"]), "Open a separate support ticket") || !strings.Contains(fmt.Sprint(prompt["blocks"]), "Create linked task") {
		t.Fatalf("ambiguous mention: %v", prompt)
	}
	choiceID := prompt["blocks"].([]any)[1].(map[string]any)["elements"].([]any)[0].(map[string]any)["value"].(string)
	event("I will investigate", "302.1", "300.1")
	var count int64
	if err := f.db.Model(&model.SupportMessage{}).Where("conversation_id=? AND is_internal", convID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("ordinary reply/command leaked as note: %d, %v", count, err)
	}
	team := uuid.NewString()
	seedTaskTeam(t, taskTestEnv{db: f.db, wsID: f.workspace}, team, "Engineering")
	action("inbox_linked_task", choiceID, "Uagent")
	if view["callback_id"] != "helpin_task" {
		t.Fatalf("linked task review: %v", view)
	}
	event(mention+"create a bug for engineering", "303.1", "300.1")
	draftID := prompt["blocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)["value"].(string)
	raw, err := s.repo.Draft(ctx, draftID, a.ID, f.userID)
	var draft slackDraft
	if err != nil || json.Unmarshal(raw, &draft) != nil || !draft.Inbox || draft.ConversationID != convID || draft.TaskType != "bug" || derefString(draft.TeamID) != team {
		t.Fatalf("linked bug draft: %s %v", raw, err)
	}
	workflow, state := uuid.NewString(), uuid.NewString()
	mustExec(t, f.db, "INSERT INTO pm_workflows(id,workspace_id,team_id,name) VALUES (?,?,?,'Engineering')", workflow, f.workspace, team)
	mustExec(t, f.db, "INSERT INTO pm_workflow_states(id,workflow_id,name,state_type,position,is_default) VALUES (?,?,'To do','unstarted',0,true)", state, workflow)
	mustExec(t, f.db, "UPDATE pm_workflows SET default_state_id=? WHERE id=?", state, workflow)
	private = true
	result := click(map[string]any{"type": "view_submission", "view": map[string]any{"id": "Vtask", "private_metadata": draftID, "callback_id": "helpin_task", "state": map[string]any{"values": map[string]any{
		"title": field("Repair CSV export"), "description": field("CSV downloads time out."), "team": selected(team), "type": selected("bug"), "state_" + team: selected(state),
	}}}}, "Uagent")
	var taskID string
	if err = f.db.Raw("SELECT linked_task_id FROM support_conversations WHERE id=?", convID).Scan(&taskID).Error; err != nil || taskID == "" {
		t.Fatalf("task link: %v %v", result, err)
	}
	var receipt model.SupportInboundJob
	if err = f.db.Where("kind=? AND payload::jsonb->>'task'=? AND jsonb_exists(payload::jsonb,'inbox')", SlackPMDeliveryJob, taskID).First(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.deliverDiscussion(ctx, &receipt); err != nil || !strings.Contains(fmt.Sprint(confirmation), "/pm/tasks/"+taskID) {
		t.Fatalf("task receipt %v %v", confirmation, err)
	}
	confirmation = nil
	private = false
	if err = s.deliverDiscussion(ctx, &receipt); err != nil || confirmation != nil {
		t.Fatalf("public PM disclosure %v %v", confirmation, err)
	}
	var discussionCount int64
	if err = f.db.Table("slack_task_threads").Where("task_id=? AND comment_id IS NOT NULL", taskID).Count(&discussionCount).Error; err != nil || discussionCount != 0 {
		t.Fatal("feed became a PM discussion")
	}
	box := model.SupportMailbox{WorkspaceID: f.workspace, Name: "Billing", Handle: "billing", CreatedByID: f.userID}
	if err = f.db.Create(&box).Error; err != nil {
		t.Fatal(err)
	}
	route.MailboxIDs = []string{"shared", box.ID}
	private = true
	if err = s.repo.SetRoute(ctx, route, false); err != nil {
		t.Fatal(err)
	}
	shortcut("400.1", "", "Another request")
	if strings.Contains(fmt.Sprint(view["blocks"]), "initial_option") || !strings.Contains(fmt.Sprint(view["blocks"]), "Billing") {
		t.Fatalf("multiple eligible inboxes: %v", view)
	}
	// A queued staff request must never become customer intake if its channel is repurposed.
	envelope, err := json.Marshal(map[string]any{"type": "event_callback", "event_id": "Echange", "team_id": a.ExternalAccountID,
		"event": slack.Message{Type: "app_mention", Channel: "Cdesk", User: "Uagent", TS: "500.1", Text: mention + "create a ticket for later"}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := strconv.FormatInt(s.channels.now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(s.cfg.SigningSecret))
	fmt.Fprintf(mac, "v0:%s:", stamp)
	mac.Write(envelope)
	if _, err = s.AcceptEvent(ctx, stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), envelope); err != nil {
		t.Fatal(err)
	}
	var queued model.SupportInboundJob
	if err = f.db.Where("id=?", inboundStableID("slack:"+a.ID+":Cdesk:500.1")).First(&queued).Error; err != nil {
		t.Fatal(err)
	}
	route.Purpose, route.MailboxIDs = "support", nil
	if err = s.repo.SetRoute(ctx, route, false); err != nil {
		t.Fatal(err)
	}
	prompt = nil
	if err = s.process(ctx, &queued); err != nil || prompt != nil {
		t.Fatalf("repurposed staff event: %v %v", prompt, err)
	}
	if conv, err := s.repo.SupportThread(ctx, a, "Cdesk:500.1"); err != nil || conv != nil {
		t.Fatalf("staff message became support intake: %v %v", conv, err)
	}
}
