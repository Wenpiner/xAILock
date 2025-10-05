package xailock

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// ShardedLockManager 分片锁管理器，提供高性能的本地锁实现
type ShardedLockManager[T any] struct {
	shards      []*LockShard[T]
	shardCount  uint32
	shardMask   uint32 // 用于快速取模运算 (shardCount - 1)
	cleanupStop chan struct{}
	cleanupDone chan struct{}
}

// LockShard 锁分片，每个分片独立管理一组锁
type LockShard[T any] struct {
	mu    sync.RWMutex
	locks map[string]*ShardedLock[T]
	pool  *sync.Pool // 锁对象池，减少内存分配
}

// ShardedLock 分片锁实例
type ShardedLock[T any] struct {
	key      string
	value    string
	data     T // 锁的具体实现数据
	mu       sync.RWMutex
	unlocked bool
	ttl      time.Duration
	expireAt time.Time
	refCount int32 // 引用计数，用于安全清理
	shard    *LockShard[T]
}

// NewShardedLockManager 创建分片锁管理器
func NewShardedLockManager[T any](shardCount int, factory func() T) *ShardedLockManager[T] {
	// 确保分片数量是2的幂次，便于位运算优化
	if shardCount <= 0 {
		shardCount = DefaultShardCount
	}

	// 调整为最接近的2的幂次
	actualShardCount := uint32(1)
	for actualShardCount < uint32(shardCount) {
		actualShardCount <<= 1
	}

	manager := &ShardedLockManager[T]{
		shards:      make([]*LockShard[T], actualShardCount),
		shardCount:  actualShardCount,
		shardMask:   actualShardCount - 1,
		cleanupStop: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	// 初始化所有分片
	for i := uint32(0); i < actualShardCount; i++ {
		manager.shards[i] = &LockShard[T]{
			locks: make(map[string]*ShardedLock[T]),
			pool: &sync.Pool{
				New: func() interface{} {
					return &ShardedLock[T]{
						data:     factory(),
						unlocked: true,
					}
				},
			},
		}
	}

	// 启动后台清理goroutine
	go manager.cleanupExpiredLocks()

	return manager
}

// GetLock 获取或创建指定键的锁
func (m *ShardedLockManager[T]) GetLock(key string) *ShardedLock[T] {
	shard := m.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 检查是否已存在
	if lock, exists := shard.locks[key]; exists {
		// 增加引用计数
		atomic.AddInt32(&lock.refCount, 1)
		return lock
	}

	// 从对象池获取锁实例
	lock := shard.pool.Get().(*ShardedLock[T])
	lock.key = key
	lock.unlocked = true
	lock.refCount = 1
	lock.shard = shard

	// 存储到分片中
	shard.locks[key] = lock

	return lock
}

// ReleaseLock 释放锁引用
func (m *ShardedLockManager[T]) ReleaseLock(lock *ShardedLock[T]) {
	if atomic.AddInt32(&lock.refCount, -1) <= 0 {
		// 引用计数为0，可以清理
		m.cleanupLock(lock)
	}
}

// getShard 根据键名获取对应的分片
func (m *ShardedLockManager[T]) getShard(key string) *LockShard[T] {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	hash := hasher.Sum32()

	// 使用位运算快速取模
	shardIndex := hash & m.shardMask
	return m.shards[shardIndex]
}

// getShardIndex 根据键名获取分片索引（用于测试）
func (m *ShardedLockManager[T]) getShardIndex(key string) uint32 {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	hash := hasher.Sum32()

	// 使用位运算快速取模
	return hash & m.shardMask
}

// cleanupLock 清理单个锁
func (m *ShardedLockManager[T]) cleanupLock(lock *ShardedLock[T]) {
	shard := lock.shard

	// 检查shard是否为nil，避免panic
	if shard == nil {
		return
	}

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 再次检查引用计数，避免竞态条件
	if atomic.LoadInt32(&lock.refCount) > 0 {
		return
	}

	// 从分片中删除
	delete(shard.locks, lock.key)

	// 重置锁状态并放回对象池
	lock.key = ""
	lock.value = ""
	lock.unlocked = true
	lock.ttl = 0
	lock.expireAt = time.Time{}
	lock.refCount = 0
	lock.shard = nil

	shard.pool.Put(lock)
}

// cleanupExpiredLocks 后台清理过期锁
func (m *ShardedLockManager[T]) cleanupExpiredLocks() {
	defer close(m.cleanupDone)

	ticker := time.NewTicker(30 * time.Second) // 每30秒清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performCleanup()
		case <-m.cleanupStop:
			return
		}
	}
}

