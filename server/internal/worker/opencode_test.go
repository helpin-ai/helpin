package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestOpenCodeResolveModelIDDefaultsToAnthropicSonnet(t *testing.T) {
	executor := NewOpenCodeExecutor("opencode", "opencode", "", "", "", "", "", "", nil, nil)

	got := executor.resolveModelID(&model.Agent{})
	want := "anthropic/claude-sonnet-4-6"
	if got != want {
		t.Fatalf("expected default model %q, got %q", want, got)
	}
}

func TestBuildOpenCodeConfigContentUsesTeampulseAgentAndPermissions(t *testing.T) {
	execCtx := &ExecutionContext{
		Agent: &model.Agent{
			Name:         "Engineer",
			PresetKey:    model.AgentPresetCodeBuilder,
			AllowedTools: []byte(`["write_file","run_command"]`),
		},
		Task:   &model.PMTask{Name: "Implement notification preferences"},
		Config: DefaultWorkflowConfig(),
	}

	payload, err := buildOpenCodeConfigContent(execCtx, "openai/gpt-5-mini", "system prompt", nil)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	var decoded struct {
		Model      string `json:"model"`
		SmallModel string `json:"small_model"`
		Agent      map[string]struct {
			Model      string         `json:"model"`
			Mode       string         `json:"mode"`
			Prompt     string         `json:"prompt"`
			Permission map[string]any `json:"permission"`
		} `json:"agent"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if decoded.Model != "openai/gpt-5-mini" {
		t.Fatalf("expected top-level model to be set, got %q", decoded.Model)
	}
	if decoded.SmallModel != "openai/gpt-5-mini" {
		t.Fatalf("expected small_model to mirror model, got %q", decoded.SmallModel)
	}

	agentCfg, ok := decoded.Agent["teampulse-engineer"]
	if !ok {
		t.Fatalf("expected teampulse-engineer agent in config")
	}
	if agentCfg.Model != "openai/gpt-5-mini" {
		t.Fatalf("expected agent model override, got %q", agentCfg.Model)
	}
	if agentCfg.Mode != "primary" {
		t.Fatalf("expected agent mode primary, got %q", agentCfg.Mode)
	}
	if agentCfg.Prompt != "system prompt" {
		t.Fatalf("expected prompt to be injected, got %q", agentCfg.Prompt)
	}

	editPermission, ok := agentCfg.Permission["edit"].(string)
	if !ok || editPermission != "allow" {
		t.Fatalf("expected edit permission allow, got %#v", agentCfg.Permission["edit"])
	}

	bashPermission, ok := agentCfg.Permission["bash"].(map[string]any)
	if !ok {
		t.Fatalf("expected bash permission rules, got %#v", agentCfg.Permission["bash"])
	}
	if bashPermission["*"] != "deny" {
		t.Fatalf("expected wildcard bash deny rule, got %#v", bashPermission["*"])
	}
	if bashPermission["go"] != "allow" || bashPermission["go *"] != "allow" {
		t.Fatalf("expected go allow rules, got %#v", bashPermission)
	}
	if _, ok := bashPermission["git *"]; ok {
		t.Fatalf("expected generic git wildcard to be absent, got %#v", bashPermission["git *"])
	}
	if bashPermission["git status"] != "allow" || bashPermission["git diff *"] != "allow" {
		t.Fatalf("expected read-only git allow rules, got %#v", bashPermission)
	}
}

func TestOpenCodeResolveModelIDStripsProviderPrefixFromStoredModel(t *testing.T) {
	executor := NewOpenCodeExecutor("opencode", "opencode", "", "", "", "", "", "", nil, nil)
	provider := model.AgentModelProviderOpenRouter
	modelName := "openrouter/qwen/qwen3.5-122b-a10b"

	got := executor.resolveModelID(&model.Agent{
		Provider: &provider,
		Model:    &modelName,
	})
	want := "openrouter/qwen/qwen3.5-122b-a10b"
	if got != want {
		t.Fatalf("expected normalized model id %q, got %q", want, got)
	}
}

func TestBuildOpenCodeConfigContentAddsOpenRouterModelAndBaseURL(t *testing.T) {
	provider := model.AgentModelProviderOpenRouter
	modelName := "qwen/qwen3.5-122b-a10b"
	execCtx := &ExecutionContext{
		Agent: &model.Agent{
			Provider: &provider,
			Model:    &modelName,
		},
	}

	providerConfig := buildOpenCodeProviderConfig(execCtx.Agent, "", "https://openrouter.ai/api/v1")
	payload, err := buildOpenCodeConfigContent(execCtx, "openrouter/qwen/qwen3.5-122b-a10b", "system prompt", providerConfig)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	var decoded struct {
		Provider map[string]struct {
			Options struct {
				BaseURL string `json:"baseURL"`
			} `json:"options"`
			Models map[string]json.RawMessage `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	openRouter, ok := decoded.Provider["openrouter"]
	if !ok {
		t.Fatalf("expected openrouter provider config")
	}
	if openRouter.Options.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected openrouter baseURL to be set, got %q", openRouter.Options.BaseURL)
	}
	if _, ok := openRouter.Models["qwen/qwen3.5-122b-a10b"]; !ok {
		t.Fatalf("expected openrouter custom model to be registered, got %#v", openRouter.Models)
	}
}

