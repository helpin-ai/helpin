package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
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
		Story: &model.PMStory{Name: "Implement metrics"},
	}, "You are a coding agent.", "Please implement the story.")

	for _, snippet := range []string{
		"running inside the Codex CLI runtime",
		"Do not wait for Helpin-native tool calls",
		"must make concrete repository changes",
		"text-only analysis with no file modifications is a failed outcome",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected codex prompt to contain %q, got:\n%s", snippet, prompt)
		}
	}
}

func TestBuildCodexConfigArtifactUsesRequestedModelOnly(t *testing.T) {
	executor := NewCodexExecutor("codex", "codex", "gpt-5-mini", "", "", "", "", nil, nil)
	payload := executor.buildConfigArtifact(&ExecutionContext{WorkDir: "/tmp/work"}, "gpt-5-mini", model.AgentModelProviderOpenAI, "api_key_forced")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal config artifact: %v", err)
	}
	if decoded["requested_model"] != "gpt-5-mini" {
		t.Fatalf("expected requested_model to be preserved, got %#v", decoded["requested_model"])
	}
	if decoded["model_selection"] != "explicit" {
		t.Fatalf("expected model_selection to note explicit model behavior, got %#v", decoded["model_selection"])
	}
	if decoded["auth_mode"] != "api_key_forced" {
		t.Fatalf("expected auth_mode to note api-key behavior, got %#v", decoded["auth_mode"])
	}
	if _, exists := decoded["model"]; exists {
		t.Fatalf("did not expect config artifact to claim an enforced model: %#v", decoded["model"])
	}
}

func TestRequestedModelIDReturnsEmptyWhenAgentModelIsUnset(t *testing.T) {
	executor := NewCodexExecutor("codex", "codex", "gpt-5-mini", "", "", "", "", nil, nil)
	if got := executor.requestedModelID(&model.Agent{}); got != "" {
		t.Fatalf("expected empty requested model, got %q", got)
	}
}

func TestBuildExecEnvWithoutModelDoesNotInjectAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "inherited-openai-key")
	t.Setenv("OPENAI_BASE_URL", "https://api.openai.example")
	t.Setenv("OPENROUTER_API_KEY", "inherited-openrouter-key")
	t.Setenv("OPENROUTER_BASE_URL", "https://openrouter.example")

	executor := NewCodexExecutor("codex", "codex", "gpt-5-mini", "openai-secret", "", "openrouter-secret", "https://openrouter.ai/api/v1", nil, nil)
	env, cleanup, err := executor.buildExecEnv(context.Background(), &model.Agent{}, false)
	if err != nil {
		t.Fatalf("build exec env: %v", err)
	}
	defer cleanup()

	for _, entry := range env {
		switch entry {
		case "OPENAI_API_KEY=openai-secret",
			"OPENAI_API_KEY=openrouter-secret",
			"OPENAI_API_KEY=inherited-openai-key",
			"OPENAI_BASE_URL=https://api.openai.example",
			"OPENROUTER_API_KEY=inherited-openrouter-key",
			"OPENROUTER_BASE_URL=https://openrouter.example":
			t.Fatalf("did not expect API auth env when model is unset: %q", entry)
		}
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
