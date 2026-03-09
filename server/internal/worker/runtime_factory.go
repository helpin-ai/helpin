package worker

import "github.com/helpin-ai/helpin/server/internal/repository"

// NewDefaultRuntimeRegistry wires the supported runtime adapters for shared runners.
func NewDefaultRuntimeRegistry(
	anthropicAPIKey string,
	braveSearchAPIKey string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *RuntimeRegistry {
	claudeClient := NewClaudeClient(anthropicAPIKey)
	braveSearchClient := NewBraveSearchClient(braveSearchAPIKey)
	nativeAdapter := NewExecutor("native_claude", claudeClient, braveSearchClient, runRepo, artifactRepo)
	claudeCodeAdapter := NewExecutor("claude_code", claudeClient, braveSearchClient, runRepo, artifactRepo)
	return NewRuntimeRegistry(nativeAdapter, claudeCodeAdapter)
}
