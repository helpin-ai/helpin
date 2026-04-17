// Package cache provides a tiered caching primitive for hot, read-heavy data
// paths such as the public help center. It pairs an in-process LRU (L1) with
// an optional Redis backend (L2) and supports tag-based invalidation so that
// writes can cheaply drop all cached entries that depend on a given entity.
//
// The package is opinionated: it caches raw bytes, leaving JSON encoding to
// the caller so repeated reads never pay the serialization cost.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned (as a sentinel) by implementations that want to signal
// a definite miss. Most callers should prefer the boolean return from Get.
var ErrMiss = errors.New("cache: miss")

// Cache is the boundary every caller depends on. Implementations may be
// in-process (LRU), remote (Redis), or composed (tiered) — behaviour is the
// same from the caller's perspective.
type Cache interface {
	// Get returns the bytes stored at key and true on hit.
	// It returns (nil, false, nil) on a miss and (nil, false, err) on transport failure.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores value under key with the given TTL. Tags associate the key with
	// logical groups so a future InvalidateTags call can drop related entries.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags ...string) error

	// InvalidateTags drops every key that was Set with any of the provided tags.
	// Implementations that do not track tags (e.g. noop) must still succeed.
	InvalidateTags(ctx context.Context, tags ...string) error
}
