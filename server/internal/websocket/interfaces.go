package websocket

// SessionStreamer sends stream events to clients subscribed to a planning session.
// Hub implements this directly (in-process), RedisSessionStreamer implements it
// cross-process via Redis Pub/Sub.
type SessionStreamer interface {
	SendToSession(sessionID string, event interface{})
}

// EventPublisher broadcasts workspace-scoped entity events.
// Publisher implements this directly (in-process + Redis relay for cross-pod).
type EventPublisher interface {
	Publish(event Event)
}
