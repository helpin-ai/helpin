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
			epic_id TEXT,
			workflow_state_id TEXT,
			estimate INTEGER,
			archived BOOLEAN NOT NULL DEFAULT 0
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
