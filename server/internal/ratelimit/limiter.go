// Package ratelimit provides shared, fail-open Redis request admission counters.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config sets per-principal minute ceilings. Zero disables that ceiling.
type Config struct {
	RequestsPerMinute  int
	ExpensivePerMinute int
}

// Limiter shares counters across replicas without storing credentials in keys.
type Limiter struct {
	client      *redis.Client
	config      Config
	now         func() time.Time
	lastWarning atomic.Int64
}

var increment = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], 120)
return count
`)

// New uses the application's Redis pool with a short admission-check timeout.
func New(client *redis.Client, config Config) *Limiter {
	if client != nil {
		client = client.WithTimeout(150 * time.Millisecond)
	}
	return &Limiter{client: client, config: config, now: time.Now}
}

// Allow checks one minute bucket; unavailable Redis fails open, with bounded logging.
// Scope and subject must come from authentication, never a submitted token or workspace header.
func (l *Limiter) Allow(ctx context.Context, scope, subject string, expensive bool) bool {
	if l == nil || l.client == nil || subject == "" {
		return true
	}
	limit, class := l.config.RequestsPerMinute, "requests"
	if expensive {
		limit, class = l.config.ExpensivePerMinute, "expensive"
	}
	if limit <= 0 {
		return true
	}
	now := l.now().Unix()
	key := fmt.Sprintf("auth:rl:%s:%s:%x:%d", scope, class, sha256.Sum256([]byte(subject)), now/60)
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	count, err := increment.Run(ctx, l.client, []string{key}).Int64()
	if err != nil {
		last := l.lastWarning.Load()
		if now-last >= 60 && l.lastWarning.CompareAndSwap(last, now) {
			slog.WarnContext(ctx, "authenticated rate limiting unavailable; allowing requests", "error", err)
		}
		return true
	}
	return count <= int64(limit)
}
