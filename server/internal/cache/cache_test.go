package cache

import (
	"context"
	"testing"
	"time"
)

// TestLRU_SetGetHit verifies the happy path.
func TestLRU_SetGetHit(t *testing.T) {
	c := NewLRU(4)
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok || string(got) != "v" {
		t.Errorf("Get = (%q, %v), want (\"v\", true)", got, ok)
	}
}

// TestLRU_GetMiss reports misses as (nil, false, nil), never as error.
func TestLRU_GetMiss(t *testing.T) {
	c := NewLRU(4)
	got, ok, err := c.Get(context.Background(), "absent")
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if ok || got != nil {
		t.Errorf("miss returned (%q, %v), want (nil, false)", got, ok)
	}
}

// TestLRU_ExpiryEvicts checks that an entry past its TTL is not returned.
func TestLRU_ExpiryEvicts(t *testing.T) {
	c := NewLRU(4)
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)

	_, ok, _ := c.Get(ctx, "k")
	if ok {
		t.Error("expired entry should have been evicted on access")
	}
}

// TestLRU_InvalidateTagsDropsMembers confirms tag invalidation deletes every
// key associated with the tag, and leaves untagged keys alone.
func TestLRU_InvalidateTagsDropsMembers(t *testing.T) {
	c := NewLRU(8)
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), time.Minute, "ws:1")
	_ = c.Set(ctx, "b", []byte("2"), time.Minute, "ws:1", "article:42")
	_ = c.Set(ctx, "c", []byte("3"), time.Minute, "ws:2")

	if err := c.InvalidateTags(ctx, "ws:1"); err != nil {
		t.Fatalf("InvalidateTags: %v", err)
	}

	if _, ok, _ := c.Get(ctx, "a"); ok {
		t.Error("key a should have been invalidated (tag ws:1)")
	}
	if _, ok, _ := c.Get(ctx, "b"); ok {
		t.Error("key b should have been invalidated (tag ws:1)")
	}
	if _, ok, _ := c.Get(ctx, "c"); !ok {
		t.Error("key c should still be present (tag ws:2)")
	}
}

// TestTiered_ReadThroughBackfillsL1 verifies that an L2 hit backfills L1 so
// the next read is an L1 hit.
func TestTiered_ReadThroughBackfillsL1(t *testing.T) {
	l1 := NewLRU(8)
	l2 := NewLRU(8) // L2 can be any Cache in tests
	ctx := context.Background()
	_ = l2.Set(ctx, "k", []byte("v"), time.Minute, "ws:1")

	tc := NewTiered(TieredConfig{L1: l1, L2: l2, L1TTL: time.Minute})

	if v, ok, _ := tc.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("first Get = (%q, %v), want (\"v\", true)", v, ok)
	}
	// L1 should now have it directly.
	if v, ok, _ := l1.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Errorf("L1 not backfilled: (%q, %v)", v, ok)
	}
}

// TestTiered_InvalidateTagsDropsBothTiers ensures invalidation reaches both
// the in-process L1 and the composed L2.
func TestTiered_InvalidateTagsDropsBothTiers(t *testing.T) {
	l1 := NewLRU(8)
	l2 := NewLRU(8)
	ctx := context.Background()

	tc := NewTiered(TieredConfig{L1: l1, L2: l2, L1TTL: time.Minute})
	_ = tc.Set(ctx, "k", []byte("v"), time.Minute, "ws:1")

	if _, ok, _ := l1.Get(ctx, "k"); !ok {
		t.Fatal("L1 should have been populated on Set")
	}
	if _, ok, _ := l2.Get(ctx, "k"); !ok {
		t.Fatal("L2 should have been populated on Set")
	}

	if err := tc.InvalidateTags(ctx, "ws:1"); err != nil {
		t.Fatalf("InvalidateTags: %v", err)
	}
	if _, ok, _ := l1.Get(ctx, "k"); ok {
		t.Error("L1 not invalidated")
	}
	if _, ok, _ := l2.Get(ctx, "k"); ok {
		t.Error("L2 not invalidated")
	}
}

// TestNoop is a smoke test confirming the Noop cache always misses.
func TestNoop(t *testing.T) {
	n := NewNoop()
	ctx := context.Background()
	_ = n.Set(ctx, "k", []byte("v"), time.Minute, "tag")
	if _, ok, _ := n.Get(ctx, "k"); ok {
		t.Error("Noop Get should always miss")
	}
	if err := n.InvalidateTags(ctx, "tag"); err != nil {
		t.Errorf("Noop InvalidateTags: %v", err)
	}
}
