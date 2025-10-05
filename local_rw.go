package xailock

import (
	"context"
	"time"
)

// localRWLock 基于分片的本地读写锁实现
type localRWLock struct {
	key     string
	backend *LocalBackend
	lock    *ShardedLock[RWLockData]
	manager *ShardedLockManager[RWLockData]
}

func newLocalRWLock(key string, backend *LocalBackend) RWLock {
	manager := GetGlobalRWLockManager(backend.ShardCount)
	shardedLock := manager.GetLock(key)

	return &localRWLock{
		key:     key,
		backend: backend,
		lock:    shardedLock,
		manager: manager,
	}
}

// Lock 获取写锁
func (l *localRWLock) Lock(ctx context.Context, opts ...LockOption) error {
	return l.lock.Lock(ctx, opts...)
}

// TryLock 尝试获取写锁
func (l *localRWLock) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return l.lock.TryLock(ctx, opts...)
}

// Unlock 释放写锁
func (l *localRWLock) Unlock(ctx context.Context) error {
	err := l.lock.Unlock(ctx)
	if err == nil {
		// 释放分片锁引用
		l.manager.ReleaseLock(l.lock)
	}
	return err
}

// Extend 续约写锁
func (l *localRWLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	return l.lock.Extend(ctx, ttl)
}

// IsLocked 检查写锁是否被持有
func (l *localRWLock) IsLocked(ctx context.Context) (bool, error) {
	return l.lock.IsLocked(ctx)
}

// GetTTL 获取写锁的剩余时间
func (l *localRWLock) GetTTL(ctx context.Context) (time.Duration, error) {
	return l.lock.GetTTL(ctx)
}

// RLock 获取读锁
func (l *localRWLock) RLock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return TryRLockRW(l.lock, ctx, opts...)
	})
}

// TryRLock 尝试获取读锁
func (l *localRWLock) TryRLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return TryRLockRW(l.lock, ctx, opts...)
}

// RUnlock 释放读锁
func (l *localRWLock) RUnlock(ctx context.Context) error {
	return RUnlockRW(l.lock, ctx)
}

// GetReadCount 获取当前读锁数量
func (l *localRWLock) GetReadCount(ctx context.Context) (int, error) {
	return GetReadCountRW(l.lock, ctx)
}

// GetKey 获取锁的键名
func (l *localRWLock) GetKey() string {
	return l.key
}

// GetValue 获取锁的值
func (l *localRWLock) GetValue() string {
	return l.lock.GetValue()
}

// IsWriteLocked 检查写锁是否被持有
func (l *localRWLock) IsWriteLocked(ctx context.Context) (bool, error) {
	return l.IsLocked(ctx)
}

// WLock 获取写锁
func (l *localRWLock) WLock(ctx context.Context, opts ...LockOption) error {
	return l.Lock(ctx, opts...)
}

// WUnlock 释放写锁
func (l *localRWLock) WUnlock(ctx context.Context) error {
	return l.Unlock(ctx)
}

// TryWLock 尝试获取写锁
func (l *localRWLock) TryWLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return l.TryLock(ctx, opts...)
}
