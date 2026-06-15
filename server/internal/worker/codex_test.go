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

func TestCodexEventMapperTracksCachedInputTokens(t *testing.T) {
	mapper := newCodexEventMapper(&ExecutionContext{}, &model.AgentRun{}, nil)
	payload, err := json.Marshal(codexThreadTokenUsageUpdatedNotification{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		TokenUsage: codexThreadTokenUsage{
			Last: codexTokenUsageBreakdown{
				CachedInputTokens: 12,
				InputTokens:       44,
				OutputTokens:      9,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal token usage payload: %v", err)
	}

	if err := mapper.HandleNotification(context.Background(), "thread/tokenUsage/updated", payload); err != nil {
		t.Fatalf("handle token usage notification: %v", err)
	}

	result := mapper.Result()
	if result == nil {
		t.Fatal("expected execution result")
	}
	if result.Usage.CachedInputTokens != 12 || result.Usage.InputTokens != 44 || result.Usage.OutputTokens != 9 {
		t.Fatalf("unexpected usage %+v", result.Usage)
	}
}

func TestApplyExecutionUsageToRunIncludesCachedInputTokens(t *testing.T) {
	run := &model.AgentRun{}

	applyExecutionUsageToRun(run, ExecutionUsage{
		CachedInputTokens: 7,
		InputTokens:       31,
		OutputTokens:      5,
	})

	if run.CachedInputTokens != 7 {
		t.Fatalf("expected cached input tokens to persist, got %d", run.CachedInputTokens)
	}
	if run.InputTokens != 31 || run.OutputTokens != 5 {
		t.Fatalf("unexpected input/output usage on run: %+v", run)
	}
	if run.TokensUsed != 36 {
		t.Fatalf("expected tokens_used to remain input+output, got %d", run.TokensUsed)
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
		"Use the local shell and file-editing capabilities available in this workspace directly.",
		"Do not rely on Helpin-specific tool wrappers or orchestration commands",
		"Do not push the branch or open a pull request from this runtime",
		"must make concrete repository changes",
		"text-only analysis with no file modifications is a failed outcome",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected codex prompt to contain %q, got:\n%s", snippet, prompt)
		}
	}
}

func TestBuildCodexRuntimeInstructionsInteractiveDoesNotRequireFileChanges(t *testing.T) {
	instructions := buildCodexRuntimeInstructions(&ExecutionContext{
		Agent: &model.Agent{
			PresetKey:    model.AgentPresetCodeBuilder,
			AllowedTools: []byte(`["write_file","run_command"]`),
		},
		Task: &model.PMTask{Name: "Implement metrics"},
	}, &model.AgentRun{InvocationMode: model.InvocationModeInteractive})

	for _, expected := range []string{
		"This is an interactive run. Continue from the latest human reply instead of restarting from scratch.",
		"Make repository changes when they materially advance the task, but they are not required on every turn.",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected interactive instructions to contain %q, got:\n%s", expected, instructions)
		}
	}
	for _, unexpected := range []string{
		"must make concrete repository changes",
		"text-only analysis with no file modifications is a failed outcome",
	} {
		if strings.Contains(instructions, unexpected) {
			t.Fatalf("did not expect interactive instructions to contain %q, got:\n%s", unexpected, instructions)
		}
	}
}

func TestBuildCodexRuntimeInstructionsAutonomousReviewStaysGeneric(t *testing.T) {
	instructions := buildCodexRuntimeInstructions(&ExecutionContext{
		Agent: &model.Agent{
			PresetKey:    model.AgentPresetReviewAgent,
			AllowedTools: []byte(`["write_file","run_command"]`),
		},
		SkillPolicy: SkillPolicy{
			InteractionContracts: []SkillInteractionContract{
				{
					Kind:   InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]SkillInteractionTransport{
						"codex": {Type: InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
			},
		},
		Task: &model.PMTask{Name: "Review metrics recorder"},
	}, &model.AgentRun{InvocationMode: model.InvocationModeAutonomous})

	if !strings.Contains(instructions, "This is an autonomous run. Make durable progress on the assigned task before stopping.") {
		t.Fatalf("expected autonomous generic instructions, got:\n%s", instructions)
	}
	for _, unexpected := range []string{
		"must make concrete repository changes",
		"text-only analysis with no file modifications is a failed outcome",
	} {
		if strings.Contains(instructions, unexpected) {
			t.Fatalf("did not expect autonomous review instructions to contain %q, got:\n%s", unexpected, instructions)
		}
	}
	for _, expected := range []string{
		"fenced code block labeled `helpin-review`",
		"`review_checkpoint_v1` schema",
		"\"phase\":\"...\"",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected autonomous review instructions to contain %q, got:\n%s", expected, instructions)
		}
	}
}

func TestBuildCodexRuntimeInstructionsUsesInteractionContractForAnySkill(t *testing.T) {
	instructions := buildCodexRuntimeInstructions(&ExecutionContext{
		Agent: &model.Agent{
			PresetKey:    model.AgentPresetCodeBuilder,
			AllowedTools: []byte(`["write_file","run_command"]`),
		},
		SkillPolicy: SkillPolicy{
			InteractionContracts: []SkillInteractionContract{
				{
					Kind:   InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]SkillInteractionTransport{
						"codex": {Type: InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
			},
		},
		Task: &model.PMTask{Name: "Review metrics recorder"},
	}, &model.AgentRun{InvocationMode: model.InvocationModeInteractive})

	for _, expected := range []string{
		"fenced code block labeled `helpin-review`",
		"`review_checkpoint_v1` schema",
		"\"phase\":\"...\"",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected Codex contract instructions to contain %q, got:\n%s", expected, instructions)
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

func TestCodexPersistEngineerWorkspaceAllowsInteractiveRunWithoutRepoChanges(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	runGitCmd(t, tempDir, "git", "init", "--bare", remoteDir)

	seedDir := filepath.Join(tempDir, "seed")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, seedDir)
	configureGitIdentity(t, seedDir)
	writeTestFile(t, filepath.Join(seedDir, "README.md"), "hello\n")
	runGitCmd(t, seedDir, "git", "add", "README.md")
	runGitCmd(t, seedDir, "git", "commit", "-m", "initial commit")
	runGitCmd(t, seedDir, "git", "branch", "-M", "task-branch")
	runGitCmd(t, seedDir, "git", "push", "-u", "origin", "task-branch")

	workDir := filepath.Join(tempDir, "work")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, workDir)
	configureGitIdentity(t, workDir)
	runGitCmd(t, workDir, "git", "checkout", "-B", "task-branch", "origin/task-branch")

	executor := NewCodexExecutor("codex", CodexRuntimeConfig{}, nil, nil, nil)
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "task-branch",
		WorkingBranch: "task-branch",
		Agent:         &model.Agent{PresetKey: model.AgentPresetReviewAgent, AllowedTools: []byte(`["write_file","run_command","apply_patch"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Review same-branch work"},
	}

	err := executor.persistEngineerWorkspace(
		execCtx,
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		&codexArtifactWriter{},
		"Review closed.",
		"assistant_turn",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("persistEngineerWorkspace returned error for interactive no-change review run: %v", err)
	}
	if execCtx.LocalGitCommit != nil {
		t.Fatalf("did not expect local git commit metadata, got %#v", execCtx.LocalGitCommit)
	}
}

func TestCodexPersistEngineerWorkspaceAllowsAutonomousReviewRunWithoutRepoChanges(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	runGitCmd(t, tempDir, "git", "init", "--bare", remoteDir)

	seedDir := filepath.Join(tempDir, "seed")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, seedDir)
	configureGitIdentity(t, seedDir)
	writeTestFile(t, filepath.Join(seedDir, "README.md"), "hello\n")
	runGitCmd(t, seedDir, "git", "add", "README.md")
	runGitCmd(t, seedDir, "git", "commit", "-m", "initial commit")
	runGitCmd(t, seedDir, "git", "branch", "-M", "task-branch")
	runGitCmd(t, seedDir, "git", "push", "-u", "origin", "task-branch")

	workDir := filepath.Join(tempDir, "work")
	runGitCmd(t, tempDir, "git", "clone", remoteDir, workDir)
	configureGitIdentity(t, workDir)
	runGitCmd(t, workDir, "git", "checkout", "-B", "task-branch", "origin/task-branch")

	executor := NewCodexExecutor("codex", CodexRuntimeConfig{}, nil, nil, nil)
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "task-branch",
		WorkingBranch: "task-branch",
		Agent:         &model.Agent{PresetKey: model.AgentPresetReviewAgent, AllowedTools: []byte(`["write_file","run_command","apply_patch"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Review same-branch work"},
	}

	err := executor.persistEngineerWorkspace(
		execCtx,
		&model.AgentRun{InvocationMode: model.InvocationModeAutonomous},
		&codexArtifactWriter{},
		"Review closed.",
		"assistant_turn",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("persistEngineerWorkspace returned error for autonomous no-change review run: %v", err)
	}
	if execCtx.LocalGitCommit != nil {
		t.Fatalf("did not expect local git commit metadata, got %#v", execCtx.LocalGitCommit)
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
	runGitCmd(t, repoDir, "git", "init", "-b", "master")
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
	modelName := "gpt-5.5"
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:   "gpt-5.5",
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
		WebSearch            string `toml:"web_search"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded.Model != "gpt-5.5" {
		t.Fatalf("expected model to be preserved, got %q", decoded.Model)
	}
	if decoded.ModelReasoningEffort != "high" {
		t.Fatalf("expected reasoning effort to be preserved, got %q", decoded.ModelReasoningEffort)
	}
	if decoded.ServiceTier != "fast" {
		t.Fatalf("expected service tier to be preserved, got %q", decoded.ServiceTier)
	}
	if decoded.WebSearch != "live" {
		t.Fatalf("expected Codex backend web search to default to live, got %q", decoded.WebSearch)
	}
}

func TestBuildCodexConfigArtifactRequiresHelpinMCPServer(t *testing.T) {
	bridgePath := filepath.Join(t.TempDir(), "helpin-mcp-bridge")
	if err := os.WriteFile(bridgePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write bridge executable: %v", err)
	}
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:             "gpt-5-mini",
		OpenAIAPIKey:             "openai-secret",
		HelpinAPIBaseURL:         "http://helpin.local",
		HelpinRunToolTokenSecret: "tool-secret",
		HelpinMCPBridgePath:      bridgePath,
	}, nil, nil, nil)
	profile, err := executor.resolveRuntimeProfile(&model.Agent{})
	if err != nil {
		t.Fatalf("resolve runtime profile: %v", err)
	}
	payload, err := executor.buildConfigArtifact(&ExecutionContext{
		RunID:       "run-1",
		WorkspaceID: "ws-1",
	}, profile, "on-request")
	if err != nil {
		t.Fatalf("build config artifact: %v", err)
	}

	var decoded struct {
		MCPServers map[string]struct {
			Command                  string            `toml:"command"`
			Env                      map[string]string `toml:"env"`
			Required                 bool              `toml:"required"`
			StartupTimeoutSec        int               `toml:"startup_timeout_sec"`
			ToolTimeoutSec           int               `toml:"tool_timeout_sec"`
			DefaultToolsApprovalMode string            `toml:"default_tools_approval_mode"`
		} `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	helpin, ok := decoded.MCPServers["helpin"]
	if !ok {
		t.Fatalf("expected helpin MCP server in config: %#v", decoded.MCPServers)
	}
	if helpin.Command != bridgePath {
		t.Fatalf("expected bridge command, got %q", helpin.Command)
	}
	if !helpin.Required {
		t.Fatal("expected Helpin MCP server to be required")
	}
	if helpin.StartupTimeoutSec != 15 || helpin.ToolTimeoutSec != 120 {
		t.Fatalf("unexpected MCP timeouts: startup=%d tool=%d", helpin.StartupTimeoutSec, helpin.ToolTimeoutSec)
	}
	if helpin.DefaultToolsApprovalMode != "approve" {
		t.Fatalf("expected approve default tool mode, got %q", helpin.DefaultToolsApprovalMode)
	}
	if helpin.Env["HELPIN_API_BASE_URL"] != "http://helpin.local/api" || helpin.Env["HELPIN_AGENT_RUN_TOOL_TOKEN"] == "" {
		t.Fatalf("unexpected MCP env: %#v", helpin.Env)
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

func TestCodexApprovalPolicyForAutonomousRunUsesNever(t *testing.T) {
	run := &model.AgentRun{InvocationMode: model.InvocationModeAutonomous}
	if got := codexApprovalPolicyForRun(run); got != "never" {
		t.Fatalf("expected autonomous codex approval policy never, got %q", got)
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

	appendInteractivePlainTextQuestionInputRequest(&ExecutionContext{}, result)

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

	appendInteractivePlainTextQuestionInputRequest(&ExecutionContext{}, result)

	if request := ExtractLatestHumanInputRequest(result.ToolInvocations); request != nil {
		t.Fatalf("did not expect a structured human-input request, got %#v", request)
	}
}

func TestAppendInteractivePlainTextQuestionInputRequestHonorsContractTransport(t *testing.T) {
	result := &ExecutionResult{
		AssistantText: strings.TrimSpace(`
1. Should the retry path keep exponential backoff?
2. Do you want result labels on the auth duration metric?
`),
	}

	appendInteractivePlainTextQuestionInputRequest(&ExecutionContext{
		SkillPolicy: SkillPolicy{
			InteractionContracts: []SkillInteractionContract{
				{
					Kind:   InteractionKindRequestUserInput,
					Schema: "request_user_input_v1",
					Transports: map[string]SkillInteractionTransport{
						"codex": {Type: InteractionTransportTypeToolCall, ToolName: ToolRequestUserInput},
					},
				},
			},
		},
	}, result)

	if request := ExtractLatestHumanInputRequest(result.ToolInvocations); request != nil {
		t.Fatalf("did not expect a structured human-input request when the codex contract is not a runtime bridge, got %#v", request)
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
