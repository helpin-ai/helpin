package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

type stubInternalCommandExecutor struct {
	executeFn func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)
}

func (s stubInternalCommandExecutor) Execute(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
	if s.executeFn != nil {
		return s.executeFn(ctx, meta, name, input)
	}
	return json.RawMessage(`{}`), nil
}

func newPlannerApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:planner-approval-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			skills TEXT NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			system_prompt TEXT,
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_story_id TEXT,
			team_id TEXT,
			allowed_tools TEXT NOT NULL DEFAULT '[]',
			allowed_commands TEXT NOT NULL DEFAULT '[]',
			allowed_targets TEXT NOT NULL DEFAULT '[]',
			schedule TEXT,
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			story_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'story',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT NOT NULL DEFAULT '{}',
			output_summary TEXT NOT NULL DEFAULT '{}',
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL DEFAULT 'text',
			storage_mode TEXT NOT NULL DEFAULT 'inline',
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT,
			story_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			plan_document_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			description TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content TEXT,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	return db
}

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

func TestDecodeApprovedStoryPlanPreviewContentAcceptsArrayTestStrategy(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_stories":[{
			"ref":"story_1",
			"name":"Story A",
			"description":"Do A",
			"story_type":"feature",
			"acceptance_criteria":["works"],
			"implementation_brief":{
				"approach":"Add the metric helper",
				"files_to_modify":[{"path":"a.go","action":"modify","description":"update helper"}],
				"test_strategy":["Add parser coverage","Add integration coverage"]
			}
		}]
	}`)

	proposal, err := decodeApprovedStoryPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedStoryPlanPreviewContent returned error: %v", err)
	}
	brief := proposal.ProposedStories[0].ImplementationBrief
	if brief == nil {
		t.Fatalf("expected implementation brief, got %#v", proposal.ProposedStories[0])
	}
	expected := "Add parser coverage\nAdd integration coverage"
	if brief.TestStrategy != expected {
		t.Fatalf("expected joined test strategy %q, got %q", expected, brief.TestStrategy)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsTitleAndTypeAliases(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_stories":[{
			"ref":"story_1",
			"title":"Add 4xx error metrics tracking infrastructure",
			"description":"Do A",
			"type":"feature",
			"acceptance_criteria":["works"]
		}]
	}`)

	proposal, err := decodeApprovedStoryPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedStoryPlanPreviewContent returned error: %v", err)
	}
	if len(proposal.ProposedStories) != 1 {
		t.Fatalf("expected one proposed story, got %#v", proposal.ProposedStories)
	}
	if proposal.ProposedStories[0].Name != "Add 4xx error metrics tracking infrastructure" {
		t.Fatalf("expected title alias to populate Name, got %#v", proposal.ProposedStories[0])
	}
	if proposal.ProposedStories[0].StoryType != "feature" {
		t.Fatalf("expected type alias to populate StoryType, got %#v", proposal.ProposedStories[0])
	}
}

func TestDecodeApprovedStoryPlanPreviewContentReturnsCanonicalShapeError(t *testing.T) {
	raw := json.RawMessage(`"{\"summary\":\"Breakdown\",\"proposed_stories\":[{\"name\":\"Story A\",\"description\":\"Do A\",\"story_type\":\"feature\",\"acceptance_criteria\":[\"works\"],\"implementation_brief\":{\"approach\":\"x\",\"files_to_modify\":[],\"test_strategy\":123}}]}"`)

	_, err := decodeApprovedStoryPlanPreviewContent(raw)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "canonical story-plan shape {summary, proposed_stories}") {
		t.Fatalf("expected canonical-shape error, got %v", err)
	}
}

