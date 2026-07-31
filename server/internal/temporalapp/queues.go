package temporalapp

const QueueAutomation = "automation-default"

// QueueConfig describes a Helpin product-automation Temporal task queue.
type QueueConfig struct {
	Name        string
	Concurrency int
}

// SharedQueues lists the Helpin-owned Temporal queue topology.
func SharedQueues() []QueueConfig {
	return []QueueConfig{{Name: QueueAutomation, Concurrency: 4}}
}
