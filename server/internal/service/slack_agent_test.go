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
	for _, outcome := range []string{"quick", "answer", "fallback", "failed", "cancelled", "timeout", "worker failure"} {
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
		if r.URL.Path == "/chat.getPermalink" {
			fmt.Fprint(w, `{"ok":false,"error":"message_not_found"}`)
			return
		}
		if r.URL.Path != "/agents.sessions.setStatus" && r.URL.Path != "/chat.update" && r.URL.Path != "/chat.postMessage" {
			t.Errorf("unexpected method %s", r.URL.Path)
		}
		var message map[string]any
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Error(err)
		}
		if r.URL.Path == "/agents.sessions.setStatus" {
			statuses = append(statuses, fmt.Sprint(message["status"]))
			if outcome == "fallback" {
				fmt.Fprint(w, `{"ok":false,"error":"feature_disabled"}`)
				return
			}
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
	expectedMessages := 1
	if outcome == "fallback" {
		expectedMessages = 2
	}
	if len(delivered) != expectedMessages || (outcome == "fallback" && delivered[1]["text"] != "Reading tasks…") {
		t.Fatalf("wrong read feedback: %v", delivered)
	}
	due()
	work(SlackAgentReplyJob, true)
	if len(delivered) != expectedMessages {
		t.Fatal("repeated identical phase")
	}
	if outcome == "worker failure" {
		mustExec(t, f.db, "DROP TABLE agent_run_messages")
		mustExec(t, f.db, "UPDATE support_inbound_jobs SET attempts=1,created_at=? WHERE kind=?", time.Now().Add(-2*time.Minute), SlackAgentReplyJob)
		due()
		work(SlackAgentReplyJob, true)
		var status string
		if err := f.db.Table("support_inbound_jobs").Select("status").Where("kind=?", SlackAgentReplyJob).Scan(&status).Error; err != nil {
			t.Fatal(err)
		}
		if status != "failed" || len(delivered) != 2 || !strings.Contains(fmt.Sprint(delivered[1]["text"]), "couldn’t confirm") || strings.Join(statuses, ",") != "processing,active" {
			t.Fatal("missing terminal failure notice", status, delivered, statuses)
		}
		return
	}

	expected := "## Update\n- **Task A** is blocked."
	if outcome == "answer" || outcome == "fallback" {
		insert("answer", "assistant", "assistant_final", expected, "[]", 3)
		insert("next", "user", "prompt", "another question", "[]", 4)
		insert("other", "assistant", "assistant_final", "Another answer.", "[]", 5)
	} else if outcome == "timeout" {
		if err := f.db.Exec("UPDATE support_inbound_jobs SET created_at=? WHERE kind=?", time.Now().Add(-6*time.Minute), SlackAgentReplyJob).Error; err != nil {
			t.Fatal(err)
		}
		expected = "I didn’t receive a final answer within five minutes. If you requested a change, check the record in Helpin before retrying."
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
	if len(delivered) != expectedMessages+1 {
		t.Fatalf("unexpected deliveries: %v", delivered)
	}
	final := delivered[expectedMessages]
	method := "/chat.postMessage"
	if outcome == "fallback" {
		method = "/chat.update"
	}
	if final["text"] != expected || final["method"] != method || final["channel"] != "D1" {
		t.Fatalf("wrong final reply: %v", delivered)
	}
	if outcome == "answer" && fmt.Sprint(final["blocks"]) != fmt.Sprint([]any{map[string]string{"type": "markdown", "text": expected}}) {
		t.Fatalf("wrong formatted answer: %v", final["blocks"])
	}
	if strings.Join(statuses, ",") != "processing,active" {
		t.Fatalf("wrong lifecycle %v", statuses)
	}
}

func TestSlackAgentScopeAndHistory(t *testing.T) {
	ctx := context.Background()
	dock := DockChatService{authz: authorization.NewAuthzService(nil, dockChatAdminMemberRepo{}, nil)}
	chat := &model.DockChat{WorkspaceID: "ws", SlackThread: true}
	agent := &model.Agent{AllowedTools: json.RawMessage(`["list_tasks","list_support_conversations","update_task","create_task","run_command","list_deals"]`)}
	tools, err := dock.scopedChatExecutionTools(ctx, chat, "user", agent)
	if err != nil || !sameNormalizedToolSet(tools, []string{"list_tasks", "list_support_conversations", "update_task", "assign_support_conversation", "set_support_conversation_status", "set_support_conversation_priority"}) {
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

func TestSlackUnlinkedSupportFollowupPromptsInThread(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	mustExec(t, f.db, "CREATE TABLE slack_members(account_id text,slack_user text,user_id text)")
	mustExec(t, f.db, "CREATE TABLE slack_link_grants(token_hash text,account_id text,slack_user text,expires_at datetime)")
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{})
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users.info" {
			fmt.Fprint(w, `{"ok":true,"user":{"id":"U1"}}`)
			return
		}
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/chat.postEphemeral" || payload["user"] != "U1" || payload["thread_ts"] != "1" || payload["text"] != "Link your Helpin account to continue." {
			t.Errorf("unexpected linking response at %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer api.Close()
	s.api.BaseURL = api.URL + "/"
	handled, err := s.supportCommand(context.Background(), account, "token", &model.SlackRoute{Purpose: "mixed"}, slack.Message{Text: "What's the latest?", User: "U1", Channel: "C1", ThreadTS: "1", TS: "2"}, true)
	if !handled || err != nil || calls != 1 {
		t.Fatalf("handled=%v error=%v calls=%d", handled, err, calls)
	}
}

func TestSlackLinkCodeRedemption(t *testing.T) {
	ctx := context.Background()
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", account.ID)
	mustExec(t, f.db, "CREATE TABLE slack_members(account_id text,slack_user text,user_id text, PRIMARY KEY(account_id,slack_user))")
	mustExec(t, f.db, "CREATE TABLE slack_link_grants(token_hash text PRIMARY KEY,account_id text,slack_user text,expires_at datetime)")
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{})
	code, err := newSlackLinkCode()
	if err != nil || len(code) != 9 || code[4] != '-' {
		t.Fatal("failed to generate a short grouped Slack code")
	}
	legacy, legacyHash, err := newWebhookSecret()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, hash, input string }{
		{"lowercase without separator", slackLinkCodeHash(code), strings.ToLower(strings.ReplaceAll(code, "-", ""))},
		{"space separator", slackLinkCodeHash(code), strings.ReplaceAll(code, "-", " ")},
		{"existing grant", legacyHash, legacy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.repo.GrantLink(ctx, account.ID, "U1", tc.hash); err != nil {
				t.Fatal(err)
			}
			if err := s.LinkMember(ctx, f.workspace, "member", tc.input); err != nil {
				t.Fatal(err)
			}
			if err := s.LinkMember(ctx, f.workspace, "member", tc.input); err == nil {
				t.Fatal("reused a consumed code")
			}
		})
	}
}

