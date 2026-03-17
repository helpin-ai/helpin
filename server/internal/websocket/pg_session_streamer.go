package websocket

import (
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"
)

// PGSessionStreamer sends planning stream events cross-process via pg_notify.
// Used by the temporal-worker to push tokens/tool events to the API server's WS hub.
type PGSessionStreamer struct {
	db *gorm.DB
}

// NewPGSessionStreamer creates a new PG NOTIFY-based session streamer.
func NewPGSessionStreamer(db *gorm.DB) *PGSessionStreamer {
	return &PGSessionStreamer{db: db}
}

// SendToSession publishes a planning stream event via pg_notify('planning_stream', ...).
func (s *PGSessionStreamer) SendToSession(sessionID string, event interface{}) {
	payload := struct {
		SessionID string      `json:"session_id"`
		Event     interface{} `json:"event"`
	}{sessionID, event}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("pg_session_streamer: marshal failed", "error", err, "session_id", sessionID)
		return
	}
	if err := s.db.Exec("SELECT pg_notify('planning_stream', ?)", string(jsonBytes)).Error; err != nil {
		slog.Error("pg_session_streamer: pg_notify failed", "error", err, "session_id", sessionID)
	}
}