// performCleanup 执行清理操作
func (m *ShardedLockManager[T]) performCleanup() {
	now := time.Now()

	for _, shard := range m.shards {
		shard.mu.Lock()

		// 收集需要清理的锁
		var toCleanup []*ShardedLock[T]
		for _, lock := range shard.locks {
			lock.mu.RLock()
			expired := lock.unlocked || now.After(lock.expireAt)
			refCount := atomic.LoadInt32(&lock.refCount)
			lock.mu.RUnlock()

			// 只清理已过期且无引用的锁
			if expired && refCount <= 1 {
				toCleanup = append(toCleanup, lock)
			}
		}

		// 执行清理
		for _, lock := range toCleanup {
			delete(shard.locks, lock.key)

			// 重置并放回对象池
			lock.key = ""
			lock.value = ""
			lock.unlocked = true
			lock.ttl = 0
			lock.expireAt = time.Time{}
			atomic.StoreInt32(&lock.refCount, 0)
			lock.shard = nil

			shard.pool.Put(lock)
		}

		shard.mu.Unlock()
	}
}

// Close 关闭分片锁管理器
func (m *ShardedLockManager[T]) Close() {
	close(m.cleanupStop)
	<-m.cleanupDone
}

// GetStats 获取分片锁统计信息
func (m *ShardedLockManager[T]) GetStats() ShardedLockStats {
	stats := ShardedLockStats{
		ShardCount: int(m.shardCount),
		Shards:     make([]ShardStats, m.shardCount),
	}

	for i, shard := range m.shards {
		shard.mu.RLock()
		stats.Shards[i] = ShardStats{
			LockCount: len(shard.locks),
		}
		stats.TotalLocks += len(shard.locks)
		shard.mu.RUnlock()
	}

	return stats
}

// ShardedLockStats 分片锁统计信息
type ShardedLockStats struct {
	ShardCount int
	TotalLocks int
	Shards     []ShardStats
}

// ShardStats 单个分片统计信息
type ShardStats struct {
	LockCount int
}

// ===== ShardedLock 方法实现 =====

// Lock 获取锁
func (l *ShardedLock[T]) Lock(ctx context.Context, opts ...LockOption) error {
	retryConfig := NewRetryConfigFromLockOptions(l.key, opts...)
	return RetryLockOperation(ctx, retryConfig, func(ctx context.Context) (bool, error) {
		return l.TryLock(ctx, opts...)
	})
}

// TryLock 尝试获取锁
func (l *ShardedLock[T]) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)

	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否已被持有
	if !l.unlocked {
		if time.Now().After(l.expireAt) {
			// 锁已过期，可以重新获取
			l.unlocked = true
		} else {
			return false, nil
		}
	}

	// 获取锁
	l.unlocked = false
	l.value = config.Value
	l.ttl = config.LockTTL
	l.expireAt = time.Now().Add(config.LockTTL)

	return true, nil
}

// Unlock 释放锁
func (l *ShardedLock[T]) Unlock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.unlocked {
		return NewLockNotFoundError(l.key)
	}

	l.unlocked = true
	return nil
}

// Extend 续约锁
func (l *ShardedLock[T]) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.unlocked {
		return false, NewLockNotFoundError(l.key)
	}

	l.ttl = ttl
	l.expireAt = time.Now().Add(ttl)
	return true, nil
}

// IsLocked 检查锁是否被持有
func (l *ShardedLock[T]) IsLocked(ctx context.Context) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.unlocked {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(l.expireAt) {
		l.unlocked = true
		return false, nil
	}

	return true, nil
}

// GetTTL 获取锁的剩余时间
func (l *ShardedLock[T]) GetTTL(ctx context.Context) (time.Duration, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.unlocked {
		return 0, NewLockNotFoundError(l.key)
	}

	// 检查是否过期
	if time.Now().After(l.expireAt) {
		l.unlocked = true
		return 0, NewLockError(ErrCodeLockExpired, "锁已过期", SeverityInfo).WithContext("key", l.key)
	}

	return time.Until(l.expireAt), nil
}

// GetKey 获取锁的键名
func (l *ShardedLock[T]) GetKey() string {
	return l.key
}

// GetValue 获取锁的值
func (l *ShardedLock[T]) GetValue() string {
	return l.value
}

// GetData 获取锁的数据
func (l *ShardedLock[T]) GetData() T {
	return l.data
}

// ===== 读写锁专用方法 =====

// TryRLockRW 尝试获取读锁（仅用于RWLockData类型）
func TryRLockRW(l *ShardedLock[RWLockData], ctx context.Context, opts ...LockOption) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 如果写锁被持有，检查是否过期
	if !l.unlocked {
		if time.Now().After(l.expireAt) {
			// 写锁已过期，可以获取读锁
			l.unlocked = true
		} else {
			return false, nil
		}
	}

	// 增加读锁计数
	atomic.AddInt32(&l.data.readers, 1)
	return true, nil
}

// RUnlockRW 释放读锁（仅用于RWLockData类型）
func RUnlockRW(l *ShardedLock[RWLockData], ctx context.Context) error {
	atomic.AddInt32(&l.data.readers, -1)
	return nil
}

// GetReadCountRW 获取读锁数量（仅用于RWLockData类型）
func GetReadCountRW(l *ShardedLock[RWLockData], ctx context.Context) (int, error) {
	return int(atomic.LoadInt32(&l.data.readers)), nil
}

