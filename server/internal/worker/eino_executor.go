package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
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

type nativeToolPressureSummary struct {
	ToolCalls          int
	OutputChars        int
	OutputLines        int
	ModelVisibleChars  int
	ModelVisibleLines  int
	CompactedResults   int
	TopToolsByPressure []string
}

func NewEinoExecutor(
	kind string,
	modelFactory *EinoModelFactory,
	webSearch WebSearchClient,
	exaSearch *ExaSearchClient,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *EinoExecutor {
	return &EinoExecutor{
		kind:         kind,
		modelFactory: modelFactory,
		tools:        NewToolRegistry(webSearch, exaSearch),
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
		config = DefaultWorkflowConfigForAgent(execCtx.Agent)
	}

	if len(execCtx.ResolvedProfile.Tools) == 0 {
		execCtx.ResolvedProfile = ResolveAgentProfile(execCtx.Agent)
	}
	if len(execCtx.AllowedTools) == 0 {
		execCtx.AllowedTools = allowedToolSet(execCtx.ResolvedProfile)
	}

	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Task, execCtx.Epic, execCtx.Conversation, execCtx.PlanningStage, execCtx.PlanningMethodology, config)

	var checklist []model.PMChecklistItem
	if execCtx.TaskID != "" && execCtx.Services != nil {
		var err error
		checklist, err = execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.TaskID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "native runtime checklist preload failed",
				"workspace_id", execCtx.WorkspaceID,
				"run_id", execCtx.RunID,
				"task_id", execCtx.TaskID,
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
		execCtx.Task,
		execCtx.Epic,
		execCtx.EpicTasks,
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
		"preset_key", strings.TrimSpace(execCtx.Agent.EffectivePresetKey()),
		"target_type", strings.TrimSpace(execCtx.TargetType),
		"native_selective_path_enabled", execCtx.NativeSelectivePathEnabled,
		"runtime_skill_ref_count", len(execCtx.RuntimeSkillRefs),
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
	run.CachedInputTokens = result.Usage.CachedInputTokens
	run.InputTokens = result.Usage.InputTokens
	run.OutputTokens = result.Usage.OutputTokens
	toolPressure := summarizeNativeToolPressure(result.Messages[len(history):])
	slog.InfoContext(execCtx.Context, "native runtime execution completed",
		"workspace_id", execCtx.WorkspaceID,
		"run_id", execCtx.RunID,
		"agent_id", execCtx.AgentID,
		"provider", provider,
		"model", modelName,
		"tool_round_results", len(result.ToolInvocations),
		"assistant_blocks", len(result.AssistantBlocks),
		"cached_input_tokens", result.Usage.CachedInputTokens,
		"input_tokens", result.Usage.InputTokens,
		"output_tokens", result.Usage.OutputTokens,
		"max_steps_reached", result.MaxStepsReached,
		"continuation_present", result.ProviderContinuation != nil && strings.TrimSpace(result.ProviderContinuation.ResponseID) != "",
	)
	if toolPressure.ToolCalls > 0 {
		slog.InfoContext(execCtx.Context, "native runtime tool pressure summary",
			"workspace_id", execCtx.WorkspaceID,
			"run_id", execCtx.RunID,
			"agent_id", execCtx.AgentID,
			"provider", provider,
			"model", modelName,
			"tool_calls", toolPressure.ToolCalls,
			"tool_output_chars", toolPressure.OutputChars,
			"tool_output_lines", toolPressure.OutputLines,
			"model_visible_chars", toolPressure.ModelVisibleChars,
			"model_visible_lines", toolPressure.ModelVisibleLines,
			"compacted_results", toolPressure.CompactedResults,
			"top_tools", toolPressure.TopToolsByPressure,
		)
	}
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
			assessment, err := extractTaskCompletionAssessmentFromResponseText(result.AssistantText)
			if err != nil {
				fallback := &model.TaskCompletionAssessment{
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
		case model.PlanningStagePlanTasks:
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
		return fmt.Errorf("%w after %d tool-call rounds; start another run to continue", ErrMaxToolStepsReached, config.MaxIterations)
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

func summarizeNativeToolPressure(messages []ExecutionMessage) nativeToolPressureSummary {
	if len(messages) == 0 {
		return nativeToolPressureSummary{}
	}

	type perTool struct {
		OutputChars       int
		ModelVisibleChars int
		CompactedResults  int
	}

	summary := nativeToolPressureSummary{}
	byTool := make(map[string]perTool)

	for _, msg := range messages {
		if msg.Role != "tool" {
			continue
		}
		for _, block := range msg.Blocks {
			if block.Type != ExecutionBlockTypeToolResult {
				continue
			}
			toolName := strings.TrimSpace(block.ToolName)
			analysis := analyzeToolOutputForModel(toolName, block.Output)
			summary.ToolCalls++
			summary.OutputChars += analysis.OriginalRunes
			summary.OutputLines += analysis.OriginalLines
			summary.ModelVisibleChars += analysis.VisibleRunes
			summary.ModelVisibleLines += analysis.VisibleLines
			if analysis.Compacted {
				summary.CompactedResults++
			}

			current := byTool[toolName]
			current.OutputChars += analysis.OriginalRunes
			current.ModelVisibleChars += analysis.VisibleRunes
			if analysis.Compacted {
				current.CompactedResults++
			}
			byTool[toolName] = current
		}
	}

	type namedTool struct {
		Name             string
		OutputChars      int
		ModelVisible     int
		CompactedResults int
	}
	named := make([]namedTool, 0, len(byTool))
	for name, stats := range byTool {
		named = append(named, namedTool{
			Name:             name,
			OutputChars:      stats.OutputChars,
			ModelVisible:     stats.ModelVisibleChars,
			CompactedResults: stats.CompactedResults,
		})
	}
	sort.Slice(named, func(i, j int) bool {
		if named[i].OutputChars == named[j].OutputChars {
			return named[i].Name < named[j].Name
		}
		return named[i].OutputChars > named[j].OutputChars
	})

	limit := 3
	if len(named) < limit {
		limit = len(named)
	}
	summary.TopToolsByPressure = make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		summary.TopToolsByPressure = append(summary.TopToolsByPressure, fmt.Sprintf("%s output=%d visible=%d compacted=%d", named[i].Name, named[i].OutputChars, named[i].ModelVisible, named[i].CompactedResults))
	}

	return summary
}
