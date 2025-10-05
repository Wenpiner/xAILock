package xailock

import (
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// 监控指标接口定义
// =============================================================================

// Metrics 监控指标接口
type Metrics interface {
	// 锁操作指标
	RecordLockAcquire(lockType string, backend string, success bool, duration time.Duration)
	RecordLockRelease(lockType string, backend string, success bool, duration time.Duration)
	RecordLockExtend(lockType string, backend string, success bool, duration time.Duration)
	RecordLockTryAcquire(lockType string, backend string, success bool, duration time.Duration)

	// 错误指标
	RecordError(lockType string, backend string, errorCode string, errorType string)

	// 并发指标
	RecordConcurrentLocks(lockType string, backend string, count int64)
	RecordQueueLength(lockType string, backend string, length int64)
	RecordWaitTime(lockType string, backend string, duration time.Duration)

	// 资源使用指标
	RecordMemoryUsage(backend string, bytes int64)
	RecordConnectionCount(backend string, count int64)
	RecordGoroutineCount(count int64)

	// 看门狗指标
	RecordWatchdogExtend(lockType string, backend string, success bool, duration time.Duration)
	RecordWatchdogStart(lockType string, backend string)
	RecordWatchdogStop(lockType string, backend string, reason string)

	// 分片锁指标
	RecordShardDistribution(shardIndex int, lockCount int64)
	RecordShardContention(shardIndex int, contentionLevel float64)

	// Redis特定指标
	RecordRedisScriptExecution(scriptName string, success bool, duration time.Duration)
	RecordRedisNetworkRoundtrips(operation string, count int64)

	// 获取统计信息
	GetStats() MetricsStats
	Reset()
}

// MetricsStats 监控统计信息
type MetricsStats struct {
	// 锁操作统计
	LockOperations   LockOperationStats   `json:"lock_operations"`
	ErrorStats       ErrorStats           `json:"error_stats"`
	ConcurrencyStats ConcurrencyStats     `json:"concurrency_stats"`
	ResourceStats    ResourceStats        `json:"resource_stats"`
	WatchdogStats    WatchdogMetricsStats `json:"watchdog_stats"`
	ShardingStats    ShardingStats        `json:"sharding_stats"`
	RedisStats       RedisMetricsStats    `json:"redis_stats"`
}

// LockOperationStats 锁操作统计
type LockOperationStats struct {
	AcquireCount       int64         `json:"acquire_count"`
	AcquireSuccess     int64         `json:"acquire_success"`
	AcquireFailed      int64         `json:"acquire_failed"`
	AverageAcquireTime time.Duration `json:"average_acquire_time"`

	ReleaseCount       int64         `json:"release_count"`
	ReleaseSuccess     int64         `json:"release_success"`
	ReleaseFailed      int64         `json:"release_failed"`
	AverageReleaseTime time.Duration `json:"average_release_time"`

	ExtendCount       int64         `json:"extend_count"`
	ExtendSuccess     int64         `json:"extend_success"`
	ExtendFailed      int64         `json:"extend_failed"`
	AverageExtendTime time.Duration `json:"average_extend_time"`

	TryAcquireCount       int64         `json:"try_acquire_count"`
	TryAcquireSuccess     int64         `json:"try_acquire_success"`
	TryAcquireFailed      int64         `json:"try_acquire_failed"`
	AverageTryAcquireTime time.Duration `json:"average_try_acquire_time"`
}

// ErrorStats 错误统计
type ErrorStats struct {
	TotalErrors     int64            `json:"total_errors"`
	ErrorsByType    map[string]int64 `json:"errors_by_type"`
	ErrorsByCode    map[string]int64 `json:"errors_by_code"`
	ErrorsByBackend map[string]int64 `json:"errors_by_backend"`
}

// ConcurrencyStats 并发统计
type ConcurrencyStats struct {
	MaxConcurrentLocks int64         `json:"max_concurrent_locks"`
	CurrentLocks       int64         `json:"current_locks"`
	MaxQueueLength     int64         `json:"max_queue_length"`
	AverageWaitTime    time.Duration `json:"average_wait_time"`
	TotalWaitTime      time.Duration `json:"total_wait_time"`
}

// ResourceStats 资源使用统计
type ResourceStats struct {
	MaxMemoryUsage     int64 `json:"max_memory_usage"`
	CurrentMemoryUsage int64 `json:"current_memory_usage"`
	MaxConnectionCount int64 `json:"max_connection_count"`
	CurrentConnections int64 `json:"current_connections"`
	MaxGoroutineCount  int64 `json:"max_goroutine_count"`
	CurrentGoroutines  int64 `json:"current_goroutines"`
}

// WatchdogMetricsStats 看门狗监控统计
type WatchdogMetricsStats struct {
	ActiveWatchdogs   int64         `json:"active_watchdogs"`
	TotalExtends      int64         `json:"total_extends"`
	SuccessfulExtends int64         `json:"successful_extends"`
	FailedExtends     int64         `json:"failed_extends"`
	AverageExtendTime time.Duration `json:"average_extend_time"`
	WatchdogStarts    int64         `json:"watchdog_starts"`
	WatchdogStops     int64         `json:"watchdog_stops"`
}

// ShardingStats 分片统计
type ShardingStats struct {
	ShardCount        int             `json:"shard_count"`
	ShardDistribution map[int]int64   `json:"shard_distribution"`
	ShardContention   map[int]float64 `json:"shard_contention"`
	MaxShardLoad      int64           `json:"max_shard_load"`
	MinShardLoad      int64           `json:"min_shard_load"`
	LoadStandardDev   float64         `json:"load_standard_dev"`
}

// RedisMetricsStats Redis监控统计
type RedisMetricsStats struct {
	ScriptExecutions  map[string]int64         `json:"script_executions"`
	ScriptSuccessRate map[string]float64       `json:"script_success_rate"`
	AverageScriptTime map[string]time.Duration `json:"average_script_time"`
	NetworkRoundtrips map[string]int64         `json:"network_roundtrips"`
	TotalRoundtrips   int64                    `json:"total_roundtrips"`
}

// =============================================================================
// 默认监控指标实现
// =============================================================================

// DefaultMetrics 默认监控指标实现
type DefaultMetrics struct {
	mu sync.RWMutex

	// 锁操作计数器
	acquireCount   int64
	acquireSuccess int64
	acquireFailed  int64
	acquireTimeSum int64 // 纳秒

	releaseCount   int64
	releaseSuccess int64
	releaseFailed  int64
	releaseTimeSum int64 // 纳秒

	extendCount   int64
	extendSuccess int64
	extendFailed  int64
	extendTimeSum int64 // 纳秒

	tryAcquireCount   int64
	tryAcquireSuccess int64
	tryAcquireFailed  int64
	tryAcquireTimeSum int64 // 纳秒

	// 错误统计
	totalErrors     int64
	errorsByType    map[string]int64
	errorsByCode    map[string]int64
	errorsByBackend map[string]int64

	// 并发统计
	maxConcurrentLocks int64
	currentLocks       int64
	maxQueueLength     int64
	waitTimeSum        int64 // 纳秒
	waitCount          int64

	// 资源统计
	maxMemoryUsage     int64
	currentMemoryUsage int64
	maxConnectionCount int64
	currentConnections int64
	maxGoroutineCount  int64
	currentGoroutines  int64

	// 看门狗统计
	activeWatchdogs       int64
	totalExtends          int64
	successfulExtends     int64
	failedExtends         int64
	extendWatchdogTimeSum int64 // 纳秒
	watchdogStarts        int64
	watchdogStops         int64

	// 分片统计
	shardDistribution map[int]int64
	shardContention   map[int]float64

	// Redis统计
	scriptExecutions   map[string]int64
	scriptTimeSum      map[string]int64 // 纳秒
	scriptSuccessCount map[string]int64
	networkRoundtrips  map[string]int64
}

// NewDefaultMetrics 创建默认监控指标实例
func NewDefaultMetrics() *DefaultMetrics {
	return &DefaultMetrics{
		errorsByType:       make(map[string]int64),
		errorsByCode:       make(map[string]int64),
		errorsByBackend:    make(map[string]int64),
		shardDistribution:  make(map[int]int64),
		shardContention:    make(map[int]float64),
		scriptExecutions:   make(map[string]int64),
		scriptTimeSum:      make(map[string]int64),
		scriptSuccessCount: make(map[string]int64),
		networkRoundtrips:  make(map[string]int64),
	}
}

// 确保DefaultMetrics实现Metrics接口
var _ Metrics = (*DefaultMetrics)(nil)

// =============================================================================
// 全局监控管理
// =============================================================================

var (
	globalMetrics Metrics = NewDefaultMetrics()
	metricsOnce   sync.Once
)

// GetGlobalMetrics 获取全局监控指标实例
func GetGlobalMetrics() Metrics {
	return globalMetrics
}

// SetGlobalMetrics 设置全局监控指标实例
func SetGlobalMetrics(metrics Metrics) {
	globalMetrics = metrics
}

// EnableMetrics 启用监控指标收集
func EnableMetrics() {
	metricsOnce.Do(func() {
		if globalMetrics == nil {
			globalMetrics = NewDefaultMetrics()
		}
	})
}

// =============================================================================
// DefaultMetrics 方法实现 - 锁操作指标
// =============================================================================

// RecordLockAcquire 记录锁获取操作
func (m *DefaultMetrics) RecordLockAcquire(lockType string, backend string, success bool, duration time.Duration) {
	atomic.AddInt64(&m.acquireCount, 1)
	atomic.AddInt64(&m.acquireTimeSum, int64(duration))

	if success {
		atomic.AddInt64(&m.acquireSuccess, 1)
	} else {
		atomic.AddInt64(&m.acquireFailed, 1)
	}
}

// RecordLockRelease 记录锁释放操作
func (m *DefaultMetrics) RecordLockRelease(lockType string, backend string, success bool, duration time.Duration) {
	atomic.AddInt64(&m.releaseCount, 1)
	atomic.AddInt64(&m.releaseTimeSum, int64(duration))

	if success {
		atomic.AddInt64(&m.releaseSuccess, 1)
	} else {
		atomic.AddInt64(&m.releaseFailed, 1)
	}
}

// RecordLockExtend 记录锁续期操作
func (m *DefaultMetrics) RecordLockExtend(lockType string, backend string, success bool, duration time.Duration) {
	atomic.AddInt64(&m.extendCount, 1)
	atomic.AddInt64(&m.extendTimeSum, int64(duration))

	if success {
		atomic.AddInt64(&m.extendSuccess, 1)
	} else {
		atomic.AddInt64(&m.extendFailed, 1)
	}
}

// RecordLockTryAcquire 记录尝试获取锁操作
func (m *DefaultMetrics) RecordLockTryAcquire(lockType string, backend string, success bool, duration time.Duration) {
	atomic.AddInt64(&m.tryAcquireCount, 1)
	atomic.AddInt64(&m.tryAcquireTimeSum, int64(duration))

	if success {
		atomic.AddInt64(&m.tryAcquireSuccess, 1)
	} else {
		atomic.AddInt64(&m.tryAcquireFailed, 1)
	}
}

// =============================================================================
// DefaultMetrics 方法实现 - 错误指标
// =============================================================================

// RecordError 记录错误
func (m *DefaultMetrics) RecordError(lockType string, backend string, errorCode string, errorType string) {
	atomic.AddInt64(&m.totalErrors, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorsByType[errorType]++
	m.errorsByCode[errorCode]++
	m.errorsByBackend[backend]++
}

// =============================================================================
// DefaultMetrics 方法实现 - 并发指标
// =============================================================================

// RecordConcurrentLocks 记录并发锁数量
func (m *DefaultMetrics) RecordConcurrentLocks(lockType string, backend string, count int64) {
	atomic.StoreInt64(&m.currentLocks, count)

	// 更新最大并发锁数量
	for {
		current := atomic.LoadInt64(&m.maxConcurrentLocks)
		if count <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxConcurrentLocks, current, count) {
			break
		}
	}
}

// RecordQueueLength 记录队列长度
func (m *DefaultMetrics) RecordQueueLength(lockType string, backend string, length int64) {
	// 更新最大队列长度
	for {
		current := atomic.LoadInt64(&m.maxQueueLength)
		if length <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxQueueLength, current, length) {
			break
		}
	}
}

