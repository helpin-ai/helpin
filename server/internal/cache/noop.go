package cache

import (
	"context"
	"time"
)

// Noop is a no-op cache. It reports misses on every Get and silently accepts
// Set and InvalidateTags. Use it as a safe default when the cache backend is
// disabled so callers can be written without nil checks.
type Noop struct{}

// NewNoop returns a Cache that never stores anything.
func NewNoop() Cache { return Noop{} }

// Get always reports a miss.
func (Noop) Get(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, nil
}

// Set accepts the call but stores nothing.
func (Noop) Set(_ context.Context, _ string, _ []byte, _ time.Duration, _ ...string) error {
	return nil
}

// InvalidateTags accepts the call but has nothing to invalidate.
func (Noop) InvalidateTags(_ context.Context, _ ...string) error { return nil }