func TestDecodeApprovedMarkdownPreviewContentReturnsRepairOrientedError(t *testing.T) {
	_, err := decodeApprovedMarkdownPreviewContent(json.RawMessage(`{"content":"not-a-string"}`), "approved story planning document preview")
	if err == nil {
		t.Fatal("expected markdown decode error")
	}
	if !strings.Contains(err.Error(), "approved story planning document preview content must be a markdown string") {
		t.Fatalf("expected repair-oriented markdown error, got %v", err)
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

func TestNormalizeApprovalStateAfterExecution(t *testing.T) {
	run := &model.AgentRun{ApprovalState: "not_required"}
	normalizeApprovalStateAfterExecution(run, true)
	if run.ApprovalState != "pending" {
		t.Fatalf("expected approval_state pending when waiting for approval, got %q", run.ApprovalState)
	}

	run = &model.AgentRun{ApprovalState: "rejected"}
	normalizeApprovalStateAfterExecution(run, false)
	if run.ApprovalState != "not_required" {
		t.Fatalf("expected approval_state to reset after non-approval pause, got %q", run.ApprovalState)
	}

	run = &model.AgentRun{ApprovalState: "approved"}
	normalizeApprovalStateAfterExecution(run, false)
	if run.ApprovalState != "approved" {
		t.Fatalf("expected approved state to be preserved, got %q", run.ApprovalState)
	}
}

func TestFinalRoundToolMessages(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "user", Content: "Initial prompt"},
		{Role: "assistant", Content: "Need clarification", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput}}},
		{Role: "tool", Content: `{"status":"paused","pause_reason":"human_input"}`, Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput, Output: `{"status":"paused","pause_reason":"human_input"}`}}},
	}

	results := finalRoundToolMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected 1 trailing tool message, got %#v", results)
	}
	if results[0].Role != "tool" || results[0].Content != `{"status":"paused","pause_reason":"human_input"}` {
		t.Fatalf("unexpected tool message %#v", results[0])
	}
}

func TestFinalRoundToolMessagesUsesLastAssistantBoundary(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "user", Content: "Initial prompt"},
		{Role: "assistant", Content: "First round", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-1", ToolName: "read_file"}}},
		{Role: "tool", Content: "first result", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-1", ToolName: "read_file", Output: "first result"}}},
		{Role: "assistant", Content: "Second round", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-2", ToolName: workerpkg.ToolRequestHumanInput}}},
		{Role: "tool", Content: "second result", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-2", ToolName: workerpkg.ToolRequestHumanInput, Output: "second result"}}},
	}

	results := finalRoundToolMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected only final round tool results, got %#v", results)
	}
	if results[0].Content != "second result" {
		t.Fatalf("expected trailing tool result only, got %#v", results[0])
	}
}

func TestBuildPersistedAssistantRunMessageUsesCanonicalBlocksAndInvocations(t *testing.T) {
	result := &workerpkg.ExecutionResult{
		AssistantBlocks: []workerpkg.ExecutionBlock{
			{Type: workerpkg.ExecutionBlockTypeText, Text: "Planned update"},
			{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
		},
		ToolInvocations: []model.ToolInvocation{
			{ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`), OutputSummary: "package main", DurationMs: 12},
		},
		Usage: workerpkg.ExecutionUsage{InputTokens: 11, OutputTokens: 7},
	}

	message, err := buildPersistedAssistantRunMessage(result)
	if err != nil {
		t.Fatalf("buildPersistedAssistantRunMessage returned error: %v", err)
	}
	if message == nil {
		t.Fatal("expected persisted assistant message")
	}
	if message.Role != "assistant" || message.MessageType != "assistant_turn" {
		t.Fatalf("unexpected assistant message envelope %#v", message)
	}
	if message.Content != "Planned update" {
		t.Fatalf("expected content derived from text block, got %#v", message.Content)
	}
	if len(message.ContentBlocks) == 0 || len(message.ToolInvocations) == 0 || len(message.TokenUsage) == 0 {
		t.Fatalf("expected canonical persisted payloads, got %#v", message)
	}
}

func TestBuildPersistedToolResultMessagesDerivesContentFromBlocks(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "assistant", Content: "Need tool", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file"}}},
		{Role: "tool", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: "package main"}}},
	}

	results := buildPersistedToolResultMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected one persisted tool result, got %#v", results)
	}
	if results[0].Content != "package main" {
		t.Fatalf("expected content derived from tool result block, got %#v", results[0])
	}
	if results[0].Role != "tool" || results[0].MessageType != "tool_result" || len(results[0].ContentBlocks) == 0 {
		t.Fatalf("unexpected persisted tool result %#v", results[0])
	}
}

func TestPersistAssistantRunMessageWritesInteractionArtifacts(t *testing.T) {
	dbName := fmt.Sprintf("file:interaction-artifacts-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantBlocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeText, Text: "Need review"}},
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolRequestHumanInput,
					Input: json.RawMessage(`{
						"questions":[{"id":"q1","type":"single_select","text":"Pick one","options":[{"value":"a","label":"A"}]}]
					}`),
				},
				{
					ToolName: workerpkg.ToolRequestHumanApproval,
					Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the draft"}`),
				},
			},
		},
	}

	assistantMessage, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx)
	if err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}
	if assistantMessage == nil {
		t.Fatal("expected assistant message to be persisted")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 interaction artifacts, got %#v", artifacts)
	}
	types := map[string]model.AgentRunArtifact{}
	for _, artifact := range artifacts {
		types[artifact.ArtifactType] = artifact
	}
	for _, artifactType := range []string{model.AgentRunArtifactTypeHumanInputRequest, model.AgentRunArtifactTypeHumanApprovalRequest} {
		artifact, ok := types[artifactType]
		if !ok {
			t.Fatalf("missing artifact type %q in %#v", artifactType, artifacts)
		}
		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(assistantMessage.SequenceNo) {
			t.Fatalf("expected assistant sequence metadata on %q, got %#v", artifactType, metadata)
		}
	}
}

