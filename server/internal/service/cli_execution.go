package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *CLIService) boundExecution(ctx context.Context, c *model.CLIConnection, runID string, epoch int64, localID string, terminal bool) (*model.CLIExecution, *model.AgentRun, error) {
	if !s.config.GatewayEnabled {
		return nil, nil, ErrCLIDisabled
	}
	e, err := s.execution(ctx, c, runID, terminal)
	if err != nil {
		return nil, nil, err
	}
	if e.RevokedAt != nil || e.Epoch != epoch || localID == "" || e.LocalRunID != localID {
		return nil, nil, ErrCLIConflict
	}
	run, err := s.agents.runRepo.GetByID(ctx, c.WorkspaceID, runID)
	if err != nil {
		return nil, nil, err
	}
	if model.IsAgentRunActiveStatus(run.Status) && !e.LeaseExpiresAt.After(time.Now()) {
		return nil, nil, ErrCLIConflict
	}
	return e, run, nil
}

// Generate checks current user/grant policy and journals one provider call.
func (s *CLIService) Generate(ctx context.Context, c *model.CLIConnection, runID string, req model.CLIModelRequest) (*model.CLINativeResponse, error) {
	e, run, err := s.boundExecution(ctx, c, runID, req.Epoch, req.LocalRunID, false)
	if err != nil {
		return nil, err
	}
	if len(req.RequestID) < 8 || len(req.RequestID) > 128 || len(req.Request.Messages) > 1000 || len(req.Request.Tools) > 32 {
		return nil, ErrCLIInvalid
	}
	var admission model.CLIAdmission
	if err = json.Unmarshal(e.Snapshot, &admission); err != nil {
		return nil, err
	}
	current, err := s.agents.GetAgent(ctx, c.WorkspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	_, currentTools, err := localAgentPolicy(current, false)
	if err != nil {
		return nil, err
	}
	for _, tool := range req.Request.Tools {
		if !slices.Contains(admission.AllowedTools, tool.Name) || !slices.Contains(currentTools, tool.Name) {
			return nil, ErrCLIForbidden
		}
	}
	req.Request.SystemPrompt = admission.Agent.SystemPrompt + "\n" + admission.Context
	payload, err := json.Marshal(req.Request)
	if err != nil {
		return nil, err
	}
	id := cliRequestKey(e.ID, req.RequestID)
	g, err := s.repo.Generation(ctx, id)
	if err != nil {
		return nil, err
	}
	if g != nil && g.RequestHash != mcpHashBytes(payload) {
		return nil, ErrCLIConflict
	}
	if g != nil {
		if e.BusyID != "" || len(g.Response) == 0 || string(g.Response) == "{}" {
			return nil, ErrCLIConflict
		}
		var cached model.CLINativeResponse
		if err := json.Unmarshal(g.Response, &cached); err != nil {
			return nil, err
		}
		return &cached, nil
	}
	if g == nil {
		g = &model.CLIGeneration{ID: id, ExecutionID: e.ID, RequestHash: mcpHashBytes(payload), CreatedAt: time.Now().UTC()}
		if s.agents.aiUsageMeter != nil && s.agents.aiUsageMeter.usage != nil {
			meter, ok := agentRunMeteringContext(run)
			if !ok {
				return nil, model.ErrPricingConfigurationMissing
			}
			prior, err := s.repo.GenerationUsage(ctx, e.ID)
			if err != nil {
				return nil, err
			}
			estimate := cliUsagePayload(prior)
			estimate.InputTokens += len(payload) + 1024
			estimate.OutputTokens += 8192
			exceeded, err := s.agents.aiUsageMeter.agentRunUsageExceedsBudget(run, estimate)
			if err != nil {
				return nil, err
			}
			if exceeded {
				return nil, model.ErrAIUsageExhausted
			}
			if err = s.agents.aiUsageMeter.usage.Heartbeat(ctx, meter); err != nil {
				return nil, err
			}
		}
		started, err := s.repo.BeginGeneration(ctx, e, g)
		if err != nil {
			return nil, err
		}
		if !started {
			return nil, ErrCLIConflict
		}
		// Keep the attempt fenced on ambiguous provider failure. Never auto-dispatch it again.
		callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 100*time.Second)
		result, callErr := s.generate(callCtx, run, req.Request)
		cancel()
		if callErr != nil {
			return nil, callErr
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		g.Response = model.JSONBlob(raw)
		g.InputTokens = result.Usage.InputTokens
		g.OutputTokens = result.Usage.OutputTokens
		g.CachedInputTokens = result.Usage.CachedInputTokens
		g.ReasoningOutputTokens = result.Usage.ReasoningOutputTokens
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		err = s.repo.SaveGeneration(persistCtx, g)
		cancel()
		if err != nil {
			return nil, err
		}
	}
	settlementCtx, stopSettlement := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer stopSettlement()
	ctx = settlementCtx
	if len(g.Response) == 0 || string(g.Response) == "{}" {
		return nil, ErrCLIConflict
	}
	run, err = s.agents.runRepo.GetByID(ctx, c.WorkspaceID, runID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.GenerationUsage(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	if s.agents.aiUsageMeter != nil {
		if model.IsAgentRunActiveStatus(run.Status) {
			err = s.agents.aiUsageMeter.checkpointAgentRun(ctx, run, cliUsagePayload(usage))
		} else {
			err = s.agents.aiUsageMeter.reconcileAgentRun(ctx, run, cliUsagePayload(usage))
		}
		if err != nil {
			return nil, err
		}
	}
	if err = s.repo.SaveProviderUsage(ctx, run, usage); err != nil {
		return nil, err
	}
	if err = s.repo.FinishGeneration(ctx, e.ID, g.ID); err != nil {
		return nil, err
	}
	// A cancellation or revocation during the provider call prevents further execution.
	if _, _, err = s.boundExecution(ctx, c, runID, req.Epoch, req.LocalRunID, false); err != nil {
		return nil, err
	}
	var response model.CLINativeResponse
	err = json.Unmarshal(g.Response, &response)
	return &response, err
}
func cliUsagePayload(u model.CLIUsage) agentRuntimeUsagePayload {
	return agentRuntimeUsagePayload{InputTokens: int(u.InputTokens), OutputTokens: int(u.OutputTokens), CachedInputTokens: int(u.CachedInputTokens), ReasoningOutputTokens: int(u.ReasoningOutputTokens), TotalTokens: int(u.InputTokens + u.OutputTokens)}
}

// Report stores local provenance without trusting billing totals or cloud events.
func (s *CLIService) Report(ctx context.Context, c *model.CLIConnection, runID string, req model.CLIResultRequest) error {
	e, run, err := s.boundExecution(ctx, c, runID, req.Epoch, req.LocalRunID, true)
	if err != nil {
		return err
	}
	if e.BusyID != "" {
		return ErrCLIConflict
	}
	if !slices.Contains([]string{"running", "paused", "completed", "failed", "cancelled"}, req.Status) || len(req.Messages) > 2000 || len(req.Events) > 10000 {
		return ErrCLIInvalid
	}
	if !model.IsAgentRunActiveStatus(run.Status) && run.Status != req.Status {
		return ErrCLIConflict
	}
	for _, m := range req.Messages {
		if m.ID == "" || len(m.ID) > 128 || !slices.Contains([]string{"user", "assistant", "tool", "system"}, m.Role) {
			return ErrCLIInvalid
		}
	}
	op := cliRequestKey(e.ID, "report")
	locked, err := s.repo.LockExecution(ctx, e, op)
	if err != nil {
		return err
	}
	if !locked {
		return ErrCLIConflict
	}
	e.BusyID = op
	defer func() {
		if err := s.repo.FinishGeneration(context.WithoutCancel(ctx), e.ID, op); err != nil {
			slog.ErrorContext(ctx, "release CLI report fence", "error", err)
		}
	}()
	usage, err := s.repo.GenerationUsage(ctx, e.ID)
	if err != nil {
		return err
	}
	if s.agents.aiUsageMeter != nil {
		if !model.IsAgentRunActiveStatus(req.Status) {
			err = s.agents.aiUsageMeter.reconcileAgentRun(ctx, run, cliUsagePayload(usage))
		} else {
			err = s.agents.aiUsageMeter.checkpointAgentRun(ctx, run, cliUsagePayload(usage))
		}
		if err != nil {
			return err
		}
	}
	var summary map[string]any
	if len(run.OutputSummary) > 0 {
		if err = json.Unmarshal(run.OutputSummary, &summary); err != nil {
			return err
		}
	}
	if summary == nil {
		summary = map[string]any{}
	}
	summary["execution_location"] = "local"
	summary["local_report"] = map[string]any{"reported_by": c.UserID, "local_run_id": req.LocalRunID, "epoch": req.Epoch, "output": req.OutputSummary}
	summary["cli_provider_usage"] = usage
	encoded, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	run.OutputSummary = encoded
	run.Status = req.Status
	if run.Status == "paused" {
		run.PauseReason = model.AgentRunPauseReasonHumanInput
	} else {
		run.PauseReason = model.AgentRunPauseReasonNone
	}
	if !model.IsAgentRunActiveStatus(run.Status) {
		now := time.Now().UTC()
		run.CompletedAt = &now
	}
	messages := make([]model.AgentRunMessage, 0, len(req.Messages))
	for i, m := range req.Messages {
		if m.ID == "" || len(m.ID) > 128 || !slices.Contains([]string{"user", "assistant", "tool", "system"}, m.Role) {
			return ErrCLIInvalid
		}
		messages = append(messages, model.AgentRunMessage{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.ID+"message"+m.ID)).String(), WorkspaceID: c.WorkspaceID, RunID: runID, Role: m.Role, Content: m.Content, ContentBlocks: m.ContentBlocks, ToolInvocations: m.ToolInvocations, MessageType: "message", DeliveryStatus: "sent", SequenceNo: i + 100, CreatedAt: time.Now().UTC()})
	}
	eventData, err := json.Marshal(map[string]any{"provenance": "local_report", "epoch": req.Epoch, "events": req.Events})
	if err != nil {
		return err
	}
	content := string(eventData)
	artifact := &model.AgentRunArtifact{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.ID+"events"+mcpHashBytes(eventData))).String(), WorkspaceID: c.WorkspaceID, RunID: runID, ArtifactType: "cli_events", Format: "json", StorageMode: "inline", InlineContent: &content, Metadata: json.RawMessage(`{"provenance":"local_report"}`), CreatedAt: time.Now().UTC()}
	if err = s.repo.SaveLocalResults(ctx, e, run, messages, artifact); err != nil {
		return err
	}
	s.agents.publishRunEvent(run, c.UserID)
	return nil
}

