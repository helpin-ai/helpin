package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type slackTestAsk struct {
	chat, client string
	calls        int
}

func (f *slackTestAsk) SendMessage(_ context.Context, workspace, user, chat string, req model.SendDockChatMessageRequest) (*model.DockChatDetail, error) {
	f.calls++
	f.chat = chat
	f.client = req.ClientMessageID
	if req.Content != "Which tasks are blocked?" || workspace == "" || user != "helpin-member" {
		return nil, fmt.Errorf("unexpected turn")
	}
	return &model.DockChatDetail{AcceptedMessage: &model.AgentRunMessage{ID: "turn"}}, nil
}

func TestSlackInstallAIDefaultAndReconnect(t *testing.T) {
	ctx := context.Background()
	f := newChannelFixture(t)
	if err := f.db.AutoMigrate(&model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, "INSERT INTO ai_workspace_settings(workspace_id,default_profile_id) VALUES (?,?)", f.workspace, "shared-model")
	mustExec(t, f.db, "CREATE TABLE slack_install_states(state_hash text PRIMARY KEY,workspace_id text,user_id text,expires_at datetime)")
	for _, table := range []string{"slack_channel_routes", "slack_task_drafts", "slack_link_grants", "slack_members"} {
		mustExec(t, f.db, "CREATE TABLE "+table+"(account_id text)")
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ok":true,"access_token":"fixture","bot_user_id":"bot","team":{"id":%q,"name":"workspace"}}`, r.FormValue("code"))
	}))
	defer api.Close()
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{ClientID: "client", ClientSecret: "secret", SigningSecret: "signing", RedirectURL: "https://example.test/callback"})
	s.api.BaseURL = api.URL + "/"
	install := func(ws string, wantEnabled bool) *model.SupportChannelAccount {
		t.Helper()
		start, err := s.StartInstall(ctx, ws, "installer")
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(start)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.CompleteInstall(ctx, ws, "installer", u.Query().Get("state"), "T-"+ws); err != nil {
			t.Fatal(err)
		}
		a, err := s.repo.Installation(ctx, ws)
		if err != nil || a == nil || !a.AIRepliesDisabled != wantEnabled {
			t.Fatalf("installation AI enabled should be %v: account found=%v, error=%v", wantEnabled, a != nil, err)
		}
		return a
	}
	for _, ws := range []string{f.workspace, f.other} {
		initial := ws == f.workspace
		a := install(ws, initial)
		if err := s.SetAI(ctx, ws, !initial); err != nil {
			t.Fatal(err)
		}
		install(ws, !initial)
		if err := s.channels.accounts.Disconnect(ctx, ws, a.ID, time.Now()); err != nil {
			t.Fatal(err)
		}
		install(ws, !initial)
	}
}

