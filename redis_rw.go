package lock

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	// 读写锁的键名格式
	readLockKeyFormat  = "%s:read"  // 读锁键名格式
	writeLockKeyFormat = "%s:write" // 写锁键名格式
)

type redisRWLock struct {
	key        string
	writeValue string // 保存写锁的value用于释放
	client     redis.UniversalClient
}

func newRedisRWLock(key string, client redis.UniversalClient) RWLock {
	return &redisRWLock{
		key:    key,
		client: client,
	}
}

// RLock 获取读锁
func (l *redisRWLock) RLock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return l.TryRLock(ctx, opts...)
	})
}

// TryRLock 尝试获取读锁
func (l *redisRWLock) TryRLock(ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)
	readKey := fmt.Sprintf(readLockKeyFormat, l.key)
	writeKey := fmt.Sprintf(writeLockKeyFormat, l.key)

	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, RWLockAcquireReadScript, []string{readKey, writeKey}, config.LockTTL.Seconds()).Int()
	if err != nil {
		return false, WrapNetworkError(err, "acquire_read_lock")
	}

	return result == 1, nil
}

// RUnlock 释放读锁
func (l *redisRWLock) RUnlock(ctx context.Context) error {
	readKey := fmt.Sprintf(readLockKeyFormat, l.key)

	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, RWLockReleaseReadScript, []string{readKey}).Int()
	if err != nil {
		return WrapNetworkError(err, "release_read_lock")
	}

	if result == 0 {
		return NewLockNotFoundError(l.key)
	}

	return nil
}

// WLock 获取写锁
func (l *redisRWLock) WLock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return l.TryWLock(ctx, opts...)
	})
}

// TryWLock 尝试获取写锁
func (l *redisRWLock) TryWLock(ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)
	l.writeValue = config.Value // 保存写锁value用于释放
	readKey := fmt.Sprintf(readLockKeyFormat, l.key)
	writeKey := fmt.Sprintf(writeLockKeyFormat, l.key)

	// 使用优化的 Lua 脚本确保原子性
	result, err := l.client.Eval(ctx, RWLockAcquireWriteScript, []string{readKey, writeKey}, config.Value, config.LockTTL.Seconds()).Int()
	if err != nil {
		return false, WrapNetworkError(err, "acquire_write_lock")
	}

	return result == 1, nil
}

// WUnlock 释放写锁
func (l *redisRWLock) WUnlock(ctx context.Context) error {
	writeKey := fmt.Sprintf(writeLockKeyFormat, l.key)

	// 使用优化的 Lua 脚本确保原子性，使用保存的writeValue
	result, err := l.client.Eval(ctx, RWLockReleaseWriteScript, []string{writeKey}, l.writeValue).Int()
	if err != nil {
		return WrapNetworkError(err, "release_write_lock")
	}

	if result == 0 {
		return NewLockNotFoundError(l.key)
	}

	return nil
}

// GetReadCount 获取当前读锁数量
func (l *redisRWLock) GetReadCount(ctx context.Context) (int, error) {
	readKey := fmt.Sprintf(readLockKeyFormat, l.key)
	writeKey := fmt.Sprintf(writeLockKeyFormat, l.key)

	// 使用优化的状态查询脚本，一次调用获取读锁数量和写锁状态
	result, err := l.client.Eval(ctx, RWLockStatusScript, []string{readKey, writeKey}).Slice()
	if err != nil {
		return 0, WrapNetworkError(err, "get_read_count")
	}

	if len(result) >= 1 {
		readCount, ok := result[0].(int64)
		if ok {
			return int(readCount), nil
		}
	}

	return 0, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "get_read_count")
}

// IsWriteLocked 检查是否有写锁
func (l *redisRWLock) IsWriteLocked(ctx context.Context) (bool, error) {
	readKey := fmt.Sprintf(readLockKeyFormat, l.key)
	writeKey := fmt.Sprintf(writeLockKeyFormat, l.key)

	// 使用优化的状态查询脚本，一次调用获取读锁数量和写锁状态
	result, err := l.client.Eval(ctx, RWLockStatusScript, []string{readKey, writeKey}).Slice()
	if err != nil {
		return false, WrapNetworkError(err, "check_write_locked")
	}

	if len(result) >= 2 {
		writeLocked, ok := result[1].(int64)
		if ok {
			return writeLocked == 1, nil
		}
	}

	return false, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "check_write_locked")
}

// GetKey 获取锁的键名
func (l *redisRWLock) GetKey() string {
	return l.key
}
