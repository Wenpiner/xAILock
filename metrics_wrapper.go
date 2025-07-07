package lock

import (
	"context"
	"runtime"
	"time"
)

// =============================================================================
// 监控包装器 - 为现有锁添加监控功能
// =============================================================================

// MetricsWrapper 监控包装器，为任何锁添加监控功能
type MetricsWrapper struct {
	lock     Lock
	lockType string
	backend  string
	metrics  Metrics
}

// NewMetricsWrapper 创建监控包装器
func NewMetricsWrapper(lock Lock, lockType, backend string, metrics Metrics) *MetricsWrapper {
	if metrics == nil {
		metrics = GetGlobalMetrics()
	}

	return &MetricsWrapper{
		lock:     lock,
		lockType: lockType,
		backend:  backend,
		metrics:  metrics,
	}
}

// TryLock 尝试获取锁（带监控）
func (w *MetricsWrapper) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	start := time.Now()
	success, err := w.lock.TryLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockTryAcquire(w.lockType, w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	// 记录并发锁数量
	if success && err == nil {
		w.updateConcurrentLocks(1)
	}

	return success, err
}

// Lock 获取锁（带监控）
func (w *MetricsWrapper) Lock(ctx context.Context, opts ...LockOption) error {
	start := time.Now()
	err := w.lock.Lock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockAcquire(w.lockType, w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		// 记录并发锁数量
		w.updateConcurrentLocks(1)
	}

	return err
}

// Unlock 释放锁（带监控）
func (w *MetricsWrapper) Unlock(ctx context.Context) error {
	start := time.Now()
	err := w.lock.Unlock(ctx)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockRelease(w.lockType, w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		// 更新并发锁数量
		w.updateConcurrentLocks(-1)
	}

	return err
}

// Extend 延长锁的生存时间（带监控）
func (w *MetricsWrapper) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := w.lock.Extend(ctx, ttl)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockExtend(w.lockType, w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	return success, err
}

// IsLocked 检查锁是否被持有
func (w *MetricsWrapper) IsLocked(ctx context.Context) (bool, error) {
	return w.lock.IsLocked(ctx)
}

// GetTTL 获取锁的剩余生存时间
func (w *MetricsWrapper) GetTTL(ctx context.Context) (time.Duration, error) {
	return w.lock.GetTTL(ctx)
}

// GetKey 获取锁的键名
func (w *MetricsWrapper) GetKey() string {
	return w.lock.GetKey()
}

// GetValue 获取锁的值
func (w *MetricsWrapper) GetValue() string {
	return w.lock.GetValue()
}

// recordError 记录错误信息
func (w *MetricsWrapper) recordError(err error) {
	if lockErr, ok := err.(*LockError); ok {
		w.metrics.RecordError(w.lockType, w.backend, string(lockErr.Code), lockErr.Severity.String())
	} else {
		w.metrics.RecordError(w.lockType, w.backend, "UNKNOWN", "ERROR")
	}
}

// updateConcurrentLocks 更新并发锁数量（简化实现）
func (w *MetricsWrapper) updateConcurrentLocks(delta int64) {
	// 这里是简化实现，实际应该维护全局计数器
	// 为了演示，我们使用goroutine数量作为近似值
	goroutineCount := int64(runtime.NumGoroutine())
	w.metrics.RecordGoroutineCount(goroutineCount)
}

// =============================================================================
// RWLock 监控包装器
// =============================================================================

// RWLockMetricsWrapper RWLock监控包装器
type RWLockMetricsWrapper struct {
	lock     RWLock
	lockType string
	backend  string
	metrics  Metrics
}

// NewRWLockMetricsWrapper 创建RWLock监控包装器
func NewRWLockMetricsWrapper(lock RWLock, backend string, metrics Metrics) *RWLockMetricsWrapper {
	if metrics == nil {
		metrics = GetGlobalMetrics()
	}

	return &RWLockMetricsWrapper{
		lock:     lock,
		lockType: "RWLock",
		backend:  backend,
		metrics:  metrics,
	}
}

// RLock 获取读锁（带监控）
func (w *RWLockMetricsWrapper) RLock(ctx context.Context, opts ...LockOption) error {
	start := time.Now()
	err := w.lock.RLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockAcquire(w.lockType+":Read", w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(1)
	}

	return err
}

