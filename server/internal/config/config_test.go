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
	if cfg.CommandRouterLLMModel != "openai/gpt-5.6-luna" {
		t.Fatalf("unexpected command router model: %q", cfg.CommandRouterLLMModel)
	}
	if string(cfg.CommandRouterOpenRouterProviderOptions) != `{"order":["google-vertex/global"],"allow_fallbacks":false}` {
		t.Fatalf("unexpected provider options: %s", string(cfg.CommandRouterOpenRouterProviderOptions))
	}
}

func TestLoadDefaultsQueryExpansionToGPT56Luna(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("QUERY_EXPANSION_MODEL", "")
	t.Setenv("QUERY_EXPANSION_PROVIDER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.QueryExpansionProvider != "openai" {
		t.Fatalf("unexpected query expansion provider: %q", cfg.QueryExpansionProvider)
	}
	if cfg.QueryExpansionModel != "gpt-5.6-luna" {
		t.Fatalf("unexpected query expansion model: %q", cfg.QueryExpansionModel)
	}
	if cfg.QueryExpansionTimeoutMS != 10000 {
		t.Fatalf("unexpected query expansion timeout: %d", cfg.QueryExpansionTimeoutMS)
	}
}

func TestLoadLeavesCRMCompletionRoutesOnBuiltInDefaults(t *testing.T) {
	setRequiredConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CRMLLMProvider != "" || cfg.CRMLLMModel != "" || cfg.CRMLLMOpenRouterProvider != "" ||
		cfg.CRMLLMFallbackProvider != "" || cfg.CRMLLMFallbackModel != "" || cfg.CRMLLMFallbackOpenRouterProvider != "" ||
		cfg.CRMMeetingFallbackProvider != "" || cfg.CRMMeetingFallbackModel != "" || cfg.CRMMeetingFallbackOpenRouterProvider != "" {
		t.Fatalf(
			"unexpected CRM route overrides: primary=%q/%q via %q, fallback=%q/%q via %q, meeting fallback=%q/%q via %q",
			cfg.CRMLLMProvider, cfg.CRMLLMModel, cfg.CRMLLMOpenRouterProvider,
			cfg.CRMLLMFallbackProvider, cfg.CRMLLMFallbackModel, cfg.CRMLLMFallbackOpenRouterProvider,
			cfg.CRMMeetingFallbackProvider, cfg.CRMMeetingFallbackModel, cfg.CRMMeetingFallbackOpenRouterProvider,
		)
	}
}

func TestLoadParsesCRMCompletionRouteOverrides(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CRM_LLM_PROVIDER", " openrouter ")
	t.Setenv("CRM_LLM_MODEL", " anthropic/claude-sonnet-5 ")
	t.Setenv("CRM_LLM_OPENROUTER_PROVIDER", " anthropic ")
	t.Setenv("CRM_LLM_FALLBACK_PROVIDER", " openrouter ")
	t.Setenv("CRM_LLM_FALLBACK_MODEL", " openai/gpt-5.6-luna ")
	t.Setenv("CRM_LLM_FALLBACK_OPENROUTER_PROVIDER", " openai ")
	t.Setenv("CRM_MEETING_LLM_FALLBACK_PROVIDER", " openrouter ")
	t.Setenv("CRM_MEETING_LLM_FALLBACK_MODEL", " google/gemini-3.7-flash ")
	t.Setenv("CRM_MEETING_LLM_FALLBACK_OPENROUTER_PROVIDER", " google-vertex/global ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CRMLLMProvider != "openrouter" || cfg.CRMLLMModel != "anthropic/claude-sonnet-5" || cfg.CRMLLMOpenRouterProvider != "anthropic" {
		t.Fatalf("primary CRM route = %q/%q via %q", cfg.CRMLLMProvider, cfg.CRMLLMModel, cfg.CRMLLMOpenRouterProvider)
	}
	if cfg.CRMLLMFallbackProvider != "openrouter" || cfg.CRMLLMFallbackModel != "openai/gpt-5.6-luna" || cfg.CRMLLMFallbackOpenRouterProvider != "openai" {
		t.Fatalf("fallback CRM route = %q/%q via %q", cfg.CRMLLMFallbackProvider, cfg.CRMLLMFallbackModel, cfg.CRMLLMFallbackOpenRouterProvider)
	}
	if cfg.CRMMeetingFallbackProvider != "openrouter" || cfg.CRMMeetingFallbackModel != "google/gemini-3.7-flash" || cfg.CRMMeetingFallbackOpenRouterProvider != "google-vertex/global" {
		t.Fatalf("meeting fallback route = %q/%q via %q", cfg.CRMMeetingFallbackProvider, cfg.CRMMeetingFallbackModel, cfg.CRMMeetingFallbackOpenRouterProvider)
	}
}

