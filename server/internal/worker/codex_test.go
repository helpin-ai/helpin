package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	toml "github.com/pelletier/go-toml/v2"
)

func TestExtractCodexAssistantTextFromJSONL(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Implemented the metrics change."}]}}`
	got := extractCodexAssistantText(line)
	if got != "Implemented the metrics change." {
		t.Fatalf("expected assistant text, got %q", got)
	}
}

func TestExtractCodexAssistantTextFromAgentMessageItem(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Hello from Codex."}}`
	got := extractCodexAssistantText(line)
	if got != "Hello from Codex." {
		t.Fatalf("expected agent_message text, got %q", got)
	}
}

func TestExtractCodexAssistantTextIgnoresErrorEvents(t *testing.T) {
	line := `{"type":"error","message":"Reconnecting..."}`
	got := extractCodexAssistantText(line)
	if got != "" {
		t.Fatalf("expected empty text for error event, got %q", got)
	}
}

func TestCanRecoverCodexMissingLastMessage(t *testing.T) {
	err := errors.New("exit status 1")
	stderr := "Warning: no last agent message; wrote empty content to /tmp/codex-last-message-123.txt"
	if !canRecoverCodexMissingLastMessage(err, stderr, "Finished implementation.") {
		t.Fatal("expected missing-last-message warning with streamed response text to recover")
	}
	if !canRecoverCodexMissingLastMessage(err, stderr, "") {
		t.Fatal("expected recovery for missing-last-message warning even when streamed response text is empty")
	}
	if !canRecoverCodexMissingLastMessage(err, "some other error", "Finished implementation.") {
		t.Fatal("expected recovery when streamed assistant response text exists")
	}
}

func TestBuildCodexPromptIncludesRuntimeSpecificEngineerInstructions(t *testing.T) {
	prompt := buildCodexPrompt(&ExecutionContext{
		Agent: &model.Agent{
			PresetKey:    model.AgentPresetCodeBuilder,
			AllowedTools: []byte(`["write_file","run_command","commit_and_push"]`),
		},
		Task: &model.PMTask{Name: "Implement metrics"},
	}, "You are a coding agent.", "Please implement the story.")

	for _, snippet := range []string{
		"running inside the Codex CLI runtime",
		"Do not wait for Helpin-native tool calls",
		"Do not push the branch or open a pull request from Codex",
		"must make concrete repository changes",
		"text-only analysis with no file modifications is a failed outcome",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected codex prompt to contain %q, got:\n%s", snippet, prompt)
		}
	}
}

func TestInstallCodexCommandGuardsBlocksGitPush(t *testing.T) {
	runRoot := t.TempDir()
	guardDir, err := installCodexCommandGuards(runRoot)
	if err != nil {
		t.Fatalf("install command guards: %v", err)
	}

	cmd := exec.Command(filepath.Join(guardDir, "git"), "push", "origin", "main")
	cmd.Dir = runRoot
	cmd.Env = append([]string{}, "PATH="+guardDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected git push to be blocked")
	}
	if !strings.Contains(string(output), codexDeliveryGuardMessage) {
		t.Fatalf("expected guard message in output, got %q", string(output))
	}
}

func TestInstallCodexCommandGuardsBlocksGitPushWithCFlag(t *testing.T) {
	runRoot := t.TempDir()
	guardDir, err := installCodexCommandGuards(runRoot)
	if err != nil {
		t.Fatalf("install command guards: %v", err)
	}

	cmd := exec.Command(filepath.Join(guardDir, "git"), "-C", runRoot, "push", "origin", "main")
	cmd.Dir = runRoot
	cmd.Env = append([]string{}, "PATH="+guardDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected git -C ... push to be blocked")
	}
	if !strings.Contains(string(output), codexDeliveryGuardMessage) {
		t.Fatalf("expected guard message in output, got %q", string(output))
	}
}

func TestInstallCodexCommandGuardsAllowsLocalGitCommands(t *testing.T) {
	runRoot := t.TempDir()
	guardDir, err := installCodexCommandGuards(runRoot)
	if err != nil {
		t.Fatalf("install command guards: %v", err)
	}

	cmd := exec.Command(filepath.Join(guardDir, "git"), "--version")
	cmd.Dir = runRoot
	cmd.Env = append([]string{}, "PATH="+guardDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected git --version to pass through, got err=%v output=%q", err, string(output))
	}
	if !strings.Contains(string(output), "git version") {
		t.Fatalf("expected git --version output, got %q", string(output))
	}
}