func TestLoadAndPersistProviderContinuationCheckpoint(t *testing.T) {
	dbName := fmt.Sprintf("file:provider-checkpoint-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run: run,
		agent: &model.Agent{
			Provider: strPtr(model.AgentModelProviderOpenAI),
		},
	}
	assistantMessage := &model.AgentRunMessage{SequenceNo: 7}
	result := &workerpkg.ExecutionResult{
		ProviderContinuation: &workerpkg.ProviderContinuation{
			Provider:           model.AgentModelProviderOpenAI,
			ResponseID:         "resp_123",
			PreviousResponseID: "resp_122",
		},
	}

	if err := activities.persistProviderResponseCheckpoint(context.Background(), state, result, assistantMessage); err != nil {
		t.Fatalf("persistProviderResponseCheckpoint returned error: %v", err)
	}

	loaded, err := activities.loadProviderContinuation(context.Background(), state)
	if err != nil {
		t.Fatalf("loadProviderContinuation returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded continuation")
	}
	if loaded.Provider != model.AgentModelProviderOpenAI || loaded.ResponseID != "resp_123" || loaded.PreviousResponseID != "resp_122" || loaded.AfterSequenceNo != 7 {
		t.Fatalf("unexpected loaded continuation %#v", loaded)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one checkpoint artifact, got %#v", artifacts)
	}
	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(7) {
		t.Fatalf("expected assistant_message_sequence_no metadata, got %#v", metadata)
	}
}

func TestEnsureRunConversationCreatesPromptWhenOnlyStatusMessageExists(t *testing.T) {
	dbName := fmt.Sprintf("file:run-conversation-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create message table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}

	if _, err := activities.createRunMessage(context.Background(), run, "assistant", "status", "Preparing workspace and loading run context.", nil, nil, nil); err != nil {
		t.Fatalf("create status message: %v", err)
	}

	history, _, _, err := activities.ensureRunConversation(context.Background(), state, "Operator notes:\nFocus on setup.", planningRunInput{})
	if err != nil {
		t.Fatalf("ensureRunConversation returned error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected only prompt history entry, got %#v", history)
	}
	if history[0].Role != "user" || !strings.Contains(history[0].Content, "Operator notes:") {
		t.Fatalf("unexpected execution history %#v", history[0])
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected status + prompt messages, got %#v", messages)
	}
	if messages[0].MessageType != "status" || messages[1].MessageType != "prompt" {
		t.Fatalf("unexpected message types %#v", messages)
	}
}

func TestEnsureRunConversationBuildsTranscriptSummaryCheckpoint(t *testing.T) {
	dbName := fmt.Sprintf("file:run-summary-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}

	for i := 0; i < 13; i++ {
		role := "assistant"
		messageType := "assistant_turn"
		content := fmt.Sprintf("Assistant update %d", i)
		if i%2 == 0 {
			role = "user"
			messageType = "prompt"
			content = fmt.Sprintf("User request %d", i)
		}
		if _, err := activities.createRunMessage(context.Background(), run, role, messageType, content, nil, nil, nil); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}

	history, _, _, err := activities.ensureRunConversation(context.Background(), state, "", planningRunInput{})
	if err != nil {
		t.Fatalf("ensureRunConversation returned error: %v", err)
	}
	if len(history) != 9 {
		t.Fatalf("expected summary + 8 recent messages, got %#v", history)
	}
	if history[0].Role != "user" || !strings.Contains(history[0].Content, "Resume context from earlier turns:") {
		t.Fatalf("expected synthetic summary message first, got %#v", history[0])
	}
	if history[1].SequenceNo != 6 {
		t.Fatalf("expected recent history to start after summarized cutoff, got %#v", history[1])
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType == workerpkg.TranscriptSummaryArtifactType {
			found = true
			if artifact.InlineContent == nil {
				t.Fatalf("unexpected transcript summary artifact %#v", artifact)
			}
			var checkpoint workerpkg.TranscriptSummaryCheckpoint
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
				t.Fatalf("unmarshal transcript summary artifact: %v", err)
			}
			if checkpoint.CoveredThroughSequenceNo != 5 || checkpoint.SourceMessageCount != 5 || strings.TrimSpace(checkpoint.Summary) == "" {
				t.Fatalf("unexpected transcript summary checkpoint %#v", checkpoint)
			}
			var metadata map[string]any
			if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
				t.Fatalf("unmarshal transcript summary metadata: %v", err)
			}
			if got := metadata["assistant_message_sequence_no"]; got != float64(4) {
				t.Fatalf("expected transcript summary to link to last covered assistant turn, got %#v", metadata)
			}
		}
	}
	if !found {
		t.Fatal("expected transcript summary artifact to be persisted")
	}
}