// TryRLock 尝试获取读锁（带监控）
func (w *RWLockMetricsWrapper) TryRLock(ctx context.Context, opts ...LockOption) (bool, error) {
	start := time.Now()
	success, err := w.lock.TryRLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockTryAcquire(w.lockType+":Read", w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	if success && err == nil {
		w.updateConcurrentLocks(1)
	}

	return success, err
}

// RUnlock 释放读锁（带监控）
func (w *RWLockMetricsWrapper) RUnlock(ctx context.Context) error {
	start := time.Now()
	err := w.lock.RUnlock(ctx)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockRelease(w.lockType+":Read", w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(-1)
	}

	return err
}

// WLock 获取写锁（带监控）
func (w *RWLockMetricsWrapper) WLock(ctx context.Context, opts ...LockOption) error {
	start := time.Now()
	err := w.lock.WLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockAcquire(w.lockType+":Write", w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(1)
	}

	return err
}

// TryWLock 尝试获取写锁（带监控）
func (w *RWLockMetricsWrapper) TryWLock(ctx context.Context, opts ...LockOption) (bool, error) {
	start := time.Now()
	success, err := w.lock.TryWLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockTryAcquire(w.lockType+":Write", w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	if success && err == nil {
		w.updateConcurrentLocks(1)
	}

	return success, err
}

// WUnlock 释放写锁（带监控）
func (w *RWLockMetricsWrapper) WUnlock(ctx context.Context) error {
	start := time.Now()
	err := w.lock.WUnlock(ctx)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockRelease(w.lockType+":Write", w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(-1)
	}

	return err
}

// GetReadCount 获取当前读锁数量
func (w *RWLockMetricsWrapper) GetReadCount(ctx context.Context) (int, error) {
	count, err := w.lock.GetReadCount(ctx)

	// 记录队列长度（读锁数量）
	if err == nil {
		w.metrics.RecordQueueLength(w.lockType+":Read", w.backend, int64(count))
	}

	return count, err
}

// IsWriteLocked 检查是否有写锁
func (w *RWLockMetricsWrapper) IsWriteLocked(ctx context.Context) (bool, error) {
	return w.lock.IsWriteLocked(ctx)
}

// GetKey 获取锁的键名
func (w *RWLockMetricsWrapper) GetKey() string {
	return w.lock.GetKey()
}

// recordError 记录错误信息
func (w *RWLockMetricsWrapper) recordError(err error) {
	if lockErr, ok := err.(*LockError); ok {
		w.metrics.RecordError(w.lockType, w.backend, string(lockErr.Code), lockErr.Severity.String())
	} else {
		w.metrics.RecordError(w.lockType, w.backend, "UNKNOWN", "ERROR")
	}
}

// updateConcurrentLocks 更新并发锁数量
func (w *RWLockMetricsWrapper) updateConcurrentLocks(delta int64) {
	goroutineCount := int64(runtime.NumGoroutine())
	w.metrics.RecordGoroutineCount(goroutineCount)
}

// =============================================================================
// FairLock 监控包装器
// =============================================================================

// FairLockMetricsWrapper FairLock监控包装器
type FairLockMetricsWrapper struct {
	lock     FairLock
	lockType string
	backend  string
	metrics  Metrics
}

// NewFairLockMetricsWrapper 创建FairLock监控包装器
func NewFairLockMetricsWrapper(lock FairLock, backend string, metrics Metrics) *FairLockMetricsWrapper {
	if metrics == nil {
		metrics = GetGlobalMetrics()
	}

	return &FairLockMetricsWrapper{
		lock:     lock,
		lockType: "FairLock",
		backend:  backend,
		metrics:  metrics,
	}
}

// TryLock 尝试获取公平锁（带监控）
func (w *FairLockMetricsWrapper) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	start := time.Now()
	success, err := w.lock.TryLock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockTryAcquire(w.lockType, w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	if success && err == nil {
		w.updateConcurrentLocks(1)
	}

	return success, err
}

// Lock 获取公平锁（带监控）
func (w *FairLockMetricsWrapper) Lock(ctx context.Context, opts ...LockOption) error {
	start := time.Now()
	err := w.lock.Lock(ctx, opts...)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockAcquire(w.lockType, w.backend, success, duration)

	// 记录等待时间（公平锁特有）
	w.metrics.RecordWaitTime(w.lockType, w.backend, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(1)
	}

	return err
}

// Unlock 释放公平锁（带监控）
func (w *FairLockMetricsWrapper) Unlock(ctx context.Context) error {
	start := time.Now()
	err := w.lock.Unlock(ctx)
	duration := time.Since(start)

	// 记录监控指标
	success := err == nil
	w.metrics.RecordLockRelease(w.lockType, w.backend, success, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	} else {
		w.updateConcurrentLocks(-1)
	}

	return err
}

// Extend 延长公平锁的生存时间（带监控）
func (w *FairLockMetricsWrapper) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	start := time.Now()
	success, err := w.lock.Extend(ctx, ttl)
	duration := time.Since(start)

	// 记录监控指标
	w.metrics.RecordLockExtend(w.lockType, w.backend, success && err == nil, duration)

	// 记录错误
	if err != nil {
		w.recordError(err)
	}

	return success, err
}

// IsLocked 检查公平锁是否被持有
func (w *FairLockMetricsWrapper) IsLocked(ctx context.Context) (bool, error) {
	return w.lock.IsLocked(ctx)
}

// GetTTL 获取公平锁的剩余生存时间
func (w *FairLockMetricsWrapper) GetTTL(ctx context.Context) (time.Duration, error) {
	return w.lock.GetTTL(ctx)
}

// GetKey 获取公平锁的键名
func (w *FairLockMetricsWrapper) GetKey() string {
	return w.lock.GetKey()
}

// GetValue 获取公平锁的值
func (w *FairLockMetricsWrapper) GetValue() string {
	return w.lock.GetValue()
}

// GetQueueLength 获取等待队列长度
func (w *FairLockMetricsWrapper) GetQueueLength(ctx context.Context) (int, error) {
	length, err := w.lock.GetQueueLength(ctx)

	// 记录队列长度
	if err == nil {
		w.metrics.RecordQueueLength(w.lockType, w.backend, int64(length))
	}

	return length, err
}

// GetQueuePosition 获取在等待队列中的位置
func (w *FairLockMetricsWrapper) GetQueuePosition(ctx context.Context) (int, error) {
	return w.lock.GetQueuePosition(ctx)
}

// GetQueueInfo 获取队列详细信息
func (w *FairLockMetricsWrapper) GetQueueInfo(ctx context.Context) ([]string, error) {
	return w.lock.GetQueueInfo(ctx)
}

// recordError 记录错误信息
func (w *FairLockMetricsWrapper) recordError(err error) {
	if lockErr, ok := err.(*LockError); ok {
		w.metrics.RecordError(w.lockType, w.backend, string(lockErr.Code), lockErr.Severity.String())
	} else {
		w.metrics.RecordError(w.lockType, w.backend, "UNKNOWN", "ERROR")
	}
}

// updateConcurrentLocks 更新并发锁数量
func (w *FairLockMetricsWrapper) updateConcurrentLocks(delta int64) {
	goroutineCount := int64(runtime.NumGoroutine())
	w.metrics.RecordGoroutineCount(goroutineCount)
}

// =============================================================================
// 便利函数 - 创建带监控的锁
// =============================================================================

// NewSimpleLockWithMetrics 创建带监控的简单锁
func NewSimpleLockWithMetrics(key string, backend Backend) (Lock, error) {
	baseLock, err := NewSimpleLock(key, backend)
	if err != nil {
		return nil, err
	}
	backendName := getBackendName(backend)
	return NewMetricsWrapper(baseLock, "SimpleLock", backendName, nil), nil
}

// NewRWLockWithMetrics 创建带监控的读写锁
func NewRWLockWithMetrics(key string, backend Backend) (RWLock, error) {
	baseLock, err := NewRWLock(key, backend)
	if err != nil {
		return nil, err
	}
	backendName := getBackendName(backend)
	return NewRWLockMetricsWrapper(baseLock, backendName, nil), nil
}

// NewFairLockWithMetrics 创建带监控的公平锁
func NewFairLockWithMetrics(key string, backend Backend) (FairLock, error) {
	baseLock, err := NewFairLock(key, backend)
	if err != nil {
		return nil, err
	}
	backendName := getBackendName(backend)
	return NewFairLockMetricsWrapper(baseLock, backendName, nil), nil
}

// getBackendName 获取后端名称
func getBackendName(backend Backend) string {
	switch backend.(type) {
	case *LocalBackend:
		return "Local"
	case *RedisBackend:
		return "Redis"
	default:
		return "Unknown"
	}
}