func TestSlackFirstMentionWithoutChannelRule(t *testing.T) {
	ctx := context.Background()
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", account.ID)
	for _, ddl := range []string{
		"CREATE TABLE slack_channel_routes(account_id text,channel_id text,channel_name text,purpose text)",
		"CREATE TABLE slack_channel_ids(account_id text,channel_id text,routing_id text,current_id text)",
		"CREATE TABLE slack_members(account_id text,slack_user text,user_id text, PRIMARY KEY(account_id,slack_user))",
		"CREATE TABLE users(id text,email text,email_verified_at datetime)",
		"CREATE TABLE workspace_members(workspace_id text,user_id text,status text)",
		"CREATE TABLE support_conversations(id text,workspace_id text,channel_account_id text,provider_thread_key text,anonymized_at datetime,status text,deleted_at datetime)",
	} {
		mustExec(t, f.db, ddl)
	}
	mustExec(t, f.db, "INSERT INTO users VALUES ('member','case@example.com',CURRENT_TIMESTAMP)")
	mustExec(t, f.db, "INSERT INTO workspace_members VALUES (?,'member','active')", f.workspace)
	s := NewSlackService(f.svc, repository.NewSlackRepository(f.db), SlackConfig{SigningSecret: "fixture"})
	s.authz = authorization.NewAuthzService(f.db, dockChatAdminMemberRepo{}, nil)
	external := false
	var methods []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users.info":
			fmt.Fprint(w, `{"ok":true,"user":{"id":"U1","team_id":"424242","is_email_confirmed":true,"profile":{"email":"CASE@example.com"}}}`)
		case "/conversations.info":
			fmt.Fprintf(w, `{"ok":true,"channel":{"id":"C1","is_member":true,"is_ext_shared":%t}}`, external)
		case "/chat.postMessage", "/chat.postEphemeral":
			methods = append(methods, r.URL.Path)
			var msg map[string]any
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil || msg["thread_ts"] == nil || !strings.Contains(fmt.Sprint(msg["text"]), "Catch up on tasks") {
				t.Error("missing threaded help reply")
			}
			fmt.Fprint(w, `{"ok":true}`)
		default:
			t.Error("unexpected Slack method", r.URL.Path)
		}
	}))
	defer api.Close()
	s.api.BaseURL = api.URL + "/"
	accept := func(kind, ts, content string) {
		body := fmt.Sprintf(`{"type":"event_callback","team_id":"424242","event_id":"Ev","event":{"type":%q,"channel":"C1","user":"U1","ts":%q,"text":%q}}`, kind, ts, content)
		stamp := fmt.Sprint(time.Now().Unix())
		mac := hmac.New(sha256.New, []byte("fixture"))
		fmt.Fprintf(mac, "v0:%s:%s", stamp, body)
		if _, err := s.AcceptEvent(ctx, stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	accept("message", "10", "ordinary discussion")
	accept("app_mention", "11", "<@"+account.Handle+"> help")
	accept("message", "11", "<@"+account.Handle+"> help")
	if ok, err := f.svc.processNextJob(ctx, SlackEventJob); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := f.svc.processNextJob(ctx, SlackEventJob); ok || err != nil {
		t.Fatal("unexpected extra job", ok, err)
	}
	linked, err := s.repo.Member(ctx, account.ID, "U1")
	if err != nil || linked != "member" {
		t.Fatal("verified member did not link", linked, err)
	}
	external = true
	accept("app_mention", "12", "<@"+account.Handle+"> help")
	if ok, err := f.svc.processNextJob(ctx, SlackEventJob); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if strings.Join(methods, ",") != "/chat.postMessage,/chat.postEphemeral" {
		t.Fatal("wrong delivery", methods)
	}
	routes, err := s.repo.Routes(ctx, account.ID)
	if err != nil || len(routes) != 0 {
		t.Fatal("unexpected channel rule", routes, err)
	}
	mustExec(t, f.db, "INSERT INTO users VALUES ('second','case@example.com',CURRENT_TIMESTAMP)")
	mustExec(t, f.db, "INSERT INTO workspace_members VALUES (?,'second','active')", f.workspace)
	if id, err := s.repo.MatchMember(ctx, account, "U2", "case@example.com"); id != "" || err != nil {
		t.Fatal("ambiguous identity matched", id, err)
	}
}

func TestSlackReviewedSupportTaskAndFollowupAssignment(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()
	actor := &authorization.Actor{WorkspaceID: env.wsID, UserID: env.userID, WorkspaceMemberID: "member-story-001", Role: model.RoleAdmin}
	ctx = authorization.WithActor(ctx, actor)
	const permalink = "https://example.slack.com/archives/C10/p1730000000000001"
	ctx = model.WithSlackSource(ctx, permalink)
	mustExec(t, env.db, "CREATE TABLE slack_task_threads(account_id text,channel_id text,thread_ts text,task_id text,PRIMARY KEY(account_id,channel_id,thread_ts))")
	mustExec(t, env.db, "CREATE TABLE slack_channel_routes(account_id text,channel_id text,purpose text)")
	conversations := repository.NewSupportConversationRepository(env.db)
	inbox := NewSupportInboxService(conversations, nil, repository.NewSupportMessageRepository(env.db), nil, repository.NewCRMAssociationRepository(env.db), nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
	inbox.SetTaskService(env.svc)
	conv := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Wrong invoice date", Status: "open", Priority: "medium"}
	if err := conversations.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	conv, err := inbox.loadConversationAccessible(ctx, env.wsID, conv.ID)
	if err != nil || conv == nil {
		t.Fatalf("source unavailable: %v", err)
	}
	_, hash := supportPMTriageEvidence(conv, nil)
	f := newChannelFixture(t)
	account := f.connect(t, env.wsID)
	mustExec(t, f.db, "UPDATE support_channel_accounts SET provider='slack' WHERE id=?", account.ID)
	for _, ddl := range []string{
		"CREATE TABLE slack_channel_ids(account_id text,channel_id text,routing_id text,current_id text)",
		"CREATE TABLE slack_members(account_id text,slack_user text,user_id text)",
		"CREATE TABLE slack_task_drafts(id text,account_id text,user_id text,content text,expires_at datetime)",
	} {
		mustExec(t, env.db, ddl)
	}
	mustExec(t, env.db, "INSERT INTO slack_members VALUES (?,'U1',?)", account.ID, env.userID)
	s := NewSlackService(f.svc, repository.NewSlackRepository(env.db), SlackConfig{SigningSecret: "fixture"})
	s.tasks = env.svc
	s.authz = authorization.NewAuthzService(env.db, dockChatAdminMemberRepo{}, nil)
	s.channels.inbox = inbox
	s.appURL = "https://helpin.example"
	thread := model.SlackThread{AccountID: account.ID, Channel: "C10", Root: "1730000000.000001", Permalink: permalink}
	draftID := "01010101-0000-4000-8000-010101010101"
	draft, err := json.Marshal(slackDraft{Thread: thread, ConversationID: conv.ID, SourceHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, env.db, "INSERT INTO slack_task_drafts VALUES (?,?,?,?,?)", draftID, account.ID, env.userID, string(draft), time.Now().Add(time.Minute))
	payload := fmt.Sprintf(`{"type":"view_submission","team":{"id":"424242"},"user":{"id":"U1"},"view":{"callback_id":"helpin_task","private_metadata":%q,"state":{"values":{"title":{"value":{"value":"Fix invoice dates"}},"description":{"value":{"value":"Fix the incorrect invoice date."}},"team":{"value":{"selected_option":{"value":%q}}},"type":{"value":{"selected_option":{"value":"bug"}}}}}}}`, draftID, env.teamID)
	raw := url.Values{"payload": {payload}}.Encode()
	stamp := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("fixture"))
	fmt.Fprintf(mac, "v0:%s:%s", stamp, raw)
	receipt, err := s.Action(context.Background(), stamp, "v0="+hex.EncodeToString(mac.Sum(nil)), []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.repo.ThreadTask(ctx, thread.AccountID, thread.Channel, thread.Root)
	if err != nil || id == "" || !strings.Contains(fmt.Sprint(receipt), "/pm/tasks/"+id) {
		t.Fatalf("missing committed task or receipt: %v %v", receipt, err)
	}
	linked, err := conversations.GetByID(ctx, env.wsID, conv.ID, "", model.RoleAdmin)
	if err != nil || linked == nil || derefString(linked.LinkedTaskID) != id {
		t.Fatal("conversation lost reviewed task", err)
	}
	cmd := NewInternalCommandService(nil, env.svc, nil, nil, nil, nil, env.svc.taskRepo, nil)
	args, err := json.Marshal(map[string]any{"task_id": id, "owner_member_ids": []string{"member-story-001"}, "priority": "high"})
	if err != nil {
		t.Fatal(err)
	}
	guard, runDB, meta := setupDockExecutionGuardTest(t)
	cmd.agentRunRepo = guard.agentRunRepo
	cmd.authz = authorization.NewAuthzService(env.db, dockChatAdminMemberRepo{}, nil)
	meta.WorkspaceID, meta.TargetID, meta.ActorRole, meta.ActorID, meta.AgentID = env.wsID, env.wsID, model.RoleAdmin, env.userID, "ask-agent"
	meta.AgentScopeResolved = true
	meta.AgentTeamIDs = []string{env.teamID}
	runInput, err := json.Marshal(model.AgentRunInputPayload{AllowedTools: []string{"update_task"}, Trigger: &model.AgentRunTriggerContext{ActorID: &env.userID, TriggerType: dockChatTriggerType, Context: json.RawMessage(`{"dock_chat_id":"dock-1","slack_thread":true,"slack_permalink":"https://example.slack.com/archives/C10/p1730000000000001"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, runDB, "UPDATE agent_runs SET workspace_id=?, target_id=?, input=? WHERE id=?", env.wsID, env.wsID, runInput, meta.RunID)
	if _, err := cmd.Execute(ctx, meta, "pm.create_task", json.RawMessage(`{}`)); err == nil {
		t.Fatal("ungranted action was accepted")
	}
	_, err = cmd.Execute(ctx, meta, "pm.update_task", args)
	if err != nil {
		t.Fatal(err)
	}
	task, err := env.svc.GetByID(ctx, id)
	if err != nil || len(task.Owners) != 1 || task.Task.Priority != "high" {
		t.Fatal("requested assignment and priority did not persist", err)
	}
}