// RecordWaitTime 记录等待时间
func (m *DefaultMetrics) RecordWaitTime(lockType string, backend string, duration time.Duration) {
	atomic.AddInt64(&m.waitTimeSum, int64(duration))
	atomic.AddInt64(&m.waitCount, 1)
}

// =============================================================================
// DefaultMetrics 方法实现 - 资源使用指标
// =============================================================================

// RecordMemoryUsage 记录内存使用
func (m *DefaultMetrics) RecordMemoryUsage(backend string, bytes int64) {
	atomic.StoreInt64(&m.currentMemoryUsage, bytes)

	// 更新最大内存使用
	for {
		current := atomic.LoadInt64(&m.maxMemoryUsage)
		if bytes <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxMemoryUsage, current, bytes) {
			break
		}
	}
}

// RecordConnectionCount 记录连接数
func (m *DefaultMetrics) RecordConnectionCount(backend string, count int64) {
	atomic.StoreInt64(&m.currentConnections, count)

	// 更新最大连接数
	for {
		current := atomic.LoadInt64(&m.maxConnectionCount)
		if count <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxConnectionCount, current, count) {
			break
		}
	}
}

// RecordGoroutineCount 记录goroutine数量
func (m *DefaultMetrics) RecordGoroutineCount(count int64) {
	atomic.StoreInt64(&m.currentGoroutines, count)

	// 更新最大goroutine数量
	for {
		current := atomic.LoadInt64(&m.maxGoroutineCount)
		if count <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxGoroutineCount, current, count) {
			break
		}
	}
}

