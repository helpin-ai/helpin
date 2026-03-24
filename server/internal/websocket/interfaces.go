package websocket

// EventPublisher broadcasts workspace-scoped entity events.
// Publisher can fan out directly in-process or publish through JetStream across processes.
type EventPublisher interface {
	Publish(event Event)
}