// ===== 公平锁专用方法 =====

// TryFairLockFair 尝试获取公平锁（仅用于FairLockData类型）
func TryFairLockFair(l *ShardedLock[FairLockData], ctx context.Context, opts ...LockOption) (bool, error) {
	config := ApplyLockOptions(opts)

	l.mu.Lock()
	defer l.mu.Unlock()

	// 如果锁已经被持有，检查是否过期
	if !l.unlocked {
		if time.Now().After(l.expireAt) {
			// 锁已过期，可以重新获取
			l.unlocked = true
			if len(l.data.queue) > 0 {
				l.data.queue = l.data.queue[1:] // 移除队首
			}
		} else {
			// 将当前请求加入队列
			l.data.queue = append(l.data.queue, config.Value)
			return false, nil
		}
	}

	// 检查是否是队列中的第一个
	if len(l.data.queue) > 0 && l.data.queue[0] != config.Value {
		// 不是第一个，加入队列
		l.data.queue = append(l.data.queue, config.Value)
		return false, nil
	}

	// 获取锁
	l.unlocked = false
	l.value = config.Value
	l.ttl = config.LockTTL
	l.expireAt = time.Now().Add(config.LockTTL)

	// 将当前值加入队列（如果还没有的话）
	if len(l.data.queue) == 0 || l.data.queue[0] != config.Value {
		l.data.queue = append([]string{config.Value}, l.data.queue...)
	}

	return true, nil
}

// UnlockFairFair 释放公平锁（仅用于FairLockData类型）
func UnlockFairFair(l *ShardedLock[FairLockData], ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.unlocked {
		return NewLockNotFoundError(l.key)
	}

	l.unlocked = true
	if len(l.data.queue) > 0 {
		l.data.queue = l.data.queue[1:] // 移除队首
	}
	return nil
}

// IsLockedFairFair 检查公平锁是否被持有（仅用于FairLockData类型）
func IsLockedFairFair(l *ShardedLock[FairLockData], ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.unlocked {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(l.expireAt) {
		l.unlocked = true
		if len(l.data.queue) > 0 {
			l.data.queue = l.data.queue[1:] // 移除队首
		}
		return false, nil
	}

	return true, nil
}

// GetTTLFairFair 获取公平锁的剩余时间（仅用于FairLockData类型）
func GetTTLFairFair(l *ShardedLock[FairLockData], ctx context.Context) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.unlocked {
		return 0, NewLockNotFoundError(l.key)
	}

	// 检查是否过期
	if time.Now().After(l.expireAt) {
		l.unlocked = true
		if len(l.data.queue) > 0 {
			l.data.queue = l.data.queue[1:] // 移除队首
		}
		return 0, NewLockError(ErrCodeLockExpired, "锁已过期", SeverityInfo).WithContext("key", l.key)
	}

	return time.Until(l.expireAt), nil
}

// ===== 具体锁类型的数据结构 =====

// SimpleLockData 简单锁的数据结构
type SimpleLockData struct {
	// 简单锁不需要额外数据
}

// RWLockData 读写锁的数据结构
type RWLockData struct {
	readers int32 // 读锁计数器
}

// FairLockData 公平锁的数据结构
type FairLockData struct {
	queue []string // 等待队列
}

// ===== 全局分片锁管理器实例 =====

var (
	// 全局简单锁管理器
	globalSimpleLockManager *ShardedLockManager[SimpleLockData]

	// 全局读写锁管理器
	globalRWLockManager *ShardedLockManager[RWLockData]

	// 全局公平锁管理器
	globalFairLockManager *ShardedLockManager[FairLockData]

	// 初始化锁
	initOnce sync.Once
)

// initGlobalManagers 初始化全局管理器
func initGlobalManagers(shardCount int) {
	initOnce.Do(func() {
		globalSimpleLockManager = NewShardedLockManager(shardCount, func() SimpleLockData {
			return SimpleLockData{}
		})

		globalRWLockManager = NewShardedLockManager(shardCount, func() RWLockData {
			return RWLockData{}
		})

		globalFairLockManager = NewShardedLockManager(shardCount, func() FairLockData {
			return FairLockData{
				queue: make([]string, 0),
			}
		})
	})
}

// GetGlobalSimpleLockManager 获取全局简单锁管理器
func GetGlobalSimpleLockManager(shardCount int) *ShardedLockManager[SimpleLockData] {
	initGlobalManagers(shardCount)
	return globalSimpleLockManager
}

// GetGlobalRWLockManager 获取全局读写锁管理器
func GetGlobalRWLockManager(shardCount int) *ShardedLockManager[RWLockData] {
	initGlobalManagers(shardCount)
	return globalRWLockManager
}

// GetGlobalFairLockManager 获取全局公平锁管理器
func GetGlobalFairLockManager(shardCount int) *ShardedLockManager[FairLockData] {
	initGlobalManagers(shardCount)
	return globalFairLockManager
}