func TestInstallCodexCommandGuardsBlocksGHPRCreateWithRepoFlag(t *testing.T) {
	runRoot := t.TempDir()
	guardDir, err := installCodexCommandGuards(runRoot)
	if err != nil {
		t.Fatalf("install command guards: %v", err)
	}
	guardPath := filepath.Join(guardDir, "gh")
	if _, err := os.Stat(guardPath); err != nil {
		if os.IsNotExist(err) {
			t.Skip("gh is not installed in test environment")
		}
		t.Fatalf("stat gh guard: %v", err)
	}

	cmd := exec.Command(guardPath, "-R", "owner/repo", "pr", "create", "--title", "Test PR")
	cmd.Dir = runRoot
	cmd.Env = append([]string{}, "PATH="+guardDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected gh -R ... pr create to be blocked")
	}
	if !strings.Contains(string(output), codexDeliveryGuardMessage) {
		t.Fatalf("expected guard message in output, got %q", string(output))
	}
}

func TestCodexPersistEngineerWorkspaceCommitsLocallyWithoutPush(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	runGitCmd(t, tempDir, "git", "init", "--bare", remoteDir)

	seedDir := filepath.Join(tempDir, "seed")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, seedDir)
	configureGitIdentity(t, seedDir)
	writeTestFile(t, filepath.Join(seedDir, "README.md"), "hello\n")
	runGitCmd(t, seedDir, "git", "add", "README.md")
	runGitCmd(t, seedDir, "git", "commit", "-m", "initial commit")
	runGitCmd(t, seedDir, "git", "branch", "-M", "main")
	runGitCmd(t, seedDir, "git", "push", "-u", "origin", "main")

	workDir := filepath.Join(tempDir, "work")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, workDir)
	configureGitIdentity(t, workDir)
	runGitCmd(t, workDir, "git", "checkout", "-B", "main", "origin/main")
	runGitCmd(t, workDir, "git", "checkout", "-b", "tp-123-implement")
	writeTestFile(t, filepath.Join(workDir, "README.md"), "hello\nupdated\n")

	executor := NewCodexExecutor("codex", CodexRuntimeConfig{}, nil, nil, nil)
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "main",
		WorkingBranch: "tp-123-implement",
		Agent:         &model.Agent{AllowedTools: []byte(`["write_file","commit_and_push","open_pr"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Implement notification preferences"},
	}

	if err := executor.persistEngineerWorkspace(execCtx, &model.AgentRun{}, &codexArtifactWriter{}, "", "", "", ""); err != nil {
		t.Fatalf("persist engineer workspace: %v", err)
	}

	if execCtx.LocalGitCommit == nil {
		t.Fatal("expected local git commit metadata to be recorded")
	}
	if execCtx.LocalGitCommit.Branch != "tp-123-implement" {
		t.Fatalf("expected local commit branch %q, got %q", "tp-123-implement", execCtx.LocalGitCommit.Branch)
	}
	if strings.TrimSpace(execCtx.LocalGitCommit.CommitSHA) == "" {
		t.Fatal("expected local commit sha to be recorded")
	}

	cmd := exec.Command("git", "--git-dir", remoteDir, "show-ref", "--verify", "refs/heads/tp-123-implement")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected remote working branch to remain absent until backend push")
	}

	status := strings.TrimSpace(runGitCmd(t, workDir, "git", "status", "--porcelain"))
	if status != "" {
		t.Fatalf("expected clean working tree after local persistence, got %q", status)
	}
}

func TestSyncPostRunExecutionStateCopiesLocalCommitMetadata(t *testing.T) {
	execCtx := &ExecutionContext{
		WorkingBranch: "feature/original",
	}
	postRunExecCtx := &ExecutionContext{
		WorkingBranch: "feature/synced",
		LocalGitCommit: &GitCommitMetadata{
			Branch:        "feature/synced",
			CommitSHA:     "abc123",
			CommitMessage: "Implement sync",
			ChangedFiles:  []string{"server/internal/worker/codex.go"},
		},
	}

	syncPostRunExecutionState(execCtx, postRunExecCtx)

	if execCtx.WorkingBranch != "feature/synced" {
		t.Fatalf("expected working branch to sync, got %q", execCtx.WorkingBranch)
	}
	if execCtx.LocalGitCommit == nil {
		t.Fatal("expected local git commit metadata to be copied")
	}
	if execCtx.LocalGitCommit.CommitSHA != "abc123" {
		t.Fatalf("expected commit sha to sync, got %q", execCtx.LocalGitCommit.CommitSHA)
	}

	postRunExecCtx.LocalGitCommit.ChangedFiles[0] = "mutated"
	if execCtx.LocalGitCommit.ChangedFiles[0] != "server/internal/worker/codex.go" {
		t.Fatalf("expected changed files to be copied defensively, got %#v", execCtx.LocalGitCommit.ChangedFiles)
	}
}

func TestValidateCodexResolvedMergeStateRejectsUnmergedPaths(t *testing.T) {
	repoDir := t.TempDir()
	runGitCmd(t, repoDir, "git", "init")
	configureGitIdentity(t, repoDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "base\n")
	runGitCmd(t, repoDir, "git", "add", "README.md")
	runGitCmd(t, repoDir, "git", "commit", "-m", "initial")

	runGitCmd(t, repoDir, "git", "checkout", "-b", "feature")
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "feature change\n")
	runGitCmd(t, repoDir, "git", "commit", "-am", "feature change")

	runGitCmd(t, repoDir, "git", "checkout", "master")
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "main change\n")
	runGitCmd(t, repoDir, "git", "commit", "-am", "main change")

	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected merge conflict, got success with output %q", string(output))
	}

	execCtx := &ExecutionContext{
		Context:          context.Background(),
		WorkDir:          repoDir,
		BranchSyncStatus: "conflicted",
	}
	err = validateCodexResolvedMergeState(execCtx)
	if err == nil {
		t.Fatal("expected unresolved merge paths to be rejected")
	}
	if !strings.Contains(err.Error(), "merge conflicts remain unresolved") {
		t.Fatalf("expected unresolved merge error, got %v", err)
	}
}

func TestValidateCodexResolvedMergeStateRejectsLeftoverConflictMarkers(t *testing.T) {
	repoDir := t.TempDir()
	runGitCmd(t, repoDir, "git", "init")
	configureGitIdentity(t, repoDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "base\n")
	runGitCmd(t, repoDir, "git", "add", "README.md")
	runGitCmd(t, repoDir, "git", "commit", "-m", "initial")

	writeTestFile(t, filepath.Join(repoDir, "README.md"), "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> origin/main\n")
	runGitCmd(t, repoDir, "git", "add", "README.md")

	execCtx := &ExecutionContext{
		Context:          context.Background(),
		WorkDir:          repoDir,
		BranchSyncStatus: "conflicted",
	}
	err := validateCodexResolvedMergeState(execCtx)
	if err == nil {
		t.Fatal("expected leftover conflict markers to be rejected")
	}
	if !strings.Contains(err.Error(), "merge conflict markers remain") {
		t.Fatalf("expected conflict marker error, got %v", err)
	}
}

func TestBuildCodexConfigArtifactAddsOpenRouterProviderConfig(t *testing.T) {
	provider := model.AgentModelProviderOpenRouter
	modelName := "qwen/qwen3.5-122b-a10b"
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:      "gpt-5-mini",
		OpenRouterAPIKey:  "openrouter-secret",
		OpenRouterBaseURL: "https://openrouter.ai/api/v1",
	}, nil, nil, nil)

	profile, err := executor.resolveRuntimeProfile(&model.Agent{
		Provider: &provider,
		Model:    &modelName,
	})
	if err != nil {
		t.Fatalf("resolve runtime profile: %v", err)
	}
	payload, err := executor.buildConfigArtifact(&ExecutionContext{}, profile, "on-request")
	if err != nil {
		t.Fatalf("build config artifact: %v", err)
	}

	var decoded struct {
		Model          string `toml:"model"`
		ApprovalPolicy string `toml:"approval_policy"`
		SandboxMode    string `toml:"sandbox_mode"`
		ModelProvider  string `toml:"model_provider"`
		ModelProviders map[string]struct {
			BaseURL            string `toml:"base_url"`
			EnvKey             string `toml:"env_key"`
			WireAPI            string `toml:"wire_api"`
			SupportsWebsockets bool   `toml:"supports_websockets"`
		} `toml:"model_providers"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded.Model != "qwen/qwen3.5-122b-a10b" {
		t.Fatalf("expected model to be preserved, got %q", decoded.Model)
	}
	if decoded.ApprovalPolicy != "on-request" {
		t.Fatalf("expected approval policy to be written, got %q", decoded.ApprovalPolicy)
	}
	if decoded.SandboxMode != codexSandboxMode(&ExecutionContext{}) {
		t.Fatalf("expected inferred sandbox mode %q, got %q", codexSandboxMode(&ExecutionContext{}), decoded.SandboxMode)
	}
	if decoded.ModelProvider != model.AgentModelProviderOpenRouter {
		t.Fatalf("expected model provider %q, got %q", model.AgentModelProviderOpenRouter, decoded.ModelProvider)
	}
	openRouter, ok := decoded.ModelProviders[model.AgentModelProviderOpenRouter]
	if !ok {
		t.Fatalf("expected openrouter provider block in config: %#v", decoded.ModelProviders)
	}
	if openRouter.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected openrouter base URL, got %q", openRouter.BaseURL)
	}
	if openRouter.EnvKey != "OPENROUTER_API_KEY" {
		t.Fatalf("expected openrouter env key, got %q", openRouter.EnvKey)
	}
	if openRouter.WireAPI != "responses" {
		t.Fatalf("expected responses wire API, got %q", openRouter.WireAPI)
	}
	if openRouter.SupportsWebsockets {
		t.Fatal("expected openrouter config to disable websockets")
	}
}

