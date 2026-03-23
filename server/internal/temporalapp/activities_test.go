package temporalapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func TestValidatePlanningProposalStoriesRejectsCycle(t *testing.T) {
	stories := []model.ProposedStory{
		{Ref: "story_a", Name: "Story A", AcceptanceCriteria: []string{"A works"}, DependencyRefs: []string{"story_b"}},
		{Ref: "story_b", Name: "Story B", AcceptanceCriteria: []string{"B works"}, DependencyRefs: []string{"story_a"}},
	}

	err := validatePlanningProposalStories(stories)
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestSelectRelevantPlanningFilesPrefersRelevantPaths(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                             "module example.com/test\n",
		"server/internal/handler/billing.go": "package handler\n",
		"server/internal/service/billing_service.go": "package service\n",
		"server/internal/model/invoice.go":           "package model\n",
		"web/src/pages/BillingPage.tsx":              "export const BillingPage = () => null\n",
		"docs/notes.md":                              "billing settings\n",
	}
	for name, content := range files {
		fullPath := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	paths, err := selectRelevantPlanningFiles(root, "We need to improve billing invoices and billing settings.")
	if err != nil {
		t.Fatalf("selectRelevantPlanningFiles: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected relevant planning files")
	}
	found := false
	for _, path := range paths {
		if path == "server/internal/service/billing_service.go" || path == "server/internal/handler/billing.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected billing-related backend file in result, got %#v", paths)
	}
}

