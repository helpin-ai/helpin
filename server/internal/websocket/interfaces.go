package websocket

// SessionStreamer sends stream events to clients subscribed to a planning session.
// Hub implements this directly (in-process), PGSessionStreamer implements it cross-process via pg_notify.
type SessionStreamer interface {
	SendToSession(sessionID string, event interface{})
}

// EventPublisher broadcasts workspace-scoped entity events.
// Publisher implements this directly (in-process), PGPublisher implements it cross-process via pg_notify.
type EventPublisher interface {
	Publish(event Event)
}
