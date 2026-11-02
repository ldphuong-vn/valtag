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
	client *redis.Client
}

// NewRedisLimiter creates a new Redis-backed rate limiter.
func NewRedisLimiter(redisURL string) (*RedisLimiter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redis.NewClient(opt)
	return &RedisLimiter{client: client}, nil
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
		// Redis failure — allow request (fail open)
		return true, fmt.Errorf("redis error: %w", err)
	}

	intCmd, ok := cmds[2].(*redis.IntCmd)
	if !ok {
		return true, fmt.Errorf("unexpected redis reply for ZCard")
	}
	return intCmd.Val() <= int64(rpm), nil
}

// Close closes the Redis connection.
func (l *RedisLimiter) Close() error {
	return l.client.Close()
}