func TestBuildCodexConfigArtifactUsesExplicitSandboxOverride(t *testing.T) {
	provider := model.AgentModelProviderOpenRouter
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		SandboxMode:      "danger-full-access",
		OpenRouterAPIKey: "openrouter-secret",
	}, nil, nil, nil)

	profile, err := executor.resolveRuntimeProfile(&model.Agent{Provider: &provider})
	if err != nil {
		t.Fatalf("resolve runtime profile: %v", err)
	}
	payload, err := executor.buildConfigArtifact(&ExecutionContext{}, profile, "on-request")
	if err != nil {
		t.Fatalf("build config artifact: %v", err)
	}

	var decoded struct {
		SandboxMode string `toml:"sandbox_mode"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded.SandboxMode != "danger-full-access" {
		t.Fatalf("expected explicit sandbox override, got %q", decoded.SandboxMode)
	}
}

func TestBuildCodexConfigArtifactIncludesOpenAIExecutionConfig(t *testing.T) {
	provider := model.AgentModelProviderOpenAI
	modelName := "gpt-5.4"
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:   "gpt-5.4",
		OpenAIAPIKey:   "openai-secret",
		OpenAIAuthMode: codexOpenAIAuthModeAPIKey,
	}, nil, nil, nil)

	profile, err := executor.resolveRuntimeProfile(&model.Agent{
		Provider:        &provider,
		Model:           &modelName,
		ExecutionConfig: model.JSONBlob(`{"reasoning_effort":"high","service_tier":"fast"}`),
	})
	if err != nil {
		t.Fatalf("resolve runtime profile: %v", err)
	}
	payload, err := executor.buildConfigArtifact(&ExecutionContext{}, profile, "on-request")
	if err != nil {
		t.Fatalf("build config artifact: %v", err)
	}

	var decoded struct {
		Model                string `toml:"model"`
		ModelReasoningEffort string `toml:"model_reasoning_effort"`
		ServiceTier          string `toml:"service_tier"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded.Model != "gpt-5.4" {
		t.Fatalf("expected model to be preserved, got %q", decoded.Model)
	}
	if decoded.ModelReasoningEffort != "high" {
		t.Fatalf("expected reasoning effort to be preserved, got %q", decoded.ModelReasoningEffort)
	}
	if decoded.ServiceTier != "fast" {
		t.Fatalf("expected service tier to be preserved, got %q", decoded.ServiceTier)
	}
}