func TestCaptureTranscriptPlanningArtifactsLinksPreviewToAssistantTurn(t *testing.T) {
	dbName := fmt.Sprintf("file:preview-linkage-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", TargetType: "epic"}
	state := &resolvedRunState{run: run, epic: &model.PMEpic{ID: "epic-1"}}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolPublishPreview,
					Input: json.RawMessage(`{
						"panel_key":"prd_draft",
						"title":"PRD draft",
						"format":"markdown",
						"content":"# Draft",
						"replace":true
					}`),
				},
			},
		},
	}

	if err := activities.captureTranscriptPlanningArtifacts(context.Background(), state, execCtx, &model.AgentRunMessage{SequenceNo: 9}, planningRunInput{}); err != nil {
		t.Fatalf("captureTranscriptPlanningArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list preview artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one preview artifact, got %#v", artifacts)
	}
	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal preview metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(9) {
		t.Fatalf("expected preview metadata to link to assistant turn, got %#v", metadata)
	}
}

func TestCaptureTranscriptPlanningArtifactsPersistsStoryPlannerPreview(t *testing.T) {
	dbName := fmt.Sprintf("file:story-preview-linkage-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-story-1", WorkspaceID: "ws-1", TargetType: "story"}
	state := &resolvedRunState{
		run:   run,
		story: &model.PMStory{ID: "story-1", WorkspaceID: "ws-1", Name: "Kafka health monitoring"},
	}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolPublishStoryPlanDoc,
					Input: json.RawMessage(`{
						"title":"Story Planning Document",
						"content":"# Outcome\n\nImplement Kafka health monitoring."
					}`),
				},
			},
		},
	}

	if err := activities.captureTranscriptPlanningArtifacts(context.Background(), state, execCtx, &model.AgentRunMessage{SequenceNo: 11}, planningRunInput{Stage: model.PlanningStageStoryPlanDoc}); err != nil {
		t.Fatalf("captureTranscriptPlanningArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list preview artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one preview artifact, got %#v", artifacts)
	}
	if artifacts[0].ArtifactType != workerpkg.RunPreviewArtifactType {
		t.Fatalf("expected run preview artifact, got %#v", artifacts[0])
	}

	var preview workerpkg.PublishedPreview
	if err := json.Unmarshal([]byte(derefString(artifacts[0].InlineContent)), &preview); err != nil {
		t.Fatalf("unmarshal preview artifact: %v", err)
	}
	if preview.PanelKey != "story_plan_doc" || preview.Format != workerpkg.PreviewFormatMarkdown {
		t.Fatalf("unexpected persisted story preview %#v", preview)
	}

	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal preview metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(11) {
		t.Fatalf("expected preview metadata to link to assistant turn, got %#v", metadata)
	}
}

