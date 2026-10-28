package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

const supportCodeCheckTestReport = `I'll look at the export settings first, then the CSV writer.

## What AI found
Confirmed: CSV exports leave out the Notes column until it is turned on under Columns. Exports keep up to 50 columns.
## Reply
Hi Sarah,

Open Reports, choose Export, and turn on Notes under Columns.

Best regards,
## Docs
How to choose export columns.
## Task
None
## Investigate
None
## Couldn't check
None
## For engineering
reports/export.go buildColumns skips notes unless include_notes is set.`

// setupSupportCodeCheckTest is an email conversation in a workspace that
// turned the code check on and connected one repository.
func setupSupportCodeCheckTest(t *testing.T) (*SupportChatService, *InternalCommandService, *gorm.DB, model.InternalCommandContext) {
	t.Helper()
	chat, commands, db, _, _, meta := setupSupportProgressTest(t)
	setSupportCodeCheckSettings(t, db, true)
	mustExec(t, db, `UPDATE support_conversations SET channel='email', source='email' WHERE id='conv'`)
	mustExec(t, db, `INSERT INTO git_repositories(id,workspace_id,integration_id,full_name) VALUES ('repo','ws','integration','acme/app')`)
	mustExec(t, db, `CREATE TABLE agent_run_messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL, actor_user_id TEXT, runtime_message_id TEXT, dock_chat_id TEXT, role TEXT NOT NULL, content TEXT NOT NULL, message_type TEXT NOT NULL, content_blocks BLOB, turn_segments BLOB, tool_invocations BLOB, token_usage BLOB, sequence_no INTEGER NOT NULL, created_at DATETIME)`)
	mustExec(t, db, `DELETE FROM command_bar_plans`)
	mustExec(t, db, `UPDATE agent_runs SET input=? WHERE id='run'`, []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`))
	commands.agentService = &AgentService{}
	return chat, commands, db, meta
}

func setSupportCodeCheckSettings(t *testing.T, db *gorm.DB, enabled bool) {
	t.Helper()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	settings.AIReplyChannels = "both"
	settings.AIEmailCodeCheckEnabled = enabled
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_widget_installations SET settings=? WHERE workspace_id='ws'`, string(raw))
}

func supportCodeCheckSteps() []dockLaunchStep {
	return []dockLaunchStep{{Instructions: "Does the CSV export include the Notes column by default?", AllowedTools: []string{"list_repositories", "repository_search", "read_files", "read_skill"}}}
}

func TestSupportCodeCheckLaunchAdmission(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, *gorm.DB)
		want   string
	}{
		{name: "email with the setting on", want: "requires agent_id"},
		{name: "chat conversation", change: func(t *testing.T, db *gorm.DB) {
			mustExec(t, db, `UPDATE support_conversations SET channel='widget', source='widget' WHERE id='conv'`)
		}, want: "not available"},
		{name: "setting off", change: func(t *testing.T, db *gorm.DB) { setSupportCodeCheckSettings(t, db, false) }, want: "not available"},
		{name: "no connected repository", change: func(t *testing.T, db *gorm.DB) { mustExec(t, db, `DELETE FROM git_repositories`) }, want: "not available"},
		{name: "already checked for this message", change: func(t *testing.T, db *gorm.DB) {
			steps, _ := json.Marshal([]model.CommandBarPlanStep{{AllowedTools: []string{"read_files"}}})
			mustExec(t, db, `INSERT INTO command_bar_plans(id,workspace_id,parent_chat_run_id,support_conversation_id,status,prompt,steps,created_at) VALUES ('earlier','ws','run','conv','completed','Check',?,?)`, steps, time.Now().Add(time.Minute))
		}, want: "already ran"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, commands, db, meta := setupSupportCodeCheckTest(t)
			if tt.change != nil {
				tt.change(t, db)
			}
			_, err := commands.executeDockLaunch(context.Background(), meta, supportCodeCheckSteps(), "", "Code check")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("launch error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSupportCodeCheckScope(t *testing.T) {
	if !supportStepsAreCodeCheck(supportCodeCheckSteps()) {
		t.Fatal("repository reads with skills must be a code check")
	}
	for name, steps := range map[string][]dockLaunchStep{
		"web research mixed in": {{AllowedTools: []string{"read_files", "web_search"}}},
		"task reads":            {{AllowedTools: []string{"read_files", "list_tasks"}}},
		"skills only":           {{AllowedTools: []string{"read_skill"}}},
		"two steps":             {{AllowedTools: []string{"read_files"}}, {AllowedTools: []string{"read_files"}}},
	} {
		if supportStepsAreCodeCheck(steps) {
			t.Errorf("%s admitted as a code check", name)
		}
	}
}

func TestSupportCodeCheckReachesAgentVerdictFirst(t *testing.T) {
	plan := &model.CommandBarPlanRecord{ID: "plan", WorkspaceID: "ws"}
	confirmed := parseSupportInvestigationReport(supportCodeCheckTestReport)
	text := renderSupportCodeCheckForAgent(confirmed)
	if !strings.HasPrefix(text, "Confirmed:") || !strings.Contains(text, "Suggested reply:") || strings.Contains(text, "export.go") {
		t.Fatalf("agent text = %q", text)
	}
	evidence := supportCodeCheckEvidence(plan, confirmed)
	if len(evidence) != 1 || strings.HasPrefix(evidence[0].Content, "Confirmed") || !strings.Contains(evidence[0].Content, "50 columns") || !strings.Contains(evidence[0].Content, "turn on Notes") {
		t.Fatalf("evidence = %+v", evidence)
	}
	likely := parseSupportInvestigationReport(strings.Replace(supportCodeCheckTestReport, "Confirmed:", "Most likely:", 1))
	if got := supportCodeCheckEvidence(plan, likely); got != nil {
		t.Fatalf("an unconfirmed finding became citable: %+v", got)
	}
	if text := renderSupportCodeCheckForAgent(likely); strings.Contains(text, "Suggested reply") {
		t.Fatalf("unconfirmed finding offered a reply: %q", text)
	}
}

func TestSupportCodeCheckResultPostsOneCardForTheTeam(t *testing.T) {
	ctx := context.Background()
	chat, _, db, _ := setupSupportCodeCheckTest(t)
	chat.agentService = &AgentService{runMessageRepo: repository.NewAgentRunMessageRepository(db)}
	chat.runRepo = repository.NewAgentRunRepository(db)
	mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status) VALUES ('child-run','ws','agent','workspace','ws','completed')`)
	mustExec(t, db, `INSERT INTO agent_run_messages(id,workspace_id,run_id,role,content,message_type,sequence_no) VALUES ('m1','ws','child-run','assistant',?,'text',1)`, supportCodeCheckTestReport)
	steps, _ := json.Marshal([]model.CommandBarPlanStep{{AllowedTools: []string{"list_repositories", "read_files"}}})
	plan := &model.CommandBarPlanRecord{ID: "plan", WorkspaceID: "ws", Status: model.CommandBarPlanStatusCompleted, Steps: steps, RunIDsByStep: json.RawMessage(`{"0":"child-run"}`)}

	block, evidence, err := chat.supportChildResult(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, `"summary":"Confirmed:`) || strings.Contains(block, "export.go") || strings.Contains(block, "I'll look at") {
		t.Fatalf("block = %s", block)
	}
	if len(evidence) != 1 || evidence[0].SourceType != supportChildSourceRepository {
		t.Fatalf("evidence = %+v", evidence)
	}

	check, runID, ok := chat.supportPlanCodeCheck(ctx, plan)
	if !ok || runID != "child-run" {
		t.Fatalf("code check not recognised: %v %q", ok, runID)
	}
	for range 2 {
		chat.fillSupportCodeCheckCard(ctx, plan, "conv", runID, check)
	}
	history, err := chat.messageRepo.ListForSupportAI(ctx, "ws", "conv", true)
	if err != nil {
		t.Fatal(err)
	}
	cards := 0
	for _, message := range history {
		if message.IsInternal && strings.Contains(message.Metadata, `"code_check"`) {
			cards++
			if !strings.Contains(message.Content, "50 columns") {
				t.Fatalf("card content = %q", message.Content)
			}
		}
	}
	if cards != 1 {
		t.Fatalf("cards = %d, want 1", cards)
	}
	if !supportCodeCheckedThisTurn(history) {
		t.Fatal("a handoff now would start a second code check")
	}
}

