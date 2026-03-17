package websocket

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// RedisSessionStreamer sends planning stream events cross-process via Redis Pub/Sub.
// It replaces PGSessionStreamer for the Temporal worker.
type RedisSessionStreamer struct {
	rdb   *redis.Client
	podID string
}

// NewRedisSessionStreamer creates a new Redis-based session streamer.
func NewRedisSessionStreamer(rdb *redis.Client, podID string) *RedisSessionStreamer {
	return &RedisSessionStreamer{rdb: rdb, podID: podID}
}

// SendToSession publishes a planning stream event via Redis Pub/Sub on
// the "planning_stream" channel. The API server's RedisRelay receives
// this and forwards it to the Hub's SendToSession.
func (s *RedisSessionStreamer) SendToSession(sessionID string, event interface{}) {
	payload := struct {
		SessionID string      `json:"session_id"`
		Event     interface{} `json:"event"`
	}{sessionID, event}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("redis_session_streamer: marshal failed", "error", err, "session_id", sessionID)
		return
	}
	if err := s.rdb.Publish(context.Background(), "planning_stream", jsonBytes).Err(); err != nil {
		slog.Error("redis_session_streamer: publish failed", "error", err, "session_id", sessionID)
	}
}
