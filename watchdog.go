package xailock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ExtendableLock 定义可续约的锁接口
// 这个接口抽象了所有支持续约的锁类型
type ExtendableLock interface {
	// Extend 续约锁，延长TTL
	Extend(ctx context.Context, ttl time.Duration) (bool, error)
	// GetKey 获取锁的键名
	GetKey() string
	// GetValue 获取锁的值（持有者标识）
	GetValue() string
}

// WatchdogImpl 看门狗实现
type WatchdogImpl struct {
	// 基础配置
	lock     ExtendableLock // 要监控的锁
	interval time.Duration  // 续约间隔
	ttl      time.Duration  // 每次续约的TTL

	// 运行状态
	running int32           // 原子操作标记是否运行中
	ctx     context.Context // 看门狗上下文
	cancel  context.CancelFunc
	done    chan struct{} // 完成信号
	mu      sync.RWMutex  // 保护统计信息

	// 统计信息
	stats WatchdogStats
}

// NewWatchdog 创建新的看门狗实例
func NewWatchdog(lock ExtendableLock, interval, ttl time.Duration) Watchdog {
	return &WatchdogImpl{
		lock:     lock,
		interval: interval,
		ttl:      ttl,
		done:     make(chan struct{}),
		stats: WatchdogStats{
			Interval: interval,
		},
	}
}

// Start 启动看门狗
func (w *WatchdogImpl) Start(ctx context.Context) error {
	// 检查是否已经在运行
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		return NewLockError(ErrCodeInvalidOperation, "看门狗已经在运行", SeverityWarning).
			WithContext("key", w.lock.GetKey())
	}

	// 创建看门狗上下文
	w.ctx, w.cancel = context.WithCancel(ctx)

	// 初始化统计信息
	w.mu.Lock()
	w.stats.StartTime = time.Now()
	w.stats.ExtendCount = 0
	w.stats.FailedExtends = 0
	w.stats.LastExtendTime = time.Time{}
	w.stats.LastExtendResult = false
	w.mu.Unlock()

	// 启动看门狗goroutine
	go w.watchdogLoop()

	return nil
}

// Stop 停止看门狗
func (w *WatchdogImpl) Stop() error {
	// 检查是否在运行
	if !atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		return NewLockError(ErrCodeInvalidOperation, "看门狗未在运行", SeverityWarning).
			WithContext("key", w.lock.GetKey())
	}

	// 取消上下文
	if w.cancel != nil {
		w.cancel()
	}

	// 等待goroutine结束
	<-w.done

	return nil
}

// IsRunning 检查看门狗是否在运行
func (w *WatchdogImpl) IsRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// GetStats 获取看门狗统计信息
func (w *WatchdogImpl) GetStats() WatchdogStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// watchdogLoop 看门狗主循环
func (w *WatchdogImpl) watchdogLoop() {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			// 上下文被取消，退出循环
			return

		case <-ticker.C:
			// 执行续约
			w.performExtend()
		}
	}
}

// performExtend 执行续约操作
func (w *WatchdogImpl) performExtend() {
	// 创建续约上下文，设置超时
	extendCtx, cancel := context.WithTimeout(w.ctx, w.interval/2)
	defer cancel()

	// 执行续约
	success, err := w.lock.Extend(extendCtx, w.ttl)

	// 更新统计信息
	w.mu.Lock()
	w.stats.LastExtendTime = time.Now()
	w.stats.LastExtendResult = success && err == nil

	if w.stats.LastExtendResult {
		w.stats.ExtendCount++
	} else {
		w.stats.FailedExtends++
	}
	w.mu.Unlock()

	// 记录续约失败（可以根据需要添加日志或其他处理）
	if !w.stats.LastExtendResult {
		// 这里可以添加日志记录或错误处理
		// 目前只是更新统计信息，不中断看门狗运行
		_ = err // 避免未使用变量警告
	}
}

// WatchdogManager 看门狗管理器
type WatchdogManager struct {
	watchdogs map[string]Watchdog
	mu        sync.RWMutex
}

// NewWatchdogManager 创建看门狗管理器
func NewWatchdogManager() *WatchdogManager {
	return &WatchdogManager{
		watchdogs: make(map[string]Watchdog),
	}
}

// StartWatchdog 启动指定锁的看门狗
func (m *WatchdogManager) StartWatchdog(ctx context.Context, lock ExtendableLock, interval, ttl time.Duration) error {
	key := lock.GetKey()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if existing, exists := m.watchdogs[key]; exists {
		if existing.IsRunning() {
			return NewLockError(ErrCodeInvalidOperation, "看门狗已存在且正在运行", SeverityWarning).
				WithContext("key", key)
		}
		// 清理已停止的看门狗
		delete(m.watchdogs, key)
	}

	// 创建并启动新的看门狗
	watchdog := NewWatchdog(lock, interval, ttl)
	if err := watchdog.Start(ctx); err != nil {
		return err
	}

	m.watchdogs[key] = watchdog
	return nil
}

