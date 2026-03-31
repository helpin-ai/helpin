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
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	toml "github.com/pelletier/go-toml/v2"
)

const (
	codexChunkFlushInterval    = 2 * time.Second
	codexChunkFlushBytes       = 4 * 1024
	codexPostRunTimeout        = 2 * time.Minute
	codexRepoChangeWaitTimeout = 60 * time.Second
	codexRepoChangePollEvery   = 2 * time.Second
	codexScannerBufferSize     = 1024 * 1024
	codexGracefulShutdownDelay = 10 * time.Second
	codexOpenAIAuthModeAPIKey  = "api_key"
	codexOpenAIAuthModeOAuth   = "chatgpt_oauth"
	codexOpenAIAuthModeDevice  = "chatgpt_device_code"
	codexForcedLoginMethodAPI  = "api"
	codexForcedLoginMethodChat = "chatgpt"
	codexOpenRouterDefaultURL  = "https://openrouter.ai/api/v1"
)

type CodexRuntimeConfig struct {
	Path                      string
	DefaultModel              string
	OpenAIAPIKey              string
	OpenAIBaseURL             string
	OpenAIAuthMode            string
	EnableManagedChatGPTOAuth bool
	ChatGPTAccessToken        string
	ChatGPTAccountID          string
	ChatGPTPlanType           string
	OpenRouterAPIKey          string
	OpenRouterBaseURL         string
}

type codexResolvedRuntimeProfile struct {
	Provider          string
	Model             string
	AuthMode          string
	ForcedLoginMethod string
	OpenAIBaseURL     string
	OpenRouterBaseURL string
}

type codexConfigArtifact struct {
	Model             string                                  `toml:"model,omitempty"`
	ApprovalPolicy    string                                  `toml:"approval_policy"`
	ApprovalsReviewer string                                  `toml:"approvals_reviewer,omitempty"`
	SandboxMode       string                                  `toml:"sandbox_mode"`
	ModelProvider     string                                  `toml:"model_provider"`
	OpenAIBaseURL     string                                  `toml:"openai_base_url,omitempty"`
	ForcedLoginMethod string                                  `toml:"forced_login_method,omitempty"`
	ModelProviders    map[string]codexConfigModelProviderInfo `toml:"model_providers,omitempty"`
}

type codexConfigModelProviderInfo struct {
	Name               string `toml:"name"`
	BaseURL            string `toml:"base_url,omitempty"`
	EnvKey             string `toml:"env_key,omitempty"`
	WireAPI            string `toml:"wire_api,omitempty"`
	SupportsWebsockets bool   `toml:"supports_websockets"`
}

// CodexExecutor shells out to the codex CLI for autonomous coder and reviewer runs.
type CodexExecutor struct {
	kind             string
	commandPath      string
	defaultModel     string
	openAIAPIKey     string
	openAIBaseURL    string
	openAIAuthMode   string
	chatGPTOAuth     bool
	chatGPTToken     string
	chatGPTAccountID string
	chatGPTPlanType  string
	openRouterAPIKey string
	openRouterURL    string
	runRepo          *repository.AgentRunRepository
	artifactRepo     *repository.AgentRunArtifactRepository
}

