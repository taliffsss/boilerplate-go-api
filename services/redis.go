package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api-boilerplate/config"

	"github.com/redis/go-redis/v9"
)

// RedisService handles Redis operations
type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisService creates a new Redis service
func NewRedisService() (*RedisService, error) {
	cfg := config.Get()

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisService{
		client: client,
		ctx:    ctx,
	}, nil
}

// Set stores a key-value pair with optional expiration
func (r *RedisService) Set(key string, value interface{}, expiration time.Duration) error {
	// Convert value to JSON if it's not a string
	var data string
	switch v := value.(type) {
	case string:
		data = v
	default:
		jsonData, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		data = string(jsonData)
	}

	return r.client.Set(r.ctx, key, data, expiration).Err()
}

// Get retrieves a value by key
func (r *RedisService) Get(key string) (string, error) {
	result, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found")
	}
	return result, err
}

// GetJSON retrieves and unmarshals a JSON value
func (r *RedisService) GetJSON(key string, dest interface{}) error {
	data, err := r.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), dest)
}

// Delete removes a key
func (r *RedisService) Delete(keys ...string) error {
	return r.client.Del(r.ctx, keys...).Err()
}

// Exists checks if a key exists
func (r *RedisService) Exists(keys ...string) (int64, error) {
	return r.client.Exists(r.ctx, keys...).Result()
}

// Expire sets expiration on a key
func (r *RedisService) Expire(key string, expiration time.Duration) error {
	return r.client.Expire(r.ctx, key, expiration).Err()
}

// TTL gets the time to live for a key
func (r *RedisService) TTL(key string) (time.Duration, error) {
	return r.client.TTL(r.ctx, key).Result()
}

// Increment increments a numeric value
func (r *RedisService) Increment(key string) (int64, error) {
	return r.client.Incr(r.ctx, key).Result()
}

// IncrementBy increments a numeric value by a specific amount
func (r *RedisService) IncrementBy(key string, value int64) (int64, error) {
	return r.client.IncrBy(r.ctx, key, value).Result()
}

// Decrement decrements a numeric value
func (r *RedisService) Decrement(key string) (int64, error) {
	return r.client.Decr(r.ctx, key).Result()
}

// SetNX sets a key only if it doesn't exist
func (r *RedisService) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	var data string
	switch v := value.(type) {
	case string:
		data = v
	default:
		jsonData, err := json.Marshal(value)
		if err != nil {
			return false, fmt.Errorf("failed to marshal value: %w", err)
		}
		data = string(jsonData)
	}

	return r.client.SetNX(r.ctx, key, data, expiration).Result()
}

// HSet sets a field in a hash
func (r *RedisService) HSet(key string, field string, value interface{}) error {
	return r.client.HSet(r.ctx, key, field, value).Err()
}

// HGet gets a field from a hash
func (r *RedisService) HGet(key string, field string) (string, error) {
	return r.client.HGet(r.ctx, key, field).Result()
}

// HGetAll gets all fields from a hash
func (r *RedisService) HGetAll(key string) (map[string]string, error) {
	return r.client.HGetAll(r.ctx, key).Result()
}

// HDelete deletes fields from a hash
func (r *RedisService) HDelete(key string, fields ...string) error {
	return r.client.HDel(r.ctx, key, fields...).Err()
}

// LPush pushes values to the left of a list
func (r *RedisService) LPush(key string, values ...interface{}) error {
	return r.client.LPush(r.ctx, key, values...).Err()
}

// RPush pushes values to the right of a list
func (r *RedisService) RPush(key string, values ...interface{}) error {
	return r.client.RPush(r.ctx, key, values...).Err()
}

// LPop pops a value from the left of a list
func (r *RedisService) LPop(key string) (string, error) {
	return r.client.LPop(r.ctx, key).Result()
}

// RPop pops a value from the right of a list
func (r *RedisService) RPop(key string) (string, error) {
	return r.client.RPop(r.ctx, key).Result()
}

// LRange gets a range of values from a list
func (r *RedisService) LRange(key string, start, stop int64) ([]string, error) {
	return r.client.LRange(r.ctx, key, start, stop).Result()
}

// LLen gets the length of a list
func (r *RedisService) LLen(key string) (int64, error) {
	return r.client.LLen(r.ctx, key).Result()
}

// SAdd adds members to a set
func (r *RedisService) SAdd(key string, members ...interface{}) error {
	return r.client.SAdd(r.ctx, key, members...).Err()
}

// SRemove removes members from a set
func (r *RedisService) SRemove(key string, members ...interface{}) error {
	return r.client.SRem(r.ctx, key, members...).Err()
}

// SMembers gets all members of a set
func (r *RedisService) SMembers(key string) ([]string, error) {
	return r.client.SMembers(r.ctx, key).Result()
}

