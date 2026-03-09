package temporalapp

const (
	QueueAgentEngineer    = "agent-engineer"
	QueueAgentPlanner     = "agent-planner"
	QueueAgentReviewer    = "agent-reviewer"
	QueueAgentSupport     = "agent-support"
	QueueAutomation       = "automation-default"
	WorkflowSignalApprove = "ApproveRun"
	WorkflowSignalHandoff = "HandoffRun"
)

// QueueConfig describes a shared Temporal task queue and its expected concurrency.
type QueueConfig struct {
	Name        string
	Concurrency int
}

// SharedQueues lists the queue topology used by the shared runner pools.
func SharedQueues() []QueueConfig {
	return []QueueConfig{
		{Name: QueueAgentEngineer, Concurrency: 1},
		{Name: QueueAgentPlanner, Concurrency: 8},
		{Name: QueueAgentReviewer, Concurrency: 8},
		{Name: QueueAgentSupport, Concurrency: 4},
		{Name: QueueAutomation, Concurrency: 4},
	}
}

// QueueForProfile maps a capability profile to its shared runner queue.
func QueueForProfile(profile string) string {
	switch profile {
	case "engineer":
		return QueueAgentEngineer
	case "product_planner", "planner", "orchestrator":
		return QueueAgentPlanner
	case "reviewer", "reviewer_tester":
		return QueueAgentReviewer
	case "support":
		return QueueAgentSupport
	default:
		return QueueAutomation
	}
}
