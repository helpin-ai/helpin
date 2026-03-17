package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// PGListener bridges Postgres LISTEN/NOTIFY to the WebSocket Hub.
// This allows the Temporal worker process (which has no WS clients)
// to broadcast events to the API server's connected browsers by
// writing a pg_notify from the worker and receiving it here.
type PGListener struct {
	dsn string
	hub *Hub
}

// NewPGListener creates a new listener that will forward pg_notify
// events on the "ws_events" and "planning_stream" channels to the WebSocket hub.
func NewPGListener(dsn string, hub *Hub) *PGListener {
	return &PGListener{dsn: dsn, hub: hub}
}

// Start begins listening for Postgres notifications. It blocks until
// ctx is cancelled. Reconnects automatically with exponential backoff.
func (l *PGListener) Start(ctx context.Context) {
	backoff := time.Second
	for {
		if err := l.listen(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("pg listener disconnected, reconnecting", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

func (l *PGListener) listen(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "LISTEN ws_events"); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "LISTEN planning_stream"); err != nil {
		return err
	}
	slog.Info("pg listener started on channels ws_events, planning_stream")

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}

		switch n.Channel {
		case "planning_stream":
			l.handlePlanningStream(n.Payload)
		default:
			l.handleWSEvent(n.Payload)
		}
	}
}

// handleWSEvent routes ws_events notifications to hub.Broadcast (existing behavior).
func (l *PGListener) handleWSEvent(payload string) {
	var event Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		slog.Warn("pg listener: invalid event payload", "error", err, "payload", payload)
		return
	}
	l.hub.Broadcast(event)
}

// handlePlanningStream routes planning_stream notifications to hub.SendToSession.
func (l *PGListener) handlePlanningStream(payload string) {
	var wrapper struct {
		SessionID string          `json:"session_id"`
		Event     json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal([]byte(payload), &wrapper); err != nil {
		slog.Warn("pg listener: invalid planning_stream payload", "error", err, "payload", payload)
		return
	}
	l.hub.SendToSession(wrapper.SessionID, json.RawMessage(wrapper.Event))
}
