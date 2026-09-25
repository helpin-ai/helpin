package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCLIModelReplayAndReportedUsageIsolation(t *testing.T) {
	s, db, _, _ := setupCLIService(t)
	usage := EnableCLIConnectedFixture(t, s, db)
	c, _ := cliTestLogin(t, s)
	ctx := context.Background()
	a, err := s.Admit(ctx, c, model.CLIAdmissionRequest{RequestID: "model-test-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(ctx, c, a.RunID, 1, "local-test"); err != nil {
		t.Fatal(err)
	}
	req := model.CLIModelRequest{RequestID: "generation-123", Epoch: 1, LocalRunID: "local-test", Request: model.CLINativeRequest{Step: 3, Messages: []model.CLINativeMessage{{Role: "user", Content: "Finish"}}, Tools: []model.CLIToolDefinition{{Name: "read_files"}}}}
	first, err := s.Generate(ctx, c, a.RunID, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Generate(ctx, c, a.RunID, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Message.Content != second.Message.Content || usage() != 100 {
		t.Fatalf("replay duplicated call: %d", usage())
	}
	req.Request.Messages[0].Content = "Changed"
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatalf("different payload: %v", err)
	}
	report := model.CLIResultRequest{Epoch: 1, LocalRunID: "local-test", Status: "completed", OutputSummary: json.RawMessage(`{"usage":{"input_tokens":999999},"billing":"paid"}`)}
	if err = s.Report(ctx, c, a.RunID, report); err != nil {
		t.Fatal(err)
	}
	if err = s.Report(ctx, c, a.RunID, report); err != nil {
		t.Fatal(err)
	}
	if usage() != 100 {
		t.Fatal("client reports affected usage")
	}
	run, err := s.agents.runRepo.GetByID(ctx, c.WorkspaceID, a.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.InputTokens != 100 || run.OutputTokens != 20 {
		t.Fatalf("shared usage counters: %d/%d", run.InputTokens, run.OutputTokens)
	}
	if codingSessionCapabilitiesForRun(run).HumanInput {
		t.Fatal("local run advertises browser input")
	}
	var summary map[string]json.RawMessage
	if err = json.Unmarshal(run.OutputSummary, &summary); err != nil {
		t.Fatal(err)
	}
	if _, ok := summary["billing"]; ok {
		t.Fatal("client overwrote authoritative summary")
	}
}

func TestCLIUncertainGenerationCannotDispatchAgain(t *testing.T) {
	s, db, _, _ := setupCLIService(t)
	EnableCLIConnectedFixture(t, s, db)
	c, _ := cliTestLogin(t, s)
	ctx := context.Background()
	calls := 0
	s.generate = func(context.Context, *model.AgentRun, model.CLINativeRequest) (*model.CLINativeResponse, error) {
		calls++
		return nil, errors.New("connection lost after dispatch")
	}
	a, err := s.Admit(ctx, c, model.CLIAdmissionRequest{RequestID: "uncertain-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(ctx, c, a.RunID, 1, "local-test"); err != nil {
		t.Fatal(err)
	}
	req := model.CLIModelRequest{RequestID: "attempt-123", Epoch: 1, LocalRunID: "local-test"}
	if _, err = s.Generate(ctx, c, a.RunID, req); err == nil {
		t.Fatal("lost response ignored")
	}
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatalf("uncertain replay: %v", err)
	}
	req.RequestID = "different-123"
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatalf("concurrent generation: %v", err)
	}
	if calls != 1 {
		t.Fatalf("provider dispatched %d times", calls)
	}
}

func TestCLIGatewayRejectsExpiredForgedAndDisallowedRequests(t *testing.T) {
	s, db, authz, _ := setupCLIService(t)
	EnableCLIConnectedFixture(t, s, db)
	c, _ := cliTestLogin(t, s)
	ctx := context.Background()
	a, err := s.Admit(ctx, c, model.CLIAdmissionRequest{RequestID: "boundary-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(ctx, c, a.RunID, 1, "local-test"); err != nil {
		t.Fatal(err)
	}
	req := model.CLIModelRequest{RequestID: "attempt-123", Epoch: 1, LocalRunID: "forged"}
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("forged local identity")
	}
	req.LocalRunID = "local-test"
	req.Request.Tools = []model.CLIToolDefinition{{Name: "delete_workspace"}}
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIForbidden) {
		t.Fatal("expanded tool policy")
	}
	req.Request.Tools = nil
	authz.revoked = true
	if _, err = s.Generate(ctx, c, a.RunID, req); err == nil {
		t.Fatal("revoked membership")
	}
	authz.revoked = false
	if err = db.Model(&model.CLIExecution{}).Where("id = ?", a.Execution.ID).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Generate(ctx, c, a.RunID, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("expired grant")
	}
}

func TestCLIDisconnectSettlesUsageAndBusyReportIsRejected(t *testing.T) {
	s, db, _, _ := setupCLIService(t)
	usage := EnableCLIConnectedFixture(t, s, db)
	c, _ := cliTestLogin(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := s.Admit(ctx, c, model.CLIAdmissionRequest{RequestID: "disconnect-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(ctx, c, a.RunID, 1, "local-test"); err != nil {
		t.Fatal(err)
	}
	original := s.generate
	s.generate = func(callCtx context.Context, run *model.AgentRun, req model.CLINativeRequest) (*model.CLINativeResponse, error) {
		report := model.CLIResultRequest{Epoch: 1, LocalRunID: "local-test", Status: "completed"}
		if err := s.Report(callCtx, c, a.RunID, report); !errors.Is(err, ErrCLIConflict) {
			t.Fatalf("report while model active: %v", err)
		}
		duplicate := model.CLIModelRequest{RequestID: "disconnect-model-123", Epoch: 1, LocalRunID: "local-test", Request: req}
		if _, err := s.Generate(callCtx, c, a.RunID, duplicate); !errors.Is(err, ErrCLIConflict) {
			t.Fatalf("concurrent generation replay: %v", err)
		}
		cancel()
		if callCtx.Err() != nil {
			t.Fatal("client disconnect cancelled provider settlement context")
		}
		return original(callCtx, run, req)
	}
	req := model.CLIModelRequest{RequestID: "disconnect-model-123", Epoch: 1, LocalRunID: "local-test", Request: model.CLINativeRequest{Step: 3}}
	if _, err = s.Generate(ctx, c, a.RunID, req); err != nil {
		t.Fatal(err)
	}
	if usage() != 100 {
		t.Fatalf("disconnected generation usage = %d", usage())
	}
	if _, err = s.Generate(context.Background(), c, a.RunID, req); err != nil {
		t.Fatal(err)
	}
	if usage() != 100 {
		t.Fatal("replay charged usage twice")
	}
}