func TestSupportCodeCheckCardRunsThenFillsInPlace(t *testing.T) {
	for _, tt := range []struct {
		name   string
		check  supportCodeCheck
		status string
		text   string
	}{
		{name: "findings", check: parseSupportInvestigationReport(supportCodeCheckTestReport), status: supportCodeCheckDone, text: "50 columns"},
		{name: "did not finish", check: supportCodeCheck{Status: supportCodeCheckFailed, Found: supportCodeCheckUnfinishedFinding}, status: supportCodeCheckFailed, text: "didn't finish"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			chat, commands, _, _ := setupSupportCodeCheckTest(t)
			commands.supportAIService.messageRepo = chat.messageRepo
			plan := &model.CommandBarPlanRecord{ID: "plan", WorkspaceID: "ws"}
			commands.postSupportCodeCheckRunning(ctx, "ws", "conv", plan.ID, "child-run", "Does the CSV export include the Notes column by default? Check acme/app export code.")
			running, err := chat.messageRepo.GetByID(ctx, supportCodeCheckCardID("ws", plan.ID))
			if err != nil || running == nil || supportCodeCheckCardStatus(running.Metadata) != supportCodeCheckRunning {
				t.Fatalf("running card = %+v, %v", running, err)
			}
			if !strings.Contains(running.Metadata, `"focus":"Does the CSV export include the Notes column by default?"`) || !strings.Contains(running.Metadata, `"phase":"before_reply"`) {
				t.Fatalf("running card metadata = %s", running.Metadata)
			}
			for range 2 {
				chat.fillSupportCodeCheckCard(ctx, plan, "conv", "child-run", tt.check)
			}
			filled, err := chat.messageRepo.GetByID(ctx, running.ID)
			if err != nil || filled == nil || supportCodeCheckCardStatus(filled.Metadata) != tt.status || !strings.Contains(filled.Content, tt.text) {
				t.Fatalf("filled card = %+v, %v", filled, err)
			}
			history, err := chat.messageRepo.ListForSupportAI(ctx, "ws", "conv", true)
			if err != nil {
				t.Fatal(err)
			}
			cards := 0
			for _, message := range history {
				if strings.Contains(message.Metadata, `"code_check"`) {
					cards++
				}
			}
			if cards != 1 {
				t.Fatalf("cards = %d, want the running card filled in place", cards)
			}
		})
	}
}

func TestSupportCodeCheckFocusKeepsOnlyAPlainQuestion(t *testing.T) {
	for in, want := range map[string]string{
		"Does the CSV export include the Notes column by default? Check acme/app.": "Does the CSV export include the Notes column by default?",
		"Why are posts saved as drafts after approval\nRepo: acme/app":             "Why are posts saved as drafts after approval",
		"Check reports/export.go buildColumns for include_notes.":                  "",
		"": "",
	} {
		if got := supportCodeCheckFocus(in); got != want {
			t.Errorf("supportCodeCheckFocus(%q) = %q, want %q", in, got, want)
		}
	}
}
