package config

import (
	"strings"
	"testing"
)

func setRequiredConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "secret")
}

func TestLoadDefaultsCommandRouterToOpenRouterGeminiFlashLite(t *testing.T) {
	setRequiredConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CommandRouterLLMProvider != "openrouter" {
		t.Fatalf("unexpected command router provider: %q", cfg.CommandRouterLLMProvider)
	}
	if cfg.CommandRouterLLMModel != "google/gemini-3.1-flash-lite" {
		t.Fatalf("unexpected command router model: %q", cfg.CommandRouterLLMModel)
	}
	if string(cfg.CommandRouterOpenRouterProviderOptions) != `{"order":["google-vertex/global"],"allow_fallbacks":false}` {
		t.Fatalf("unexpected provider options: %s", string(cfg.CommandRouterOpenRouterProviderOptions))
	}
}

func TestLoadAllowsCommandRouterDefaultsToBeOverridden(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("COMMAND_ROUTER_LLM_PROVIDER", "anthropic")
	t.Setenv("COMMAND_ROUTER_LLM_MODEL", "claude-sonnet-4-6")
	t.Setenv("COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS", `{"order":["openai"],"allow_fallbacks":true}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CommandRouterLLMProvider != "anthropic" {
		t.Fatalf("unexpected command router provider: %q", cfg.CommandRouterLLMProvider)
	}
	if cfg.CommandRouterLLMModel != "claude-sonnet-4-6" {
		t.Fatalf("unexpected command router model: %q", cfg.CommandRouterLLMModel)
	}
	if string(cfg.CommandRouterOpenRouterProviderOptions) != `{"order":["openai"],"allow_fallbacks":true}` {
		t.Fatalf("unexpected provider options: %s", string(cfg.CommandRouterOpenRouterProviderOptions))
	}
}

func TestLoadParsesCommandRouterOpenRouterProviderOptions(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS", `{"order":["openai"],"allow_fallbacks":false}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if string(cfg.CommandRouterOpenRouterProviderOptions) != `{"order":["openai"],"allow_fallbacks":false}` {
		t.Fatalf("unexpected provider options: %s", string(cfg.CommandRouterOpenRouterProviderOptions))
	}
}

func TestLoadSetsDefaultCodexHelpinMCPBridgePath(t *testing.T) {
	setRequiredConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CodexHelpinMCPBridgePath == "" {
		t.Fatal("expected Codex Helpin MCP bridge path default")
	}
}

func TestLoadExpandsCodexHelpinMCPBridgePath(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("PWD", "/tmp/helpin-server")
	t.Setenv("CODEX_HELPIN_MCP_BRIDGE_PATH", "$PWD/bin/helpin-mcp-bridge")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CodexHelpinMCPBridgePath != "/tmp/helpin-server/bin/helpin-mcp-bridge" {
		t.Fatalf("expected expanded bridge path, got %q", cfg.CodexHelpinMCPBridgePath)
	}
}

func TestLoadDefaultsCodexHelpinAPIBaseURLToLocalAPI(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("APP_BASE_URL", "https://frontend.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CodexHelpinAPIBaseURL != "http://127.0.0.1:9090/api" {
		t.Fatalf("expected local API base URL, got %q", cfg.CodexHelpinAPIBaseURL)
	}
}

func TestLoadAllowsCodexHelpinAPIBaseURLOverride(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CODEX_HELPIN_API_BASE_URL", " http://helpin-server-svc:8080/api/ ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CodexHelpinAPIBaseURL != "http://helpin-server-svc:8080/api" {
		t.Fatalf("expected override API base URL, got %q", cfg.CodexHelpinAPIBaseURL)
	}
}

func TestLoadRejectsInvalidCommandRouterOpenRouterProviderOptions(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS", `{not-json}`)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS") {
		t.Fatalf("expected named config error, got %v", err)
	}
}

func TestLoadRejectsNonObjectCommandRouterOpenRouterProviderOptions(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS", `["openai"]`)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS must be a JSON object") {
		t.Fatalf("expected object config error, got %v", err)
	}
}
