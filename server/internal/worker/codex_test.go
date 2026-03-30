package worker

import (
	"errors"
	"strings"
	"testing"

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

func TestBuildCodexConfigArtifactAddsOpenRouterProviderConfig(t *testing.T) {
	provider := model.AgentModelProviderOpenRouter
	modelName := "qwen/qwen3.5-122b-a10b"
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{
		DefaultModel:      "gpt-5-mini",
		OpenRouterAPIKey:  "openrouter-secret",
		OpenRouterBaseURL: "https://openrouter.ai/api/v1",
	}, nil, nil)

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

func TestRequestedModelIDReturnsEmptyWhenAgentModelIsUnset(t *testing.T) {
	executor := NewCodexExecutor("codex", CodexRuntimeConfig{DefaultModel: "gpt-5-mini"}, nil, nil)
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
	}, nil, nil)
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
	}, nil, nil)

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
	}, nil, nil)

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
