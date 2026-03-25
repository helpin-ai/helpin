package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	codexChunkFlushInterval    = 2 * time.Second
	codexChunkFlushBytes       = 4 * 1024
	codexPostRunTimeout        = 2 * time.Minute
	codexScannerBufferSize     = 1024 * 1024
	codexGracefulShutdownDelay = 10 * time.Second
)

// CodexExecutor shells out to the codex CLI for autonomous coder and reviewer runs.
type CodexExecutor struct {
	kind             string
	commandPath      string
	defaultModel     string
	openAIAPIKey     string
	openAIBaseURL    string
	openRouterAPIKey string
	openRouterURL    string
	runRepo          *repository.AgentRunRepository
	artifactRepo     *repository.AgentRunArtifactRepository
}

func NewCodexExecutor(
	kind string,
	commandPath string,
	defaultModel string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterURL string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *CodexExecutor {
	if strings.TrimSpace(commandPath) == "" {
		commandPath = "codex"
	}
	return &CodexExecutor{
		kind:             kind,
		commandPath:      commandPath,
		defaultModel:     strings.TrimSpace(defaultModel),
		openAIAPIKey:     strings.TrimSpace(openAIAPIKey),
		openAIBaseURL:    strings.TrimSpace(openAIBaseURL),
		openRouterAPIKey: strings.TrimSpace(openRouterAPIKey),
		openRouterURL:    strings.TrimSpace(openRouterURL),
		runRepo:          runRepo,
		artifactRepo:     artifactRepo,
	}
}

func (e *CodexExecutor) Kind() string {
	return e.kind
}

func (e *CodexExecutor) Execute(execCtx *ExecutionContext, run *model.AgentRun) error {
	config := execCtx.Config
	if config == nil {
		config = DefaultWorkflowConfig()
	}

	var checklist []model.PMChecklistItem
	if execCtx.StoryID != "" && execCtx.Services != nil && execCtx.Services.ListChecklist != nil {
		items, err := execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.StoryID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "failed to list checklist for codex run",
				"error", err, "story_id", execCtx.StoryID)
		} else {
			checklist = items
		}
	}

	var ticketMessages []model.SupportMessage
	if execCtx.ConversationID != "" && execCtx.Services != nil && execCtx.Services.ListConversationMessages != nil {
		messages, err := execCtx.Services.ListConversationMessages(execCtx.Context, execCtx.WorkspaceID, execCtx.ConversationID)
		if err != nil {
			slog.WarnContext(execCtx.Context, "failed to list ticket messages for codex run",
				"error", err, "conversation_id", execCtx.ConversationID)
		} else {
			ticketMessages = messages
		}
	}

	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Story, execCtx.Epic, execCtx.Conversation, execCtx.PlanningStage, execCtx.PlanningMethodology, config)
	if supplement := BuildExecutionSupplementPrompt(run, execCtx.RunFacts, execCtx.ArtifactContext); supplement != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n## Current Run State\n" + supplement)
	}
	userPrompt := BuildUserPrompt(
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
	combinedPrompt := buildCodexPrompt(execCtx, systemPrompt, userPrompt)

	modelID := e.requestedModelID(execCtx.Agent)
	provider := e.resolveProvider(execCtx.Agent)
	authMode := "account_default"
	if modelID != "" {
		authMode = "api_key_forced"
	}
	configContent := e.buildConfigArtifact(execCtx, modelID, provider, authMode)

	lastMessageFile, err := os.CreateTemp("", "codex-last-message-*.txt")
	if err != nil {
		return fmt.Errorf("create codex output file: %w", err)
	}
	lastMessagePath := lastMessageFile.Name()
	if err := lastMessageFile.Close(); err != nil {
		return fmt.Errorf("close codex output file: %w", err)
	}
	defer os.Remove(lastMessagePath)

	args := []string{
		"exec",
		"--json",
		"--full-auto",
		"--skip-git-repo-check",
		"--color", "never",
		"--sandbox", codexSandboxMode(execCtx),
		"-C", execCtx.WorkDir,
		"--output-last-message", lastMessagePath,
	}
	execEnv, cleanupExecEnv, err := e.buildExecEnv(execCtx.Context, execCtx.Agent, modelID != "")
	if err != nil {
		return err
	}
	defer cleanupExecEnv()
	if modelID != "" {
		args = append(args, "--model", modelID)
	}
	args = append(args, combinedPrompt)

	cmd := exec.CommandContext(execCtx.Context, e.commandPath, args...)
	cmd.Dir = execCtx.WorkDir
	cmd.Env = execEnv
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = codexGracefulShutdownDelay

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create codex stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create codex stderr pipe: %w", err)
	}

	artifactWriter := newCodexArtifactWriter(e, run)
	artifactWriter.Save(execCtx.Context, "codex_config", "json", configContent, true)
	artifactWriter.Save(execCtx.Context, "codex_prompt", "markdown", combinedPrompt, false)

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("codex_starting")
	}

	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("codex executable %q was not found on PATH", e.commandPath)
		}
		return fmt.Errorf("start codex run: %w", err)
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
					_ = execCtx.Heartbeat("codex_running")
				case <-done:
					return
				case <-execCtx.Context.Done():
					return
				}
			}
		}()
	}

	streamCollector := newCodexStreamCollector(run.ID, artifactWriter)
	streamErrs := make(chan error, 2)
	var streamWG sync.WaitGroup
	streamWG.Add(2)
	go consumeCodexTextStream(stdoutPipe, "stdout", execCtx.Context, streamCollector, &streamWG, streamErrs)
	go consumeCodexTextStream(stderrPipe, "stderr", execCtx.Context, streamCollector, &streamWG, streamErrs)

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
	streamResponseText := streamCollector.ResponseText()
	artifactWriter.Save(artifactCtx, "codex_stdout", "text", stdoutText, false)
	artifactWriter.Save(artifactCtx, "codex_stderr", "text", stderrText, false)
	if failureSummary, failed := extractCodexEventFailure(stdoutText); failed {
		return fmt.Errorf("codex run failed: %s", strings.TrimSpace(firstNonEmptyText(failureSummary, stderrText, "turn.failed")))
	}

	for streamErr := range streamErrs {
		if streamErr == nil {
			continue
		}
		if waitErr == nil {
			waitErr = streamErr
			continue
		}
		slog.WarnContext(execCtx.Context, "failed to stream codex output",
			"error", streamErr, "run_id", run.ID)
	}

	missingLastMessageWarning := isCodexMissingLastMessageWarning(stderrText)

	if waitErr != nil {
		if execCtx.Context.Err() != nil {
			return ErrRunCancelled
		}
		if !canRecoverCodexMissingLastMessage(waitErr, stderrText, streamResponseText) {
			return fmt.Errorf("codex run failed: %s", strings.TrimSpace(firstNonEmptyText(stderrText, waitErr.Error())))
		}
		slog.WarnContext(execCtx.Context, "codex exited without a final last-message payload; continuing with streamed/fallback output",
			"run_id", run.ID)
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("codex_finished")
	}

	responseBytes, err := os.ReadFile(lastMessagePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read codex final response: %w", err)
	}
	responseSource := "last_message"
	responseText := strings.TrimSpace(string(responseBytes))
	if responseText == "" {
		responseText = streamResponseText
		if responseText != "" {
			responseSource = "stream"
		}
	}
	if responseText == "" {
		if missingLastMessageWarning {
			responseText = codexFallbackSummary(execCtx)
			if responseText != "" {
				responseSource = "fallback"
			}
		}
	}
	if responseText == "" {
		responseText = sanitizeOpenCodeOutput(firstNonEmptyText(stdoutText, stderrText))
		if responseText != "" {
			responseSource = "raw_output"
		}
	}
	if strings.TrimSpace(responseText) == "" {
		return fmt.Errorf("codex returned no response")
	}

	postRunCtx, cancelPostRun := context.WithTimeout(execCtx.Context, codexPostRunTimeout)
	defer cancelPostRun()
	postRunExecCtx := cloneExecutionContext(execCtx, postRunCtx)

	if isEngineerStoryRun(postRunExecCtx) {
		if err := e.persistEngineerWorkspace(postRunExecCtx, run, artifactWriter, responseText, responseSource, stdoutText, stderrText); err != nil {
			return normalizeCodexPostRunError(postRunCtx, err)
		}
	}

	if postRunExecCtx.Heartbeat != nil {
		_ = postRunExecCtx.Heartbeat("finalizing")
	}
	artifactWriter.Save(postRunCtx, "agent_summary", "markdown", responseText, false)
	artifactWriter.Save(postRunCtx, "codex_response", "markdown", responseText, false)

	switch {
	case execCtx.TargetType == "epic" && execCtx.Epic != nil:
		switch execCtx.PlanningStage {
		case model.PlanningStageDraftSpec:
			draft, err := extractProductSpecDraftFromResponseText(responseText)
			if err != nil {
				return normalizeCodexPostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "product_spec_draft", draft); err != nil {
				return err
			}
		case model.PlanningStagePlanStories:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, 0)
			if err != nil {
				return normalizeCodexPostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "story_plan_proposal", proposal); err != nil {
				return err
			}
		default:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, 0)
			if err != nil {
				return normalizeCodexPostRunError(postRunCtx, err)
			}
			if err := e.saveOutputSummary(postRunCtx, run, artifactWriter, "orchestration_proposal", proposal); err != nil {
				return err
			}
		}
	case execCtx.TargetType == "support_conversation" && execCtx.Conversation != nil:
		summary, err := extractSupportRunSummaryFromResponseText(responseText)
		if err != nil {
			return normalizeCodexPostRunError(postRunCtx, err)
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
			return normalizeCodexPostRunError(postRunCtx, err)
		}
	}

	return normalizeCodexPostRunError(postRunCtx, nil)
}