func TestNormalizeCodexSandboxMode(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"read-only":           "read-only",
		"readonly":            "read-only",
		"workspace-write":     "workspace-write",
		"workspace_write":     "workspace-write",
		"danger-full-access":  "danger-full-access",
		"danger_full_access":  "danger-full-access",
		"danger":              "danger-full-access",
		" something-unknown ": "",
	}

	for input, want := range cases {
		if got := normalizeCodexSandboxMode(input); got != want {
			t.Fatalf("normalizeCodexSandboxMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCodexApprovalPolicyForInteractiveRunUsesOnRequest(t *testing.T) {
	run := &model.AgentRun{InvocationMode: model.InvocationModeInteractive}
	if got := codexApprovalPolicyForRun(run); got != "on-request" {
		t.Fatalf("expected interactive codex approval policy on-request, got %q", got)
	}
}

func TestPendingResponseRequestIDFailsFastWhenReplayDoesNotArrive(t *testing.T) {
	previousTimeout := codexPendingReplayGraceTimeout
	codexPendingReplayGraceTimeout = time.Millisecond
	defer func() {
		codexPendingReplayGraceTimeout = previousTimeout
	}()

	host := &codexSessionHost{
		execCtx: &ExecutionContext{Context: context.Background()},
		run:     &model.AgentRun{ID: "run-1"},
	}
	client := &codexAppServerClient{
		lines: make(chan codexRPCMessage),
		done:  make(chan error, 1),
	}
	pending := &codexPendingRequest{
		Kind:         codexPendingRequestKindCommandApproval,
		RequestID:    "7",
		RequestIDRaw: json.RawMessage(`7`),
	}

	_, err := host.pendingResponseRequestID(context.Background(), client, pending)
	if err == nil {
		t.Fatal("expected replay timeout to fail")
	}
	if !strings.Contains(err.Error(), "did not replay the pending command_execution") {
		t.Fatalf("expected protocol replay error, got %v", err)
	}
	if !strings.Contains(err.Error(), "still-running thread") {
		t.Fatalf("expected protocol explanation in error, got %v", err)
	}
}

func TestPendingResponsePayloadForSignalReturnsFollowupInputForRequestChanges(t *testing.T) {
	host := &codexSessionHost{}
	pending := &codexPendingRequest{
		Kind:    codexPendingRequestKindCommandApproval,
		Payload: json.RawMessage(`{"command":"git commit"}`),
	}

	response, followupInput, err := host.pendingResponsePayloadForSignal(pending, &LiveExecutionResumeSignal{
		Intent:  model.AgentRunResumeIntentRequestChanges,
		Content: "Please adjust the commit message.",
	})
	if err != nil {
		t.Fatalf("pendingResponsePayloadForSignal returned error: %v", err)
	}
	decoded, ok := response.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %#v", response)
	}
	if got := strings.TrimSpace(fmt.Sprint(decoded["decision"])); got != "cancel" {
		t.Fatalf("expected cancel decision, got %q", got)
	}
	if followupInput != "Please adjust the commit message." {
		t.Fatalf("expected followup input to be preserved, got %q", followupInput)
	}
}

func TestShouldStartCodexFollowupTurnAllowsCompletedAndInterruptedTurns(t *testing.T) {
	for _, status := range []string{"completed", "interrupted"} {
		if !shouldStartCodexFollowupTurn(&codexTurn{Status: status}, "Please revise the auth path.") {
			t.Fatalf("expected follow-up turn to start for status %q", status)
		}
	}
	if shouldStartCodexFollowupTurn(&codexTurn{Status: "failed"}, "Please revise the auth path.") {
		t.Fatal("did not expect follow-up turn to start for failed turns")
	}
	if shouldStartCodexFollowupTurn(&codexTurn{Status: "completed"}, "") {
		t.Fatal("did not expect follow-up turn without input")
	}
}

func TestAppendInteractivePlainTextQuestionInputRequestCreatesStructuredPause(t *testing.T) {
	result := &ExecutionResult{
		AssistantText: strings.TrimSpace(`
1. When both api_key and token are present but conflict, should auth prioritize token or allow either one to pass?
2. For pipeline_auth_duration_seconds, do you want result labels or no labels at all?
3. For requests with no credentials, should behavior stay 401 Unauthorized or continue downstream?
`),
	}

	appendInteractivePlainTextQuestionInputRequest(result)

	request := ExtractLatestHumanInputRequest(result.ToolInvocations)
	if request == nil {
		t.Fatal("expected a structured human-input request")
	}
	if len(request.Questions) != 3 {
		t.Fatalf("expected 3 questions, got %#v", request.Questions)
	}
	if request.Questions[0].Question != "When both api_key and token are present but conflict, should auth prioritize token or allow either one to pass?" {
		t.Fatalf("unexpected first question %#v", request.Questions[0])
	}
}

func TestAppendInteractivePlainTextQuestionInputRequestIgnoresNormalCompletionText(t *testing.T) {
	result := &ExecutionResult{
		AssistantText: "Implemented the auth changes and added tests. Anything else?",
	}

	appendInteractivePlainTextQuestionInputRequest(result)

	if request := ExtractLatestHumanInputRequest(result.ToolInvocations); request != nil {
		t.Fatalf("did not expect a structured human-input request, got %#v", request)
	}
}

func TestRequestedModelIDReturnsEmptyWhenAgentModelIsUnset(t *testing.T) {
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{DefaultModel: "gpt-5-mini"}, nil, nil, nil)
	if got := executor.requestedModelID(&model.Agent{}); got != "" {
		t.Fatalf("expected empty requested model, got %q", got)
	}
}