// SIsMember checks if a value is a member of a set
func (r *RedisService) SIsMember(key string, member interface{}) (bool, error) {
	return r.client.SIsMember(r.ctx, key, member).Result()
}

// ZAdd adds members to a sorted set
func (r *RedisService) ZAdd(key string, members ...redis.Z) error {
	return r.client.ZAdd(r.ctx, key, members...).Err()
}

// ZRange gets a range of members from a sorted set
func (r *RedisService) ZRange(key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(r.ctx, key, start, stop).Result()
}

// ZRangeWithScores gets a range of members with scores from a sorted set
func (r *RedisService) ZRangeWithScores(key string, start, stop int64) ([]redis.Z, error) {
	return r.client.ZRangeWithScores(r.ctx, key, start, stop).Result()
}

// ZScore gets the score of a member in a sorted set
func (r *RedisService) ZScore(key string, member string) (float64, error) {
	return r.client.ZScore(r.ctx, key, member).Result()
}

// ZRem removes members from a sorted set
func (r *RedisService) ZRem(key string, members ...interface{}) error {
	return r.client.ZRem(r.ctx, key, members...).Err()
}

// Publish publishes a message to a channel
func (r *RedisService) Publish(channel string, message interface{}) error {
	return r.client.Publish(r.ctx, channel, message).Err()
}

// Subscribe subscribes to channels
func (r *RedisService) Subscribe(channels ...string) *redis.PubSub {
	return r.client.Subscribe(r.ctx, channels...)
}

// PSubscribe subscribes to channel patterns
func (r *RedisService) PSubscribe(patterns ...string) *redis.PubSub {
	return r.client.PSubscribe(r.ctx, patterns...)
}

// Pipeline creates a pipeline for batch operations
func (r *RedisService) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// Watch watches keys for changes
func (r *RedisService) Watch(fn func(*redis.Tx) error, keys ...string) error {
	return r.client.Watch(r.ctx, fn, keys...)
}

// FlushDB flushes the current database
func (r *RedisService) FlushDB() error {
	return r.client.FlushDB(r.ctx).Err()
}

// Close closes the Redis connection
func (r *RedisService) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client
func (r *RedisService) GetClient() *redis.Client {
	return r.client
}

// HealthCheck performs a health check on Redis
func (r *RedisService) HealthCheck() error {
	return r.client.Ping(r.ctx).Err()
}

// Cache-specific methods

// CacheSet sets a value with a specific cache key pattern
func (r *RedisService) CacheSet(prefix, key string, value interface{}, expiration time.Duration) error {
	cacheKey := fmt.Sprintf("%s:%s", prefix, key)
	return r.Set(cacheKey, value, expiration)
}

// CacheGet gets a value with a specific cache key pattern
func (r *RedisService) CacheGet(prefix, key string) (string, error) {
	cacheKey := fmt.Sprintf("%s:%s", prefix, key)
	return r.Get(cacheKey)
}

// CacheGetJSON gets and unmarshals a JSON value with a specific cache key pattern
func (r *RedisService) CacheGetJSON(prefix, key string, dest interface{}) error {
	cacheKey := fmt.Sprintf("%s:%s", prefix, key)
	return r.GetJSON(cacheKey, dest)
}

// CacheDelete deletes values with a specific cache key pattern
func (r *RedisService) CacheDelete(prefix string, keys ...string) error {
	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = fmt.Sprintf("%s:%s", prefix, key)
	}
	return r.Delete(cacheKeys...)
}

// CacheFlush flushes all keys with a specific prefix
func (r *RedisService) CacheFlush(prefix string) error {
	pattern := fmt.Sprintf("%s:*", prefix)
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.Delete(keys...)
	}

	return nil
}

// Rate limiting methods

// RateLimitCheck checks if a rate limit has been exceeded
func (r *RedisService) RateLimitCheck(key string, limit int, window time.Duration) (bool, int, error) {
	current, err := r.Increment(key)
	if err != nil {
		return false, 0, err
	}

	if current == 1 {
		if err := r.Expire(key, window); err != nil {
			return false, 0, err
		}
	}

	remaining := limit - int(current)
	if remaining < 0 {
		remaining = 0
	}

	return current <= int64(limit), remaining, nil
}

// Session management methods

// SessionSet stores session data
func (r *RedisService) SessionSet(sessionID string, data interface{}, expiration time.Duration) error {
	return r.CacheSet("session", sessionID, data, expiration)
}

// SessionGet retrieves session data
func (r *RedisService) SessionGet(sessionID string, dest interface{}) error {
	return r.CacheGetJSON("session", sessionID, dest)
}

// SessionDelete deletes a session
func (r *RedisService) SessionDelete(sessionID string) error {
	return r.CacheDelete("session", sessionID)
}

// SessionExtend extends session expiration
func (r *RedisService) SessionExtend(sessionID string, expiration time.Duration) error {
	return r.Expire(fmt.Sprintf("session:%s", sessionID), expiration)
}