func TestBuildDurableRunFactsCollectsGenericIDsFromStateAndRunInput(t *testing.T) {
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:               "run-1",
			WorkspaceID:      "ws-1",
			AgentID:          "agent-1",
			TargetType:       "crm_deal",
			TargetID:         "deal-1",
			StoryID:          strPtr("story-1"),
			ConversationID:   strPtr("conv-1"),
			RepositoryID:     strPtr("repo-run"),
			DeliveryTargetID: strPtr("delivery-run"),
			BaseBranch:       strPtr("main"),
			WorkingBranch:    strPtr("tp-123"),
			Input: json.RawMessage(`{
				"document_id":"doc-1",
				"crm":{
					"contact_id":"contact-1",
					"pipeline_stage_id":"stage-1",
					"participant_ids":["person-1","person-2"]
				}
			}`),
		},
		agent: &model.Agent{
			ID:          "agent-1",
			WorkspaceID: "ws-1",
			TeamID:      strPtr("team-agent"),
		},
		story: &model.PMStory{
			ID:             "story-1",
			EpicID:         strPtr("epic-1"),
			TeamID:         strPtr("team-story"),
			OwnerID:        strPtr("owner-1"),
			PlanDocumentID: strPtr("story-doc-1"),
		},
		epic: &model.PMEpic{
			ID:                    "epic-1",
			SpecDocumentID:        strPtr("spec-epic"),
			ApprovedSpecVersionID: strPtr("ver-approved"),
			PlanningRepositoryID:  strPtr("repo-plan"),
		},
		conversation: &model.SupportConversation{
			ID:           "conv-1",
			CRMContactID: strPtr("contact-conversation"),
		},
		deliveryTarget: &model.StoryDeliveryTarget{
			ID:           "delivery-1",
			RepositoryID: strPtr("repo-delivery"),
		},
		repository: &model.GitRepository{
			ID:            "repo-1",
			IntegrationID: "int-1",
			FullName:      "helpin-ai/helpin",
		},
		integration: &model.GitIntegration{
			ID:             "int-1",
			InstallationID: strPtr("install-1"),
		},
		teamDefault: &model.PMTeamRepoDefault{
			ID:           "team-default-1",
			TeamID:       "team-story",
			RepositoryID: "repo-team",
		},
	}

	facts := buildDurableRunFacts(state, planningRunInput{
		SpecDocumentID: "spec-input",
		SpecVersionID:  "ver-input",
	})

	for key, expected := range map[string]string{
		"workspace_id":                "ws-1",
		"run_id":                      "run-1",
		"agent_id":                    "agent-1",
		"target_type":                 "crm_deal",
		"target_id":                   "deal-1",
		"crm_deal_id":                 "deal-1",
		"document_id":                 "doc-1",
		"crm_contact_id":              "contact-1",
		"crm_pipeline_stage_id":       "stage-1",
		"crm_participant_ids":         "person-1, person-2",
		"story_id":                    "story-1",
		"epic_id":                     "epic-1",
		"plan_document_id":            "story-doc-1",
		"spec_document_id":            "spec-input",
		"spec_version_id":             "ver-input",
		"approved_spec_version_id":    "ver-approved",
		"conversation_id":             "conv-1",
		"conversation_crm_contact_id": "contact-conversation",
		"repository_id":               "repo-run",
		"delivery_target_id":          "delivery-run",
		"repo_full_name":              "helpin-ai/helpin",
		"working_branch":              "tp-123",
		"epic_planning_repository_id": "repo-plan",
		"team_default_repository_id":  "repo-team",
	} {
		if got := facts[key]; got != expected {
			t.Fatalf("expected fact %q=%q, got %q", key, expected, got)
		}
	}
}

func TestFormatInteractivePlanningFacts(t *testing.T) {
	facts := formatInteractivePlanningFacts(planningRunInput{
		SpecDocumentID: "doc-1",
		SpecVersionID:  "ver-1",
	}, true, 3)

	for _, marker := range []string{
		"Current durable planning facts:",
		"- approved_spec_exists=true",
		"- draft_spec_exists=true",
		"- existing_story_count=3",
		"- spec_document_id=doc-1",
		"- approved_spec_version_id=ver-1",
	} {
		if !strings.Contains(facts, marker) {
			t.Fatalf("expected facts summary to contain %q\n%s", marker, facts)
		}
	}
}