func buildCodexPrompt(execCtx *ExecutionContext, systemPrompt, userPrompt string) string {
	sections := []string{
		"System instructions:\n" + strings.TrimSpace(systemPrompt),
	}
	if runtimeInstructions := buildCodexRuntimeInstructions(execCtx); runtimeInstructions != "" {
		sections = append(sections, "Codex runtime instructions:\n"+runtimeInstructions)
	}
	sections = append(sections, "User task:\n"+strings.TrimSpace(buildOpenCodeUserPrompt(execCtx, userPrompt)))
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func buildCodexRuntimeInstructions(execCtx *ExecutionContext) string {
	resolved := resolvedProfileFor(execCtx)
	var parts []string

	parts = append(parts, "You are running inside the Codex CLI runtime, not the Helpin native tool runtime.")
	parts = append(parts, "Do not wait for Helpin-native tool calls like read_file, write_file, run_command, create_branch, commit_and_push, or open_pr. In this runtime, use Codex's own shell/file-edit capabilities directly inside the workspace.")

	if hasRepoMutationTools(resolved.Tools) {
		parts = append(parts, "This is an autonomous implementation run. You must make concrete repository changes in the working tree unless you can prove the task is already complete or blocked by a real external constraint.")
		parts = append(parts, "Start by inspecting the repository with fast shell commands such as rg, ls, git status, and targeted file reads. Then edit the relevant files, run practical validation, and stop only after the repository reflects your implementation.")
		parts = append(parts, "A text-only analysis with no file modifications is a failed outcome for this run.")
	}

	if len(resolved.Commands) > 0 {
		parts = append(parts, "Prefer these command families when they fit the task: "+strings.Join(resolved.Commands, ", ")+".")
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func codexSandboxMode(execCtx *ExecutionContext) string {
	resolved := resolvedProfileFor(execCtx)
	if resolved.RequiresRepo || len(resolved.Commands) > 0 || hasRepoMutationTools(resolved.Tools) {
		return "workspace-write"
	}
	return "read-only"
}

func (e *CodexExecutor) resolveProvider(agent *model.Agent) string {
	if agent != nil && agent.Provider != nil {
		switch normalizeOpenCodeProvider(strings.TrimSpace(*agent.Provider)) {
		case model.AgentModelProviderOpenRouter:
			return model.AgentModelProviderOpenRouter
		case model.AgentModelProviderOpenAI:
			return model.AgentModelProviderOpenAI
		}
	}
	if e.openAIAPIKey != "" {
		return model.AgentModelProviderOpenAI
	}
	if e.openRouterAPIKey != "" {
		return model.AgentModelProviderOpenRouter
	}
	return model.AgentModelProviderOpenAI
}

func (e *CodexExecutor) requestedModelID(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	modelName := strings.TrimSpace(derefOpenCodeString(agent.Model))
	if modelName == "" {
		return ""
	}
	return normalizeOpenCodeConfiguredModelName(e.resolveProvider(agent), modelName)
}

func (e *CodexExecutor) buildExecEnv(ctx context.Context, agent *model.Agent, forceAPIAuth bool) ([]string, func(), error) {
	env := e.buildBaseEnv()
	if !forceAPIAuth {
		return env, func() {}, nil
	}

	provider := e.resolveProvider(agent)
	apiKey := strings.TrimSpace(e.apiKeyForProvider(provider))
	if apiKey == "" {
		switch provider {
		case model.AgentModelProviderOpenRouter:
			return nil, nil, fmt.Errorf("codex explicit model selection requires OPENROUTER_API_KEY")
		default:
			return nil, nil, fmt.Errorf("codex explicit model selection requires OPENAI_API_KEY")
		}
	}

	tempHome, err := os.MkdirTemp("", "codex-api-home-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create isolated codex home: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tempHome)
	}

	codexHome := filepath.Join(tempHome, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("create isolated CODEX_HOME: %w", err)
	}

	env = upsertEnv(env, "HOME", tempHome)
	env = upsertEnv(env, "CODEX_HOME", codexHome)
	env = e.upsertAPIEnv(env, agent)

	loginCmd := exec.CommandContext(ctx, e.commandPath, "login", "--with-api-key")
	loginCmd.Env = env
	loginCmd.Stdin = strings.NewReader(apiKey + "\n")
	loginOutput, loginErr := loginCmd.CombinedOutput()
	if loginErr != nil {
		cleanup()
		return nil, nil, fmt.Errorf("initialize codex API auth: %s", strings.TrimSpace(firstNonEmptyText(string(loginOutput), loginErr.Error())))
	}

	return env, cleanup, nil
}

func (e *CodexExecutor) apiKeyForProvider(provider string) string {
	switch provider {
	case model.AgentModelProviderOpenRouter:
		return e.openRouterAPIKey
	default:
		return e.openAIAPIKey
	}
}

func (e *CodexExecutor) buildBaseEnv() []string {
	env := os.Environ()
	env = removeEnvKeys(env,
		"OPENAI_API_KEY",
		"OPENAI_BASE_URL",
		"OPENROUTER_API_KEY",
		"OPENROUTER_BASE_URL",
	)
	env = upsertEnv(env, "NO_COLOR", "1")
	return env
}

func (e *CodexExecutor) upsertAPIEnv(env []string, agent *model.Agent) []string {
	switch e.resolveProvider(agent) {
	case model.AgentModelProviderOpenRouter:
		env = upsertEnv(env, "OPENAI_API_KEY", e.openRouterAPIKey)
		if e.openRouterURL != "" {
			env = upsertEnv(env, "OPENAI_BASE_URL", e.openRouterURL)
		}
	default:
		env = upsertEnv(env, "OPENAI_API_KEY", e.openAIAPIKey)
		if e.openAIBaseURL != "" {
			env = upsertEnv(env, "OPENAI_BASE_URL", e.openAIBaseURL)
		}
	}
	return env
}

func removeEnvKeys(env []string, keys ...string) []string {
	if len(env) == 0 || len(keys) == 0 {
		return env
	}
	blocked := make(map[string]bool, len(keys))
	for _, key := range keys {
		blocked[key] = true
	}
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok && blocked[name] {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func (e *CodexExecutor) buildConfigArtifact(execCtx *ExecutionContext, modelID, provider, authMode string) string {
	payload := map[string]any{
		"runtime_kind":    e.kind,
		"provider":        provider,
		"requested_model": modelID,
		"model_selection": map[bool]string{true: "explicit", false: "codex_cli_default"}[modelID != ""],
		"auth_mode":       authMode,
		"sandbox":         codexSandboxMode(execCtx),
		"cwd":             execCtx.WorkDir,
	}
	return toJSONString(payload)
}

func (e *CodexExecutor) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) {
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

func (e *CodexExecutor) notifyRun(ctx context.Context, run *model.AgentRun) {
	if e.runRepo == nil || run == nil {
		return
	}
	notifyCtx, cancel := backgroundContextOnCancel(ctx)
	defer cancel()
	e.runRepo.Notify(notifyCtx, run)
}

func (e *CodexExecutor) saveOutputSummary(
	ctx context.Context,
	run *model.AgentRun,
	writer *codexArtifactWriter,
	artifactType string,
	value any,
) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s output: %w", artifactType, err)
	}
	run.OutputSummary = payload
	if err := e.runRepo.Update(ctx, run); err != nil {
		return normalizeCodexPostRunError(ctx, err)
	}
	writer.Save(ctx, artifactType, "json", string(payload), false)
	return nil
}

func (e *CodexExecutor) persistEngineerWorkspace(execCtx *ExecutionContext, run *model.AgentRun, artifactWriter *codexArtifactWriter, responseText, responseSource, stdoutText, stderrText string) error {
	if !isEngineerStoryRun(execCtx) {
		return nil
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("persisting_changes")
	}

	changed, err := openCodeRunProducedRepoChanges(execCtx)
	if err != nil {
		return err
	}
	if !changed {
		statusSummary, diffStatSummary := e.captureCodexNoChangeDiagnostics(execCtx, artifactWriter)
		return fmt.Errorf("codex completed without modifying the repository; response_source=%s; response=%s; codex_stdout=%s; codex_stderr=%s; git_status=%s; git_diff=%s",
			truncateSingleLine(responseSource, 40),
			truncateSingleLine(responseText, 240),
			truncateSingleLine(summarizeCodexJSONLOutput(stdoutText), 200),
			truncateSingleLine(stderrText, 200),
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
	if strings.TrimSpace(diff) == "" || len(changedFiles) == 0 {
		return fmt.Errorf("codex completed without producing a staged repository diff")
	}

	artifactWriter.Save(execCtx.Context, "diff", "patch", diff, false)
	artifactWriter.Save(execCtx.Context, "file_bundle", "json", toJSONString(changedFiles), false)

	commitMessage := buildEngineerCommitMessage(execCtx.Story)
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

func (e *CodexExecutor) captureCodexNoChangeDiagnostics(execCtx *ExecutionContext, artifactWriter *codexArtifactWriter) (string, string) {
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

func truncateSingleLine(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return "none"
	}
	if limit > 0 && len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}

func summarizeCodexJSONLOutput(stdoutText string) string {
	lines := strings.Split(stdoutText, "\n")
	summaries := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		eventType := strings.TrimSpace(lookupString(payload, "type"))
		if eventType == "" {
			continue
		}
		itemType := ""
		if item, ok := payload["item"].(map[string]any); ok {
			itemType = strings.TrimSpace(lookupString(item, "type"))
		}
		if itemType != "" {
			summaries = append(summaries, eventType+":"+itemType)
		} else {
			summaries = append(summaries, eventType)
		}
		if len(summaries) >= 8 {
			break
		}
	}
	if len(summaries) == 0 {
		return strings.TrimSpace(stdoutText)
	}
	return strings.Join(summaries, ", ")
}

func extractCodexEventFailure(stdoutText string) (string, bool) {
	lines := strings.Split(stdoutText, "\n")
	var summaries []string
	failed := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		eventType := strings.TrimSpace(lookupString(payload, "type"))
		switch eventType {
		case "turn.failed":
			failed = true
			if summary := strings.TrimSpace(firstNonEmptyText(
				lookupString(payload, "message"),
				lookupNestedString(payload, "error", "message"),
				lookupNestedString(payload, "error", "type"),
			)); summary != "" {
				summaries = append(summaries, summary)
			}
		case "error":
			if summary := strings.TrimSpace(firstNonEmptyText(
				lookupString(payload, "message"),
				lookupNestedString(payload, "error", "message"),
			)); summary != "" {
				summaries = append(summaries, summary)
			}
		case "item.completed":
			if item, ok := payload["item"].(map[string]any); ok {
				if strings.TrimSpace(lookupString(item, "type")) == "error" {
					failed = true
					if summary := strings.TrimSpace(firstNonEmptyText(
						lookupString(item, "message"),
						lookupNestedString(item, "error", "message"),
					)); summary != "" {
						summaries = append(summaries, summary)
					}
				}
			}
		}
	}
	if !failed {
		return "", false
	}
	return strings.Join(dedupeOrderedStrings(summaries), "; "), true
}

func lookupNestedString(payload map[string]any, keys ...string) string {
	var current any = payload
	for _, key := range keys {
		typed, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = typed[key]
		if !ok {
			return ""
		}
	}
	if value, ok := current.(string); ok {
		return value
	}
	return ""
}

func normalizeCodexPostRunError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("codex finalization timed out after %s", codexPostRunTimeout)
	}
	return err
}

func canRecoverCodexMissingLastMessage(waitErr error, stderrText, responseText string) bool {
	if waitErr == nil {
		return false
	}
	if isCodexMissingLastMessageWarning(stderrText) {
		return true
	}
	return strings.TrimSpace(responseText) != ""
}

func isCodexMissingLastMessageWarning(stderrText string) bool {
	return strings.Contains(strings.ToLower(stderrText), "no last agent message")
}

func codexFallbackSummary(execCtx *ExecutionContext) string {
	if isEngineerStoryRun(execCtx) {
		if execCtx.Story != nil && strings.TrimSpace(execCtx.Story.Name) != "" {
			return fmt.Sprintf("Codex completed work for story %q. Review the repository changes, diff, and git persistence artifacts for the implementation details.", execCtx.Story.Name)
		}
		return "Codex completed the requested story work. Review the repository changes, diff, and git persistence artifacts for the implementation details."
	}
	return "Codex completed successfully."
}

type codexArtifactWriter struct {
	executor *CodexExecutor
	run      *model.AgentRun
	mu       sync.Mutex
	seqNo    int
}

func newCodexArtifactWriter(executor *CodexExecutor, run *model.AgentRun) *codexArtifactWriter {
	return &codexArtifactWriter{executor: executor, run: run}
}

func (w *codexArtifactWriter) Save(ctx context.Context, artifactType, format, content string, notify bool) {
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

type codexStreamCollector struct {
	runID  string
	writer *codexArtifactWriter
	mu     sync.Mutex

	stdoutFull  strings.Builder
	stderrFull  strings.Builder
	stdoutChunk strings.Builder
	stderrChunk strings.Builder
	response    strings.Builder
}

func newCodexStreamCollector(runID string, writer *codexArtifactWriter) *codexStreamCollector {
	return &codexStreamCollector{runID: runID, writer: writer}
}

func (c *codexStreamCollector) AppendLine(ctx context.Context, stream, line string) {
	logLine := strings.TrimSpace(line)
	if logLine != "" {
		slog.DebugContext(ctx, "codex stream line",
			"run_id", shortRunID(c.runID),
			"stream", stream,
			"line", truncate(logLine, 1000),
		)
	}

	c.mu.Lock()
	full, chunk := c.buffersFor(stream)
	full.WriteString(line)
	full.WriteByte('\n')
	chunk.WriteString(line)
	chunk.WriteByte('\n')
	shouldFlush := chunk.Len() >= codexChunkFlushBytes
	c.mu.Unlock()

	if shouldFlush {
		c.FlushStream(ctx, stream, true)
	}

	if stream == "stdout" {
		c.AppendAssistantText(extractCodexAssistantText(line))
	}
}

func (c *codexStreamCollector) AppendAssistantText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	current := strings.TrimSpace(c.response.String())
	if current == text || strings.HasSuffix(current, text) {
		return
	}
	if c.response.Len() > 0 {
		c.response.WriteString("\n")
	}
	c.response.WriteString(text)
}

func (c *codexStreamCollector) FlushLoop(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(codexChunkFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.FlushPending(ctx, true)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *codexStreamCollector) FlushPending(ctx context.Context, notify bool) {
	c.FlushStream(ctx, "stdout", notify)
	c.FlushStream(ctx, "stderr", notify)
}

func (c *codexStreamCollector) FlushStream(ctx context.Context, stream string, notify bool) {
	c.mu.Lock()
	_, chunk := c.buffersFor(stream)
	content := chunk.String()
	chunk.Reset()
	c.mu.Unlock()

	if content == "" {
		return
	}
	c.writer.Save(ctx, "codex_"+stream+"_chunk", "text", content, notify)
}

func (c *codexStreamCollector) Outputs() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdoutFull.String(), c.stderrFull.String()
}

func (c *codexStreamCollector) ResponseText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(c.response.String())
}

func (c *codexStreamCollector) buffersFor(stream string) (*strings.Builder, *strings.Builder) {
	if stream == "stderr" {
		return &c.stderrFull, &c.stderrChunk
	}
	return &c.stdoutFull, &c.stdoutChunk
}

func consumeCodexTextStream(reader io.Reader, stream string, ctx context.Context, collector *codexStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), codexScannerBufferSize)
	for scanner.Scan() {
		collector.AppendLine(ctx, stream, stripANSI(scanner.Text()))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read %s: %w", stream, err)
		return
	}
	errCh <- nil
}

