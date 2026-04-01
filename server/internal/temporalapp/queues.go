package temporalapp

const (
	QueueAgentNativeInteractive = "agent-native-interactive"
	QueueAgentNativeAutonomous  = "agent-native-autonomous"
	QueueAgentOpenCode          = "agent-opencode-autonomous"
	QueueAgentCodexAutonomous   = "agent-codex-autonomous"
	QueueAgentCodexInteractive  = "agent-codex-interactive"
	QueueAutomation             = "automation-default"
	WorkflowSignalResume        = "ResumeRun"
	WorkflowSignalApprove       = "ApproveRun"
	WorkflowSignalHandoff       = "HandoffRun"
	WorkflowSignalMessage       = "RunMessage"
)

// QueueConfig describes a shared Temporal task queue and its expected concurrency.
type QueueConfig struct {
	Name        string
	Concurrency int
}

// SharedQueues lists the queue topology used by the shared runner pools.
func SharedQueues() []QueueConfig {
	return []QueueConfig{
		{Name: QueueAgentNativeInteractive, Concurrency: 8},
		{Name: QueueAgentNativeAutonomous, Concurrency: 6},
		{Name: QueueAgentOpenCode, Concurrency: 4},
		{Name: QueueAgentCodexAutonomous, Concurrency: 4},
		{Name: QueueAgentCodexInteractive, Concurrency: 4},
		{Name: QueueAutomation, Concurrency: 4},
	}
}

// QueueForRuntime maps runtime and invocation mode to the shared runner queue.
func QueueForRuntime(runtimeKind, invocationMode string) string {
	switch runtimeKind {
	case "native_sdk":
		if invocationMode == "interactive" {
			return QueueAgentNativeInteractive
		}
		return QueueAgentNativeAutonomous
	case "opencode":
		return QueueAgentOpenCode
	case "codex":
		if invocationMode == "interactive" {
			return QueueAgentCodexInteractive
		}
		return QueueAgentCodexAutonomous
	default:
		return QueueAutomation
	}
}