func TestBuildOpenCodeConfigContentAddsAnthropicBaseURL(t *testing.T) {
	provider := model.AgentModelProviderAnthropic
	modelName := "anthropic/claude-sonnet-4-6"
	execCtx := &ExecutionContext{
		Agent: &model.Agent{
			Provider: &provider,
			Model:    &modelName,
		},
	}

	providerConfig := buildOpenCodeProviderConfig(execCtx.Agent, "https://api.anthropic.com/v1", "")
	payload, err := buildOpenCodeConfigContent(execCtx, "anthropic/claude-sonnet-4-6", "system prompt", providerConfig)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	var decoded struct {
		Provider map[string]struct {
			Options struct {
				BaseURL string `json:"baseURL"`
			} `json:"options"`
			Models map[string]json.RawMessage `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	anthropic, ok := decoded.Provider["anthropic"]
	if !ok {
		t.Fatalf("expected anthropic provider config")
	}
	if anthropic.Options.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected anthropic baseURL to be set, got %q", anthropic.Options.BaseURL)
	}
	if _, ok := anthropic.Models["claude-sonnet-4-6"]; !ok {
		t.Fatalf("expected anthropic model to be registered, got %#v", anthropic.Models)
	}
}

func TestBuildOpenCodeUserPromptRequiresImplementationForEngineerStory(t *testing.T) {
	result := buildOpenCodeUserPrompt(&ExecutionContext{
		Agent: &model.Agent{AllowedTools: []byte(`["write_file"]`)},
		Task: &model.PMTask{Name: "Story"},
	}, "Please implement the story.")

	if result == "Please implement the story." {
		t.Fatalf("expected engineer user prompt to include implementation guardrails")
	}
}

func TestParseOpenCodeJSONEventTextAndTokens(t *testing.T) {
	parsed, err := parseOpenCodeJSONEvent(`{"type":"text","part":{"id":"msg_1","text":"{\"status\":\"success\"}"}}`)
	if err != nil {
		t.Fatalf("parse text event: %v", err)
	}
	if parsed.DisplayLine != "{\"status\":\"success\"}" {
		t.Fatalf("expected display line to match text content, got %q", parsed.DisplayLine)
	}
	if parsed.ResponseText != "{\"status\":\"success\"}" {
		t.Fatalf("expected response text to match text content, got %q", parsed.ResponseText)
	}
	if parsed.TokensUsed != 0 {
		t.Fatalf("expected no tokens on text event, got %d", parsed.TokensUsed)
	}
	if parsed.EventError != "" {
		t.Fatalf("expected no event error, got %q", parsed.EventError)
	}

	parsed, err = parseOpenCodeJSONEvent(`{"type":"step_finish","part":{"stepID":"step_1","stopReason":"end_turn","tokens":{"input":120,"output":45,"reasoning":10,"cache":{"read":8,"write":3}}}}`)
	if err != nil {
		t.Fatalf("parse step_finish event: %v", err)
	}
	if !strings.Contains(parsed.DisplayLine, "tokens=186") {
		t.Fatalf("expected display line to include aggregated tokens, got %q", parsed.DisplayLine)
	}
	if parsed.ResponseText != "" {
		t.Fatalf("expected no response text on step_finish event, got %q", parsed.ResponseText)
	}
	if parsed.TokensUsed != 186 {
		t.Fatalf("expected aggregated tokens 186, got %d", parsed.TokensUsed)
	}
	if parsed.EventError != "" {
		t.Fatalf("expected no event error, got %q", parsed.EventError)
	}
	if parsed.Usage.InputTokens != 131 || parsed.Usage.OutputTokens != 55 {
		t.Fatalf("expected usage to preserve prompt/completion split, got %+v", parsed.Usage)
	}
}

func TestParseOpenCodeJSONEventError(t *testing.T) {
	parsed, err := parseOpenCodeJSONEvent(`{"type":"error","message":"provider quota exceeded"}`)
	if err != nil {
		t.Fatalf("parse error event: %v", err)
	}
	if !strings.Contains(parsed.DisplayLine, "provider quota exceeded") {
		t.Fatalf("expected display line to include error message, got %q", parsed.DisplayLine)
	}
	if parsed.ResponseText != "" {
		t.Fatalf("expected no response text on error event, got %q", parsed.ResponseText)
	}
	if parsed.TokensUsed != 0 {
		t.Fatalf("expected no tokens on error event, got %d", parsed.TokensUsed)
	}
	if parsed.EventError != "provider quota exceeded" {
		t.Fatalf("expected event error to be recorded, got %q", parsed.EventError)
	}
}

func TestConsumeOpenCodeJSONStreamCollectsResponseAndTokens(t *testing.T) {
	payload := strings.Join([]string{
		`{"type":"step_start","part":{"title":"Planning"}}`,
		`{"type":"tool_use","part":{"title":"Read backend/internal/models/user.go"}}`,
		`{"type":"text","part":{"text":"{\"status\":\"success\"}"}}`,
		`{"type":"step_finish","part":{"stopReason":"end_turn","tokens":{"input":100,"output":20}}}`,
	}, "\n")

	collector := newOpenCodeStreamCollector("run_12345678", &openCodeArtifactWriter{}, nil)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go consumeOpenCodeJSONStream(strings.NewReader(payload), context.Background(), collector, &wg, errCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("consume json stream: %v", err)
		}
	}

	collector.FlushPending(context.Background(), false)
	stdoutText, stderrText := collector.Outputs()
	if !strings.Contains(stdoutText, "Step started: Planning") {
		t.Fatalf("expected stdout text to include step start, got %q", stdoutText)
	}
	if !strings.Contains(stdoutText, "Tool: Read backend/internal/models/user.go") {
		t.Fatalf("expected stdout text to include tool use, got %q", stdoutText)
	}
	if !strings.Contains(stdoutText, "{\"status\":\"success\"}") {
		t.Fatalf("expected stdout text to include response text, got %q", stdoutText)
	}
	if !strings.Contains(stdoutText, "tokens=120") {
		t.Fatalf("expected stdout text to include token summary, got %q", stdoutText)
	}
	if stderrText != "" {
		t.Fatalf("expected no stderr text, got %q", stderrText)
	}
	if collector.ResponseText() != "{\"status\":\"success\"}" {
		t.Fatalf("expected response text to be captured, got %q", collector.ResponseText())
	}
	if collector.TokensUsed() != 120 {
		t.Fatalf("expected aggregated tokens 120, got %d", collector.TokensUsed())
	}
	if collector.Usage().InputTokens != 100 || collector.Usage().OutputTokens != 20 {
		t.Fatalf("expected usage to be captured, got %+v", collector.Usage())
	}
}

func TestConsumeOpenCodeJSONStreamFallsBackToPlainText(t *testing.T) {
	payload := "plain progress line\n"

	collector := newOpenCodeStreamCollector("run_12345678", &openCodeArtifactWriter{}, nil)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go consumeOpenCodeJSONStream(strings.NewReader(payload), context.Background(), collector, &wg, errCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("consume json stream: %v", err)
		}
	}

	stdoutText, _ := collector.Outputs()
	if !strings.Contains(stdoutText, "plain progress line") {
		t.Fatalf("expected fallback stdout text, got %q", stdoutText)
	}
}

func TestConsumeOpenCodeJSONStreamEmitsStructuredExecutionEvents(t *testing.T) {
	payload := strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"id":"text_1","messageID":"msg_1","type":"text","text":"Hello"}}}`,
		`{"type":"message.part.delta","properties":{"sessionID":"sess_1","messageID":"msg_1","partID":"text_1","field":"text","delta":" world"}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"reason_1","messageID":"msg_1","type":"reasoning","text":"Inspecting file","time":{"start":10,"end":20}}}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"tool_1","messageID":"msg_1","type":"tool","callID":"call_1","tool":"read_file","state":{"status":"completed","input":{"path":"README.md"},"output":"README contents","time":{"start":1000,"end":1500}}}}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"step_1","messageID":"msg_1","type":"step-finish","reason":"end_turn","tokens":{"input":10,"output":5,"reasoning":2,"cache":{"read":1,"write":1}}}}}`,
	}, "\n")

	var events []ExecutionEvent
	collector := newOpenCodeStreamCollector("run_12345678", &openCodeArtifactWriter{}, func(event ExecutionEvent) {
		events = append(events, event)
	})
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go consumeOpenCodeJSONStream(strings.NewReader(payload), context.Background(), collector, &wg, errCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("consume json stream: %v", err)
		}
	}

	got := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Type {
		case "assistant_message_delta", "reasoning_message_delta":
			got = append(got, event.Type+":"+event.Text)
		case "tool_call_started", "tool_call_result", "tool_call_finished":
			got = append(got, event.Type+":"+event.ToolCallID)
		default:
			got = append(got, event.Type)
		}
	}

	want := []string{
		"assistant_message_started",
		"assistant_message_delta:Hello",
		"assistant_message_delta: world",
		"reasoning_message_started",
		"reasoning_message_delta:Inspecting file",
		"reasoning_message_completed",
		"tool_call_started:call_1",
		"tool_call_result:call_1",
		"tool_call_finished:call_1",
		"activity_delta",
		"assistant_message_completed",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d execution events, got %d: %#v", len(want), len(got), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected event sequence at %d: got %q want %q (full=%#v)", index, got[index], want[index], got)
		}
	}

	if collector.ResponseText() != "Hello world" {
		t.Fatalf("expected collector response text to match assistant output, got %q", collector.ResponseText())
	}
	if collector.TokensUsed() != 19 {
		t.Fatalf("expected total tokens 19, got %d", collector.TokensUsed())
	}
	if collector.Usage().InputTokens != 12 || collector.Usage().OutputTokens != 7 {
		t.Fatalf("expected usage 12/7, got %+v", collector.Usage())
	}
	invocations := collector.ToolInvocations()
	if len(invocations) != 1 {
		t.Fatalf("expected a single tool invocation, got %#v", invocations)
	}
	if invocations[0].ToolName != "read_file" {
		t.Fatalf("expected tool invocation to preserve tool name, got %#v", invocations[0])
	}
	if invocations[0].OutputSummary != "README contents" {
		t.Fatalf("expected tool invocation to preserve output summary, got %#v", invocations[0])
	}
	if invocations[0].DurationMs != 500 {
		t.Fatalf("expected tool duration 500ms, got %#v", invocations[0])
	}
	if string(invocations[0].Input) != `{"path":"README.md"}` {
		t.Fatalf("expected tool input to be normalized JSON, got %s", string(invocations[0].Input))
	}
}