// =============================================================================
// DefaultMetrics 方法实现 - 看门狗指标
// =============================================================================

// RecordWatchdogExtend 记录看门狗续期操作
func (m *DefaultMetrics) RecordWatchdogExtend(lockType string, backend string, success bool, duration time.Duration) {
	atomic.AddInt64(&m.totalExtends, 1)
	atomic.AddInt64(&m.extendWatchdogTimeSum, int64(duration))

	if success {
		atomic.AddInt64(&m.successfulExtends, 1)
	} else {
		atomic.AddInt64(&m.failedExtends, 1)
	}
}

// RecordWatchdogStart 记录看门狗启动
func (m *DefaultMetrics) RecordWatchdogStart(lockType string, backend string) {
	atomic.AddInt64(&m.watchdogStarts, 1)
	atomic.AddInt64(&m.activeWatchdogs, 1)
}

// RecordWatchdogStop 记录看门狗停止
func (m *DefaultMetrics) RecordWatchdogStop(lockType string, backend string, reason string) {
	atomic.AddInt64(&m.watchdogStops, 1)
	atomic.AddInt64(&m.activeWatchdogs, -1)
}

// =============================================================================
// DefaultMetrics 方法实现 - 分片锁指标
// =============================================================================

// RecordShardDistribution 记录分片分布
func (m *DefaultMetrics) RecordShardDistribution(shardIndex int, lockCount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shardDistribution[shardIndex] = lockCount
}

