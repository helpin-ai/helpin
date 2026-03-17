package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

// relayEnvelope wraps an Event with origin pod metadata for cross-pod deduplication.
type relayEnvelope struct {
	OriginPod string `json:"origin_pod"`
	Event     Event  `json:"event"`
}

// RedisRelay is a bidirectional Redis Pub/Sub bridge for cross-pod WebSocket
// event broadcasting. It manages dynamic per-workspace channel subscriptions
// with ref-counting so pods only subscribe to channels with active local clients.
//
// For publish-only usage (e.g. Temporal worker), create with NewRedisEventPublisher.
type RedisRelay struct {
	rdb        *redis.Client
	hub        *Hub   // nil for publish-only mode (Temporal worker)
	podID      string
	pubsub     *redis.PubSub
	mu         sync.Mutex
	wsRefCount map[string]int // workspaceID → number of local clients
}

// NewRedisRelay creates a relay that can both publish and subscribe.
// hub must not be nil — use NewRedisEventPublisher for publish-only mode.
func NewRedisRelay(rdb *redis.Client, hub *Hub, podID string) *RedisRelay {
	ps := rdb.Subscribe(context.Background()) // initial empty subscription
	return &RedisRelay{
		rdb:        rdb,
		hub:        hub,
		podID:      podID,
		pubsub:     ps,
		wsRefCount: make(map[string]int),
	}
}

// NewRedisEventPublisher creates a publish-only relay for processes that
// need to emit events but have no local Hub (e.g. Temporal worker).
// hub is nil, Start() must NOT be called. Only Publish() is used.
func NewRedisEventPublisher(rdb *redis.Client, podID string) *RedisRelay {
	return &RedisRelay{
		rdb:   rdb,
		podID: podID,
		// hub, pubsub, wsRefCount are nil — publish-only
	}
}

// Start subscribes to the global channel and begins the receive loop.
// Called as a goroutine on startup. Blocks until ctx is cancelled.
// Only valid for relays created with NewRedisRelay (not NewRedisEventPublisher).
func (r *RedisRelay) Start(ctx context.Context) {
	if r.hub == nil || r.pubsub == nil {
		slog.Error("RedisRelay.Start called on publish-only relay — this is a bug")
		return
	}

	// Subscribe to the global channel for events without a workspace_id.
	if err := r.pubsub.Subscribe(ctx, "ws:events:global"); err != nil {
		slog.Error("redis relay: subscribe to global channel", "error", err)
		return
	}
	slog.Info("redis relay started", "pod_id", r.podID)

	ch := r.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("redis relay shutting down", "pod_id", r.podID)
			r.pubsub.Close()
			return
		case msg, ok := <-ch:
			if !ok {
				slog.Info("redis relay channel closed", "pod_id", r.podID)
				return
			}
			r.handleMessage(msg)
		}
	}
}

// handleMessage processes a single Redis Pub/Sub message, forwarding it
// to the local Hub if it originated from a different pod.
func (r *RedisRelay) handleMessage(msg *redis.Message) {
	var env relayEnvelope
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		slog.Warn("redis relay: unmarshal envelope", "error", err, "channel", msg.Channel)
		return
	}

	// Skip events that originated on this pod — they were already broadcast locally.
	if env.OriginPod == r.podID {
		return
	}

	slog.Debug("redis relay: forwarding event from remote pod",
		"origin_pod", env.OriginPod,
		"entity", env.Event.Entity,
		"action", env.Event.Action,
		"workspace_id", env.Event.WorkspaceID,
	)

	r.hub.Broadcast(env.Event)
}

// Publish sends an event to the appropriate Redis channel.
// Events with a WorkspaceID go to the workspace-specific channel;
// events without go to the global channel.
func (r *RedisRelay) Publish(ctx context.Context, event Event) error {
	channel := "ws:events:global"
	if event.WorkspaceID != "" {
		channel = "ws:events:" + event.WorkspaceID
	}

	envelope, err := json.Marshal(relayEnvelope{OriginPod: r.podID, Event: event})
	if err != nil {
		return err
	}

	return r.rdb.Publish(ctx, channel, envelope).Err()
}

// EnsureWorkspaceSubscription subscribes to a workspace channel when the
// first local client for that workspace connects. Thread-safe, ref-counted.
func (r *RedisRelay) EnsureWorkspaceSubscription(workspaceID string) {
	if r.pubsub == nil {
		return // publish-only mode
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.wsRefCount[workspaceID]++
	if r.wsRefCount[workspaceID] == 1 {
		// First client for this workspace on this pod — subscribe.
		channel := "ws:events:" + workspaceID
		if err := r.pubsub.Subscribe(context.Background(), channel); err != nil {
			slog.Error("redis relay: subscribe to workspace channel",
				"error", err, "workspace_id", workspaceID)
		} else {
			slog.Debug("redis relay: subscribed to workspace channel",
				"workspace_id", workspaceID, "channel", channel)
		}
	}
}

// ReleaseWorkspaceSubscription unsubscribes from a workspace channel when
// the last local client for that workspace disconnects. Thread-safe.
func (r *RedisRelay) ReleaseWorkspaceSubscription(workspaceID string) {
	if r.pubsub == nil {
		return // publish-only mode
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.wsRefCount[workspaceID]--
	if r.wsRefCount[workspaceID] <= 0 {
		delete(r.wsRefCount, workspaceID)
		channel := "ws:events:" + workspaceID
		if err := r.pubsub.Unsubscribe(context.Background(), channel); err != nil {
			slog.Error("redis relay: unsubscribe from workspace channel",
				"error", err, "workspace_id", workspaceID)
		} else {
			slog.Debug("redis relay: unsubscribed from workspace channel",
				"workspace_id", workspaceID, "channel", channel)
		}
	}
}

// RefCount returns the current ref count for a workspace. Used in tests.
func (r *RedisRelay) RefCount(workspaceID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.wsRefCount[workspaceID]
}
