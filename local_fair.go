package xailock

import (
	"context"
	"time"
)

// localFairLock 基于分片的本地公平锁实现
type localFairLock struct {
	key     string
	backend *LocalBackend
	lock    *ShardedLock[FairLockData]
	manager *ShardedLockManager[FairLockData]
}

func newLocalFairLock(key string, backend *LocalBackend) FairLock {
	manager := GetGlobalFairLockManager(backend.ShardCount)
	shardedLock := manager.GetLock(key)

	return &localFairLock{
		key:     key,
		backend: backend,
		lock:    shardedLock,
		manager: manager,
	}
}

// Lock 获取公平锁
func (l *localFairLock) Lock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return TryFairLockFair(l.lock, ctx, opts...)
	})
}

// TryLock 尝试获取公平锁
func (l *localFairLock) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return TryFairLockFair(l.lock, ctx, opts...)
}

// Unlock 释放锁
func (l *localFairLock) Unlock(ctx context.Context) error {
	err := UnlockFairFair(l.lock, ctx)
	if err == nil {
		// 释放分片锁引用
		l.manager.ReleaseLock(l.lock)
	}
	return err
}

// Extend 续约锁
func (l *localFairLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	return l.lock.Extend(ctx, ttl)
}

// IsLocked 检查锁是否被持有
func (l *localFairLock) IsLocked(ctx context.Context) (bool, error) {
	return IsLockedFairFair(l.lock, ctx)
}

// GetTTL 获取锁的剩余时间
func (l *localFairLock) GetTTL(ctx context.Context) (time.Duration, error) {
	return GetTTLFairFair(l.lock, ctx)
}

// GetQueuePosition 获取在等待队列中的位置，0表示当前持有锁
func (l *localFairLock) GetQueuePosition(ctx context.Context) (int, error) {
	l.lock.mu.RLock()
	defer l.lock.mu.RUnlock()

	value := l.lock.GetValue()
	if !l.lock.unlocked && len(l.lock.data.queue) > 0 && value == l.lock.data.queue[0] {
		return 0, nil
	}

	for i, v := range l.lock.data.queue {
		if v == value {
			return i + 1, nil
		}
	}

	return -1, NewLockNotFoundError(l.key)
}

// GetQueueLength 获取等待队列长度
func (l *localFairLock) GetQueueLength(ctx context.Context) (int, error) {
	l.lock.mu.RLock()
	defer l.lock.mu.RUnlock()
	return len(l.lock.data.queue), nil
}

// GetQueueInfo 获取队列详细信息
func (l *localFairLock) GetQueueInfo(ctx context.Context) ([]string, error) {
	l.lock.mu.RLock()
	defer l.lock.mu.RUnlock()

	// 创建队列的副本
	queue := make([]string, len(l.lock.data.queue))
	copy(queue, l.lock.data.queue)
	return queue, nil
}

// GetKey 获取锁的键名
func (l *localFairLock) GetKey() string {
	return l.key
}

// GetValue 获取锁的值
func (l *localFairLock) GetValue() string {
	return l.lock.GetValue()
}
