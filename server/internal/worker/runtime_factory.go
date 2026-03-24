package worker

import "github.com/helpin-ai/helpin/server/internal/repository"

// NewDefaultRuntimeRegistry wires the supported runtime adapters for shared runners.
func NewDefaultRuntimeRegistry(
	opencodePath string,
	anthropicAPIKey string,
	anthropicBaseURL string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterBaseURL string,
	braveSearchAPIKey string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *RuntimeRegistry {
	braveSearchClient := NewBraveSearchClient(braveSearchAPIKey)
	modelFactory := &EinoModelFactory{
		AnthropicAPIKey: anthropicAPIKey,
		OpenAIAPIKey:    openAIAPIKey,
		OpenAIBaseURL:   openAIBaseURL,
		OpenRouterKey:   openRouterAPIKey,
		OpenRouterURL:   openRouterBaseURL,
	}
	opencodeAdapter := NewOpenCodeExecutor("opencode", opencodePath, anthropicAPIKey, anthropicBaseURL, openAIAPIKey, openAIBaseURL, openRouterAPIKey, openRouterBaseURL, runRepo, artifactRepo)
	// native_sdk is the only in-process SDK-backed runtime exposed today.
	// A real terminal-backed claude_code runtime can be added later as a distinct adapter.
	nativeAdapter := NewEinoExecutor("native_sdk", modelFactory, braveSearchClient, runRepo, artifactRepo)
	return NewRuntimeRegistry(opencodeAdapter, nativeAdapter)
}