func TestLoadRejectsPartialCRMCompletionRouteOverrides(t *testing.T) {
	for _, test := range []struct {
		name string
		env  string
	}{
		{name: "primary", env: "CRM_LLM_PROVIDER"},
		{name: "fallback", env: "CRM_LLM_FALLBACK_MODEL"},
		{name: "meeting fallback", env: "CRM_MEETING_LLM_FALLBACK_PROVIDER"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setRequiredConfigEnv(t)
			t.Setenv(test.env, "openrouter")
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "must be set together") {
				t.Fatalf("expected paired route error, got %v", err)
			}
		})
	}
}

func TestLoadRejectsOpenRouterSelectionForDirectProvider(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CRM_LLM_PROVIDER", "anthropic")
	t.Setenv("CRM_LLM_MODEL", "claude-sonnet-5")
	t.Setenv("CRM_LLM_OPENROUTER_PROVIDER", "anthropic")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "requires the corresponding LLM provider to be openrouter") {
		t.Fatalf("expected OpenRouter selection error, got %v", err)
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

func TestLoadAgentRuntimeConfig(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("AGENT_RUNTIME_BASE_URL", " https://runtime.internal/ ")
	t.Setenv("AGENT_RUNTIME_SERVICE_TOKEN", " runtime-token ")
	t.Setenv("AGENT_RUNTIME_APP_ID", " helpin-stage ")
	t.Setenv("AGENT_RUNTIME_LAUNCH_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.AgentRuntimeBaseURL != "https://runtime.internal" {
		t.Fatalf("unexpected runtime base URL: %q", cfg.AgentRuntimeBaseURL)
	}
	if cfg.AgentRuntimeServiceToken != "runtime-token" {
		t.Fatalf("unexpected runtime token: %q", cfg.AgentRuntimeServiceToken)
	}
	if cfg.AgentRuntimeAppID != "helpin-stage" {
		t.Fatalf("unexpected runtime app id: %q", cfg.AgentRuntimeAppID)
	}
	if !cfg.AgentRuntimeLaunchEnabled {
		t.Fatal("expected runtime launch flag")
	}
}

func TestLoadGoogleWebRiskAPIKey(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("GOOGLE_WEB_RISK_API_KEY", " web-risk-key ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.GoogleWebRiskAPIKey != "web-risk-key" {
		t.Fatalf("unexpected Google Web Risk API key: %q", cfg.GoogleWebRiskAPIKey)
	}
}

func TestLoadMeetingCaptureProvider(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CRM_MEETING_CAPTURE_PROVIDER", " VEXA ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CRMMeetingCaptureProvider != "vexa" {
		t.Fatalf("unexpected meeting capture provider: %q", cfg.CRMMeetingCaptureProvider)
	}
}

func TestLoadDefaultsMeetingCaptureProviderToRecall(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CRM_MEETING_CAPTURE_PROVIDER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CRMMeetingCaptureProvider != "recall" {
		t.Fatalf("unexpected default meeting capture provider: %q", cfg.CRMMeetingCaptureProvider)
	}
}

func TestLoadRejectsInvalidMeetingCaptureProvider(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("CRM_MEETING_CAPTURE_PROVIDER", "unknown")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CRM_MEETING_CAPTURE_PROVIDER must be recall or vexa") {
		t.Fatalf("expected meeting provider config error, got %v", err)
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