func TestApplyApprovedInteractivePreviewReturnsPersistPRDAction(t *testing.T) {
	dbName := fmt.Sprintf("file:approved-preview-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT,
			story_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			plan_document_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			description TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content TEXT,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)

	run := &model.AgentRun{
		ID:             "run-1",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}

	epic := &model.PMEpic{
		ID:                    "epic-1",
		WorkspaceID:           "ws-1",
		Name:                  "Epic",
		SpecDocumentID:        strPtr("doc-1"),
		SpecClarifications:    json.RawMessage(`[]`),
		ApprovedSpecVersionID: nil,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"doc-1", "ws-1", "space-1", "Epic Product Spec", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, run.AgentID,
	).Error; err != nil {
		t.Fatalf("create docs document: %v", err)
	}

	markdownJSON, err := json.Marshal("# Problem\n\nApproved draft")
	if err != nil {
		t.Fatalf("marshal markdown: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:    "prd",
		PanelKey: "prd_draft",
		Format:   workerpkg.PreviewFormatMarkdown,
		Content:  markdownJSON,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	artifact := &model.AgentRunArtifact{
		ID:            "approved-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executedCommands []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executedCommands = append(executedCommands, name)
			if name == "pm.approve_epic_spec" {
				if err := db.Exec("UPDATE pm_epics SET approved_spec_version_id = ? WHERE id = ?", "ver-1", epic.ID).Error; err != nil {
					return nil, err
				}
			}
			return json.RawMessage(`{}`), nil
		},
	}

	activity := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		epic: epic,
	}
	input := planningRunInput{}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "persist_prd" {
		t.Fatalf("expected persist_prd action, got %q", action)
	}
	if input.SpecDocumentID != "doc-1" || input.SpecVersionID != "ver-1" {
		t.Fatalf("expected planning input to be updated with approved spec ids, got %#v", input)
	}
	if len(executedCommands) != 2 || executedCommands[0] != "docs.write_document_content" || executedCommands[1] != "pm.approve_epic_spec" {
		t.Fatalf("expected docs write then epic spec approval commands, got %#v", executedCommands)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}

	updatedEpic, err := epicRepo.GetByID(context.Background(), epic.ID)
	if err != nil {
		t.Fatalf("get updated epic: %v", err)
	}
	if updatedEpic == nil || updatedEpic.Epic.SpecDocumentID == nil || updatedEpic.Epic.ApprovedSpecVersionID == nil {
		t.Fatalf("expected epic spec document/version to be set, got %#v", updatedEpic)
	}

	if updatedEpic.Epic.ApprovedSpecVersionID == nil || *updatedEpic.Epic.ApprovedSpecVersionID != "ver-1" {
		t.Fatalf("expected approved spec version to be updated, got %#v", updatedEpic.Epic.ApprovedSpecVersionID)
	}
}

