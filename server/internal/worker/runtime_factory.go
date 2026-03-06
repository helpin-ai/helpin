package worker

import "github.com/helpin-ai/helpin/server/internal/repository"

// NewDefaultRuntimeRegistry wires the supported runtime adapters for shared runners.
func NewDefaultRuntimeRegistry(
	anthropicAPIKey string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *RuntimeRegistry {
	claudeClient := NewClaudeClient(anthropicAPIKey)
	nativeAdapter := NewExecutor("native_claude", claudeClient, runRepo, artifactRepo)
	claudeCodeAdapter := NewExecutor("claude_code", claudeClient, runRepo, artifactRepo)
	return NewRuntimeRegistry(nativeAdapter, claudeCodeAdapter)
}
