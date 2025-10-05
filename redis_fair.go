package xailock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisFairLock struct {
	key      string
	queueKey string
	value    string
	client   redis.UniversalClient
}

func newRedisFairLock(key string, client redis.UniversalClient) FairLock {
	return &redisFairLock{
		key:      key,
		queueKey: key + ":queue",
		client:   client,
	}
}

// Lock 公平加锁
func (l *redisFairLock) Lock(ctx context.Context, opts ...LockOption) error {
	config := ApplyLockOptions(opts)
	l.value = config.Value

	lockTTL := config.LockTTL
	if lockTTL <= 0 {
		lockTTL = 30 * time.Second
	}

	queueTTL := config.QueueTTL
	if queueTTL <= 0 {
		queueTTL = 5 * time.Minute
	}

	acquireTimeout := config.AcquireTimeout
	if acquireTimeout <= 0 {
		acquireTimeout = 10 * time.Second
	}

	// 1. 首先尝试原子性获取锁（可能直接成功或加入队列）
	result, err := l.client.Eval(ctx, FairLockTryAcquireScript,
		[]string{l.key, l.queueKey},
		l.value, lockTTL.Seconds(), queueTTL.Seconds()).Int()
	if err != nil {
		return WrapNetworkError(err, "fair_lock_initial_attempt")
	}

	if result == 1 {
		// 直接获取成功
		return nil
	}

	// 2. 需要等待，使用优化的重试逻辑
	return RetryWithCustomLogic(ctx, acquireTimeout, l.key, func(ctx context.Context, deadline time.Time) error {
		for {
			// 使用原子性脚本检查队列位置并尝试获取锁
			result, err := l.client.Eval(ctx, FairLockAcquireScript,
				[]string{l.key, l.queueKey},
				l.value, lockTTL.Seconds()).Int()
			if err != nil {
				return WrapNetworkError(err, "fair_lock_acquire_attempt")
			}

			if result == 1 {
				// 获取成功
				return nil
			} else if result == 0 {
				// 不在队首，继续等待
			} else if result == -1 {
				// 锁仍被持有，继续等待
			}

			// 检查超时
			if time.Now().After(deadline) {
				// 超时，清理队列
				_ = l.client.Eval(ctx, FairLockCleanupScript, []string{l.queueKey}, l.value)
				return NewAcquireTimeoutError(l.key, acquireTimeout)
			}

			time.Sleep(50 * time.Millisecond)
		}
	})
}

// TryLock 公平尝试加锁（不等待）
func (l *redisFairLock) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)
	l.value = config.Value

	lockTTL := config.LockTTL
	if lockTTL <= 0 {
		lockTTL = 30 * time.Second
	}

	queueTTL := config.QueueTTL
	if queueTTL <= 0 {
		queueTTL = 5 * time.Minute
	}

	// 使用优化的原子性脚本，一次调用完成所有操作
	result, err := l.client.Eval(ctx, FairLockTryAcquireScript,
		[]string{l.key, l.queueKey},
		l.value, lockTTL.Seconds(), queueTTL.Seconds()).Int()
	if err != nil {
		return false, WrapNetworkError(err, "try_acquire_fair_lock")
	}

	// result: 1=success, 0=failed, -1=queued
	return result == 1, nil
}

// Unlock 公平解锁
func (l *redisFairLock) Unlock(ctx context.Context) error {
	// 使用优化的原子性脚本释放锁并清理队列
	result, err := l.client.Eval(ctx, FairLockReleaseScript, []string{l.key, l.queueKey}, l.value).Int()
	if err != nil {
		return WrapNetworkError(err, "fair_lock_unlock")
	}

	if result == 0 {
		return NewLockNotFoundError(l.key)
	}

	return nil
}

// Extend 续约
func (l *redisFairLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	// 使用优化的续期脚本
	ms := int(ttl.Milliseconds())
	result, err := l.client.Eval(ctx, FairLockExtendScript, []string{l.key}, l.value, ms).Int()
	if err != nil {
		return false, WrapNetworkError(err, "fair_lock_extend")
	}
	return result == 1, nil
}

// IsLocked 判断锁是否被持有
func (l *redisFairLock) IsLocked(ctx context.Context) (bool, error) {
	// 使用优化的状态查询脚本
	result, err := l.client.Eval(ctx, FairLockStatusScript, []string{l.key, l.queueKey}).Slice()
	if err != nil {
		return false, WrapNetworkError(err, "check_fair_lock_status")
	}

	if len(result) >= 1 {
		locked, ok := result[0].(int64)
		if ok {
			return locked == 1, nil
		}
	}

	return false, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "check_fair_lock_status")
}

// GetTTL 获取锁的剩余时间
func (l *redisFairLock) GetTTL(ctx context.Context) (time.Duration, error) {
	// 使用优化的状态查询脚本
	result, err := l.client.Eval(ctx, FairLockStatusScript, []string{l.key, l.queueKey}).Slice()
	if err != nil {
		return 0, WrapNetworkError(err, "get_fair_lock_ttl")
	}

	if len(result) >= 2 {
		locked, lockedOk := result[0].(int64)
		ttlMs, ttlOk := result[1].(int64)

		if lockedOk && ttlOk {
			if locked == 0 {
				return 0, NewLockNotFoundError(l.key)
			}
			if ttlMs < 0 {
				return -1, nil // 永不过期
			}
			return time.Duration(ttlMs) * time.Millisecond, nil
		}
	}

	return 0, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "get_fair_lock_ttl")
}

// GetQueuePosition 获取自己在队列中的位置（0为队首）
func (l *redisFairLock) GetQueuePosition(ctx context.Context) (int, error) {
	// 使用优化的状态查询脚本，传入value参数
	result, err := l.client.Eval(ctx, FairLockStatusScript, []string{l.key, l.queueKey}, l.value).Slice()
	if err != nil {
		return -1, WrapNetworkError(err, "get_queue_position")
	}

	if len(result) >= 4 {
		position, ok := result[3].(int64)
		if ok {
			return int(position), nil
		}
	}

	return -1, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "get_queue_position")
}

// GetQueueLength 获取队列长度
func (l *redisFairLock) GetQueueLength(ctx context.Context) (int, error) {
	// 使用优化的状态查询脚本
	result, err := l.client.Eval(ctx, FairLockStatusScript, []string{l.key, l.queueKey}).Slice()
	if err != nil {
		return 0, WrapNetworkError(err, "get_queue_length")
	}

	if len(result) >= 3 {
		queueLength, ok := result[2].(int64)
		if ok {
			return int(queueLength), nil
		}
	}

	return 0, NewLockError(ErrCodeInternalError, "unexpected response from status script", SeverityError).
		WithContext("operation", "get_queue_length")
}

// GetQueueInfo 获取队列所有元素
func (l *redisFairLock) GetQueueInfo(ctx context.Context) ([]string, error) {
	return l.client.LRange(ctx, l.queueKey, 0, -1).Result()
}

func (l *redisFairLock) GetKey() string   { return l.key }
func (l *redisFairLock) GetValue() string { return l.value }
