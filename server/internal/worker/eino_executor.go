package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// EinoExecutor runs the native agent path through Eino-backed chat models and tools.
type EinoExecutor struct {
	kind         string
	modelFactory *EinoModelFactory
	tools        *ToolRegistry
	runRepo      *repository.AgentRunRepository
	artifactRepo *repository.AgentRunArtifactRepository
}

func NewEinoExecutor(
	kind string,
	modelFactory *EinoModelFactory,
	webSearch WebSearchClient,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *EinoExecutor {
	return &EinoExecutor{
		kind:         kind,
		modelFactory: modelFactory,
		tools:        NewToolRegistry(webSearch),
		runRepo:      runRepo,
		artifactRepo: artifactRepo,
	}
}

func (e *EinoExecutor) Kind() string {
	return e.kind
}

func (e *EinoExecutor) Execute(execCtx *ExecutionContext, run *model.AgentRun) error {
	config := execCtx.Config
	if config == nil {
		config = DefaultWorkflowConfig()
	}

	if len(execCtx.ResolvedProfile.Tools) == 0 {
		execCtx.ResolvedProfile = ResolveAgentProfile(execCtx.Agent)
	}
	if len(execCtx.AllowedTools) == 0 {
		execCtx.AllowedTools = allowedToolSet(execCtx.ResolvedProfile)
	}

	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Story, execCtx.Epic, execCtx.Conversation, execCtx.PlanningStage, execCtx.PlanningMethodology, config)

	var checklist []model.PMChecklistItem
	if execCtx.StoryID != "" && execCtx.Services != nil {
		var err error
		checklist, err = execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.StoryID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "native runtime checklist preload failed",
				"workspace_id", execCtx.WorkspaceID,
				"run_id", execCtx.RunID,
				"story_id", execCtx.StoryID,
				"error", err,
			)
		}
	}

	var ticketMessages []model.SupportMessage
	if execCtx.ConversationID != "" && execCtx.Services != nil && execCtx.Services.ListConversationMessages != nil {
		var err error
		ticketMessages, err = execCtx.Services.ListConversationMessages(execCtx.Context, execCtx.WorkspaceID, execCtx.ConversationID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "native runtime conversation preload failed",
				"workspace_id", execCtx.WorkspaceID,
				"run_id", execCtx.RunID,
				"conversation_id", execCtx.ConversationID,
				"error", err,
			)
		}
	}

	userPrompt := BuildUserPrompt(
		execCtx.Agent,
		execCtx.Story,
		execCtx.Epic,
		execCtx.EpicStories,
		execCtx.Conversation,
		ticketMessages,
		checklist,
		execCtx.ArtifactContext,
		execCtx.PlanningStage,
		execCtx.InitialInstructions,
	)

	history := append([]ExecutionMessage(nil), execCtx.ConversationHistory...)
	if supplement := BuildExecutionSupplementPrompt(run, execCtx.RunFacts, execCtx.ArtifactContext); supplement != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n## Current Run State\n" + supplement)
	}
	if len(history) == 0 {
		history = []ExecutionMessage{{
			Role:    "user",
			Content: userPrompt,
		}}
	}
	provider, modelName := resolveProviderAndModel(execCtx.Agent)
	toolDefs := e.tools.DefinitionsFor(execCtx.AllowedTools)
	slog.InfoContext(execCtx.Context, "native runtime execution starting",
		"workspace_id", execCtx.WorkspaceID,
		"run_id", execCtx.RunID,
		"agent_id", execCtx.AgentID,
		"provider", provider,
		"model", modelName,
		"runtime_kind", e.kind,
		"history_messages", len(history),
		"artifact_entries", lenArtifactEntries(execCtx.ArtifactContext),
		"tool_count", len(toolDefs),
		"continuation_present", execCtx.ProviderContinuation != nil && strings.TrimSpace(execCtx.ProviderContinuation.ResponseID) != "",
	)

	timeout := time.Duration(config.TimeoutMinutes) * time.Minute
	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()

	runCtx := *execCtx
	runCtx.Context = ctx
	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("native_sdk_starting")
	}
	done := make(chan struct{})
	var stopOnce sync.Once
	stopHeartbeat := func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
	defer stopHeartbeat()
	if execCtx.Heartbeat != nil {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = execCtx.Heartbeat("native_sdk_running")
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	result, execErr := ExecuteWithEino(ctx, e.modelFactory, execCtx.Agent, systemPrompt, history, toolDefs, &runCtx, e.tools, config.MaxIterations, func(event ExecutionEvent) {
		if execCtx.OnExecutionEvent != nil {
			execCtx.OnExecutionEvent(event)
		}
		if execCtx.Heartbeat == nil {
			return
		}
		switch event.Type {
		case "assistant_message_started":
			_ = execCtx.Heartbeat("assistant_started")
		case "tool_call_started":
			_ = execCtx.Heartbeat("tool_" + event.ToolName)
		}
	})
	stopHeartbeat()
	if execErr != nil && !errors.Is(execErr, ErrMaxToolStepsReached) {
		slog.ErrorContext(execCtx.Context, "native runtime execution failed",
			"workspace_id", execCtx.WorkspaceID,
			"run_id", execCtx.RunID,
			"agent_id", execCtx.AgentID,
			"provider", provider,
			"model", modelName,
			"error", execErr,
		)
		return fmt.Errorf("eino execution: %w", execErr)
	}

	totalTokens := result.Usage.InputTokens + result.Usage.OutputTokens
	execCtx.LastExecutionResult = result
	slog.InfoContext(execCtx.Context, "native runtime execution completed",
		"workspace_id", execCtx.WorkspaceID,
		"run_id", execCtx.RunID,
		"agent_id", execCtx.AgentID,
		"provider", provider,
		"model", modelName,
		"tool_round_results", len(result.ToolInvocations),
		"assistant_blocks", len(result.AssistantBlocks),
		"input_tokens", result.Usage.InputTokens,
		"output_tokens", result.Usage.OutputTokens,
		"max_steps_reached", result.MaxStepsReached,
		"continuation_present", result.ProviderContinuation != nil && strings.TrimSpace(result.ProviderContinuation.ResponseID) != "",
	)
	if execCtx.Agent.MonthlyTokenBudget != nil {
		budget := *execCtx.Agent.MonthlyTokenBudget
		if execCtx.Agent.TokensUsedThisMonth+totalTokens > budget {
			return fmt.Errorf("token budget exceeded (%d/%d)", execCtx.Agent.TokensUsedThisMonth+totalTokens, budget)
		}
	}
	currentRun, err := e.runRepo.GetByID(ctx, run.WorkspaceID, run.ID)
	if err == nil && currentRun != nil && currentRun.Status == "cancelled" {
		return ErrRunCancelled
	}

	run.TokensUsed = totalTokens
	_ = e.runRepo.Update(ctx, run)

	seqNo := 0
	if strings.TrimSpace(result.AssistantText) != "" {
		seqNo++
		e.saveArtifact(ctx, run, "agent_summary", "markdown", result.AssistantText, seqNo)
	}
	for _, invocation := range result.ToolInvocations {
		seqNo++
		logContent := fmt.Sprintf("Tool: %s\nInput: %s\nResult: %s", invocation.ToolName, string(invocation.Input), truncate(invocation.OutputSummary, 5000))
		e.saveArtifact(ctx, run, "tool_log", "text", logContent, seqNo)
		if invocation.ToolName == "run_command" && looksLikeTestCommand(invocation.Input) {
			seqNo++
			e.saveArtifact(ctx, run, "test_report", "text", truncate(invocation.OutputSummary, 50000), seqNo)
		}
	}
	convLog, _ := json.MarshalIndent(result.Messages, "", "  ")
	seqNo++
	e.saveArtifact(ctx, run, "conversation_log", "json", string(convLog), seqNo)
	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("native_sdk_finished")
	}

	if execCtx.PendingSupportDraft != nil {
		summary, _ := json.Marshal(map[string]any{
			"draft_reply": execCtx.PendingSupportDraft,
		})
		run.OutputSummary = json.RawMessage(summary)
		_ = e.runRepo.Update(ctx, run)
	}

	if flowOutputKind := flowOutputKindFromRunInput(run.Input); flowOutputKind != "" {
		switch flowOutputKind {
		case "pm.story_completion_followups":
			assessment, err := extractStoryCompletionAssessmentFromResponseText(result.AssistantText)
			if err != nil {
				fallback := &model.StoryCompletionAssessment{
					Summary: strings.TrimSpace(result.AssistantText),
				}
				seqNo++
				e.saveArtifact(ctx, run, "story_completion_assessment_raw", "text", truncate(result.AssistantText, 50000), seqNo)
				seqNo++
				e.saveArtifact(ctx, run, "story_completion_assessment_parse_error", "text", err.Error(), seqNo)
				assessment = fallback
			}
			payload, _ := json.Marshal(assessment)
			run.OutputSummary = payload
			_ = e.runRepo.Update(ctx, run)
			seqNo++
			e.saveArtifact(ctx, run, "story_completion_assessment", "json", string(payload), seqNo)
		case "crm.deal_review_actions":
			plan, err := extractCRMDealReviewActionPlanFromResponseText(result.AssistantText)
			if err != nil {
				fallback := &model.CRMDealReviewActionPlan{
					Summary: strings.TrimSpace(result.AssistantText),
				}
				seqNo++
				e.saveArtifact(ctx, run, "crm_deal_review_plan_raw", "text", truncate(result.AssistantText, 50000), seqNo)
				seqNo++
				e.saveArtifact(ctx, run, "crm_deal_review_plan_parse_error", "text", err.Error(), seqNo)
				plan = fallback
			}
			payload, _ := json.Marshal(plan)
			run.OutputSummary = payload
			_ = e.runRepo.Update(ctx, run)
			seqNo++
			e.saveArtifact(ctx, run, "crm_deal_review_plan", "json", string(payload), seqNo)
		}
	}

	if execCtx.TargetType == "epic" && execCtx.Epic != nil {
		switch execCtx.PlanningStage {
		case model.PlanningStageDraftSpec:
			draft, err := extractProductSpecDraftFromResponseText(result.AssistantText)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(draft)
			run.OutputSummary = payload
			_ = e.runRepo.Update(ctx, run)

			seqNo++
			e.saveArtifact(ctx, run, "product_spec_draft", "json", string(payload), seqNo)
		case model.PlanningStagePlanStories:
			proposal, err := extractPlanningProposalFromResponseText(result.AssistantText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, totalTokens)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			_ = e.runRepo.Update(ctx, run)

			seqNo++
			e.saveArtifact(ctx, run, "story_plan_proposal", "json", string(payload), seqNo)
			seqNo++
			e.saveArtifact(ctx, run, "orchestration_proposal", "json", string(payload), seqNo)
		case "":
			// Direct epic planner runs manage phase state in the Temporal activity layer.
		default:
			proposal, err := extractPlanningProposalFromResponseText(result.AssistantText, execCtx.Epic.ID, "", totalTokens)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			_ = e.runRepo.Update(ctx, run)

			seqNo++
			e.saveArtifact(ctx, run, "orchestration_proposal", "json", string(payload), seqNo)
		}
	}

	if execCtx.LatestPRMetadata != nil {
		prPayload, _ := json.MarshalIndent(execCtx.LatestPRMetadata, "", "  ")
		seqNo++
		e.saveArtifact(ctx, run, "pr_metadata", "json", string(prPayload), seqNo)
	}

	if execCtx.WorkDir != "" {
		diff, err := runGit(execCtx, "diff")
		if err == nil && diff != "" {
			seqNo++
			e.saveArtifact(ctx, run, "diff", "patch", diff, seqNo)
		}

		files, err := runGit(execCtx, "diff", "--name-only")
		if err == nil && strings.TrimSpace(files) != "" {
			seqNo++
			e.saveArtifact(ctx, run, "file_bundle", "json", toJSONString(strings.Fields(files)), seqNo)
		}
	}

	if errors.Is(execErr, ErrMaxToolStepsReached) {
		return ErrMaxToolStepsReached
	}
	return nil
}

func (e *EinoExecutor) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) {
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	if err := e.artifactRepo.Create(ctx, artifact); err != nil {
		slog.WarnContext(ctx, "native runtime artifact save failed",
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"artifact_type", artifactType,
			"sequence_no", seqNo,
			"error", err,
		)
	}
}

func lenArtifactEntries(ctx *ArtifactContext) int {
	if ctx == nil {
		return 0
	}
	return len(ctx.Entries)
}
