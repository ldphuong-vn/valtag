package ratelimit

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements sliding window rate limiting using Redis.
type RedisLimiter struct {
	client   *redis.Client
	failOpen bool
}

// NewRedisLimiter creates a new Redis-backed rate limiter. failOpen decides
// what happens when Redis is unreachable: false (default, fail-closed)
// rejects agent requests with 503 until the limiter recovers; true lets
// them through (availability over bypass-resistance).
func NewRedisLimiter(redisURL string, failOpen bool) (*RedisLimiter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redis.NewClient(opt)
	return &RedisLimiter{client: client, failOpen: failOpen}, nil
}

// Allow checks if agentID is within the rpm (requests per minute) limit.
// Returns true if the request is allowed, false if rate limited.
// Uses Redis sliding window: ZADD + ZREMRANGEBYSCORE + ZCARD.
func (l *RedisLimiter) Allow(ctx context.Context, agentID string, rpm int) (bool, error) {
	if rpm <= 0 {
		return true, nil
	}
	key := fmt.Sprintf("ratelimit:agent:%s", agentID)
	now := time.Now().UnixMilli()
	windowStart := now - 60_000 // 60 seconds ago in ms

	// Use timestamp:random member to prevent millisecond deduplication collisions.
	member := fmt.Sprintf("%d-%d", now, rand.Int63())
	pipe := l.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: member})
	pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, 65*time.Second)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		// Redis failure — the middleware decides: fail-closed (default) or
		// fail-open via the failOpen constructor flag (report §4.12).
		return l.failOpen, fmt.Errorf("redis error: %w", err)
	}

	intCmd, ok := cmds[2].(*redis.IntCmd)
	if !ok {
		return l.failOpen, fmt.Errorf("unexpected redis reply for ZCard")
	}
	return intCmd.Val() <= int64(rpm), nil
}

// Close closes the Redis connection.
func (l *RedisLimiter) Close() error {
	return l.client.Close()
}
