package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter uses Redis fixed-window counters.
type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// Allow returns true if under limit for key within window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if l == nil || l.rdb == nil || limit <= 0 {
		return true, nil
	}
	full := "rl:" + key
	n, err := l.rdb.Incr(ctx, full).Result()
	if err != nil {
		return true, err // fail open on redis errors
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, full, window).Err()
	}
	return n <= int64(limit), nil
}

// LoginFail tracks failed logins; Lockout returns remaining lock duration if locked.
func (l *Limiter) RecordLoginFail(ctx context.Context, identity string, maxFails int, window, lockTTL time.Duration) error {
	if l == nil || l.rdb == nil {
		return nil
	}
	key := "login:fail:" + identity
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, key, window).Err()
	}
	if int(n) >= maxFails {
		_ = l.rdb.Set(ctx, "login:lock:"+identity, "1", lockTTL).Err()
	}
	return nil
}

func (l *Limiter) ClearLoginFails(ctx context.Context, identity string) {
	if l == nil || l.rdb == nil {
		return
	}
	_ = l.rdb.Del(ctx, "login:fail:"+identity, "login:lock:"+identity).Err()
}

func (l *Limiter) IsLocked(ctx context.Context, identity string) (bool, time.Duration, error) {
	if l == nil || l.rdb == nil {
		return false, 0, nil
	}
	ttl, err := l.rdb.TTL(ctx, "login:lock:"+identity).Result()
	if err != nil {
		return false, 0, err
	}
	if ttl > 0 {
		return true, ttl, nil
	}
	return false, 0, nil
}

func ClientKey(prefix, ip string) string {
	return fmt.Sprintf("%s:%s", prefix, ip)
}
