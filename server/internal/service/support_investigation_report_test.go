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
	tests := []struct {
		name           string
		raw            string
		wantContent    string
		wantReply      string
		wantStructured bool
	}{
		{
			name: "drops working notes before the report",
			raw: supportInvestigationWorkingNotes + "## What's happening\nThe post was saved as a draft, so approving it does not schedule it.\n\n" +
				"## How sure\nLikely. Ask whether the post was saved as a draft.\n\n## What to do now\n- Open the post and click Schedule.\n\n" +
				"## Suggested reply\nThe approval went through. Open the post and click Schedule.\n\n## For engineering\n`ApprovalBuilder.php:233` skips drafts.",
			wantContent: "## What's happening\n\nThe post was saved as a draft, so approving it does not schedule it.\n\n" +
				"## How sure\n\nLikely. Ask whether the post was saved as a draft.\n\n## What to do now\n\n- Open the post and click Schedule.\n\n" +
				"## Suggested reply\n\nThe approval went through. Open the post and click Schedule.\n\n## For engineering\n\n`ApprovalBuilder.php:233` skips drafts.",
			wantReply:      "The approval went through. Open the post and click Schedule.",
			wantStructured: true,
		},
		{
			name:           "keeps only the last report when the model drafts twice",
			raw:            "## What's happening\nFirst draft.\n## For engineering\nOld.\n\n**What’s happening:**\nFinal summary.\n**How sure:**\nUnknown.",
			wantContent:    "## What's happening\n\nFinal summary.\n\n## How sure\n\nUnknown.",
			wantStructured: true,
		},
		{
			name:           "omits a suggested reply of none",
			raw:            "## What's happening\nNo cause found.\n## Suggested reply\nNone.\n## For engineering\nNothing matched.",
			wantContent:    "## What's happening\n\nNo cause found.\n\n## For engineering\n\nNothing matched.",
			wantStructured: true,
		},
		{
			name:           "keeps an earlier heading inside a later section as text",
			raw:            "## What's happening\nSummary.\n## For engineering\nSteps:\nHow sure\nchecked twice.",
			wantContent:    "## What's happening\n\nSummary.\n\n## For engineering\n\nSteps:\nHow sure\nchecked twice.",
			wantStructured: true,
		},
		{
			name:        "keeps a report without the layout as written",
			raw:         "  Observed facts and hypotheses.  ",
			wantContent: "Observed facts and hypotheses.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSupportInvestigationReport(tt.raw)
			if got.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.SuggestedReply != tt.wantReply {
				t.Errorf("suggested reply = %q, want %q", got.SuggestedReply, tt.wantReply)
			}
			if got.Structured != tt.wantStructured {
				t.Errorf("structured = %v, want %v", got.Structured, tt.wantStructured)
			}
		})
	}
}

func TestParseSupportInvestigationReportShortensEngineeringFirst(t *testing.T) {
	engineering := "```go\n" + strings.Repeat("line of code evidence\n", 400) + "```"
	got := parseSupportInvestigationReport("## What's happening\nThe export button times out.\n## For engineering\n" + engineering)
	if n := len([]rune(got.Content)); n > supportInvestigationNoteLimit {
		t.Fatalf("note has %d characters, limit %d", n, supportInvestigationNoteLimit)
	}
	if !strings.HasPrefix(got.Content, "## What's happening\n\nThe export button times out.") {
		t.Errorf("support summary was cut: %q", got.Content[:80])
	}
	if !strings.HasSuffix(got.Content, supportInvestigationShortened) {
		t.Errorf("missing shortened marker")
	}
	if strings.Count(got.Content, "```")%2 != 0 {
		t.Errorf("code block left open")
	}
}

func TestParseSupportInvestigationReportDropsLongReplyFromMetadata(t *testing.T) {
	reply := strings.Repeat("word ", supportInvestigationReplyLimit)
	got := parseSupportInvestigationReport("## What's happening\nSummary.\n## Suggested reply\n" + reply)
	if got.SuggestedReply != "" {
		t.Fatal("an over-long reply must not be offered for one-click use")
	}
}

func TestFinishSupportInvestigationReportStoresSupportLayout(t *testing.T) {
	ctx := context.Background()
	svc, db, conv, _ := setupAIControlTest(t)
	if err := db.AutoMigrate(&model.SupportInvestigation{}); err != nil {
		t.Fatal(err)
	}
	job := newSupportInvestigation(conv, "draft-approval", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "Why does an approved draft stay in Draft?"}, time.Now().UTC())
	transition := func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error) {
		return map[string]any{"ai_state": "escalated", "human_takeover": true}, nil, nil
	}
	if _, changed, err := svc.conversationRepo.ChangeAIControlWithInvestigation(ctx, "ws", "conv", transition, job); err != nil || !changed {
		t.Fatalf("handover %v %v", changed, err)
	}
	repo := repository.NewSupportInvestigationRepository(db)
	claimed, err := repo.Claim(ctx, time.Now().Add(time.Second))
	if err != nil || claimed == nil {
		t.Fatalf("claim %v %v", claimed, err)
	}
	chat := &SupportChatService{investigationRepo: repo}
	report := parseSupportInvestigationReport("## What's happening\nSummary.\n## Suggested reply\nOpen the post and click Schedule.")
	if err := chat.finishSupportInvestigationReport(ctx, claimed, report); err != nil {
		t.Fatal(err)
	}
	var note model.SupportMessage
	if err := db.Where("id = ?", job.ID).First(&note).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(note.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["report_format"] != supportInvestigationReportFormat || metadata["suggested_reply"] != "Open the post and click Schedule." {
		t.Errorf("metadata = %v", metadata)
	}
	if derefString(note.SenderDisplayName) != "Code check" || !note.IsInternal {
		t.Errorf("note = %q internal=%v", derefString(note.SenderDisplayName), note.IsInternal)
	}
}
