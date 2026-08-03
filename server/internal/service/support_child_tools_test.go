package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportStepsAreReadOnly(t *testing.T) {
	tests := []struct {
		name  string
		steps []dockLaunchStep
		want  bool
	}{
		{
			name: "all read-only tools",
			steps: []dockLaunchStep{
				{Instructions: "check repo", AllowedTools: []string{"read_file", "search_files"}},
				{Instructions: "check docs", AllowedTools: []string{"search_knowledge"}},
			},
			want: true,
		},
		{
			name:  "no steps",
			steps: nil,
			want:  false,
		},
		{
			name: "step without allowed_tools",
			steps: []dockLaunchStep{
				{Instructions: "check repo"},
			},
			want: false,
		},
		{
			name: "mutating tool present",
			steps: []dockLaunchStep{
				{Instructions: "fix it", AllowedTools: []string{"read_file", "create_task"}},
			},
			want: false,
		},
		{
			name: "one clean step and one dirty step",
			steps: []dockLaunchStep{
				{Instructions: "read", AllowedTools: []string{"read_file"}},
				{Instructions: "write", AllowedTools: []string{"update_document"}},
			},
			want: false,
		},
		{
			name: "whitespace tolerated",
			steps: []dockLaunchStep{
				{Instructions: "read", AllowedTools: []string{" read_file "}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportStepsAreReadOnly(tt.steps); got != tt.want {
				t.Errorf("supportStepsAreReadOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportChildResearchKinds(t *testing.T) {
	steps := []model.CommandBarPlanStep{
		{AllowedTools: []string{"web_search_exa", "fetch_url"}},
		{AllowedTools: []string{"checkout_repository", "ripgrep", "read_file"}},
	}
	hasWeb, hasRepository := supportChildResearchKinds(steps)
	if !hasWeb || !hasRepository {
		t.Fatalf("supportChildResearchKinds() = web:%v repo:%v, want both", hasWeb, hasRepository)
	}
}

func TestOfficialURLFromChildResult(t *testing.T) {
	block := `Third party: https://example.com/pricing. Official facts: https://www.usermaven.com/pricing.`
	if got := officialURLFromChildResult(block, "https://usermaven.com"); got != "https://www.usermaven.com/pricing" {
		t.Fatalf("officialURLFromChildResult() = %q", got)
	}
	if got := officialURLFromChildResult("https://example.com/pricing", "https://usermaven.com"); got != "" {
		t.Fatalf("third-party URL accepted: %q", got)
	}
	if got := officialURLFromChildResult("https://docs.usermaven.com/setup", "usermaven.com"); got != "https://docs.usermaven.com/setup" {
		t.Fatalf("official subdomain rejected: %q", got)
	}
}

func TestChildRunResultEvidenceID(t *testing.T) {
	result := dockChildRunResult{
		PlanID: "plan-1",
		Status: model.CommandBarPlanStatusCompleted,
		Prompt: "Visitor guessed the price is $99",
		Runs:   []dockChildRunReport{{Summary: "Official pricing lists Growth at $84.", ResultAvailable: true}},
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	block, err := addEvidenceIDToChildRunResultBlock(dockChildResultOpenTag+string(payload)+dockChildResultCloseTag, "child-result:plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if !childRunResultHasSummary(block) {
		t.Fatal("expected child summary to remain available")
	}
	evidenceContent := childRunResultEvidenceContent(block)
	if strings.Contains(evidenceContent, "$99") || !strings.Contains(evidenceContent, "$84") {
		t.Fatalf("evidence content must include only child summaries, got %q", evidenceContent)
	}
	var decoded dockChildRunResult
	jsonPayload := block[len(dockChildResultOpenTag) : len(block)-len(dockChildResultCloseTag)]
	if err := json.Unmarshal([]byte(jsonPayload), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.EvidenceID != "child-result:plan-1" {
		t.Fatalf("evidence_id = %q", decoded.EvidenceID)
	}
}

func TestOrchestratorOwnsPlan(t *testing.T) {
	dockID := "dock-1"
	otherDockID := "dock-2"
	convID := "conv-1"
	otherConvID := "conv-2"

	dockRun := &model.AgentRun{ID: "run-1", DockChatID: &dockID}
	supportRun := &model.AgentRun{ID: "run-2", TargetType: "support_conversation", TargetID: convID}

	tests := []struct {
		name string
		run  *model.AgentRun
		kind orchestratorRunKind
		plan *model.CommandBarPlanRecord
		want bool
	}{
		{
			name: "dock plan matches",
			run:  dockRun,
			kind: orchestratorRunDock,
			plan: &model.CommandBarPlanRecord{DockChatID: &dockID},
			want: true,
		},
		{
			name: "dock plan from other chat",
			run:  dockRun,
			kind: orchestratorRunDock,
			plan: &model.CommandBarPlanRecord{DockChatID: &otherDockID},
			want: false,
		},
		{
			name: "support plan matches",
			run:  supportRun,
			kind: orchestratorRunSupport,
			plan: &model.CommandBarPlanRecord{SupportConversationID: &convID},
			want: true,
		},
		{
			name: "support plan from other conversation",
			run:  supportRun,
			kind: orchestratorRunSupport,
			plan: &model.CommandBarPlanRecord{SupportConversationID: &otherConvID},
			want: false,
		},
		{
			name: "support run cannot claim dock plan",
			run:  supportRun,
			kind: orchestratorRunSupport,
			plan: &model.CommandBarPlanRecord{DockChatID: &dockID},
			want: false,
		},
		{
			name: "nil plan",
			run:  dockRun,
			kind: orchestratorRunDock,
			plan: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orchestratorOwnsPlan(tt.run, tt.kind, tt.plan); got != tt.want {
				t.Errorf("orchestratorOwnsPlan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportPlanDeliveryBlocked(t *testing.T) {
	takeover := true
	escalated := "escalated"
	pending := "pending"
	now := time.Now().UTC()

	tests := []struct {
		name string
		conv *model.SupportConversation
		want bool
	}{
		{name: "ai handling", conv: &model.SupportConversation{AIState: &pending}, want: false},
		{name: "human takeover", conv: &model.SupportConversation{HumanTakeover: &takeover}, want: true},
		{name: "escalated", conv: &model.SupportConversation{AIState: &escalated}, want: true},
		{name: "customer requested human", conv: &model.SupportConversation{CustomerRequestedHumanAt: &now}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportPlanDeliveryBlocked(tt.conv); got != tt.want {
				t.Errorf("supportPlanDeliveryBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