// StopWatchdog 停止指定锁的看门狗
func (m *WatchdogManager) StopWatchdog(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	watchdog, exists := m.watchdogs[key]
	if !exists {
		return NewLockError(ErrCodeLockNotFound, "看门狗不存在", SeverityWarning).
			WithContext("key", key)
	}

	if err := watchdog.Stop(); err != nil {
		return err
	}

	delete(m.watchdogs, key)
	return nil
}

// GetWatchdog 获取指定锁的看门狗
func (m *WatchdogManager) GetWatchdog(key string) (Watchdog, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	watchdog, exists := m.watchdogs[key]
	return watchdog, exists
}

// StopAll 停止所有看门狗
func (m *WatchdogManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for key, watchdog := range m.watchdogs {
		if err := watchdog.Stop(); err != nil {
			lastErr = err
		}
		delete(m.watchdogs, key)
	}

	return lastErr
}

// GetAllStats 获取所有看门狗的统计信息
func (m *WatchdogManager) GetAllStats() map[string]WatchdogStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]WatchdogStats)
	for key, watchdog := range m.watchdogs {
		stats[key] = watchdog.GetStats()
	}

	return stats
}

// 全局看门狗管理器实例
var globalWatchdogManager = NewWatchdogManager()

// GetGlobalWatchdogManager 获取全局看门狗管理器
func GetGlobalWatchdogManager() *WatchdogManager {
	return globalWatchdogManager
}

// =============================================================================
// 带看门狗的锁包装器
// =============================================================================

// LockWithWatchdogImpl 带看门狗的锁实现
type LockWithWatchdogImpl struct {
	lock     Lock             // 原始锁实例
	watchdog Watchdog         // 看门狗实例
	manager  *WatchdogManager // 看门狗管理器
}

// NewLockWithWatchdog 创建带看门狗的锁
func NewLockWithWatchdog(lock Lock, config *LockConfig) LockWithWatchdog {
	// 确保锁实现了ExtendableLock接口
	extendableLock, ok := lock.(ExtendableLock)
	if !ok {
		// 如果锁没有实现ExtendableLock接口，创建一个适配器
		extendableLock = &lockAdapter{lock}
	}

	watchdog := NewWatchdog(extendableLock, config.WatchdogInterval, config.LockTTL)

	return &LockWithWatchdogImpl{
		lock:     lock,
		watchdog: watchdog,
		manager:  GetGlobalWatchdogManager(),
	}
}

// 实现Lock接口的方法
func (l *LockWithWatchdogImpl) Lock(ctx context.Context, opts ...LockOption) error {
	return l.lock.Lock(ctx, opts...)
}

func (l *LockWithWatchdogImpl) TryLock(ctx context.Context, opts ...LockOption) (bool, error) {
	return l.lock.TryLock(ctx, opts...)
}

func (l *LockWithWatchdogImpl) Unlock(ctx context.Context) error {
	return l.lock.Unlock(ctx)
}

func (l *LockWithWatchdogImpl) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	return l.lock.Extend(ctx, ttl)
}

func (l *LockWithWatchdogImpl) IsLocked(ctx context.Context) (bool, error) {
	return l.lock.IsLocked(ctx)
}

func (l *LockWithWatchdogImpl) GetTTL(ctx context.Context) (time.Duration, error) {
	return l.lock.GetTTL(ctx)
}

func (l *LockWithWatchdogImpl) GetKey() string {
	return l.lock.GetKey()
}

func (l *LockWithWatchdogImpl) GetValue() string {
	return l.lock.GetValue()
}

// Start 启动看门狗
func (l *LockWithWatchdogImpl) Start(ctx context.Context) error {
	return l.watchdog.Start(ctx)
}

// Stop 停止看门狗
func (l *LockWithWatchdogImpl) Stop() error {
	return l.watchdog.Stop()
}

// IsRunning 检查看门狗是否在运行
func (l *LockWithWatchdogImpl) IsRunning() bool {
	return l.watchdog.IsRunning()
}

// GetStats 获取看门狗统计信息
func (l *LockWithWatchdogImpl) GetStats() WatchdogStats {
	return l.watchdog.GetStats()
}

// lockAdapter 锁适配器，为不直接实现ExtendableLock的锁提供适配
type lockAdapter struct {
	Lock
}

// 确保lockAdapter实现ExtendableLock接口
var _ ExtendableLock = (*lockAdapter)(nil)

// =============================================================================
// 便利函数
// =============================================================================

// StartWatchdogForLock 为指定锁启动看门狗
func StartWatchdogForLock(ctx context.Context, lock ExtendableLock, interval, ttl time.Duration) error {
	return GetGlobalWatchdogManager().StartWatchdog(ctx, lock, interval, ttl)
}

// StopWatchdogForLock 停止指定锁的看门狗
func StopWatchdogForLock(key string) error {
	return GetGlobalWatchdogManager().StopWatchdog(key)
}

// GetWatchdogStats 获取指定锁的看门狗统计信息
func GetWatchdogStats(key string) (WatchdogStats, bool) {
	watchdog, exists := GetGlobalWatchdogManager().GetWatchdog(key)
	if !exists {
		return WatchdogStats{}, false
	}
	return watchdog.GetStats(), true
}
