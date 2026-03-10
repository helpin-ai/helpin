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
// events on the "ws_events" channel to the WebSocket hub.
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
	slog.Info("pg listener started on channel ws_events")

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}

		var event Event
		if err := json.Unmarshal([]byte(n.Payload), &event); err != nil {
			slog.Warn("pg listener: invalid event payload", "error", err, "payload", n.Payload)
			continue
		}
		l.hub.Broadcast(event)
	}
}
