package agentcontract

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ArtifactContext contains persisted artifacts supplied to prompt assembly.
type ArtifactContext struct {
	Entries []ArtifactContextEntry
}

// ArtifactContextEntry is one persisted artifact supplied to an agent prompt.
type ArtifactContextEntry struct {
	Label        string
	Source       string
	Status       string
	Format       string
	Content      string
	PreserveFull bool
}

// WorkflowConfig contains host-side limits and prompt additions passed to the runtime.
type WorkflowConfig struct {
	MaxIterations   int
	TimeoutMinutes  int
	AllowedCommands []string
	HandoffState    string
	CommandTimeout  time.Duration
	ExtraPrompt     string
}

const (
	defaultWorkflowMaxIterations  = model.MaxNativeToolSteps
	defaultWorkflowTimeoutMinutes = 30
	defaultWorkflowCommandTimeout = 2 * time.Minute
)

// DefaultWorkflowConfigForAgent returns host-side runtime defaults for an agent.
func DefaultWorkflowConfigForAgent(_ *model.Agent) *WorkflowConfig {
	return &WorkflowConfig{
		MaxIterations:  defaultWorkflowMaxIterations,
		TimeoutMinutes: defaultWorkflowTimeoutMinutes,
		CommandTimeout: defaultWorkflowCommandTimeout,
	}
}
