package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const supportInvestigationWorkingNotes = "Look at `approvalAction`:\n```php\nif ($status !== 'draft') { $status = 'scheduled'; }\n```\nNow let's check why.\nLet's draft the report.\n"

func TestParseSupportInvestigationReport(t *testing.T) {
	raw := supportInvestigationWorkingNotes + "## What AI found\nMost likely: the post was saved as a draft, so approving it does not schedule it.\n\n" +
		"## Reply\nHi Jonas,\nThe approval was saved. Open the post and click Schedule.\n\n" +
		"## Docs\nExplain that approving a draft does not schedule it.\n\n" +
		"## Task\nExisting: APP-4240 — keep the chosen time when a post is approved\n\n" +
		"## Investigate\nWhether last week's release changed approvals.\n\n" +
		"## Couldn't check\n- Server error logs\n- Approval audit trail\n\n" +
		"## For engineering\n`ApprovalBuilder.php:233` skips drafts."
	got := parseSupportInvestigationReport(raw)
	want := supportCodeCheck{
		Status:          supportCodeCheckDone,
		Found:           "Most likely: the post was saved as a draft, so approving it does not schedule it.",
		Reply:           "Hi Jonas,\n\nThe approval was saved. Open the post and click Schedule.",
		Docs:            "Explain that approving a draft does not schedule it.",
		Task:            &supportCodeCheckTask{Kind: "existing", Key: "APP-4240", Text: "keep the chosen time when a post is approved"},
		Investigate:     "Whether last week's release changed approvals.",
		UnavailableData: []string{"Server error logs", "Approval audit trail"},
		Engineering:     "`ApprovalBuilder.php:233` skips drafts.",
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("report = %s\nwant     %s", gotJSON, wantJSON)
	}
}

