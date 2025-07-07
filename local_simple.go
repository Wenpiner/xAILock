package lock

import (
	"context"
	"time"
)

// localSimpleLock 基于分片的本地简单锁实现
type localSimpleLock struct {
	key     string
	backend *LocalBackend
	lock    *ShardedLock[SimpleLockData]
	manager *ShardedLockManager[SimpleLockData]
}

func newLocalSimpleLock(key string, backend *LocalBackend) Lock {
	manager := GetGlobalSimpleLockManager(backend.ShardCount)
	shardedLock := manager.GetLock(key)

	return &localSimpleLock{
		key:     key,
		backend: backend,
		lock:    shardedLock,
		manager: manager,
	}
}

// Lock 获取锁，如果获取失败会重试直到超时
func (l *localSimpleLock) Lock(ctx context.Context, opts ...LockOption) error {
	return l.lock.Lock(ctx, opts...)
}

// TryLock 尝试获取锁，立即返回结果
func (l *localSimpleLock) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return l.lock.TryLock(ctx, opts...)
}

// Unlock 释放锁
func (l *localSimpleLock) Unlock(ctx context.Context) error {
	err := l.lock.Unlock(ctx)
	if err == nil {
		// 释放分片锁引用
		l.manager.ReleaseLock(l.lock)
	}
	return err
}

// Extend 续约锁，延长 TTL
func (l *localSimpleLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	return l.lock.Extend(ctx, ttl)
}

// IsLocked 检查锁是否被持有
func (l *localSimpleLock) IsLocked(ctx context.Context) (bool, error) {
	return l.lock.IsLocked(ctx)
}

// GetTTL 获取锁的剩余时间
func (l *localSimpleLock) GetTTL(ctx context.Context) (time.Duration, error) {
	return l.lock.GetTTL(ctx)
}

// GetKey 获取锁的键名
func (l *localSimpleLock) GetKey() string {
	return l.key
}

// GetValue 获取锁的值（持有者标识）
func (l *localSimpleLock) GetValue() string {
	return l.lock.GetValue()
}
