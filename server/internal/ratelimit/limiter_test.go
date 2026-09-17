package ratelimit

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedAdmissionIsolationAndWindows(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: -1})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	first := New(client, Config{RequestsPerMinute: 2, ExpensivePerMinute: 1})
	second := New(client, first.config)
	now := time.Unix(120, 0)
	first.now = func() time.Time { return now }
	second.now = first.now
	ctx := context.Background()
	if !first.Allow(ctx, "api", "alice", false) || !second.Allow(ctx, "api", "alice", false) || first.Allow(ctx, "api", "alice", false) {
		t.Fatal("replicas did not share budget")
	}
	if !first.Allow(ctx, "api", "bob", false) || !first.Allow(ctx, "mcp", "alice", false) {
		t.Fatal("unrelated principal or surface throttled")
	}
	if !first.Allow(ctx, "api", "alice", true) || second.Allow(ctx, "api", "alice", true) {
		t.Fatal("expensive budget not independent/shared")
	}
	for _, key := range mini.Keys() {
		if mini.TTL(key) <= 0 {
			t.Fatal("counter has no expiry")
		}
	}
	now = now.Add(time.Minute)
	if !first.Allow(ctx, "api", "alice", false) {
		t.Fatal("new minute denied")
	}
	mini.FastForward(3 * time.Minute)
	if len(mini.Keys()) != 0 {
		t.Fatal("counters did not expire")
	}
	mini.Close()
	if !first.Allow(ctx, "api", "alice", false) {
		t.Fatal("Redis outage must fail open")
	}
}
func TestConcurrentAdmission(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	limiter := New(client, Config{RequestsPerMinute: 10})
	limiter.now = func() time.Time { return time.Unix(120, 0) }
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow(context.Background(), "api", "alice", false) {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 10 {
		t.Fatalf("admitted %d, want 10", admitted.Load())
	}
}
func TestDisabledAndMissingRedis(t *testing.T) {
	for _, l := range []*Limiter{nil, New(nil, Config{RequestsPerMinute: 1}), New(nil, Config{})} {
		if !l.Allow(context.Background(), "api", "alice", false) {
			t.Fatal("disabled limiter rejected")
		}
	}
}