// Artifact persists private, bounded text evidence with explicit local provenance.
func (s *CLIService) Artifact(ctx context.Context, c *model.CLIConnection, runID string, req model.CLIArtifactRequest) (string, error) {
	e, _, err := s.boundExecution(ctx, c, runID, req.Epoch, req.LocalRunID, true)
	if err != nil {
		return "", err
	}
	if len(req.RequestID) < 8 || len(req.RequestID) > 128 || len(req.Content) > 256<<10 || !slices.Contains([]string{"patch", "test_log", "report"}, req.Kind) {
		return "", ErrCLIInvalid
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.ID+req.RequestID+mcpHash(req.Content))).String()
	metadata, err := json.Marshal(map[string]any{"provenance": "local_report", "reported_by": c.UserID, "kind": req.Kind})
	if err != nil {
		return "", err
	}
	a := &model.AgentRunArtifact{ID: id, WorkspaceID: c.WorkspaceID, RunID: runID, ArtifactType: "cli_" + req.Kind, Format: "text", StorageMode: "inline", InlineContent: &req.Content, Metadata: metadata, CreatedAt: time.Now().UTC()}
	if err = s.repo.SaveLocalArtifact(ctx, a); err != nil {
		return "", fmt.Errorf("save local artifact: %w", err)
	}
	return id, nil
}
