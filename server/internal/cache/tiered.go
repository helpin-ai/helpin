package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tiered composes an L1 (in-process) cache in front of an L2 (remote) cache.
// Reads flow L1 → L2 and hits at L2 populate L1 for fast subsequent reads.
// Writes populate both tiers so later reads from any pod hit L2 at minimum.
// Invalidation fans out across pods via a Redis pub/sub channel so every
// pod drops its L1 entries for the invalidated tags.
type Tiered struct {
	l1 *LRU
	l2 Cache

	rdb     *redis.Client
	channel string
	podID   string

	l1TTL time.Duration

	subOnce   sync.Once
	closeOnce sync.Once
	closeCh   chan struct{}
}

// TieredConfig configures a Tiered cache. Channel is the Redis pub/sub channel
// used to broadcast tag invalidations; PodID identifies this pod so it can
// ignore its own broadcasts.
type TieredConfig struct {
	L1      *LRU
	L2      Cache
	Redis   *redis.Client
	Channel string
	PodID   string
	L1TTL   time.Duration
}

// NewTiered returns a composed cache. If cfg.Redis is nil, invalidations are
// still broadcast locally via L1 (single-pod mode).
func NewTiered(cfg TieredConfig) *Tiered {
	if cfg.L1TTL == 0 {
		cfg.L1TTL = 60 * time.Second
	}
	if cfg.Channel == "" {
		cfg.Channel = "cache:invalidate"
	}
	return &Tiered{
		l1:      cfg.L1,
		l2:      cfg.L2,
		rdb:     cfg.Redis,
		channel: cfg.Channel,
		podID:   cfg.PodID,
		l1TTL:   cfg.L1TTL,
		closeCh: make(chan struct{}),
	}
}

// StartInvalidationSubscriber runs a background loop that listens for
// cross-pod invalidation messages. Must be called exactly once after NewTiered.
// Safe to call even when Redis is nil — it becomes a no-op.
func (t *Tiered) StartInvalidationSubscriber(ctx context.Context) {
	t.subOnce.Do(func() {
		if t.rdb == nil {
			return
		}
		go t.runSubscriber(ctx)
	})
}

// Close stops the pub/sub subscriber. Idempotent.
func (t *Tiered) Close() {
	t.closeOnce.Do(func() { close(t.closeCh) })
}

// Get reads from L1, then L2; on an L2 hit it backfills L1.
func (t *Tiered) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if v, ok, err := t.l1.Get(ctx, key); err == nil && ok {
		return v, true, nil
	}
	v, ok, err := t.l2.Get(ctx, key)
	if err != nil || !ok {
		return v, ok, err
	}
	// Backfill L1 without tags — L2 owns authoritative tag membership.
	_ = t.l1.Set(ctx, key, v, t.l1TTL)
	return v, true, nil
}

// Set writes through both tiers. L1 holds a short-TTL copy; L2 owns the
// authoritative entry and tag set. L1's copy is not tagged because a remote
// invalidation broadcast handles cross-pod drops.
func (t *Tiered) Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags ...string) error {
	if err := t.l2.Set(ctx, key, value, ttl, tags...); err != nil {
		return err
	}
	l1TTL := t.l1TTL
	if ttl > 0 && ttl < l1TTL {
		l1TTL = ttl
	}
	return t.l1.Set(ctx, key, value, l1TTL, tags...)
}

// InvalidateTags invalidates in L2, then L1, then broadcasts so peer pods
// also drop L1 entries for these tags.
func (t *Tiered) InvalidateTags(ctx context.Context, tags ...string) error {
	if err := t.l2.InvalidateTags(ctx, tags...); err != nil {
		// Don't bail — still drop L1 so this pod at least stays consistent.
		slog.ErrorContext(ctx, "cache: L2 invalidate failed", "error", err, "tags", tags)
	}
	_ = t.l1.InvalidateTags(ctx, tags...)
	t.publishInvalidation(ctx, tags)
	return nil
}

type invalidationMsg struct {
	Pod  string   `json:"pod"`
	Tags []string `json:"tags"`
}

func (t *Tiered) publishInvalidation(ctx context.Context, tags []string) {
	if t.rdb == nil || len(tags) == 0 {
		return
	}
	payload, err := json.Marshal(invalidationMsg{Pod: t.podID, Tags: tags})
	if err != nil {
		slog.ErrorContext(ctx, "cache: marshal invalidation msg failed", "error", err)
		return
	}
	if err := t.rdb.Publish(ctx, t.channel, payload).Err(); err != nil {
		slog.ErrorContext(ctx, "cache: publish invalidation failed", "error", err, "channel", t.channel)
	}
}

func (t *Tiered) runSubscriber(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("cache: invalidation subscriber panicked", "panic", r)
		}
	}()

	sub := t.rdb.Subscribe(ctx, t.channel)
	defer sub.Close()
	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.closeCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var m invalidationMsg
			if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
				slog.ErrorContext(ctx, "cache: bad invalidation message", "error", err)
				continue
			}
			if m.Pod == t.podID {
				continue // our own broadcast
			}
			_ = t.l1.InvalidateTags(ctx, m.Tags...)
		}
	}
}