func TestParseSupportInvestigationReportLayouts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want supportCodeCheck
	}{
		{
			name: "keeps only the last report when the model drafts twice",
			raw:  "## What AI found\nFirst draft.\n## For engineering\nOld.\n\n**What AI found:**\nNo cause found: nothing limits imports.",
			want: supportCodeCheck{Status: supportCodeCheckDone, Found: "No cause found: nothing limits imports."},
		},
		{
			name: "omits sections of none",
			raw:  "## What AI found\nNo connected repository matches this product.\n## Reply\nNone.\n## Task\nNone\n## Couldn't check\nNone",
			want: supportCodeCheck{Status: supportCodeCheckDone, Found: "No connected repository matches this product."},
		},
		{
			name: "keeps an earlier heading inside a later section as text",
			raw:  "## What AI found\nSummary.\n## For engineering\nSteps:\nReply\nchecked twice.",
			want: supportCodeCheck{Status: supportCodeCheckDone, Found: "Summary.", Engineering: "Steps:\nReply\nchecked twice."},
		},
		{
			name: "keeps a report without the layout as the finding",
			raw:  "  Observed facts and hypotheses.  ",
			want: supportCodeCheck{Status: supportCodeCheckDone, Found: "Observed facts and hypotheses."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, _ := json.Marshal(parseSupportInvestigationReport(tt.raw))
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("report = %s\nwant     %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestParseSupportCodeCheckTask(t *testing.T) {
	tests := []struct {
		in   string
		want *supportCodeCheckTask
	}{
		{in: "Existing: APP-4240 — keep the chosen time", want: &supportCodeCheckTask{Kind: "existing", Key: "APP-4240", Text: "keep the chosen time"}},
		{in: "New: Bug: CSV import stops after 1,000 rows", want: &supportCodeCheckTask{Kind: "new", Text: "Bug: CSV import stops after 1,000 rows"}},
		{in: "CSV import stops after 1,000 rows", want: &supportCodeCheckTask{Kind: "new", Text: "CSV import stops after 1,000 rows"}},
		{in: "Existing: the approvals task", want: &supportCodeCheckTask{Kind: "new", Text: "the approvals task"}},
		{in: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := parseSupportCodeCheckTask(tt.in)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("task = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseSupportInvestigationReportShortensEngineering(t *testing.T) {
	engineering := "```go\n" + strings.Repeat("line of code evidence\n", 400) + "```"
	got := parseSupportInvestigationReport("## What AI found\nThe export button times out.\n## For engineering\n" + engineering)
	if got.Found != "The export button times out." {
		t.Errorf("support finding was cut: %q", got.Found)
	}
	if n := len([]rune(got.Engineering)); n > supportInvestigationNoteLimit || !strings.HasSuffix(got.Engineering, supportInvestigationShortened) || strings.Count(got.Engineering, "```")%2 != 0 {
		t.Errorf("engineering not bounded cleanly: %d characters", n)
	}
}

func setupCodeCheckCardTest(t *testing.T) (*SupportChatService, *repository.SupportInvestigationRepository, *model.SupportInvestigation, time.Time) {
	t.Helper()
	ctx := context.Background()
	svc, db, conv, _ := setupAIControlTest(t)
	if err := db.AutoMigrate(&model.SupportInvestigation{}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO workspaces(id,name,slug,workspace_key,owner_id) VALUES ('ws','Acme','acme','APP','owner')`)
	mustExec(t, db, `INSERT INTO pm_tasks(id,workspace_id,display_id,name,workflow_id,workflow_state_id) VALUES ('task-4240','ws',4240,'Keep the chosen time','wf','state')`)
	job := newSupportInvestigation(conv, "draft-approval", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "Why does an approved draft stay in Draft?"}, time.Now().UTC())
	transition := func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error) {
		return map[string]any{"ai_state": "escalated", "human_takeover": true}, nil, nil
	}
	posted := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	card := newSupportCodeCheckNote(job, supportCodeCheck{Status: supportCodeCheckRunning, Focus: "How approving a post sets its publishing time"}, posted)
	for range 2 {
		if _, changed, _, err := svc.conversationRepo.ChangeAIControlWithCodeCheck(ctx, "ws", "conv", transition, job, card); err != nil || !changed {
			t.Fatalf("handover %v %v", changed, err)
		}
		mustExec(t, db, `UPDATE support_conversations SET ai_state='pending', human_takeover=false WHERE id='conv'`)
	}
	repo := repository.NewSupportInvestigationRepository(db)
	claimed, err := repo.Claim(ctx, time.Now().Add(time.Second))
	if err != nil || claimed == nil {
		t.Fatalf("claim %v %v", claimed, err)
	}
	return &SupportChatService{investigationRepo: repo, conversationRepo: svc.conversationRepo}, repo, claimed, posted
}

func codeCheckNotes(t *testing.T, chat *SupportChatService, id string) []model.SupportMessage {
	t.Helper()
	var notes []model.SupportMessage
	if err := chat.conversationRepo.DB().Where("id = ?", id).Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	return notes
}

func TestSupportCodeCheckCardIsFilledInPlace(t *testing.T) {
	chat, _, job, posted := setupCodeCheckCardTest(t)
	running := codeCheckNotes(t, chat, job.ID)
	if len(running) != 1 || !strings.Contains(running[0].Metadata, `"status":"running"`) {
		t.Fatalf("running card = %+v", running)
	}
	report := parseSupportInvestigationReport("## What AI found\nMost likely: drafts are not scheduled on approval.\n## Reply\nHi,\nOpen the post and click Schedule.\n## Task\nExisting: APP-4240 — keep the chosen time")
	if err := chat.finishSupportInvestigationReport(context.Background(), job, report); err != nil {
		t.Fatal(err)
	}
	notes := codeCheckNotes(t, chat, job.ID)
	if len(notes) != 1 || derefString(notes[0].SenderDisplayName) != "Code check" || !notes[0].IsInternal {
		t.Fatalf("notes = %+v", notes)
	}
	if !notes[0].CreatedAt.Equal(posted) {
		t.Errorf("card moved: created_at %v, want %v", notes[0].CreatedAt, posted)
	}
	var metadata struct {
		Check supportCodeCheck `json:"code_check"`
	}
	if err := json.Unmarshal([]byte(notes[0].Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	check := metadata.Check
	if check.Status != supportCodeCheckDone || check.Found != "Most likely: drafts are not scheduled on approval." || check.Reply != "Hi,\n\nOpen the post and click Schedule." {
		t.Errorf("card = %+v", check)
	}
	if check.Task == nil || check.Task.ID != "task-4240" || check.Task.Key != "APP-4240" {
		t.Errorf("task = %+v", check.Task)
	}
}

func TestSupportCodeCheckOnlyLinksTasksInThisWorkspace(t *testing.T) {
	chat, _, job, _ := setupCodeCheckCardTest(t)
	for _, key := range []string{"APP-9999", "OTHER-4240"} {
		if id := chat.supportCodeCheckTaskID(context.Background(), job.WorkspaceID, key); id != "" {
			t.Errorf("task %s resolved to %q", key, id)
		}
	}
}

func TestSupportCodeCheckFailureReplacesRunningCard(t *testing.T) {
	chat, _, job, _ := setupCodeCheckCardTest(t)
	if err := chat.finishSupportInvestigation(context.Background(), job, "The code check didn't finish, so it has no findings. Handle this conversation as usual."); err != nil {
		t.Fatal(err)
	}
	notes := codeCheckNotes(t, chat, job.ID)
	if len(notes) != 1 || !strings.Contains(notes[0].Metadata, `"status":"failed"`) || !strings.Contains(notes[0].Content, "didn't finish") {
		t.Fatalf("notes = %+v", notes)
	}
}

func TestSupportCodeCheckThatNeverRanRemovesRunningCard(t *testing.T) {
	chat, _, job, _ := setupCodeCheckCardTest(t)
	if err := chat.finishSupportInvestigationWithoutNote(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if notes := codeCheckNotes(t, chat, job.ID); len(notes) != 0 {
		t.Fatalf("running card left behind: %+v", notes)
	}
}