func TestPersistEngineerWorkspaceCommitsAndPushesChanges(t *testing.T) {
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

	executor := NewOpenCodeExecutor("opencode", "opencode", "", "", "", "", "", "", nil, nil)
	var pushedBranch string
	var pushedSHA string
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "main",
		WorkingBranch: "tp-123-implement",
		Agent:         &model.Agent{AllowedTools: []byte(`["write_file","commit_and_push","open_pr"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Implement notification preferences"},
		OnGitPush: func(branch, sha string) error {
			pushedBranch = branch
			pushedSHA = sha
			return nil
		},
	}

	if err := executor.persistEngineerWorkspace(execCtx, &model.AgentRun{}, &openCodeArtifactWriter{}); err != nil {
		t.Fatalf("persist engineer workspace: %v", err)
	}

	if pushedBranch != "tp-123-implement" {
		t.Fatalf("expected pushed branch to be recorded, got %q", pushedBranch)
	}
	if strings.TrimSpace(pushedSHA) == "" {
		t.Fatalf("expected pushed sha to be recorded")
	}

	remoteSHA := strings.TrimSpace(runGitCmd(t, tempDir, "git", "--git-dir", remoteDir, "rev-parse", "refs/heads/tp-123-implement"))
	if remoteSHA != pushedSHA {
		t.Fatalf("expected remote SHA %q to match pushed SHA %q", remoteSHA, pushedSHA)
	}

	status := strings.TrimSpace(runGitCmd(t, workDir, "git", "status", "--porcelain"))
	if status != "" {
		t.Fatalf("expected clean working tree after persistence, got %q", status)
	}
}

func TestPersistEngineerWorkspacePushesExistingLocalCommit(t *testing.T) {
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
	runGitCmd(t, workDir, "git", "add", "README.md")
	runGitCmd(t, workDir, "git", "commit", "-m", "agent created local commit")
	localSHA := strings.TrimSpace(runGitCmd(t, workDir, "git", "rev-parse", "HEAD"))

	executor := NewOpenCodeExecutor("opencode", "opencode", "", "", "", "", "", "", nil, nil)
	var pushedBranch string
	var pushedSHA string
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "main",
		WorkingBranch: "tp-123-implement",
		Agent:         &model.Agent{AllowedTools: []byte(`["write_file","commit_and_push","open_pr"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Implement notification preferences"},
		OnGitPush: func(branch, sha string) error {
			pushedBranch = branch
			pushedSHA = sha
			return nil
		},
	}

	if err := executor.persistEngineerWorkspace(execCtx, &model.AgentRun{}, &openCodeArtifactWriter{}); err != nil {
		t.Fatalf("persist engineer workspace with existing commit: %v", err)
	}

	if pushedBranch != "tp-123-implement" {
		t.Fatalf("expected pushed branch to be recorded, got %q", pushedBranch)
	}
	if pushedSHA != localSHA {
		t.Fatalf("expected pushed sha %q to match local sha %q", pushedSHA, localSHA)
	}

	remoteSHA := strings.TrimSpace(runGitCmd(t, tempDir, "git", "--git-dir", remoteDir, "rev-parse", "refs/heads/tp-123-implement"))
	if remoteSHA != localSHA {
		t.Fatalf("expected remote SHA %q to match local SHA %q", remoteSHA, localSHA)
	}

	status := strings.TrimSpace(runGitCmd(t, workDir, "git", "status", "--porcelain"))
	if status != "" {
		t.Fatalf("expected clean working tree after persistence, got %q", status)
	}
}

func TestPersistEngineerWorkspaceAllowsAutonomousReviewRunWithoutRepoChanges(t *testing.T) {
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

	executor := NewOpenCodeExecutor("opencode", "opencode", "", "", "", "", "", "", nil, nil)
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "task-branch",
		WorkingBranch: "task-branch",
		Agent:         &model.Agent{PresetKey: model.AgentPresetReviewAgent, AllowedTools: []byte(`["write_file","run_command","apply_patch"]`)},
		Task:          &model.PMTask{DisplayID: 123, Name: "Review same-branch work"},
	}

	if err := executor.persistEngineerWorkspace(execCtx, &model.AgentRun{InvocationMode: model.InvocationModeAutonomous}, &openCodeArtifactWriter{}); err != nil {
		t.Fatalf("persistEngineerWorkspace returned error for autonomous no-change review run: %v", err)
	}
	if execCtx.LocalGitCommit != nil {
		t.Fatalf("did not expect pushed local git metadata, got %#v", execCtx.LocalGitCommit)
	}
}

func TestDetectCommittedEngineerChangeUsesRemoteWorkingBranchBaseline(t *testing.T) {
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

	writeTestFile(t, filepath.Join(workDir, "README.md"), "hello\nreview fix\n")
	runGitCmd(t, workDir, "git", "add", "README.md")
	runGitCmd(t, workDir, "git", "commit", "-m", "lens local follow-up")
	localSHA := strings.TrimSpace(runGitCmd(t, workDir, "git", "rev-parse", "HEAD"))

	change, err := detectCommittedEngineerChange(&ExecutionContext{
		Context:       context.Background(),
		WorkDir:       workDir,
		BaseBranch:    "task-branch",
		WorkingBranch: "task-branch",
	})
	if err != nil {
		t.Fatalf("detectCommittedEngineerChange returned error: %v", err)
	}
	if change == nil {
		t.Fatal("expected local same-branch commit to be detected against origin/task-branch")
	}
	if change.CommitSHA != localSHA {
		t.Fatalf("expected detected sha %q, got %q", localSHA, change.CommitSHA)
	}
	if len(change.ChangedFiles) != 1 || change.ChangedFiles[0] != "README.md" {
		t.Fatalf("unexpected changed files %#v", change.ChangedFiles)
	}
}

func configureGitIdentity(t *testing.T, dir string) {
	t.Helper()
	runGitCmd(t, dir, "git", "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "git", "config", "user.name", "Test User")
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGitCmd(t *testing.T, dir string, command string, args ...string) string {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", command, args, err, string(output))
	}
	return string(output)
}

func TestWaitForOpenCodeRepoChangesDetectsDelayedChanges(t *testing.T) {
	repoDir := t.TempDir()
	runGitCmd(t, repoDir, "git", "init")
	configureGitIdentity(t, repoDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "hello\n")
	runGitCmd(t, repoDir, "git", "add", "README.md")
	runGitCmd(t, repoDir, "git", "commit", "-m", "initial")
	runGitCmd(t, repoDir, "git", "branch", "-M", "main")

	execCtx := &ExecutionContext{
		Context:    context.Background(),
		WorkDir:    repoDir,
		BaseBranch: "main",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(repoDir, "changed.txt"), []byte("changed\n"), 0o644)
	}()

	changed, err := waitForOpenCodeRepoChanges(execCtx, 500*time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("wait for repo changes: %v", err)
	}
	if !changed {
		t.Fatal("expected delayed filesystem change to be detected")
	}
}