func TestUpsertProviderEnvForOpenRouterDoesNotInjectOpenAIKeys(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "inherited-openai-key")
	t.Setenv("OPENAI_BASE_URL", "https://api.openai.example")
	t.Setenv("OPENROUTER_API_KEY", "inherited-openrouter-key")
	t.Setenv("OPENROUTER_BASE_URL", "https://openrouter.example")

	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:      "gpt-5-mini",
		OpenAIAPIKey:      "openai-secret",
		OpenRouterAPIKey:  "openrouter-secret",
		OpenRouterBaseURL: "https://openrouter.ai/api/v1",
	}, nil, nil, nil)
	env := executor.buildBaseEnv()
	env = executor.upsertProviderEnv(env, model.AgentModelProviderOpenRouter)

	sawOpenRouterKey := false
	for _, entry := range env {
		switch entry {
		case "OPENAI_API_KEY=openai-secret",
			"OPENAI_API_KEY=inherited-openai-key",
			"OPENAI_BASE_URL=https://api.openai.example",
			"OPENROUTER_BASE_URL=https://openrouter.example":
			t.Fatalf("did not expect inherited OpenAI/OpenRouter env entry in session env: %q", entry)
		case "OPENROUTER_API_KEY=openrouter-secret":
			sawOpenRouterKey = true
		}
	}
	if !sawOpenRouterKey {
		t.Fatal("expected openrouter API key to be injected for custom provider auth")
	}
}

