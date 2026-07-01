package worker

import "github.com/helpin-ai/helpin/server/internal/repository"

// NewDefaultRuntimeRegistry wires the supported runtime adapters for shared runners.
func NewDefaultRuntimeRegistry(
	opencodePath string,
	codexConfig CodexRuntimeConfig,
	anthropicAPIKey string,
	anthropicBaseURL string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterBaseURL string,
	braveSearchAPIKey string,
	exaSearchAPIKey string,
	crawlerProxyURLs string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	workspaceAuth *CodexWorkspaceAuthStore,
	usagePreflighter AgentRunUsagePreflighter,
	usageRecorder AgentRunUsageRecorder,
) *RuntimeRegistry {
	braveSearchClient := NewBraveSearchClient(braveSearchAPIKey)
	exaSearchClient := NewExaSearchClient(exaSearchAPIKey)
	modelFactory := &EinoModelFactory{
		AnthropicAPIKey: anthropicAPIKey,
		OpenAIAPIKey:    openAIAPIKey,
		OpenAIBaseURL:   openAIBaseURL,
		OpenRouterKey:   openRouterAPIKey,
		OpenRouterURL:   openRouterBaseURL,
	}
	opencodeAdapter := NewOpenCodeExecutor("opencode", opencodePath, anthropicAPIKey, anthropicBaseURL, openAIAPIKey, openAIBaseURL, openRouterAPIKey, openRouterBaseURL, runRepo, artifactRepo, usageRecorder)
	codexAdapter := NewCodexExecutor("codex", codexConfig, runRepo, artifactRepo, workspaceAuth, usageRecorder)
	// native_sdk is the only in-process SDK-backed runtime exposed today.
	// A real terminal-backed claude_code runtime can be added later as a distinct adapter.
	nativeAdapter := NewEinoExecutor(
		"native_sdk",
		modelFactory,
		braveSearchClient,
		exaSearchClient,
		crawlerProxyURLs,
		codexConfig.HelpinAPIBaseURL,
		codexConfig.HelpinRunToolTokenSecret,
		codexConfig.HelpinMCPBridgePath,
		runRepo,
		artifactRepo,
		usageRecorder,
	)
	return NewRuntimeRegistry(
		usagePreflightRuntimeAdapter{adapter: opencodeAdapter, preflighter: usagePreflighter},
		usagePreflightRuntimeAdapter{adapter: codexAdapter, preflighter: usagePreflighter},
		usagePreflightRuntimeAdapter{adapter: nativeAdapter, preflighter: usagePreflighter},
	)
}
