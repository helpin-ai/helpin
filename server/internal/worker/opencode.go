package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	openCodeChunkFlushInterval    = 2 * time.Second
	openCodeChunkFlushBytes       = 4 * 1024
	openCodePostRunTimeout        = 2 * time.Minute
	openCodeRepoChangeWaitTimeout = 60 * time.Second
	openCodeRepoChangePollEvery   = 2 * time.Second
	openCodeScannerBufferSize     = 1024 * 1024
	openCodeGracefulShutdownDelay = 10 * time.Second
)

// OpenCodeExecutor shells out to the opencode CLI for each agent run.
type OpenCodeExecutor struct {
	kind              string
	commandPath       string
	anthropicAPIKey   string
	anthropicBaseURL  string
	openAIAPIKey      string
	openAIBaseURL     string
	openRouterAPIKey  string
	openRouterBaseURL string
	runRepo           *repository.AgentRunRepository
	artifactRepo      *repository.AgentRunArtifactRepository
}

// NewOpenCodeExecutor constructs an OpenCodeExecutor with the given dependencies.
func NewOpenCodeExecutor(
	kind string,
	commandPath string,
	anthropicAPIKey string,
	anthropicBaseURL string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterBaseURL string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *OpenCodeExecutor {
	if strings.TrimSpace(commandPath) == "" {
		commandPath = "opencode"
	}
	return &OpenCodeExecutor{
		kind:              kind,
		commandPath:       commandPath,
		anthropicAPIKey:   strings.TrimSpace(anthropicAPIKey),
		anthropicBaseURL:  strings.TrimSpace(anthropicBaseURL),
		openAIAPIKey:      strings.TrimSpace(openAIAPIKey),
		openAIBaseURL:     strings.TrimSpace(openAIBaseURL),
		openRouterAPIKey:  strings.TrimSpace(openRouterAPIKey),
		openRouterBaseURL: strings.TrimSpace(openRouterBaseURL),
		runRepo:           runRepo,
		artifactRepo:      artifactRepo,
	}
}

func (e *OpenCodeExecutor) Kind() string {
	return e.kind
}

// Execute runs the opencode CLI as a subprocess and collects its output.
func (e *OpenCodeExecutor) Execute(execCtx *ExecutionContext, run *model.AgentRun) error {
	config := execCtx.Config
	if config == nil {
		config = DefaultWorkflowConfigForAgent(execCtx.Agent)
	}

	var checklist []model.PMChecklistItem
	if execCtx.TaskID != "" && execCtx.Services != nil && execCtx.Services.ListChecklist != nil {
		items, err := execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.TaskID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "failed to list checklist for opencode run",
				"error", err, "task_id", execCtx.TaskID)
		} else {
			checklist = items
		}
	}

	var ticketMessages []model.SupportMessage
	if execCtx.ConversationID != "" && execCtx.Services != nil && execCtx.Services.ListConversationMessages != nil {
		messages, err := execCtx.Services.ListConversationMessages(execCtx.Context, execCtx.WorkspaceID, execCtx.ConversationID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "failed to list ticket messages for opencode run",
				"error", err, "conversation_id", execCtx.ConversationID)
		} else {
			ticketMessages = messages
		}
	}

	includeInlineSkills := strings.TrimSpace(execCtx.StagedRuntimeSkillRoot) == ""
	systemPrompt := BuildRuntimeSystemPrompt(
		execCtx.Agent,
		execCtx.Task,
		execCtx.Epic,
		execCtx.Conversation,
		execCtx.PlanningStage,
		execCtx.PlanningMethodology,
		config,
		includeInlineSkills,
		includeInlineSkills,
	)
	if supplement := BuildRuntimeExecutionSupplementPrompt(run, execCtx.RunFacts); supplement != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n## Current Run State\n" + supplement)
	}
	if runtimeInstructions := buildOpenCodeRuntimeInstructions(execCtx, run); runtimeInstructions != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n## OpenCode Runtime Instructions\n" + runtimeInstructions)
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
	if execCtx.Conversation != nil {
		systemPrompt += "\nFor support conversations, respond with valid JSON only in this shape: " +
			`{"status":"open|waiting_on_customer|resolved|spam","draft_reply":{"content":"...","is_internal":false,"sender_display_name":"optional","approval_required":true}}.`
	}

	userPrompt = buildOpenCodeUserPrompt(execCtx, run, userPrompt)
	modelID := e.resolveModelID(execCtx.Agent)
	providerConfig := buildOpenCodeProviderConfig(execCtx.Agent, e.anthropicBaseURL, e.openRouterBaseURL)
	configContent, err := buildOpenCodeConfigContent(execCtx, modelID, systemPrompt, providerConfig)
	if err != nil {
		return fmt.Errorf("build opencode config: %w", err)
	}

	args := []string{"run"}
	if agentName := openCodeAgentName(execCtx); agentName != "" {
		args = append(args, "--agent", agentName)
	}
	if modelID != "" {
		args = append(args, "--model", modelID)
	}
	args = append(args, "--format", "json")
	args = append(args, userPrompt)

	cmd := exec.CommandContext(execCtx.Context, e.commandPath, args...)
	cmd.Dir = execCtx.WorkDir
	cmd.Env, err = e.buildEnv(execCtx, configContent)
	if err != nil {
		return fmt.Errorf("build opencode env: %w", err)
	}

	// Run the subprocess in its own process group so we can signal the entire
	// tree on cancellation instead of only the main PID.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = openCodeGracefulShutdownDelay

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create opencode stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create opencode stderr pipe: %w", err)
	}

	artifactWriter := newOpenCodeArtifactWriter(e, run)
	artifactWriter.Save(execCtx.Context, "opencode_config", "json", configContent, true)
	artifactWriter.Save(execCtx.Context, "opencode_prompt", "markdown", buildOpenCodePromptArtifact(systemPrompt, userPrompt), false)

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("opencode_starting")
	}

	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("opencode executable %q was not found on PATH", e.commandPath)
		}
		return fmt.Errorf("start opencode run: %w", err)
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
					_ = execCtx.Heartbeat("opencode_running")
				case <-done:
					return
				case <-execCtx.Context.Done():
					return
				}
			}
		}()
	}

	streamCollector := newOpenCodeStreamCollector(run.ID, artifactWriter, func(event ExecutionEvent) {
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
	streamErrs := make(chan error, 2)
	var streamWG sync.WaitGroup
	streamWG.Add(2)
	go consumeOpenCodeJSONStream(stdoutPipe, execCtx.Context, streamCollector, &streamWG, streamErrs)
	go consumeOpenCodeTextStream(stderrPipe, "stderr", execCtx.Context, streamCollector, &streamWG, streamErrs)

	flushDone := make(chan struct{})
	go streamCollector.FlushLoop(execCtx.Context, flushDone)

	waitErr := cmd.Wait()
	stopHeartbeat()
	close(flushDone)

	streamWG.Wait()
	close(streamErrs)

	artifactCtx, cancelArtifacts := backgroundContextOnCancel(execCtx.Context)
	defer cancelArtifacts()

	streamCollector.FlushPending(artifactCtx, true)
	stdoutText, stderrText := streamCollector.Outputs()
	artifactWriter.Save(artifactCtx, "opencode_stdout", "text", stdoutText, false)
	artifactWriter.Save(artifactCtx, "opencode_stderr", "text", stderrText, false)

	for streamErr := range streamErrs {
		if streamErr == nil {
			continue
		}
		if waitErr == nil {
			waitErr = streamErr
			continue
		}
		slog.WarnContext(execCtx.Context, "failed to stream opencode output",
			"error", streamErr, "run_id", run.ID)
	}

	if waitErr != nil {
		if execCtx.Context.Err() != nil {
			return ErrRunCancelled
		}
		return fmt.Errorf("opencode run failed: %s", strings.TrimSpace(firstNonEmptyText(stderrText, waitErr.Error())))
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("opencode_finished")
	}

	if eventErr := streamCollector.EventError(); eventErr != "" {
		return fmt.Errorf("opencode reported an error: %s", eventErr)
	}

	responseText := sanitizeOpenCodeOutput(firstNonEmptyText(streamCollector.ResponseText(), stdoutText, stderrText))
	if strings.TrimSpace(responseText) == "" {
		return fmt.Errorf("opencode returned no response")
	}
	execCtx.CurrentAssistantText = responseText
	result := &ExecutionResult{
		AssistantText: responseText,
		AssistantBlocks: []ExecutionBlock{{
			Type: ExecutionBlockTypeText,
			Text: responseText,
		}},
		ToolInvocations: streamCollector.ToolInvocations(),
		Usage:           streamCollector.Usage(),
	}
	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		appendInteractivePlainTextQuestionInputRequestForRuntime(execCtx, result, "opencode")
	}
	execCtx.LastExecutionResult = result

	if ExtractLatestApprovalRequest(result.ToolInvocations) != nil || ExtractLatestReviewCheckpointRequest(result.ToolInvocations) != nil || ExtractLatestHumanInputRequest(result.ToolInvocations) != nil {
		return nil
	}

	postRunCtx, cancelPostRun := context.WithTimeout(execCtx.Context, openCodePostRunTimeout)
	defer cancelPostRun()
	postRunExecCtx := cloneExecutionContext(execCtx, postRunCtx)

	if isEngineerStoryRun(postRunExecCtx) {
		if err := e.persistEngineerWorkspace(postRunExecCtx, run, artifactWriter); err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
	}

	if postRunExecCtx.Heartbeat != nil {
		_ = postRunExecCtx.Heartbeat("finalizing")
	}
	artifactWriter.Save(postRunCtx, "agent_summary", "markdown", responseText, false)

	run.TokensUsed = streamCollector.TokensUsed()

	switch {
	case execCtx.TargetType == "epic" && execCtx.Epic != nil:
		switch execCtx.PlanningStage {
		case model.PlanningStageDraftSpec:
			draft, err := extractProductSpecDraftFromResponseText(responseText)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "product_spec_draft", draft); err != nil {
				return err
			}
		case model.PlanningStagePlanTasks:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, run.TokensUsed)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "story_plan_proposal", proposal); err != nil {
				return err
			}
		default:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, run.TokensUsed)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "orchestration_proposal", proposal); err != nil {
				return err
			}
		}
	case execCtx.TargetType == "support_conversation" && execCtx.Conversation != nil:
		summary, err := extractSupportRunSummaryFromResponseText(responseText)
		if err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
		if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "support_draft", summary); err != nil {
			return err
		}
		if summary.Status != nil && execCtx.Services != nil && execCtx.Services.UpdateConversationStatus != nil {
			if err := execCtx.Services.UpdateConversationStatus(postRunCtx, execCtx.WorkspaceID, execCtx.ConversationID, *summary.Status); err != nil {
				slog.WarnContext(postRunCtx, "failed to update support conversation status",
					"error", err, "conversation_id", execCtx.ConversationID)
			}
		}
	default:
		run.OutputSummary = json.RawMessage(`{"status":"success"}`)
		if err := e.runRepo.Update(postRunCtx, run); err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
	}

	return normalizeOpenCodePostRunError(postRunCtx, nil)
}