func NewCodexExecutor(
	kind string,
	config CodexRuntimeConfig,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *CodexExecutor {
	commandPath := strings.TrimSpace(config.Path)
	if strings.TrimSpace(commandPath) == "" {
		commandPath = "codex"
	}
	return &CodexExecutor{
		kind:             kind,
		commandPath:      commandPath,
		defaultModel:     strings.TrimSpace(config.DefaultModel),
		openAIAPIKey:     strings.TrimSpace(config.OpenAIAPIKey),
		openAIBaseURL:    strings.TrimSpace(config.OpenAIBaseURL),
		openAIAuthMode:   normalizeCodexOpenAIAuthMode(config.OpenAIAuthMode),
		chatGPTOAuth:     config.EnableManagedChatGPTOAuth,
		chatGPTToken:     strings.TrimSpace(config.ChatGPTAccessToken),
		chatGPTAccountID: strings.TrimSpace(config.ChatGPTAccountID),
		chatGPTPlanType:  strings.TrimSpace(config.ChatGPTPlanType),
		openRouterAPIKey: strings.TrimSpace(config.OpenRouterAPIKey),
		openRouterURL:    strings.TrimSpace(config.OpenRouterBaseURL),
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
	timeout := time.Duration(config.TimeoutMinutes) * time.Minute
	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()
	runExecCtx := cloneExecutionContext(execCtx, ctx)

	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Story, execCtx.Epic, execCtx.Conversation, execCtx.PlanningStage, execCtx.PlanningMethodology, config)
	if execCtx.Conversation != nil {
		systemPrompt += "\nFor support conversations, respond with valid JSON only in this shape: " +
			`{"status":"open|in_progress|pending|resolved|closed","draft_reply":{"content":"...","is_internal":false,"sender_display_name":"optional","approval_required":true}}.`
	}
	if supplement := BuildExecutionSupplementPrompt(run, execCtx.RunFacts, execCtx.ArtifactContext); supplement != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n## Current Run State\n" + supplement)
	}
	developerInstructions := strings.TrimSpace(systemPrompt)
	if runtimeInstructions := buildCodexRuntimeInstructions(execCtx); runtimeInstructions != "" {
		developerInstructions = strings.TrimSpace(developerInstructions + "\n\n## Codex Runtime Instructions\n" + runtimeInstructions)
	}
	artifactWriter := newCodexArtifactWriter(e, run)

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("codex_starting")
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
					stage := "codex_running"
					if execCtx.HeartbeatStageProvider != nil {
						if provided := strings.TrimSpace(execCtx.HeartbeatStageProvider()); provided != "" {
							stage = provided
						}
					}
					_ = execCtx.Heartbeat(stage)
				case <-done:
					return
				case <-execCtx.Context.Done():
					return
				}
			}
		}()
	}

	sessionHost := newCodexSessionHost(e, newCodexThreadStore(e.artifactRepo), artifactWriter, runExecCtx, run, developerInstructions)
	result, err := sessionHost.Execute()
	stopHeartbeat()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("codex runtime timed out after %s", timeout)
		}
		if execCtx.Context.Err() != nil {
			return ErrRunCancelled
		}
		return err
	}
	runExecCtx.LastExecutionResult = result
	execCtx.LastExecutionResult = result
	if result == nil {
		return fmt.Errorf("codex returned no response")
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("codex_finished")
	}

	run.TokensUsed = result.Usage.InputTokens + result.Usage.OutputTokens

	if result.CodexAuthState != nil {
		return nil
	}

	if ExtractLatestHumanApprovalRequest(result.ToolInvocations) != nil || ExtractLatestHumanInputRequest(result.ToolInvocations) != nil {
		return nil
	}

	responseSource := "assistant_turn"
	responseText := strings.TrimSpace(result.AssistantText)
	if responseText == "" {
		responseText = codexFallbackSummary(execCtx)
		responseSource = "fallback"
	}
	if strings.TrimSpace(responseText) == "" {
		return fmt.Errorf("codex returned no response")
	}

	postRunCtx, cancelPostRun := context.WithTimeout(execCtx.Context, codexPostRunTimeout)
	defer cancelPostRun()
	postRunExecCtx := cloneExecutionContext(execCtx, postRunCtx)

	if isEngineerStoryRun(postRunExecCtx) {
		if err := e.persistEngineerWorkspace(postRunExecCtx, run, artifactWriter, responseText, responseSource, "", ""); err != nil {
			if errors.Is(err, ErrInteractiveRepoChangePending) {
				appendInteractiveRepoFollowupInputRequest(result)
				runExecCtx.LastExecutionResult = result
				execCtx.LastExecutionResult = result
				return nil
			}
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
	if e.openAIConfigured() {
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

func (e *CodexExecutor) upsertProviderEnv(env []string, provider string) []string {
	switch normalizeOpenCodeProvider(provider) {
	case model.AgentModelProviderOpenRouter:
		env = upsertEnv(env, "OPENROUTER_API_KEY", e.openRouterAPIKey)
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

func (e *CodexExecutor) buildConfigArtifact(execCtx *ExecutionContext, profile codexResolvedRuntimeProfile, approvalPolicy string) (string, error) {
	config := codexConfigArtifact{
		Model:             strings.TrimSpace(profile.Model),
		ApprovalPolicy:    strings.TrimSpace(approvalPolicy),
		ApprovalsReviewer: "user",
		SandboxMode:       codexSandboxMode(execCtx),
		ModelProvider:     strings.TrimSpace(profile.Provider),
		ForcedLoginMethod: strings.TrimSpace(profile.ForcedLoginMethod),
	}
	if strings.TrimSpace(profile.OpenAIBaseURL) != "" {
		config.OpenAIBaseURL = strings.TrimSpace(profile.OpenAIBaseURL)
	}
	if strings.TrimSpace(profile.Provider) == model.AgentModelProviderOpenRouter {
		config.ModelProviders = map[string]codexConfigModelProviderInfo{
			model.AgentModelProviderOpenRouter: {
				Name:               "OpenRouter",
				BaseURL:            firstNonEmptyText(strings.TrimSpace(profile.OpenRouterBaseURL), codexOpenRouterDefaultURL),
				EnvKey:             "OPENROUTER_API_KEY",
				WireAPI:            "responses",
				SupportsWebsockets: false,
			},
		}
	}
	payload, err := toml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal codex config.toml: %w", err)
	}
	return strings.TrimSpace(string(payload)) + "\n", nil
}

func normalizeCodexOpenAIAuthMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", codexOpenAIAuthModeAPIKey, "api", "api-key":
		return codexOpenAIAuthModeAPIKey
	case codexOpenAIAuthModeOAuth, "oauth", "chatgpt", "chatgpt-auth":
		return codexOpenAIAuthModeOAuth
	case codexOpenAIAuthModeDevice, "device_code", "chatgpt-device", "chatgpt-device-code", "chatgpt-managed":
		return codexOpenAIAuthModeDevice
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func (e *CodexExecutor) openAIConfigured() bool {
	switch normalizeCodexOpenAIAuthMode(e.openAIAuthMode) {
	case codexOpenAIAuthModeOAuth:
		return e.chatGPTOAuth && e.chatGPTToken != "" && e.chatGPTAccountID != ""
	case codexOpenAIAuthModeDevice:
		return true
	case codexOpenAIAuthModeAPIKey:
		return strings.TrimSpace(e.openAIAPIKey) != ""
	default:
		return false
	}
}

func (e *CodexExecutor) resolveRuntimeProfile(agent *model.Agent) (codexResolvedRuntimeProfile, error) {
	profile := codexResolvedRuntimeProfile{
		Provider: e.resolveProvider(agent),
		Model:    e.selectedModelID(agent),
	}

	switch profile.Provider {
	case model.AgentModelProviderOpenRouter:
		if strings.TrimSpace(e.openRouterAPIKey) == "" {
			return codexResolvedRuntimeProfile{}, fmt.Errorf("codex runtime requires OPENROUTER_API_KEY for provider %q", profile.Provider)
		}
		profile.AuthMode = codexOpenAIAuthModeAPIKey
		profile.OpenRouterBaseURL = firstNonEmptyText(strings.TrimSpace(e.openRouterURL), codexOpenRouterDefaultURL)
		return profile, nil
	case model.AgentModelProviderOpenAI:
		switch normalizeCodexOpenAIAuthMode(e.openAIAuthMode) {
		case codexOpenAIAuthModeAPIKey:
			if strings.TrimSpace(e.openAIAPIKey) == "" {
				return codexResolvedRuntimeProfile{}, fmt.Errorf("codex runtime requires OPENAI_API_KEY for provider %q", profile.Provider)
			}
			profile.AuthMode = codexOpenAIAuthModeAPIKey
			profile.ForcedLoginMethod = codexForcedLoginMethodAPI
			profile.OpenAIBaseURL = strings.TrimSpace(e.openAIBaseURL)
			return profile, nil
		case codexOpenAIAuthModeOAuth:
			if !e.chatGPTOAuth || e.chatGPTToken == "" || e.chatGPTAccountID == "" {
				return codexResolvedRuntimeProfile{}, fmt.Errorf("codex runtime requires Helpin-managed ChatGPT OAuth when CODEX_OPENAI_AUTH_MODE=%q", codexOpenAIAuthModeOAuth)
			}
			profile.AuthMode = codexOpenAIAuthModeOAuth
			profile.ForcedLoginMethod = codexForcedLoginMethodChat
			return profile, nil
		case codexOpenAIAuthModeDevice:
			profile.AuthMode = codexOpenAIAuthModeDevice
			profile.ForcedLoginMethod = codexForcedLoginMethodChat
			return profile, nil
		default:
			return codexResolvedRuntimeProfile{}, fmt.Errorf("unsupported CODEX_OPENAI_AUTH_MODE %q", strings.TrimSpace(e.openAIAuthMode))
		}
	default:
		return codexResolvedRuntimeProfile{}, fmt.Errorf("codex runtime requires provider openai or openrouter")
	}
}

func (e *CodexExecutor) loginSession(ctx context.Context, client *codexAppServerClient, profile codexResolvedRuntimeProfile) error {
	if client == nil {
		return fmt.Errorf("codex app-server client is required")
	}
	params, err := e.loginPayloadForProfile(profile)
	if err != nil {
		return err
	}
	if params == nil {
		return nil
	}
	_, err = client.Request(ctx, "account/login/start", params)
	return err
}

func (e *CodexExecutor) loginPayloadForProfile(profile codexResolvedRuntimeProfile) (map[string]any, error) {
	switch strings.TrimSpace(profile.Provider) {
	case model.AgentModelProviderOpenRouter:
		return nil, nil
	case model.AgentModelProviderOpenAI:
		switch strings.TrimSpace(profile.AuthMode) {
		case codexOpenAIAuthModeAPIKey:
			if strings.TrimSpace(e.openAIAPIKey) == "" {
				return nil, fmt.Errorf("codex runtime requires OPENAI_API_KEY for OpenAI API-key auth")
			}
			return map[string]any{
				"type":   "apiKey",
				"apiKey": strings.TrimSpace(e.openAIAPIKey),
			}, nil
		case codexOpenAIAuthModeOAuth:
			payload, err := e.chatGPTAuthPayload()
			if err != nil {
				return nil, err
			}
			payload["type"] = "chatgptAuthTokens"
			return payload, nil
		case codexOpenAIAuthModeDevice:
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported codex auth mode %q", strings.TrimSpace(profile.AuthMode))
		}
	default:
		return nil, nil
	}
}

func (e *CodexExecutor) chatGPTAuthPayload() (map[string]any, error) {
	if !e.chatGPTOAuth {
		return nil, fmt.Errorf("Helpin-managed ChatGPT OAuth is disabled")
	}
	if e.chatGPTToken == "" || e.chatGPTAccountID == "" {
		return nil, fmt.Errorf("Helpin-managed ChatGPT OAuth requires CODEX_CHATGPT_ACCESS_TOKEN and CODEX_CHATGPT_ACCOUNT_ID")
	}
	payload := map[string]any{
		"accessToken":      e.chatGPTToken,
		"chatgptAccountId": e.chatGPTAccountID,
	}
	if e.chatGPTPlanType != "" {
		payload["chatgptPlanType"] = e.chatGPTPlanType
	}
	return payload, nil
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

	changed, err := waitForOpenCodeRepoChanges(execCtx, codexRepoChangeWaitTimeout, codexRepoChangePollEvery)
	if err != nil {
		return err
	}
	if !changed {
		if isInteractiveRunInvocation(run) {
			slog.InfoContext(execCtx.Context, "interactive codex run produced no repository changes yet; requesting follow-up input",
				"run_id", run.ID)
			return ErrInteractiveRepoChangePending
		}
		statusSummary, diffStatSummary := e.captureCodexNoChangeDiagnostics(execCtx, artifactWriter)
		return fmt.Errorf("codex completed without modifying the repository within %s; response_source=%s; response=%s; codex_stdout=%s; codex_stderr=%s; git_status=%s; git_diff=%s",
			codexRepoChangeWaitTimeout,
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
		committedChange, err := detectCommittedEngineerChange(execCtx)
		if err != nil {
			return err
		}
		if committedChange != nil {
			return persistExistingEngineerCommit(execCtx, artifactWriter, committedChange)
		}
		if isInteractiveRunInvocation(run) {
			slog.InfoContext(execCtx.Context, "interactive codex run ended without staged repository diff; requesting follow-up input",
				"run_id", run.ID)
			return ErrInteractiveRepoChangePending
		}
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

func appendInteractiveRepoFollowupInputRequest(result *ExecutionResult) {
	if result == nil || ExtractLatestHumanInputRequest(result.ToolInvocations) != nil {
		return
	}
	request := HumanInputRequest{
		Questions: []HumanInputQuestion{
			{
				ID:   "next_action",
				Type: QuestionTypeSingleSelect,
				Text: "No repository changes were detected yet. How should the coding run continue?",
				Options: []HumanInputOption{
					{Value: "continue_coding", Label: "Continue coding now"},
					{Value: "provide_guidance", Label: "I will provide additional guidance"},
				},
			},
		},
	}
	input, _ := json.Marshal(request)
	result.ToolInvocations = append(result.ToolInvocations, model.ToolInvocation{
		ToolName:      ToolRequestHumanInput,
		Input:         input,
		OutputSummary: "interactive run paused: no repository changes detected",
	})
	if strings.TrimSpace(result.AssistantText) == "" {
		result.AssistantText = "I did not produce repository changes yet. Reply with guidance to continue."
	}
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
