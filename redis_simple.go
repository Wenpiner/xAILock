package xailock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisSimpleLock struct {
	key    string
	value  string
	client redis.UniversalClient
}

func newRedisSimpleLock(key string, client redis.UniversalClient) Lock {
	return &redisSimpleLock{
		key:    key,
		client: client,
	}
}

// Lock 获取锁，如果获取失败会重试直到超时
func (l *redisSimpleLock) Lock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return l.TryLock(ctx, opts...)
	})
}

// TryLock 尝试获取锁，立即返回结果
func (l *redisSimpleLock) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)
	l.value = config.Value // 保存 value 用于后续解锁

	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, SimpleLockAcquireScript, []string{l.key}, config.Value, config.LockTTL.Seconds()).Int()
	if err != nil {
		return false, WrapNetworkError(err, "acquire_lock")
	}

	return result == 1, nil
}

// Unlock 释放锁
func (l *redisSimpleLock) Unlock(ctx context.Context) error {
	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, SimpleLockReleaseScript, []string{l.key}, l.value).Int()
	if err != nil {
		return WrapNetworkError(err, "unlock")
	}

	if result == 0 {
		return NewLockNotFoundError(l.key)
	}

	return nil
}

// Extend 续约锁，延长 TTL
func (l *redisSimpleLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, SimpleLockExtendScript, []string{l.key}, l.value, ttl.Seconds()).Int()
	if err != nil {
		return false, WrapNetworkError(err, "extend")
	}

	return result == 1, nil
}

// IsLocked 检查锁是否被持有
func (l *redisSimpleLock) IsLocked(ctx context.Context) (bool, error) {
	// 使用优化的状态查询脚本，一次调用获取存在性和TTL
	result, err := l.client.Eval(ctx, SimpleLockStatusScript, []string{l.key}).Slice()
	if err != nil {
		return false, WrapNetworkError(err, "check_lock_status")
	}

	if len(result) >= 1 {
		exists, ok := result[0].(int64)
		if ok {
			return exists == 1, nil
		}
	}

	return false, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "check_lock_status")
}

// GetTTL 获取锁的剩余时间
func (l *redisSimpleLock) GetTTL(ctx context.Context) (time.Duration, error) {
	// 使用优化的状态查询脚本，一次调用获取存在性和TTL
	result, err := l.client.Eval(ctx, SimpleLockStatusScript, []string{l.key}).Slice()
	if err != nil {
		return 0, WrapNetworkError(err, "get_lock_ttl")
	}

	if len(result) >= 2 {
		exists, existsOk := result[0].(int64)
		ttlSeconds, ttlOk := result[1].(int64)

		if existsOk && ttlOk {
			if exists == 0 {
				return 0, NewLockNotFoundError(l.key)
			}
			if ttlSeconds < 0 {
				return -1, nil // 永不过期
			}
			return time.Duration(ttlSeconds) * time.Second, nil
		}
	}

	return 0, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "get_lock_ttl")
}

// GetKey 获取锁的键名
func (l *redisSimpleLock) GetKey() string {
	return l.key
}

// GetValue 获取锁的值（持有者标识）
func (l *redisSimpleLock) GetValue() string {
	return l.value
}
