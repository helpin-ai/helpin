package websocket

// Publisher is a thin wrapper for services to broadcast events.
// It is nil-safe: calling Publish on a nil publisher is a no-op.
type Publisher struct {
	hub *Hub
}

// NewPublisher creates a new publisher.
func NewPublisher(hub *Hub) *Publisher {
	return &Publisher{hub: hub}
}

// Publish broadcasts an event asynchronously. Safe to call on a nil receiver.
func (p *Publisher) Publish(event Event) {
	if p == nil {
		return
	}
	go p.hub.Broadcast(event)
}