func TestResolveProviderDefaultsToOpenAIWhenManagedOAuthConfigured(t *testing.T) {
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:              "gpt-5-mini",
		OpenAIAuthMode:            codexOpenAIAuthModeOAuth,
		EnableManagedChatGPTOAuth: true,
		ChatGPTAccessToken:        "token",
		ChatGPTAccountID:          "account-123",
		OpenRouterAPIKey:          "openrouter-secret",
	}, nil, nil, nil)

	if got := executor.resolveProvider(&model.Agent{}); got != model.AgentModelProviderOpenAI {
		t.Fatalf("expected OpenAI to remain the default provider when managed OAuth is configured, got %q", got)
	}
}

func TestBuildCodexConfigArtifactForOAuthForcesChatGPTLogin(t *testing.T) {
	provider := model.AgentModelProviderOpenAI
	modelName := "gpt-5-mini"
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:              "gpt-5-mini",
		OpenAIBaseURL:             "https://api.openai.example",
		OpenAIAuthMode:            codexOpenAIAuthModeOAuth,
		EnableManagedChatGPTOAuth: true,
		ChatGPTAccessToken:        "token",
		ChatGPTAccountID:          "account-123",
		ChatGPTPlanType:           "pro",
	}, nil, nil, nil)

	profile, err := executor.resolveRuntimeProfile(&model.Agent{
		Provider: &provider,
		Model:    &modelName,
	})
	if err != nil {
		t.Fatalf("resolve runtime profile: %v", err)
	}
	payload, err := executor.buildConfigArtifact(&ExecutionContext{}, profile, "on-request")
	if err != nil {
		t.Fatalf("build config artifact: %v", err)
	}
	loginPayload, err := executor.loginPayloadForProfile(profile)
	if err != nil {
		t.Fatalf("build login payload: %v", err)
	}

	var decoded struct {
		Model             string `toml:"model"`
		ModelProvider     string `toml:"model_provider"`
		ForcedLoginMethod string `toml:"forced_login_method"`
		OpenAIBaseURL     string `toml:"openai_base_url"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded.Model != "gpt-5-mini" {
		t.Fatalf("expected model to be preserved, got %q", decoded.Model)
	}
	if decoded.ModelProvider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected model provider %q, got %q", model.AgentModelProviderOpenAI, decoded.ModelProvider)
	}
	if decoded.ForcedLoginMethod != codexForcedLoginMethodChat {
		t.Fatalf("expected forced login method %q, got %q", codexForcedLoginMethodChat, decoded.ForcedLoginMethod)
	}
	if decoded.OpenAIBaseURL != "" {
		t.Fatalf("expected OAuth mode to rely on Codex's ChatGPT backend, got openai_base_url=%q", decoded.OpenAIBaseURL)
	}
	if loginPayload["type"] != "chatgptAuthTokens" {
		t.Fatalf("expected chatgptAuthTokens login payload, got %#v", loginPayload["type"])
	}
	if loginPayload["accessToken"] != "token" || loginPayload["chatgptAccountId"] != "account-123" || loginPayload["chatgptPlanType"] != "pro" {
		t.Fatalf("expected managed ChatGPT auth payload, got %#v", loginPayload)
	}
}

func TestExtractCodexEventFailureFromTurnFailedStream(t *testing.T) {
	stdout := strings.Join([]string{
		`{"type":"thread.started","thread_id":"abc"}`,
		`{"type":"turn.started"}`,
		`{"type":"error","message":"Model provider rejected the request"}`,
		`{"type":"turn.failed","error":{"message":"Rate limit exceeded"}}`,
	}, "\n")

	summary, failed := extractCodexEventFailure(stdout)
	if !failed {
		t.Fatal("expected turn.failed stream to be treated as a failure")
	}
	for _, snippet := range []string{"Model provider rejected the request", "Rate limit exceeded"} {
		if !strings.Contains(summary, snippet) {
			t.Fatalf("expected failure summary to contain %q, got %q", snippet, summary)
		}
	}
}

func TestIsUnhandledCodexServerRequest(t *testing.T) {
	if !isUnhandledCodexServerRequest(codexRPCMessage{
		ID:     json.RawMessage(`60`),
		Method: "item/tool/call",
	}) {
		t.Fatal("expected unknown JSON-RPC request with an id to be treated as unhandled")
	}

	if isUnhandledCodexServerRequest(codexRPCMessage{
		ID:     json.RawMessage(`61`),
		Method: "item/tool/requestUserInput",
	}) {
		t.Fatal("expected pause requests to be excluded from the unhandled-request guard")
	}

	if isUnhandledCodexServerRequest(codexRPCMessage{
		Method: "turn/started",
	}) {
		t.Fatal("expected notifications without ids to remain stream notifications")
	}
}

func TestUnsupportedCodexServerRequestError(t *testing.T) {
	err := unsupportedCodexServerRequestError(codexRPCMessage{
		ID:     json.RawMessage(`62`),
		Method: "mcpServer/elicitation/request",
	})
	if err == nil {
		t.Fatal("expected an error for unsupported server requests")
	}
	for _, snippet := range []string{"mcpServer/elicitation/request", "waiting for a client response"} {
		if !strings.Contains(err.Error(), snippet) {
			t.Fatalf("expected error to contain %q, got %v", snippet, err)
		}
	}
}
