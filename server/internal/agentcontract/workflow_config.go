package agentcontract

import (
	"strings"
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
	defaultWorkflowMaxIterations  = 50
	plannerWorkflowMaxIterations  = 300
	askAgentWorkflowMaxIterations = model.MaxNativeToolSteps
	defaultWorkflowTimeoutMinutes = 30
	defaultWorkflowCommandTimeout = 2 * time.Minute
)

// DefaultWorkflowConfigForAgent returns host-side runtime defaults for an agent.
func DefaultWorkflowConfigForAgent(agent *model.Agent) *WorkflowConfig {
	maxIterations := defaultWorkflowMaxIterations
	if agent != nil && strings.TrimSpace(agent.EffectivePresetKey()) == model.AgentPresetAskAgent {
		maxIterations = askAgentWorkflowMaxIterations
	} else if isHighToolBudgetAgent(agent) {
		maxIterations = plannerWorkflowMaxIterations
	}
	return &WorkflowConfig{
		MaxIterations:  maxIterations,
		TimeoutMinutes: defaultWorkflowTimeoutMinutes,
		CommandTimeout: defaultWorkflowCommandTimeout,
	}
}

func isHighToolBudgetAgent(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	if !agent.IsSystem {
		return true
	}
	if strings.TrimSpace(agent.RuntimeKind) != "native_sdk" {
		return false
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner:
		return true
	default:
		return false
	}
}
