package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	tclient "go.temporal.io/sdk/client"
	"testing"
	"time"
)

type reanalysisClient struct {
	tclient.Client
	options tclient.StartWorkflowOptions
	input   temporalapp.CoverageWorkspaceAnalysisInput
	err     error
}

func (c *reanalysisClient) ExecuteWorkflow(ctx context.Context, options tclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (tclient.WorkflowRun, error) {
	c.options = options
	c.input = args[0].(temporalapp.CoverageWorkspaceAnalysisInput)
	return nil, c.err
}
func TestCoverageReanalysisExplicitlyReassessesLastThirtyDays(t *testing.T) {
	client := &reanalysisClient{}
	svc := NewSupportCoverageService(nil)
	svc.SetTemporalClient(client)
	if err := svc.TriggerReanalysis(context.Background(), "ws-1"); err != nil {
		t.Fatal(err)
	}
	if !client.input.Reanalyze || client.input.RequestID == "" {
		t.Fatalf("not a reassessment: %+v", client.input)
	}
	if client.input.WindowEnd.Sub(client.input.WindowStart) != 30*24*time.Hour {
		t.Fatal("unexpected window")
	}
	if client.options.ID != "coverage-reanalysis-ws-1" {
		t.Fatal("missing workspace duplicate fence")
	}
	client.err = &serviceerror.WorkflowExecutionAlreadyStarted{}
	if err := svc.TriggerReanalysis(context.Background(), "ws-1"); err != ErrReanalysisAlreadyRunning {
		t.Fatalf("duplicate: %v", err)
	}
}

type reanalysisStatusClient struct {
	tclient.Client
	status             enums.WorkflowExecutionStatus
	activityState      enums.PendingActivityState
	err                error
	describedWorkspace string
}

func (c *reanalysisStatusClient) DescribeWorkflowExecution(ctx context.Context, id, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	c.describedWorkspace = id
	return &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: c.status}, PendingActivities: []*workflowpb.PendingActivityInfo{{State: c.activityState}}}, c.err
}
func TestCoverageReanalysisStatusSurvivesReloadAndIsWorkspaceScoped(t *testing.T) {
	client := &reanalysisStatusClient{status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING}
	svc := NewSupportCoverageV2Service(nil).SetTemporalClient(client)
	status, err := svc.reanalysisStatus(context.Background(), "ws-1")
	if err != nil || status != "queued" || client.describedWorkspace != "coverage-reanalysis-ws-1" {
		t.Fatalf("status %q, err %v", status, err)
	}
	client.activityState = enums.PENDING_ACTIVITY_STATE_STARTED
	status, err = svc.reanalysisStatus(context.Background(), "ws-1")
	if err != nil || status != "running" {
		t.Fatalf("running status %q, err %v", status, err)
	}
	client.status = enums.WORKFLOW_EXECUTION_STATUS_COMPLETED
	status, err = svc.reanalysisStatus(context.Background(), "ws-1")
	if err != nil || status != "" {
		t.Fatalf("completed status %q, err %v", status, err)
	}
	client.err = &serviceerror.NotFound{}
	status, err = svc.reanalysisStatus(context.Background(), "ws-2")
	if err != nil || status != "" {
		t.Fatalf("first run %q, err %v", status, err)
	}
}

func TestCoverageReanalysisIgnoresIncrementalCursor(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	for _, sql := range []string{
		`CREATE TABLE support_coverage_analysis_runs (id TEXT PRIMARY KEY,workspace_id TEXT,window_start DATETIME,window_end DATETIME,cursor_started_at DATETIME,cursor_ended_at DATETIME,analyzer_version TEXT,status TEXT,conversation_cnt INTEGER,gap_count INTEGER,error_message TEXT,metadata TEXT,started_at DATETIME,completed_at DATETIME,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY,workspace_id TEXT,subject TEXT,status TEXT,updated_at DATETIME,resolved_at DATETIME)`,
		`CREATE TABLE sample_data_items (entity_id TEXT)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-20 * 24 * time.Hour)
	if err := db.Exec(`INSERT INTO support_conversations VALUES ('c','ws-1','Billing help','open',?,NULL)`, old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,message_type,content,created_at) VALUES ('m','ws-1','c','customer','reply','How do I update billing details?',?)`, old).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewSupportCoverageAnalysisRepository(db)
	run, err := repo.CreateRun(ctx, &model.SupportCoverageAnalysisRun{WorkspaceID: "ws-1", WindowStart: now.Add(-4 * time.Hour), WindowEnd: now.Add(-time.Hour), CursorStartedAt: now.Add(-4 * time.Hour), CursorEndedAt: now.Add(-time.Hour), AnalyzerVersion: coverageAnalyzerVersion, Status: "running", StartedAt: now.Add(-time.Hour), Metadata: []byte("{}")})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteRun(ctx, run.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	response := llm.ChatResponse{Content: `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"support question","has_gap":false,"gap_kind":"","gap_category":"","canonical_title":"","customer_need":"Billing help","ai_failure":"","human_resolution":"","decision_reason":"Covered by current knowledge","search_query":"","should_run_retrieval":false,"recommended_fixes":[],"confidence":0.9}`}
	provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{response}}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5").SetCoverageRepositories(repository.NewSupportCoverageRepository(db), repo).SetConversationRepositories(repository.NewSupportConversationRepository(db), repository.NewSupportMessageRepository(db))
	if err := analyzer.RunWorkspaceReanalysis(ctx, "ws-1", now.Add(-30*24*time.Hour), now, "request"); err != nil {
		t.Fatal(err)
	}
	var analyses []model.SupportCoverageConversationAnalysis
	if err := db.Find(&analyses).Error; err != nil {
		t.Fatal(err)
	}
	if len(analyses) != 1 || analyses[0].ConversationID != "c" {
		t.Fatalf("old conversation not reassessed: %+v", analyses)
	}
}