func (e *OpenCodeExecutor) buildEnv(execCtx *ExecutionContext, configContent string) ([]string, error) {
	env := os.Environ()
	var agent *model.Agent
	if execCtx != nil {
		agent = execCtx.Agent
	}
	provider := model.AgentModelProviderAnthropic
	if agent != nil {
		if resolvedProvider := normalizeOpenCodeProvider(strings.TrimSpace(derefOpenCodeString(agent.Provider))); resolvedProvider != "" {
			provider = resolvedProvider
		}
	}
	switch provider {
	case "", model.AgentModelProviderAnthropic:
		env = appendIfMissingEnv(env, "ANTHROPIC_API_KEY", e.anthropicAPIKey)
	case model.AgentModelProviderOpenAI:
		env = appendIfMissingEnv(env, "OPENAI_API_KEY", e.openAIAPIKey)
		env = appendIfMissingEnv(env, "OPENAI_BASE_URL", e.openAIBaseURL)
	case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		env = appendIfMissingEnv(env, "OPENROUTER_API_KEY", e.openRouterAPIKey)
	}
	env = upsertEnv(env, "NO_COLOR", "1")
	env = upsertEnv(env, "OPENCODE_CONFIG_CONTENT", configContent)
	if execCtx != nil && strings.TrimSpace(execCtx.RunID) != "" {
		runRoot := filepath.Join(os.TempDir(), openCodeRuntimeRootDir, sanitizeWorkspacePathComponent(execCtx.RunID))
		homeDir := filepath.Join(runRoot, "home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			return nil, fmt.Errorf("create opencode runtime home: %w", err)
		}
		env = upsertEnv(env, "HOME", homeDir)
		env = upsertEnv(env, "XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
		env = upsertEnv(env, "XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
		env = upsertEnv(env, "XDG_CACHE_HOME", filepath.Join(homeDir, ".cache"))
		env = upsertEnv(env, "OPENCODE_HOME", filepath.Join(homeDir, ".opencode"))
	}
	return env, nil
}

func (e *OpenCodeExecutor) resolveModelID(agent *model.Agent) string {
	provider := model.AgentModelProviderAnthropic
	modelName := ""
	if agent != nil {
		if resolvedProvider := normalizeOpenCodeProvider(strings.TrimSpace(derefOpenCodeString(agent.Provider))); resolvedProvider != "" {
			provider = resolvedProvider
		}
		modelName = normalizeOpenCodeConfiguredModelName(provider, strings.TrimSpace(derefOpenCodeString(agent.Model)))
	}
	if provider == "" {
		provider = model.AgentModelProviderAnthropic
	}
	if modelName == "" {
		modelName = defaultOpenCodeModelForProvider(provider)
	}
	if modelName == "" {
		return ""
	}
	return provider + "/" + modelName
}

func (e *OpenCodeExecutor) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) {
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
		slog.WarnContext(ctx, "failed to save artifact",
			"error", err, "run_id", run.ID, "artifact_type", artifactType)
	}
}