// RecordShardContention 记录分片竞争
func (m *DefaultMetrics) RecordShardContention(shardIndex int, contentionLevel float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shardContention[shardIndex] = contentionLevel
}

// =============================================================================
// DefaultMetrics 方法实现 - Redis指标
// =============================================================================

// RecordRedisScriptExecution 记录Redis脚本执行
func (m *DefaultMetrics) RecordRedisScriptExecution(scriptName string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scriptExecutions[scriptName]++
	m.scriptTimeSum[scriptName] += int64(duration)

	if success {
		m.scriptSuccessCount[scriptName]++
	}
}

// RecordRedisNetworkRoundtrips 记录Redis网络往返次数
func (m *DefaultMetrics) RecordRedisNetworkRoundtrips(operation string, count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.networkRoundtrips[operation] += count
}

// =============================================================================
// DefaultMetrics 方法实现 - 统计信息获取
// =============================================================================

// GetStats 获取统计信息
func (m *DefaultMetrics) GetStats() MetricsStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算平均时间
	var avgAcquireTime, avgReleaseTime, avgExtendTime, avgTryAcquireTime, avgWaitTime, avgWatchdogExtendTime time.Duration

	if acquireCount := atomic.LoadInt64(&m.acquireCount); acquireCount > 0 {
		avgAcquireTime = time.Duration(atomic.LoadInt64(&m.acquireTimeSum) / acquireCount)
	}

	if releaseCount := atomic.LoadInt64(&m.releaseCount); releaseCount > 0 {
		avgReleaseTime = time.Duration(atomic.LoadInt64(&m.releaseTimeSum) / releaseCount)
	}

	if extendCount := atomic.LoadInt64(&m.extendCount); extendCount > 0 {
		avgExtendTime = time.Duration(atomic.LoadInt64(&m.extendTimeSum) / extendCount)
	}

	if tryAcquireCount := atomic.LoadInt64(&m.tryAcquireCount); tryAcquireCount > 0 {
		avgTryAcquireTime = time.Duration(atomic.LoadInt64(&m.tryAcquireTimeSum) / tryAcquireCount)
	}

	if waitCount := atomic.LoadInt64(&m.waitCount); waitCount > 0 {
		avgWaitTime = time.Duration(atomic.LoadInt64(&m.waitTimeSum) / waitCount)
	}

	if totalExtends := atomic.LoadInt64(&m.totalExtends); totalExtends > 0 {
		avgWatchdogExtendTime = time.Duration(atomic.LoadInt64(&m.extendWatchdogTimeSum) / totalExtends)
	}

	// 复制map数据
	errorsByType := make(map[string]int64)
	for k, v := range m.errorsByType {
		errorsByType[k] = v
	}

	errorsByCode := make(map[string]int64)
	for k, v := range m.errorsByCode {
		errorsByCode[k] = v
	}

	errorsByBackend := make(map[string]int64)
	for k, v := range m.errorsByBackend {
		errorsByBackend[k] = v
	}

	shardDistribution := make(map[int]int64)
	for k, v := range m.shardDistribution {
		shardDistribution[k] = v
	}

	shardContention := make(map[int]float64)
	for k, v := range m.shardContention {
		shardContention[k] = v
	}

	scriptExecutions := make(map[string]int64)
	for k, v := range m.scriptExecutions {
		scriptExecutions[k] = v
	}

	scriptSuccessRate := make(map[string]float64)
	for k, execCount := range m.scriptExecutions {
		if execCount > 0 {
			successCount := m.scriptSuccessCount[k]
			scriptSuccessRate[k] = float64(successCount) / float64(execCount)
		}
	}

	averageScriptTime := make(map[string]time.Duration)
	for k, execCount := range m.scriptExecutions {
		if execCount > 0 {
			timeSum := m.scriptTimeSum[k]
			averageScriptTime[k] = time.Duration(timeSum / execCount)
		}
	}

	networkRoundtrips := make(map[string]int64)
	var totalRoundtrips int64
	for k, v := range m.networkRoundtrips {
		networkRoundtrips[k] = v
		totalRoundtrips += v
	}

	// 计算分片统计
	var maxShardLoad, minShardLoad int64 = 0, 0
	var loadSum, loadSumSquares float64
	shardCount := len(m.shardDistribution)

	if shardCount > 0 {
		first := true
		for _, load := range m.shardDistribution {
			if first {
				maxShardLoad = load
				minShardLoad = load
				first = false
			} else {
				if load > maxShardLoad {
					maxShardLoad = load
				}
				if load < minShardLoad {
					minShardLoad = load
				}
			}
			loadSum += float64(load)
			loadSumSquares += float64(load) * float64(load)
		}
	}

	// 计算标准差
	var loadStandardDev float64
	if shardCount > 1 {
		mean := loadSum / float64(shardCount)
		variance := (loadSumSquares - loadSum*mean) / float64(shardCount-1)
		if variance > 0 {
			loadStandardDev = variance // 简化版，实际应该开平方根
		}
	}

	return MetricsStats{
		LockOperations: LockOperationStats{
			AcquireCount:          atomic.LoadInt64(&m.acquireCount),
			AcquireSuccess:        atomic.LoadInt64(&m.acquireSuccess),
			AcquireFailed:         atomic.LoadInt64(&m.acquireFailed),
			AverageAcquireTime:    avgAcquireTime,
			ReleaseCount:          atomic.LoadInt64(&m.releaseCount),
			ReleaseSuccess:        atomic.LoadInt64(&m.releaseSuccess),
			ReleaseFailed:         atomic.LoadInt64(&m.releaseFailed),
			AverageReleaseTime:    avgReleaseTime,
			ExtendCount:           atomic.LoadInt64(&m.extendCount),
			ExtendSuccess:         atomic.LoadInt64(&m.extendSuccess),
			ExtendFailed:          atomic.LoadInt64(&m.extendFailed),
			AverageExtendTime:     avgExtendTime,
			TryAcquireCount:       atomic.LoadInt64(&m.tryAcquireCount),
			TryAcquireSuccess:     atomic.LoadInt64(&m.tryAcquireSuccess),
			TryAcquireFailed:      atomic.LoadInt64(&m.tryAcquireFailed),
			AverageTryAcquireTime: avgTryAcquireTime,
		},
		ErrorStats: ErrorStats{
			TotalErrors:     atomic.LoadInt64(&m.totalErrors),
			ErrorsByType:    errorsByType,
			ErrorsByCode:    errorsByCode,
			ErrorsByBackend: errorsByBackend,
		},
		ConcurrencyStats: ConcurrencyStats{
			MaxConcurrentLocks: atomic.LoadInt64(&m.maxConcurrentLocks),
			CurrentLocks:       atomic.LoadInt64(&m.currentLocks),
			MaxQueueLength:     atomic.LoadInt64(&m.maxQueueLength),
			AverageWaitTime:    avgWaitTime,
			TotalWaitTime:      time.Duration(atomic.LoadInt64(&m.waitTimeSum)),
		},
		ResourceStats: ResourceStats{
			MaxMemoryUsage:     atomic.LoadInt64(&m.maxMemoryUsage),
			CurrentMemoryUsage: atomic.LoadInt64(&m.currentMemoryUsage),
			MaxConnectionCount: atomic.LoadInt64(&m.maxConnectionCount),
			CurrentConnections: atomic.LoadInt64(&m.currentConnections),
			MaxGoroutineCount:  atomic.LoadInt64(&m.maxGoroutineCount),
			CurrentGoroutines:  atomic.LoadInt64(&m.currentGoroutines),
		},
		WatchdogStats: WatchdogMetricsStats{
			ActiveWatchdogs:   atomic.LoadInt64(&m.activeWatchdogs),
			TotalExtends:      atomic.LoadInt64(&m.totalExtends),
			SuccessfulExtends: atomic.LoadInt64(&m.successfulExtends),
			FailedExtends:     atomic.LoadInt64(&m.failedExtends),
			AverageExtendTime: avgWatchdogExtendTime,
			WatchdogStarts:    atomic.LoadInt64(&m.watchdogStarts),
			WatchdogStops:     atomic.LoadInt64(&m.watchdogStops),
		},
		ShardingStats: ShardingStats{
			ShardCount:        shardCount,
			ShardDistribution: shardDistribution,
			ShardContention:   shardContention,
			MaxShardLoad:      maxShardLoad,
			MinShardLoad:      minShardLoad,
			LoadStandardDev:   loadStandardDev,
		},
		RedisStats: RedisMetricsStats{
			ScriptExecutions:  scriptExecutions,
			ScriptSuccessRate: scriptSuccessRate,
			AverageScriptTime: averageScriptTime,
			NetworkRoundtrips: networkRoundtrips,
			TotalRoundtrips:   totalRoundtrips,
		},
	}
}

