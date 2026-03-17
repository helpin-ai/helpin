package websocket

import (
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"
)

// PGPublisher broadcasts workspace-scoped events via pg_notify.
// Drop-in replacement for Publisher when running outside the API server process.
type PGPublisher struct {
	db *gorm.DB
}

// NewPGPublisher creates a new PG NOTIFY-based event publisher.
func NewPGPublisher(db *gorm.DB) *PGPublisher {
	return &PGPublisher{db: db}
}

// Publish sends an event via pg_notify('ws_events', ...). Safe to call on a nil receiver.
func (p *PGPublisher) Publish(event Event) {
	if p == nil {
		return
	}
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		slog.Error("pg_publisher: marshal failed", "error", err)
		return
	}
	if err := p.db.Exec("SELECT pg_notify('ws_events', ?)", string(jsonBytes)).Error; err != nil {
		slog.Error("pg_publisher: pg_notify failed", "error", err)
	}
}