func (e *OpenCodeExecutor) notifyRun(ctx context.Context, run *model.AgentRun) {
	if e.runRepo == nil || run == nil {
		return
	}
	notifyCtx, cancel := backgroundContextOnCancel(ctx)
	defer cancel()
	e.runRepo.Notify(notifyCtx, run)
}

// saveOutputSummary marshals a post-run result into the run's OutputSummary,
// persists the run, and saves the payload as an artifact.
func (e *OpenCodeExecutor) saveOutputSummary(
	ctx context.Context,
	run *model.AgentRun,
	writer *openCodeArtifactWriter,
	artifactType string,
	value any,
) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s output: %w", artifactType, err)
	}
	run.OutputSummary = payload
	if err := e.runRepo.Update(ctx, run); err != nil {
		return normalizeOpenCodePostRunError(ctx, err)
	}
	writer.Save(ctx, artifactType, "json", string(payload), false)
	return nil
}

type openCodeArtifactWriter struct {
	executor *OpenCodeExecutor
	run      *model.AgentRun
	mu       sync.Mutex
	seqNo    int
}

func newOpenCodeArtifactWriter(executor *OpenCodeExecutor, run *model.AgentRun) *openCodeArtifactWriter {
	return &openCodeArtifactWriter{executor: executor, run: run}
}

