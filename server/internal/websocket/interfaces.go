package websocket

// SessionStreamer sends stream events to clients subscribed to a planning session.
// Hub implements this directly in-process, while JetStreamSessionStreamer relays across processes.
type SessionStreamer interface {
	SendToSession(sessionID string, event interface{})
}

// EventPublisher broadcasts workspace-scoped entity events.
// Publisher can fan out directly in-process or publish through JetStream across processes.
type EventPublisher interface {
	Publish(event Event)
}