func TestCollectPlanningTreeBoundsDepth(t *testing.T) {
	root := t.TempDir()
	deepPath := filepath.Join(root, "server", "internal", "service", "payments", "v2", "handler.go")
	if err := os.MkdirAll(filepath.Dir(deepPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(deepPath, []byte("package service\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tree, err := collectPlanningTree(root, 3, 20)
	if err != nil {
		t.Fatalf("collectPlanningTree: %v", err)
	}
	for _, item := range tree {
		if item == "        server/internal/service/payments/v2/handler.go" {
			t.Fatalf("expected deep file to be truncated by depth, got %#v", tree)
		}
	}
}

func TestValidatePlanningProposalStoriesNormalizesMissingRefs(t *testing.T) {
	stories := []model.ProposedStory{
		{Name: "Story A", AcceptanceCriteria: []string{"A works"}},
		{Name: "Story B", AcceptanceCriteria: []string{"B works"}, DependencyRefs: []string{"story_1"}},
	}

	if err := validatePlanningProposalStories(stories); err != nil {
		t.Fatalf("validatePlanningProposalStories returned error: %v", err)
	}
	if stories[0].Ref != "story_1" {
		t.Fatalf("expected first story ref to default to story_1, got %q", stories[0].Ref)
	}
	if stories[1].Ref != "story_2" {
		t.Fatalf("expected second story ref to default to story_2, got %q", stories[1].Ref)
	}
}

func TestMarkdownToDocsJSONPreservesHeadingsAndBullets(t *testing.T) {
	raw := tiptap.MarkdownToJSON("# Problem\n\n- first item\n- second item\n\nPlain paragraph")

	var doc struct {
		Type    string                   `json:"type"`
		Content []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal docs json: %v", err)
	}
	if doc.Type != "doc" {
		t.Fatalf("expected root type doc, got %q", doc.Type)
	}
	if len(doc.Content) < 3 {
		t.Fatalf("expected heading, list, and paragraph nodes, got %d nodes", len(doc.Content))
	}
	if doc.Content[0]["type"] != "heading" {
		t.Fatalf("expected first node to be heading, got %#v", doc.Content[0]["type"])
	}
	if doc.Content[1]["type"] != "bulletList" {
		t.Fatalf("expected second node to be bulletList, got %#v", doc.Content[1]["type"])
	}
	if doc.Content[2]["type"] != "paragraph" {
		t.Fatalf("expected third node to be paragraph, got %#v", doc.Content[2]["type"])
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsStringifiedJSON(t *testing.T) {
	raw := json.RawMessage(`"{\"summary\":\"Breakdown\",\"proposed_stories\":[{\"ref\":\"story_1\",\"name\":\"Story A\",\"description\":\"Do A\",\"story_type\":\"feature\",\"acceptance_criteria\":[\"works\"]}]}"`)

	proposal, err := decodeApprovedStoryPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedStoryPlanPreviewContent returned error: %v", err)
	}
	if proposal.Summary != "Breakdown" {
		t.Fatalf("expected summary Breakdown, got %q", proposal.Summary)
	}
	if len(proposal.ProposedStories) != 1 || proposal.ProposedStories[0].Ref != "story_1" {
		t.Fatalf("unexpected proposal stories: %#v", proposal.ProposedStories)
	}
}

func TestRenderProductSpecMarkdownAppendsNormalizedResearchSources(t *testing.T) {
	rendered := renderProductSpecMarkdown("# Problem\n\nBase spec", []model.PlanningResearchSource{
		{Title: "NIST", URL: "https://example.com/nist", Note: "security baseline"},
		{Title: "NIST duplicate", URL: "https://example.com/nist"},
		{Title: "WCAG", URL: "https://example.com/wcag", PublishedAt: "2025-01-01"},
	})

	if !strings.Contains(rendered, "## Research Sources") {
		t.Fatalf("expected research sources section, got %q", rendered)
	}
	if strings.Count(rendered, "https://example.com/nist") != 1 {
		t.Fatalf("expected duplicate source URLs to be de-duplicated, got %q", rendered)
	}
	if !strings.Contains(rendered, "published 2025-01-01") {
		t.Fatalf("expected published date note in rendered markdown, got %q", rendered)
	}
}

func TestBuildDraftSpecClarificationsIncludesQuestionsAndAssumptions(t *testing.T) {
	items := buildDraftSpecClarifications(model.ProductSpecDraft{
		Assumptions:   []string{"Launch for paid plans only"},
		OpenQuestions: []string{"Who gets access on day one?"},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 clarifications, got %#v", items)
	}
	if items[0].Kind != model.SpecClarificationKindOpenQuestion {
		t.Fatalf("expected first clarification to be an open question, got %#v", items[0])
	}
	if items[1].Kind != model.SpecClarificationKindAssumption {
		t.Fatalf("expected second clarification to be an assumption, got %#v", items[1])
	}
}

func TestResolveExecutionWaitState(t *testing.T) {
	baseRun := &model.AgentRun{
		InvocationMode: model.InvocationModeInteractive,
		TargetType:     "epic",
		ApprovalState:  "not_required",
	}

	waitForApproval, waitForInput := resolveExecutionWaitState(baseRun, nil, nil)
	if waitForApproval || waitForInput {
		t.Fatalf("expected no wait state without interaction tools, got approval=%v input=%v", waitForApproval, waitForInput)
	}

	waitForApproval, waitForInput = resolveExecutionWaitState(baseRun, &workerpkg.HumanInputRequest{
		Questions: []workerpkg.HumanInputQuestion{{ID: "q1", Text: "Who is this for?", Options: []workerpkg.HumanInputOption{{Value: "a", Label: "A"}}}},
	}, nil)
	if waitForApproval || !waitForInput {
		t.Fatalf("expected human input tool to pause for input, got approval=%v input=%v", waitForApproval, waitForInput)
	}

	waitForApproval, waitForInput = resolveExecutionWaitState(baseRun, nil, &model.ApprovalRequest{
		Phase: "prd",
		Title: "Approve PRD",
	})
	if !waitForApproval || waitForInput {
		t.Fatalf("expected inline approval tool to pause for approval, got approval=%v input=%v", waitForApproval, waitForInput)
	}

	waitForApproval, waitForInput = resolveExecutionWaitState(&model.AgentRun{
		InvocationMode: model.InvocationModeInteractive,
		TargetType:     "epic",
		ApprovalState:  "pending",
	}, nil, nil)
	if !waitForApproval || waitForInput {
		t.Fatalf("expected pending approval state to wait for approval, got approval=%v input=%v", waitForApproval, waitForInput)
	}
}

func TestFinalRoundToolMessages(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "user", Content: "Initial prompt"},
		{Role: "assistant", Content: "Need clarification", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput}}},
		{Role: "tool", Content: `{"status":"awaiting_input"}`, Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput, Output: `{"status":"awaiting_input"}`}}},
	}

	results := finalRoundToolMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected 1 trailing tool message, got %#v", results)
	}
	if results[0].Role != "tool" || results[0].Content != `{"status":"awaiting_input"}` {
		t.Fatalf("unexpected tool message %#v", results[0])
	}
}
