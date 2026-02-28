package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CacheKeyPrefix = "gateway:cache:"
)

// RedisCache handles cache operations using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

// Get retrieves a cache entry from Redis
func (c *RedisCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	// Add prefix
	fullKey := CacheKeyPrefix + key

	// Get from Redis
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	// Deserialize
	entry, err := Deserialize(data)
	if err != nil {
		slog.Error("failed to deserialize cache entry", "key", key, "error", err)
		return nil, err
	}

	return entry, nil
}

// Set stores a cache entry in Redis
func (c *RedisCache) Set(ctx context.Context, key string, entry *CacheEntry) error {
	// Serialize
	data, err := entry.Serialize()
	if err != nil {
		return err
	}

	// Add prefix
	fullKey := CacheKeyPrefix + key

	// Store with TTL
	return c.client.Set(ctx, fullKey, data, time.Duration(entry.TTL)*time.Second).Err()
}

// Delete removes a cache entry from Redis
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := CacheKeyPrefix + key
	return c.client.Del(ctx, fullKey).Err()
}

// DeletePattern removes all cache entries matching a pattern
// Pattern can include wildcards (e.g., "gateway:cache:GET:api:users:*")
// Returns the number of keys deleted
func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	fullPattern := CacheKeyPrefix + pattern

	iter := c.client.Scan(ctx, 0, fullPattern, 0).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan for keys: %w", err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	deleted, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys: %w", err)
	}

	slog.Info("cache pattern deleted", "pattern", fullPattern, "count", deleted)
	return deleted, nil
}

// FlushPrefix removes all cache entries for a specific route prefix
// Example: FlushPrefix(ctx, "api:users") deletes all entries for /api/users/*
func (c *RedisCache) FlushPrefix(ctx context.Context, prefix string) (int64, error) {
	pattern := fmt.Sprintf("%s*:%s*", CacheKeyPrefix, prefix)

	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan for keys with prefix %s: %w", prefix, err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	deleted, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys with prefix %s: %w", prefix, err)
	}

	slog.Info("cache prefix flushed", "prefix", prefix, "count", deleted)
	return deleted, nil
}

// FlushAll removes all cache entries managed by the gateway
func (c *RedisCache) FlushAll(ctx context.Context) error {
	pattern := CacheKeyPrefix + "*"

	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan for all cache keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete all cache keys: %w", err)
	}

	slog.Info("all cache flushed", "count", len(keys))
	return nil
}

// IsHealthy checks if Redis cache is healthy
func (c *RedisCache) IsHealthy(ctx context.Context) bool {
	_, err := c.client.Ping(ctx).Result()
	return err == nil
}