func TestApplyApprovedInteractivePreviewCreatesStoriesFromApprovedStoryPlan(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	agent := &model.Agent{
		ID:                    "agent-epic",
		WorkspaceID:           "ws-1",
		Name:                  "Epic Planner",
		Status:                "running",
		RuntimeKind:           "native_sdk",
		Skills:                json.RawMessage(`[]`),
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "preset_default",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-story-plan",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       "epic-1",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-1",
		WorkspaceID:        "ws-1",
		Name:               "Epic",
		PlanningState:      model.EpicPlanningStateReadyForStoryPlanning,
		SpecClarifications: json.RawMessage(`[]`),
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_workflow_states (id, state_type) VALUES (?, ?)`, "state-1", model.PMStateTypeUnstarted).Error; err != nil {
		t.Fatalf("create workflow state: %v", err)
	}

	previewPayload, err := json.Marshal(map[string]any{
		"summary": "Breakdown",
		"proposed_stories": []map[string]any{
			{
				"ref":                 "story_1",
				"name":                "Add tracking helper",
				"description":         "Create shared metric helper",
				"story_type":          "chore",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{},
			},
			{
				"ref":                 "story_2",
				"name":                "Wire tracking into capture errors",
				"description":         "Use the helper in capture",
				"story_type":          "feature",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{"story_1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal preview payload: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:    "stories",
		PanelKey: "story_plan",
		Format:   workerpkg.PreviewFormatJSON,
		Content:  previewPayload,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal approved preview: %v", err)
	}
	if err := db.Create(&model.AgentRunArtifact{
		ID:            "approved-stories-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executed []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			if name != "pm.create_story_batch" {
				return json.RawMessage(`{}`), nil
			}
			var payload struct {
				Stories []model.ProposedStory `json:"stories"`
			}
			if err := json.Unmarshal(input, &payload); err != nil {
				return nil, err
			}
			for _, planned := range payload.Stories {
				story := &model.PMStory{
					ID:              "db-" + planned.Ref,
					WorkspaceID:     run.WorkspaceID,
					Name:            planned.Name,
					StoryType:       planned.StoryType,
					WorkflowID:      "wf-1",
					WorkflowStateID: "state-1",
					EpicID:          &epic.ID,
					Priority:        model.PMStoryPriorityNone,
					Severity:        model.PMStorySeverityNone,
				}
				if err := storyRepo.Create(ctx, story); err != nil {
					return nil, err
				}
			}
			return mustJSON(workerpkg.CreateStoryBatchResult{
				Stories: []workerpkg.CreateStoryBatchStoryResult{
					{Ref: "story_1", StoryID: "db-story_1", Name: "Add tracking helper"},
					{Ref: "story_2", StoryID: "db-story_2", Name: "Wire tracking into capture errors"},
				},
			}), nil
		},
	}

	activity := &AgentRunActivities{
		runRepo:         runRepo,
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		storyRepo:       storyRepo,
		agentRepo:       agentRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		epic: epic,
	}
	input := planningRunInput{Stage: model.PlanningStageStoryPlanDoc}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "create_stories" {
		t.Fatalf("expected create_stories action, got %q", action)
	}
	if len(executed) != 1 || executed[0] != "pm.create_story_batch" {
		t.Fatalf("expected story batch command, got %#v", executed)
	}

	var createdStories []model.PMStory
	if err := db.Where("epic_id = ?", epic.ID).Find(&createdStories).Error; err != nil {
		t.Fatalf("list created stories: %v", err)
	}
	if len(createdStories) != 2 {
		t.Fatalf("expected 2 created stories, got %d", len(createdStories))
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil || updatedRun.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected run to complete, got %#v", updatedRun)
	}

	updatedAgent, err := agentRepo.GetByID(context.Background(), agent.WorkspaceID, agent.ID)
	if err != nil {
		t.Fatalf("get updated agent: %v", err)
	}
	if updatedAgent == nil || updatedAgent.Status != "idle" {
		t.Fatalf("expected agent to be idle, got %#v", updatedAgent)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}
}

func TestApplyApprovedInteractivePreviewPersistsStoryDocAndLinksIt(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)

	agent := &model.Agent{
		ID:                    "agent-story",
		WorkspaceID:           "ws-1",
		Name:                  "Story Planner",
		Status:                "running",
		RuntimeKind:           "native_sdk",
		Skills:                json.RawMessage(`[]`),
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "preset_default",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	docID := "doc-story-plan-1"
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		docID, "ws-1", "space-1", "Track 4xx errors Plan", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, agent.ID,
	).Error; err != nil {
		t.Fatalf("create docs document: %v", err)
	}

	story := &model.PMStory{
		ID:              "story-1",
		WorkspaceID:     "ws-1",
		Name:            "Track 4xx errors",
		DisplayID:       1,
		StoryType:       model.PMStoryTypeFeature,
		WorkflowID:      "wf-1",
		WorkflowStateID: "state-1",
		Priority:        model.PMStoryPriorityNone,
		Severity:        model.PMStorySeverityNone,
		PlanDocumentID:  &docID,
	}
	if err := db.Create(story).Error; err != nil {
		t.Fatalf("create story: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-story-doc",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		StoryID:        &story.ID,
		TargetType:     "story",
		TargetID:       story.ID,
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	markdownJSON, err := json.Marshal("# Outcome\n\nAdd the helper and wire it into capture.")
	if err != nil {
		t.Fatalf("marshal markdown: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:           "story_doc",
		PanelKey:        "story_plan_doc",
		Format:          workerpkg.PreviewFormatMarkdown,
		Content:         markdownJSON,
		ApprovalSummary: "Approved story plan",
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal approved preview: %v", err)
	}
	if err := db.Create(&model.AgentRunArtifact{
		ID:            "approved-story-doc-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executed []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			return json.RawMessage(`{}`), nil
		},
	}

	activity := &AgentRunActivities{
		runRepo:         runRepo,
		artifactRepo:    artifactRepo,
		storyRepo:       storyRepo,
		agentRepo:       agentRepo,
		docsSpaceRepo:   docsSpaceRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		docsVersionRepo: docsVersionRepo,
		docsLinkRepo:    docsLinkRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:   run,
		story: story,
	}
	input := planningRunInput{Stage: model.PlanningStageStoryPlanDoc}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "persist_story_doc" {
		t.Fatalf("expected persist_story_doc action, got %q", action)
	}
	if len(executed) != 2 || executed[0] != "docs.write_document_content" || executed[1] != "docs.link_document_to_object" {
		t.Fatalf("expected docs write then docs link commands, got %#v", executed)
	}

	updatedStory, err := storyRepo.GetRawByID(context.Background(), story.ID)
	if err != nil {
		t.Fatalf("get updated story: %v", err)
	}
	if updatedStory == nil || updatedStory.PlanDocumentID == nil || *updatedStory.PlanDocumentID == "" {
		t.Fatalf("expected story plan document id to be set, got %#v", updatedStory)
	}
	if input.PlanDocumentID == "" || input.PlanDocumentID != *updatedStory.PlanDocumentID {
		t.Fatalf("expected planning input plan_document_id to be set, got %#v", input)
	}

	links, err := docsLinkRepo.ListByObject(context.Background(), run.WorkspaceID, model.LinkedObjectStory, story.ID)
	if err != nil {
		t.Fatalf("list docs links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 story plan doc link, got %d", len(links))
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil || updatedRun.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected run to complete, got %#v", updatedRun)
	}

	updatedAgent, err := agentRepo.GetByID(context.Background(), agent.WorkspaceID, agent.ID)
	if err != nil {
		t.Fatalf("get updated agent: %v", err)
	}
	if updatedAgent == nil || updatedAgent.Status != "idle" {
		t.Fatalf("expected agent to be idle, got %#v", updatedAgent)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}
}

func TestResolvePlanningRunInputClearsDeletedEpicSpecReferences(t *testing.T) {
	dbName := fmt.Sprintf("file:resolve-planning-input-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY,
			epic_id TEXT,
			workflow_state_id TEXT,
			estimate INTEGER,
			plan_document_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)

	epic := &model.PMEpic{
		ID:                    "epic-1",
		WorkspaceID:           "ws-1",
		Name:                  "Epic",
		SpecDocumentID:        strPtr("doc-missing"),
		ApprovedSpecVersionID: strPtr("ver-missing"),
		SpecClarifications:    json.RawMessage(`[]`),
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	activity := &AgentRunActivities{
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		docsVersionRepo: docsVersionRepo,
	}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:          "run-1",
			WorkspaceID: "ws-1",
			TargetType:  "epic",
			TargetID:    epic.ID,
			Input:       json.RawMessage(`{"epic_id":"epic-1","spec_document_id":"doc-stale","spec_version_id":"ver-stale"}`),
		},
		epic: epic,
	}

	input, err := activity.resolvePlanningRunInput(context.Background(), state)
	if err != nil {
		t.Fatalf("resolvePlanningRunInput returned error: %v", err)
	}
	if input.SpecDocumentID != "" || input.SpecVersionID != "" {
		t.Fatalf("expected stale spec references to be cleared, got %#v", input)
	}
	if state.epic.SpecDocumentID != nil || state.epic.ApprovedSpecVersionID != nil {
		t.Fatalf("expected in-memory epic spec state to be cleared, got %#v", state.epic)
	}

	updatedEpic, err := epicRepo.GetByID(context.Background(), epic.ID)
	if err != nil {
		t.Fatalf("get updated epic: %v", err)
	}
	if updatedEpic == nil {
		t.Fatal("expected epic after update")
	}
	if updatedEpic.Epic.SpecDocumentID != nil || updatedEpic.Epic.ApprovedSpecVersionID != nil {
		t.Fatalf("expected persisted epic spec state to be cleared, got %#v", updatedEpic.Epic)
	}
}
