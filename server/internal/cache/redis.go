package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is an L2 cache backed by a Redis instance. Tag membership is kept in
// Redis SETs so invalidation spans every pod reading from the same Redis,
// which is the whole point of having a shared L2.
type Redis struct {
	rdb    *redis.Client
	prefix string
}

// NewRedis returns an L2 cache. The prefix is applied to every key so multiple
// caches can safely share a single Redis DB.
func NewRedis(rdb *redis.Client, prefix string) *Redis {
	if prefix == "" {
		prefix = "cache"
	}
	return &Redis{rdb: rdb, prefix: prefix}
}

// Get returns the cached bytes and true on hit. Redis misses do not produce an
// error — we surface them as (nil, false, nil) so callers can treat transport
// failures distinctly from plain misses.
func (c *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	raw, err := c.rdb.Get(ctx, c.dataKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// Set stores the value with the given TTL and registers the key against each
// tag via a Redis SET. Tag SETs inherit the value TTL so orphaned tag indexes
// don't leak unbounded over time.
func (c *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags ...string) error {
	pipe := c.rdb.TxPipeline()
	pipe.Set(ctx, c.dataKey(key), value, ttl)
	for _, tag := range tags {
		tagKey := c.tagKey(tag)
		pipe.SAdd(ctx, tagKey, key)
		if ttl > 0 {
			// Bump the tag set's TTL so it doesn't accumulate forever.
			// Use a longer TTL than the data so in-flight invalidations
			// don't race expiry.
			pipe.Expire(ctx, tagKey, ttl*2)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidateTags reads each tag SET, deletes every key it references, then
// drops the tag SET itself. A single pipeline keeps network overhead bounded
// even with many tags.
func (c *Redis) InvalidateTags(ctx context.Context, tags ...string) error {
	if len(tags) == 0 {
		return nil
	}
	keys := make(map[string]struct{})
	for _, tag := range tags {
		members, err := c.rdb.SMembers(ctx, c.tagKey(tag)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		for _, m := range members {
			keys[m] = struct{}{}
		}
	}

	pipe := c.rdb.Pipeline()
	for k := range keys {
		pipe.Del(ctx, c.dataKey(k))
	}
	for _, tag := range tags {
		pipe.Del(ctx, c.tagKey(tag))
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Redis) dataKey(k string) string { return c.prefix + ":d:" + k }
func (c *Redis) tagKey(t string) string  { return c.prefix + ":t:" + t }