func extractCodexAssistantText(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return ""
	}

	var payload any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return ""
	}

	texts := collectCodexAssistantTexts(payload, false)
	if len(texts) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func collectCodexAssistantTexts(node any, assistantContext bool) []string {
	switch typed := node.(type) {
	case map[string]any:
		nodeType := strings.ToLower(strings.TrimSpace(lookupString(typed, "type")))
		if nodeType == "error" {
			return nil
		}

		role := strings.ToLower(strings.TrimSpace(lookupString(typed, "role")))
		nodeAssistant := assistantContext || role == "assistant" || nodeType == "agent_message"
		var texts []string

		if nodeAssistant {
			if text := strings.TrimSpace(firstNonEmptyText(
				lookupString(typed, "text"),
				lookupString(typed, "output_text"),
			)); text != "" {
				texts = append(texts, text)
			}
		}

		if nodeAssistant {
			for _, key := range []string{"content", "message", "item", "output", "parts"} {
				if child, ok := typed[key]; ok {
					texts = append(texts, collectCodexAssistantTexts(child, true)...)
				}
			}
		}

		for key, child := range typed {
			if nodeAssistant && (key == "content" || key == "message" || key == "item" || key == "output" || key == "parts") {
				continue
			}
			texts = append(texts, collectCodexAssistantTexts(child, nodeAssistant)...)
		}
		return dedupeOrderedStrings(texts)
	case []any:
		var texts []string
		for _, child := range typed {
			texts = append(texts, collectCodexAssistantTexts(child, assistantContext)...)
		}
		return dedupeOrderedStrings(texts)
	default:
		return nil
	}
}

func dedupeOrderedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}