func TestSlackAgentDMDelivery(t *testing.T) {
	for _, outcome := range []string{"quick", "answer", "failed", "cancelled", "timeout"} {
		t.Run(outcome, func(t *testing.T) { testSlackAgentDMDelivery(t, outcome) })
	}
}
func testSlackAgentDMDelivery(t *testing.T, outcome string) {
	ctx := context.Background()
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	for _, query := range []string{
		"UPDATE support_channel_accounts SET provider='slack',ai_replies_disabled=false",
		"CREATE TABLE agent_runs(id text,workspace_id text,status text,error_message text)",
		"CREATE TABLE slack_members(account_id text,slack_user text,user_id text)",
		"CREATE TABLE dock_chats(id text PRIMARY KEY,workspace_id text,user_id text,title text,visibility text,slack_thread boolean,created_at datetime,updated_at datetime)",
		"CREATE TABLE agent_run_messages(id text,workspace_id text,run_id text,dock_chat_id text,dock_chat_sequence integer,role text,message_type text,content text,tool_invocations text)",
	} {
		if err := f.db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.Exec("INSERT INTO slack_members VALUES (?, 'U1','helpin-member')", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{SigningSecret: "fixture"})
	svc.authz = authorization.NewAuthzService(f.db, dockChatAdminMemberRepo{}, nil)
	agent := &slackTestAsk{}
	svc.dock = agent
	var delivered []map[string]any
	var statuses []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents.sessions.setStatus" && r.URL.Path != "/chat.update" && r.URL.Path != "/chat.postMessage" {
			t.Errorf("unexpected method %s", r.URL.Path)
		}
		var message map[string]any
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Error(err)
		}
		if r.URL.Path == "/agents.sessions.setStatus" {
			statuses = append(statuses, fmt.Sprint(message["status"]))
		} else {
			message["method"] = r.URL.Path
			delivered = append(delivered, message)
		}
		fmt.Fprint(w, `{"ok":true,"ts":"30.2"}`)
	}))
	defer api.Close()
	svc.api.BaseURL = api.URL + "/"
	accept := func(event string) {
		body := fmt.Sprintf(`{"type":"event_callback","event_id":"event","team_id":"424242","event":%s}`, event)
		stamp := fmt.Sprint(time.Now().Unix())
		mac := hmac.New(sha256.New, []byte("fixture"))
		fmt.Fprintf(mac, "v0:%s:%s", stamp, body)
		if _, err := svc.AcceptEvent(ctx, stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	work := func(kind string, want bool) {
		t.Helper()
		ok, err := f.svc.processNextJob(ctx, kind)
		if err != nil || ok != want {
			t.Fatalf("%s processed=%v err=%v", kind, ok, err)
		}
	}
	for range 2 {
		accept(`{"type":"app_home_opened","tab":"messages","channel":"D1","user":"U1"}`)
	}
	work(SlackEventJob, true)
	work(SlackEventJob, false)
	if len(delivered) != 1 || !strings.Contains(fmt.Sprint(delivered[0]["text"]), "Which tasks are blocked on my team?") {
		t.Fatalf("welcome: %v", delivered)
	}
	for range 2 {
		accept(`{"type":"message","channel_type":"im","channel":"D1","user":"U1","text":"Which tasks are blocked?","ts":"20.1"}`)
	}
	work(SlackEventJob, true)
	work(SlackEventJob, false)
	if agent.calls != 1 || agent.chat == "" {
		t.Fatal("agent was not invoked once")
	}
	insert := func(id, role, kind, content, tools string, seq int) {
		t.Helper()
		if err := f.db.Exec("INSERT INTO agent_run_messages VALUES (?,?,'run',?,?,?,?,?,?)", id, f.workspace, agent.chat, seq, role, kind, content, tools).Error; err != nil {
			t.Fatal(err)
		}
	}
	due := func() {
		t.Helper()
		if err := f.db.Exec("UPDATE support_inbound_jobs SET available_at=? WHERE kind=?", time.Now().Add(-time.Minute), SlackAgentReplyJob).Error; err != nil {
			t.Fatal(err)
		}
	}
	if len(delivered) != 1 || strings.Join(statuses, ",") != "processing" {
		t.Fatalf("initial feedback: %v %v", delivered, statuses)
	}
	insert("turn", "user", "prompt", "which tasks", "[]", 1)
	if outcome == "quick" {
		insert("answer", "assistant", "assistant_final", "Quick answer", "[]", 2)
		work(SlackAgentReplyJob, true)
		if len(delivered) != 2 || delivered[1]["method"] != "/chat.postMessage" || delivered[1]["text"] != "Quick answer" || delivered[1]["thread_ts"] != "20.1" || strings.Join(statuses, ",") != "processing,active" {
			t.Fatalf("quick reply: %v %v", delivered, statuses)
		}
		return
	}
	insert("progress", "assistant", "assistant_turn", "private work", `[{"tool_name":"mcp__helpin__list_tasks","input":{"private":"sensitive"}}]`, 2)
	work(SlackAgentReplyJob, true)
	if len(delivered) != 2 || delivered[1]["text"] != "Reading tasks…" || delivered[1]["method"] != "/chat.postMessage" || delivered[1]["thread_ts"] != "20.1" {
		t.Fatalf("wrong read feedback: %v", delivered)
	}
	due()
	work(SlackAgentReplyJob, true)
	if len(delivered) != 2 {
		t.Fatal("repeated identical phase")
	}
	expected := "Task A is blocked."
	if outcome == "answer" {
		insert("answer", "assistant", "assistant_final", "Task A is blocked.", "[]", 3)
		insert("next", "user", "prompt", "another question", "[]", 4)
		insert("other", "assistant", "assistant_final", "Another answer.", "[]", 5)
	} else if outcome == "timeout" {
		if err := f.db.Exec("UPDATE support_inbound_jobs SET created_at=? WHERE kind=?", time.Now().Add(-6*time.Minute), SlackAgentReplyJob).Error; err != nil {
			t.Fatal(err)
		}
		expected = "Helpin didn’t receive a final answer within five minutes. Try again with a narrower question."
	} else {
		if err := f.db.Exec("INSERT INTO agent_runs VALUES ('run',?,?,?)", f.workspace, outcome, `skill "document_editing" requires tool "private-internal-name"`).Error; err != nil {
			t.Fatal(err)
		}
		expected = "The Slack agent’s configuration is missing a required tool. Ask a workspace admin to contact Helpin support."
		if outcome == "cancelled" {
			expected = "This request was cancelled. Send it again to start a new run."
		}
	}
	due()
	work(SlackAgentReplyJob, true)
	if len(delivered) != 3 || delivered[2]["text"] != expected || delivered[2]["method"] != "/chat.update" || delivered[2]["ts"] != "30.2" || delivered[2]["channel"] != "D1" {
		t.Fatalf("wrong final reply: %v", delivered)
	}
	if strings.Join(statuses, ",") != "processing,active" {
		t.Fatalf("wrong lifecycle %v", statuses)
	}
}

func TestSlackAgentScopeAndHistory(t *testing.T) {
	ctx := context.Background()
	dock := DockChatService{authz: authorization.NewAuthzService(nil, dockChatAdminMemberRepo{}, nil)}
	chat := &model.DockChat{WorkspaceID: "ws", SlackThread: true}
	agent := &model.Agent{AllowedTools: json.RawMessage(`["list_tasks","list_support_conversations","create_task","run_command","list_deals"]`)}
	tools, err := dock.scopedChatExecutionTools(ctx, chat, "user", agent)
	if err != nil || !sameNormalizedToolSet(tools, []string{"list_tasks", "list_support_conversations"}) {
		t.Fatalf("tools: %v %v", tools, err)
	}
	a := &authorization.Actor{UserID: "user", WorkspaceMemberID: "member", Role: "member"}
	m := slack.Message{Channel: "D1", TS: "1"}
	access := slackAccess{PM: true}
	root := access.chatID("install", m, a)
	m.TS = "2"
	m.ThreadTS = "1"
	if root != access.chatID("install", m, a) {
		t.Fatal("follow-up lost context")
	}
	a.UserID = "other"
	if root == access.chatID("install", m, a) {
		t.Fatal("cross-user history")
	}
	a.UserID = "user"
	access.Support = true
	if root == access.chatID("install", m, a) {
		t.Fatal("history reused across access changes")
	}
}

func TestSlackHelpRespectsAccessAndAI(t *testing.T) {
	pm := (slackAccess{PM: true}).help(false, false)
	if !strings.Contains(pm, "task’s key") || strings.Contains(pm, "blocked") || strings.Contains(pm, "Create a task") || strings.Contains(pm, "support conversation") {
		t.Fatalf("PM reader guide: %s", pm)
	}
	support := (slackAccess{Support: true}).help(true, true)
	if !strings.Contains(support, "customer report") || strings.Contains(support, "ENG-123") || strings.Contains(support, "Create a task") {
		t.Fatalf("Support guide: %s", support)
	}
}

func TestSlackAskRuntimeSkillContract(t *testing.T) {
	agent := &model.Agent{ID: "ask", IsSystem: true, PresetKey: model.AgentPresetAskAgent, PresetVersionKey: "ask_agent_default", RuntimeKind: "native_sdk", AllowedTools: mustJSONStringSlice(askAgentPresetTools())}
	original := runtimeAgentFromHelpinAgent(agent, "helpin")
	if len(original.Skills) == 0 {
		t.Fatal("expected in-app Ask skills")
	}
	input := model.AgentRunInputPayload{AllowedTools: []string{"list_tasks"}, Trigger: &model.AgentRunTriggerContext{TriggerType: dockChatTriggerType, Context: json.RawMessage(`{"dock_chat_id":"chat","slack_thread":true}`)}}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{ID: "run", WorkspaceID: "ws", TargetType: "workspace", TargetID: "ws", AgentID: "ask", DockChatID: stringPointer("chat"), Input: raw}
	projected := runtimeAgentForDockExecution(run, original)
	if len(projected.Skills) != 0 || projected.ID == original.ID || runtimeAgentBaseID(projected.ID) != original.ID {
		t.Fatal("Slack launch inherited in-app skill contract")
	}
	req, err := runtimeStartRunRequest(run, agent, projected)
	if err != nil || !sameNormalizedToolSet(req.AllowedTools, input.AllowedTools) {
		t.Fatalf("scoped launch: %+v %v", req.AllowedTools, err)
	}
}