// Reset 重置所有统计信息
func (m *DefaultMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 重置原子计数器
	atomic.StoreInt64(&m.acquireCount, 0)
	atomic.StoreInt64(&m.acquireSuccess, 0)
	atomic.StoreInt64(&m.acquireFailed, 0)
	atomic.StoreInt64(&m.acquireTimeSum, 0)

	atomic.StoreInt64(&m.releaseCount, 0)
	atomic.StoreInt64(&m.releaseSuccess, 0)
	atomic.StoreInt64(&m.releaseFailed, 0)
	atomic.StoreInt64(&m.releaseTimeSum, 0)

	atomic.StoreInt64(&m.extendCount, 0)
	atomic.StoreInt64(&m.extendSuccess, 0)
	atomic.StoreInt64(&m.extendFailed, 0)
	atomic.StoreInt64(&m.extendTimeSum, 0)

	atomic.StoreInt64(&m.tryAcquireCount, 0)
	atomic.StoreInt64(&m.tryAcquireSuccess, 0)
	atomic.StoreInt64(&m.tryAcquireFailed, 0)
	atomic.StoreInt64(&m.tryAcquireTimeSum, 0)

	atomic.StoreInt64(&m.totalErrors, 0)
	atomic.StoreInt64(&m.maxConcurrentLocks, 0)
	atomic.StoreInt64(&m.currentLocks, 0)
	atomic.StoreInt64(&m.maxQueueLength, 0)
	atomic.StoreInt64(&m.waitTimeSum, 0)
	atomic.StoreInt64(&m.waitCount, 0)

	atomic.StoreInt64(&m.maxMemoryUsage, 0)
	atomic.StoreInt64(&m.currentMemoryUsage, 0)
	atomic.StoreInt64(&m.maxConnectionCount, 0)
	atomic.StoreInt64(&m.currentConnections, 0)
	atomic.StoreInt64(&m.maxGoroutineCount, 0)
	atomic.StoreInt64(&m.currentGoroutines, 0)

	atomic.StoreInt64(&m.activeWatchdogs, 0)
	atomic.StoreInt64(&m.totalExtends, 0)
	atomic.StoreInt64(&m.successfulExtends, 0)
	atomic.StoreInt64(&m.failedExtends, 0)
	atomic.StoreInt64(&m.extendWatchdogTimeSum, 0)
	atomic.StoreInt64(&m.watchdogStarts, 0)
	atomic.StoreInt64(&m.watchdogStops, 0)

	// 重置map
	m.errorsByType = make(map[string]int64)
	m.errorsByCode = make(map[string]int64)
	m.errorsByBackend = make(map[string]int64)
	m.shardDistribution = make(map[int]int64)
	m.shardContention = make(map[int]float64)
	m.scriptExecutions = make(map[string]int64)
	m.scriptTimeSum = make(map[string]int64)
	m.scriptSuccessCount = make(map[string]int64)
	m.networkRoundtrips = make(map[string]int64)
}