func (w *openCodeArtifactWriter) Save(ctx context.Context, artifactType, format, content string, notify bool) {
	if w == nil || w.executor == nil || w.run == nil || content == "" {
		return
	}
	w.mu.Lock()
	w.seqNo++
	seqNo := w.seqNo
	w.mu.Unlock()
	w.executor.saveArtifact(ctx, w.run, artifactType, format, content, seqNo)
	if notify {
		w.executor.notifyRun(ctx, w.run)
	}
}

func buildOpenCodePromptArtifact(systemPrompt, userPrompt string) string {
	sections := make([]string, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		sections = append(sections, "Developer prompt:\n"+strings.TrimSpace(systemPrompt))
	}
	if strings.TrimSpace(userPrompt) != "" {
		sections = append(sections, "User prompt:\n"+strings.TrimSpace(userPrompt))
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func buildOpenCodeRuntimeInstructions(execCtx *ExecutionContext, run *model.AgentRun) string {
	var parts []string

	parts = append(parts, "You are running inside the OpenCode CLI runtime, not the Helpin native tool runtime.")
	parts = append(parts, "Use the local shell and file-editing capabilities available in this workspace directly.")
	parts = append(parts, "Do not rely on Helpin-specific tool wrappers or orchestration commands to inspect files, edit code, create branches, push changes, or open pull requests.")
	parts = append(parts, "Do not push the branch or open a pull request from this runtime. If you complete implementation, finish with a local git commit only; the platform will handle remote delivery.")

	if strings.TrimSpace(execCtx.BranchSyncStatus) == "conflicted" {
		parts = append(parts, "Before continuing the task, resolve the current git merge conflict that came from syncing the base branch into the working branch.")
		parts = append(parts, "Preserve the task's intended changes while incorporating the incoming base-branch changes. Remove all conflict markers, stage the resolved files, and complete the merge commit before doing additional implementation work.")
		if len(execCtx.BranchSyncConflictFiles) > 0 {
			parts = append(parts, "Conflicted files: "+strings.Join(execCtx.BranchSyncConflictFiles, ", ")+".")
		}
	}
	reviewContractInstructions := reviewCheckpointRuntimeInstructions(execCtx.SkillPolicy, "opencode")

	switch strings.TrimSpace(runInvocationMode(run, execCtx)) {
	case model.InvocationModeInteractive:
		parts = append(parts, "This is an interactive run. Continue from the latest human reply instead of restarting from scratch.")
		parts = append(parts, "Make repository changes when they materially advance the task, but they are not required on every turn.")
		parts = append(parts, "If you are blocked, ask for the next focused input or approval through the interactive run flow instead of ending with broad open questions.")
		if len(reviewContractInstructions) > 0 {
			parts = append(parts, reviewContractInstructions...)
		}
	default:
		if allowsCleanReviewNoop(execCtx) {
			parts = append(parts, "This is an autonomous run. Make durable progress on the assigned task before stopping.")
			parts = append(parts, "Inspect the relevant repository context carefully and make code changes only when they materially improve the review outcome.")
		} else if isEngineerStoryRun(execCtx) {
			parts = append(parts, "This is an autonomous implementation run. You must make concrete repository changes in the working tree unless you can prove the task is already complete or blocked by a real external constraint.")
			parts = append(parts, "Start by inspecting the repository with fast shell commands such as rg, ls, git status, and targeted file reads. Then edit the relevant files, run practical validation, and stop only after the repository reflects your implementation.")
			parts = append(parts, "A text-only analysis with no file modifications is a failed outcome for this run.")
		} else {
			parts = append(parts, "This is an autonomous run. Make durable progress on the assigned task before stopping.")
			parts = append(parts, "Inspect the relevant repository context before making changes, and validate any code changes you do make with practical checks when possible.")
		}
		if len(reviewContractInstructions) > 0 {
			parts = append(parts, reviewContractInstructions...)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (e *OpenCodeExecutor) persistEngineerWorkspace(execCtx *ExecutionContext, run *model.AgentRun, artifactWriter *openCodeArtifactWriter) error {
	if !isEngineerStoryRun(execCtx) {
		return nil
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("persisting_changes")
	}

	var (
		changed bool
		err     error
	)
	if isInteractiveRunInvocation(run) || allowsCleanReviewNoop(execCtx) {
		changed, err = openCodeRunProducedRepoChanges(execCtx)
		if err != nil {
			return err
		}
		if !changed {
			slog.InfoContext(execCtx.Context, "skipping strict repo-change requirement for opencode review/noop run",
				"run_id", run.ID)
			return nil
		}
	} else {
		changed, err = waitForOpenCodeRepoChanges(execCtx, openCodeRepoChangeWaitTimeout, openCodeRepoChangePollEvery)
		if err != nil {
			return err
		}
	}
	if !changed {
		statusSummary, diffStatSummary := captureOpenCodeNoChangeDiagnostics(execCtx, artifactWriter)
		return fmt.Errorf(
			"opencode completed without modifying the repository within %s; git_status=%s; git_diff=%s",
			openCodeRepoChangeWaitTimeout,
			truncateSingleLine(statusSummary, 160),
			truncateSingleLine(diffStatSummary, 160),
		)
	}

	if out, err := runGit(execCtx, "add", "-A"); err != nil {
		return fmt.Errorf("stage repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	diff, err := runGit(execCtx, "diff", "--cached")
	if err != nil {
		return fmt.Errorf("inspect staged diff: %s", strings.TrimSpace(firstNonEmptyText(diff, err.Error())))
	}

	filesOutput, err := runGit(execCtx, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("inspect staged files: %s", strings.TrimSpace(firstNonEmptyText(filesOutput, err.Error())))
	}
	changedFiles := strings.Fields(filesOutput)
	commitMessage := buildEngineerCommitMessage(execCtx.Task)
	if strings.TrimSpace(diff) == "" || len(changedFiles) == 0 {
		committedChange, err := detectCommittedEngineerChange(execCtx)
		if err != nil {
			return err
		}
		if committedChange != nil {
			return persistExistingEngineerCommit(execCtx, artifactWriter, committedChange)
		}
		if isInteractiveRunInvocation(run) || allowsCleanReviewNoop(execCtx) {
			slog.InfoContext(execCtx.Context, "skipping strict staged-diff requirement for opencode review/noop run",
				"run_id", run.ID)
			return nil
		}
		return fmt.Errorf("opencode completed without producing a staged repository diff")
	}

	artifactWriter.Save(execCtx.Context, "diff", "patch", diff, false)
	artifactWriter.Save(execCtx.Context, "file_bundle", "json", toJSONString(changedFiles), false)

	if out, err := runGit(execCtx, "commit", "-m", commitMessage); err != nil {
		return fmt.Errorf("commit repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("pushing_changes")
	}

	branch, err := resolveWorkingBranch(execCtx)
	if err != nil {
		return err
	}
	if out, err := runGit(execCtx, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	shaOutput, err := runGit(execCtx, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve commit SHA: %s", strings.TrimSpace(firstNonEmptyText(shaOutput, err.Error())))
	}
	sha := strings.TrimSpace(shaOutput)
	execCtx.WorkingBranch = branch
	if execCtx.OnGitPush != nil {
		if err := execCtx.OnGitPush(branch, sha); err != nil {
			return fmt.Errorf("record pushed branch: %w", err)
		}
	}

	persistenceResult := map[string]any{
		"branch":         branch,
		"commit_sha":     sha,
		"commit_message": commitMessage,
		"changed_files":  changedFiles,
	}
	artifactWriter.Save(execCtx.Context, "git_persistence_result", "json", toJSONString(persistenceResult), false)
	return nil
}

func cloneExecutionContext(execCtx *ExecutionContext, ctx context.Context) *ExecutionContext {
	if execCtx == nil {
		return nil
	}
	clone := *execCtx
	clone.Context = ctx
	return &clone
}

func normalizeOpenCodePostRunError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("opencode finalization timed out after %s", openCodePostRunTimeout)
	}
	return err
}

func isEngineerStoryRun(execCtx *ExecutionContext) bool {
	return execCtx != nil && execCtx.Task != nil && hasRepoMutationTools(resolvedProfileFor(execCtx).Tools)
}

func allowsCleanReviewNoop(execCtx *ExecutionContext) bool {
	return execCtx != nil && execCtx.Agent != nil && strings.TrimSpace(execCtx.Agent.EffectivePresetKey()) == model.AgentPresetReviewAgent
}

func isInteractiveRunInvocation(run *model.AgentRun) bool {
	return run != nil && strings.TrimSpace(run.InvocationMode) == model.InvocationModeInteractive
}

func resolveWorkingBranch(execCtx *ExecutionContext) (string, error) {
	if execCtx != nil && strings.TrimSpace(execCtx.WorkingBranch) != "" {
		return strings.TrimSpace(execCtx.WorkingBranch), nil
	}
	branchOutput, err := runGit(execCtx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve working branch: %s", strings.TrimSpace(firstNonEmptyText(branchOutput, err.Error())))
	}
	branch := strings.TrimSpace(branchOutput)
	if branch == "" {
		return "", fmt.Errorf("resolve working branch: git returned an empty branch name")
	}
	return branch, nil
}

func buildEngineerCommitMessage(story *model.PMTask) string {
	if story == nil {
		return "tp: apply engineer run changes"
	}
	if story.DisplayID > 0 {
		return fmt.Sprintf("tp: task #%d %s", story.DisplayID, story.Name)
	}
	return fmt.Sprintf("tp: task %s", story.Name)
}

type engineerCommittedChange struct {
	Diff          string
	ChangedFiles  []string
	CommitSHA     string
	CommitMessage string
}

func detectCommittedEngineerChange(execCtx *ExecutionContext) (*engineerCommittedChange, error) {
	if execCtx == nil || strings.TrimSpace(execCtx.WorkDir) == "" {
		return nil, nil
	}

	statusOutput, err := runGit(execCtx, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("inspect repository status: %s", strings.TrimSpace(firstNonEmptyText(statusOutput, err.Error())))
	}
	if strings.TrimSpace(statusOutput) != "" {
		return nil, nil
	}

	diffOutput, filesOutput, err := repoDiffAgainstBase(execCtx)
	if err != nil {
		return nil, err
	}
	changedFiles := strings.Fields(filesOutput)
	if strings.TrimSpace(diffOutput) == "" || len(changedFiles) == 0 {
		return nil, nil
	}

	shaOutput, err := runGit(execCtx, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve existing commit SHA: %s", strings.TrimSpace(firstNonEmptyText(shaOutput, err.Error())))
	}
	messageOutput, err := runGit(execCtx, "log", "-1", "--pretty=%s")
	if err != nil {
		return nil, fmt.Errorf("resolve existing commit message: %s", strings.TrimSpace(firstNonEmptyText(messageOutput, err.Error())))
	}

	return &engineerCommittedChange{
		Diff:          diffOutput,
		ChangedFiles:  changedFiles,
		CommitSHA:     strings.TrimSpace(shaOutput),
		CommitMessage: strings.TrimSpace(messageOutput),
	}, nil
}

func repoDiffAgainstBase(execCtx *ExecutionContext) (diffOutput string, filesOutput string, err error) {
	if execCtx == nil {
		return "", "", nil
	}

	workingBranch := strings.TrimSpace(execCtx.WorkingBranch)
	if workingBranch != "" {
		files, diff, ok, diffErr := repoDiffForRef(execCtx, "origin/"+workingBranch)
		if diffErr != nil {
			return "", "", diffErr
		}
		if ok {
			return diff, files, nil
		}
	}

	var lastErr error
	for _, ref := range repoComparisonRefs(execCtx) {
		files, diff, ok, diffErr := repoDiffForRef(execCtx, ref)
		if diffErr != nil {
			lastErr = diffErr
			continue
		}
		if ok {
			return diff, files, nil
		}
	}
	if lastErr != nil {
		return "", "", lastErr
	}

	baseBranch := strings.TrimSpace(execCtx.BaseBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	return "", "", fmt.Errorf("inspect repository diff: unable to compare HEAD against %q", baseBranch)
}

func repoComparisonRefs(execCtx *ExecutionContext) []string {
	baseBranch := strings.TrimSpace(execCtx.BaseBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	return []string{"origin/" + baseBranch, baseBranch}
}

func repoDiffForRef(execCtx *ExecutionContext, ref string) (filesOutput string, diffOutput string, ok bool, err error) {
	filesOutput, err = runGit(execCtx, "diff", "--name-only", ref+"...HEAD")
	if err != nil {
		return "", "", false, nil
	}
	ok = true
	if strings.TrimSpace(filesOutput) == "" {
		return "", "", true, nil
	}
	diffOutput, err = runGit(execCtx, "diff", ref+"...HEAD")
	if err != nil {
		return "", "", true, fmt.Errorf("inspect committed diff: %s", strings.TrimSpace(firstNonEmptyText(diffOutput, err.Error())))
	}
	return filesOutput, diffOutput, true, nil
}

func persistExistingEngineerCommit(execCtx *ExecutionContext, artifactWriter interface {
	Save(context.Context, string, string, string, bool)
}, change *engineerCommittedChange) error {
	if execCtx == nil || change == nil {
		return nil
	}

	artifactWriter.Save(execCtx.Context, "diff", "patch", change.Diff, false)
	artifactWriter.Save(execCtx.Context, "file_bundle", "json", toJSONString(change.ChangedFiles), false)

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("pushing_changes")
	}

	branch, err := resolveWorkingBranch(execCtx)
	if err != nil {
		return err
	}
	if out, err := runGit(execCtx, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	execCtx.WorkingBranch = branch
	if execCtx.OnGitPush != nil {
		if err := execCtx.OnGitPush(branch, change.CommitSHA); err != nil {
			return fmt.Errorf("record pushed branch: %w", err)
		}
	}

	persistenceResult := map[string]any{
		"branch":         branch,
		"commit_sha":     change.CommitSHA,
		"commit_message": change.CommitMessage,
		"changed_files":  change.ChangedFiles,
	}
	artifactWriter.Save(execCtx.Context, "git_persistence_result", "json", toJSONString(persistenceResult), false)
	return nil
}

func openCodeRunProducedRepoChanges(execCtx *ExecutionContext) (bool, error) {
	if execCtx == nil || strings.TrimSpace(execCtx.WorkDir) == "" {
		return false, nil
	}

	statusOutput, err := runGit(execCtx, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("inspect repository status: %s", strings.TrimSpace(firstNonEmptyText(statusOutput, err.Error())))
	}
	if strings.TrimSpace(statusOutput) != "" {
		return true, nil
	}

	_, filesOutput, err := repoDiffAgainstBase(execCtx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(filesOutput) != "", nil
}

func waitForOpenCodeRepoChanges(execCtx *ExecutionContext, timeout, interval time.Duration) (bool, error) {
	if execCtx == nil {
		return false, nil
	}
	if timeout <= 0 {
		timeout = openCodeRepoChangeWaitTimeout
	}
	if interval <= 0 {
		interval = openCodeRepoChangePollEvery
	}

	checkCtx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastErr error
	for {
		changed, err := openCodeRunProducedRepoChanges(execCtx)
		if err == nil {
			lastErr = nil
			if changed {
				return true, nil
			}
		} else {
			lastErr = err
		}

		if checkCtx.Err() != nil {
			break
		}
		if execCtx.Heartbeat != nil {
			_ = execCtx.Heartbeat("awaiting_repo_changes")
		}
		select {
		case <-checkCtx.Done():
		case <-ticker.C:
		}
	}

	if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
		return false, nil
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, checkCtx.Err()
}

func captureOpenCodeNoChangeDiagnostics(execCtx *ExecutionContext, artifactWriter *openCodeArtifactWriter) (string, string) {
	if execCtx == nil || artifactWriter == nil {
		return "", ""
	}
	var statusSummary string
	if status, err := runGit(execCtx, "status", "--short", "--branch"); err == nil && strings.TrimSpace(status) != "" {
		statusSummary = strings.TrimSpace(status)
		artifactWriter.Save(execCtx.Context, "git_status", "text", status, false)
	} else if err == nil {
		statusSummary = "clean working tree"
	}
	var diffStatSummary string
	if diffStat, err := runGit(execCtx, "diff", "--stat"); err == nil && strings.TrimSpace(diffStat) != "" {
		diffStatSummary = strings.TrimSpace(diffStat)
		artifactWriter.Save(execCtx.Context, "git_diff_stat", "text", diffStat, false)
	} else if err == nil {
		diffStatSummary = "no diff"
	}
	return statusSummary, diffStatSummary
}

func backgroundContextOnCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil && ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func shortRunID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
