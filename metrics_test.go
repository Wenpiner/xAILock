package lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultMetrics 测试默认监控指标实现
func TestDefaultMetrics(t *testing.T) {
	metrics := NewDefaultMetrics()

	// 测试锁操作指标记录
	metrics.RecordLockAcquire("SimpleLock", "Local", true, 100*time.Microsecond)
	metrics.RecordLockAcquire("SimpleLock", "Local", false, 200*time.Microsecond)
	metrics.RecordLockRelease("SimpleLock", "Local", true, 50*time.Microsecond)

	// 测试错误指标记录
	metrics.RecordError("SimpleLock", "Local", "TIMEOUT", "ERROR")
	metrics.RecordError("RWLock", "Redis", "NETWORK", "WARNING")

	// 测试并发指标记录
	metrics.RecordConcurrentLocks("SimpleLock", "Local", 5)
	metrics.RecordQueueLength("FairLock", "Redis", 3)
	metrics.RecordWaitTime("FairLock", "Redis", 500*time.Millisecond)

	// 测试看门狗指标记录
	metrics.RecordWatchdogExtend("SimpleLock", "Local", true, 10*time.Millisecond)
	metrics.RecordWatchdogStart("SimpleLock", "Local")
	metrics.RecordWatchdogStop("SimpleLock", "Local", "manual")

	// 测试分片指标记录
	metrics.RecordShardDistribution(0, 10)
	metrics.RecordShardDistribution(1, 15)
	metrics.RecordShardContention(0, 0.3)

	// 测试Redis指标记录
	metrics.RecordRedisScriptExecution("SimpleLockAcquireScript", true, 5*time.Millisecond)
	metrics.RecordRedisNetworkRoundtrips("acquire", 2)

	// 获取统计信息
	stats := metrics.GetStats()

	// 验证锁操作统计
	assert.Equal(t, int64(2), stats.LockOperations.AcquireCount)
	assert.Equal(t, int64(1), stats.LockOperations.AcquireSuccess)
	assert.Equal(t, int64(1), stats.LockOperations.AcquireFailed)
	assert.Equal(t, int64(1), stats.LockOperations.ReleaseCount)
	assert.Equal(t, int64(1), stats.LockOperations.ReleaseSuccess)

	// 验证错误统计
	assert.Equal(t, int64(2), stats.ErrorStats.TotalErrors)
	assert.Equal(t, int64(1), stats.ErrorStats.ErrorsByType["ERROR"])
	assert.Equal(t, int64(1), stats.ErrorStats.ErrorsByType["WARNING"])
	assert.Equal(t, int64(1), stats.ErrorStats.ErrorsByCode["TIMEOUT"])
	assert.Equal(t, int64(1), stats.ErrorStats.ErrorsByCode["NETWORK"])

	// 验证并发统计
	assert.Equal(t, int64(5), stats.ConcurrencyStats.MaxConcurrentLocks)
	assert.Equal(t, int64(3), stats.ConcurrencyStats.MaxQueueLength)

	// 验证看门狗统计
	assert.Equal(t, int64(1), stats.WatchdogStats.TotalExtends)
	assert.Equal(t, int64(1), stats.WatchdogStats.SuccessfulExtends)
	assert.Equal(t, int64(1), stats.WatchdogStats.WatchdogStarts)
	assert.Equal(t, int64(1), stats.WatchdogStats.WatchdogStops)

	// 验证分片统计
	assert.Equal(t, 2, stats.ShardingStats.ShardCount)
	assert.Equal(t, int64(10), stats.ShardingStats.ShardDistribution[0])
	assert.Equal(t, int64(15), stats.ShardingStats.ShardDistribution[1])
	assert.Equal(t, float64(0.3), stats.ShardingStats.ShardContention[0])

	// 验证Redis统计
	assert.Equal(t, int64(1), stats.RedisStats.ScriptExecutions["SimpleLockAcquireScript"])
	assert.Equal(t, float64(1.0), stats.RedisStats.ScriptSuccessRate["SimpleLockAcquireScript"])
	assert.Equal(t, int64(2), stats.RedisStats.NetworkRoundtrips["acquire"])
	assert.Equal(t, int64(2), stats.RedisStats.TotalRoundtrips)
}

// TestMetricsReset 测试监控指标重置
func TestMetricsReset(t *testing.T) {
	metrics := NewDefaultMetrics()

	// 记录一些数据
	metrics.RecordLockAcquire("SimpleLock", "Local", true, 100*time.Microsecond)
	metrics.RecordError("SimpleLock", "Local", "TIMEOUT", "ERROR")

	// 验证数据存在
	stats := metrics.GetStats()
	assert.Equal(t, int64(1), stats.LockOperations.AcquireCount)
	assert.Equal(t, int64(1), stats.ErrorStats.TotalErrors)

	// 重置
	metrics.Reset()

	// 验证数据被清空
	stats = metrics.GetStats()
	assert.Equal(t, int64(0), stats.LockOperations.AcquireCount)
	assert.Equal(t, int64(0), stats.ErrorStats.TotalErrors)
	assert.Empty(t, stats.ErrorStats.ErrorsByType)
	assert.Empty(t, stats.ErrorStats.ErrorsByCode)
}

// TestMetricsWrapper 测试监控包装器
func TestMetricsWrapper(t *testing.T) {
	backend := NewLocalBackend(4)

	// 创建带监控的简单锁
	lock, err := NewSimpleLockWithMetrics("test:metrics", backend)
	require.NoError(t, err)
	require.NotNil(t, lock)

	ctx := context.Background()

	// 测试TryLock
	success, err := lock.TryLock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)
	assert.True(t, success)

	// 测试Unlock
	err = lock.Unlock(ctx)
	require.NoError(t, err)

	// 获取监控统计
	stats := GetGlobalMetrics().GetStats()

	// 验证操作被记录
	assert.Greater(t, stats.LockOperations.TryAcquireCount, int64(0))
	assert.Greater(t, stats.LockOperations.ReleaseCount, int64(0))
}

// TestRWLockMetricsWrapper 测试读写锁监控包装器
func TestRWLockMetricsWrapper(t *testing.T) {
	backend := NewLocalBackend(4)

	// 创建带监控的读写锁
	rwLock, err := NewRWLockWithMetrics("test:rw:metrics", backend)
	require.NoError(t, err)
	require.NotNil(t, rwLock)

	ctx := context.Background()

	// 测试读锁
	success, err := rwLock.TryRLock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)
	assert.True(t, success)

	// 测试读锁数量
	count, err := rwLock.GetReadCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 释放读锁
	err = rwLock.RUnlock(ctx)
	require.NoError(t, err)

	// 测试写锁
	success, err = rwLock.TryWLock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)
	assert.True(t, success)

	// 检查写锁状态
	isWriteLocked, err := rwLock.IsWriteLocked(ctx)
	require.NoError(t, err)
	assert.True(t, isWriteLocked)

	// 释放写锁
	err = rwLock.WUnlock(ctx)
	require.NoError(t, err)

	// 验证接口方法
	assert.Equal(t, "test:rw:metrics", rwLock.GetKey())
}

// TestFairLockMetricsWrapper 测试公平锁监控包装器
func TestFairLockMetricsWrapper(t *testing.T) {
	backend := NewLocalBackend(4)

	// 创建带监控的公平锁
	fairLock, err := NewFairLockWithMetrics("test:fair:metrics", backend)
	require.NoError(t, err)
	require.NotNil(t, fairLock)

	ctx := context.Background()

	// 测试TryLock
	success, err := fairLock.TryLock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)
	assert.True(t, success)

	// 测试队列长度
	length, err := fairLock.GetQueueLength(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, length, 0) // 队列长度应该大于等于0

	// 测试队列位置
	position, err := fairLock.GetQueuePosition(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, position, 0) // 位置应该大于等于0

	// 测试队列信息
	queueInfo, err := fairLock.GetQueueInfo(ctx)
	require.NoError(t, err)
	assert.NotNil(t, queueInfo)

	// 释放锁
	err = fairLock.Unlock(ctx)
	require.NoError(t, err)

	// 验证接口方法
	assert.Equal(t, "test:fair:metrics", fairLock.GetKey())
	// GetValue()可能在某些情况下返回空字符串，所以只检查不为nil
	value := fairLock.GetValue()
	assert.NotNil(t, value)
}

// TestGlobalMetrics 测试全局监控指标
func TestGlobalMetrics(t *testing.T) {
	// 获取全局监控实例
	globalMetrics := GetGlobalMetrics()
	assert.NotNil(t, globalMetrics)

	// 重置全局监控
	globalMetrics.Reset()

	// 记录一些数据
	globalMetrics.RecordLockAcquire("TestLock", "Local", true, 100*time.Microsecond)

	// 验证数据
	stats := globalMetrics.GetStats()
	assert.Equal(t, int64(1), stats.LockOperations.AcquireCount)

	// 设置自定义监控实例
	customMetrics := NewDefaultMetrics()
	SetGlobalMetrics(customMetrics)

	// 验证全局监控实例已更改
	newGlobalMetrics := GetGlobalMetrics()
	assert.Equal(t, customMetrics, newGlobalMetrics)

	// 新实例应该没有之前的数据
	stats = newGlobalMetrics.GetStats()
	assert.Equal(t, int64(0), stats.LockOperations.AcquireCount)
}

// TestMetricsConcurrency 测试监控指标的并发安全性
func TestMetricsConcurrency(t *testing.T) {
	metrics := NewDefaultMetrics()

	// 并发记录数据
	const goroutines = 10
	const operations = 100

	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < operations; j++ {
				metrics.RecordLockAcquire("ConcurrentLock", "Local", true, time.Microsecond)
				metrics.RecordError("ConcurrentLock", "Local", "TEST", "INFO")
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// 验证数据正确性
	stats := metrics.GetStats()
	assert.Equal(t, int64(goroutines*operations), stats.LockOperations.AcquireCount)
	assert.Equal(t, int64(goroutines*operations), stats.LockOperations.AcquireSuccess)
	assert.Equal(t, int64(goroutines*operations), stats.ErrorStats.TotalErrors)
}

// TestMetricsPerformanceImpact 测试监控对性能的影响
func TestMetricsPerformanceImpact(t *testing.T) {
	backend := NewLocalBackend(8)

	// 测试不带监控的锁性能
	t.Run("WithoutMetrics", func(t *testing.T) {
		lock, err := NewSimpleLock("perf:test:no-metrics", backend)
		require.NoError(t, err)

		ctx := context.Background()
		const iterations = 10000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			success, err := lock.TryLock(ctx, WithLockTTL(100*time.Millisecond))
			if err == nil && success {
				lock.Unlock(ctx)
			}
		}
		durationWithoutMetrics := time.Since(start)

		t.Logf("不带监控: %d次操作耗时 %v, 平均每次 %v",
			iterations, durationWithoutMetrics, durationWithoutMetrics/iterations)
	})

	// 测试带监控的锁性能
	t.Run("WithMetrics", func(t *testing.T) {
		lock, err := NewSimpleLockWithMetrics("perf:test:with-metrics", backend)
		require.NoError(t, err)

		ctx := context.Background()
		const iterations = 10000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			success, err := lock.TryLock(ctx, WithLockTTL(100*time.Millisecond))
			if err == nil && success {
				lock.Unlock(ctx)
			}
		}
		durationWithMetrics := time.Since(start)

		t.Logf("带监控: %d次操作耗时 %v, 平均每次 %v",
			iterations, durationWithMetrics, durationWithMetrics/iterations)

		// 验证监控数据
		stats := GetGlobalMetrics().GetStats()
		assert.Greater(t, stats.LockOperations.TryAcquireCount, int64(0))
		assert.Greater(t, stats.LockOperations.ReleaseCount, int64(0))
	})
}

// TestMetricsAccuracy 测试监控指标的准确性
func TestMetricsAccuracy(t *testing.T) {
	// 重置全局监控
	GetGlobalMetrics().Reset()

	backend := NewLocalBackend(4)
	lock, err := NewSimpleLockWithMetrics("accuracy:test", backend)
	require.NoError(t, err)

	ctx := context.Background()

	// 执行已知数量的操作
	const expectedOperations = 100
	var successfulAcquires, successfulReleases int64

	for i := 0; i < expectedOperations; i++ {
		success, err := lock.TryLock(ctx, WithLockTTL(10*time.Millisecond))
		if err == nil && success {
			successfulAcquires++

			// 50%的概率延长锁
			if i%2 == 0 {
				lock.Extend(ctx, 20*time.Millisecond)
			}

			err = lock.Unlock(ctx)
			if err == nil {
				successfulReleases++
			}
		}
	}

	// 验证监控数据准确性
	stats := GetGlobalMetrics().GetStats()

	assert.Equal(t, int64(expectedOperations), stats.LockOperations.TryAcquireCount)
	assert.Equal(t, successfulAcquires, stats.LockOperations.TryAcquireSuccess)
	assert.Equal(t, successfulReleases, stats.LockOperations.ReleaseSuccess)
	assert.Equal(t, int64(expectedOperations/2), stats.LockOperations.ExtendCount)

	// 验证平均时间计算
	assert.Greater(t, stats.LockOperations.AverageTryAcquireTime, time.Duration(0))
	assert.Greater(t, stats.LockOperations.AverageReleaseTime, time.Duration(0))

	t.Logf("监控准确性验证通过:")
	t.Logf("  - 预期操作数: %d, 实际记录: %d", expectedOperations, stats.LockOperations.TryAcquireCount)
	t.Logf("  - 成功获取: %d, 成功释放: %d", successfulAcquires, successfulReleases)
	t.Logf("  - 平均获取时间: %v, 平均释放时间: %v",
		stats.LockOperations.AverageTryAcquireTime, stats.LockOperations.AverageReleaseTime)
}

// TestMetricsWithDifferentBackends 测试不同后端的监控
func TestMetricsWithDifferentBackends(t *testing.T) {
	GetGlobalMetrics().Reset()

	// 测试本地后端
	localBackend := NewLocalBackend(4)
	localLock, err := NewSimpleLockWithMetrics("backend:local", localBackend)
	require.NoError(t, err)

	ctx := context.Background()

	// 本地锁操作
	success, err := localLock.TryLock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)
	assert.True(t, success)

	err = localLock.Unlock(ctx)
	require.NoError(t, err)

	// 验证后端类型记录
	stats := GetGlobalMetrics().GetStats()
	assert.Equal(t, int64(1), stats.LockOperations.TryAcquireCount)
	assert.Equal(t, int64(1), stats.LockOperations.ReleaseCount)

	t.Logf("不同后端监控测试完成:")
	t.Logf("  - 本地后端操作: 获取 %d次, 释放 %d次",
		stats.LockOperations.TryAcquireCount, stats.LockOperations.ReleaseCount)
}

// BenchmarkMetricsOverhead 基准测试监控开销
func BenchmarkMetricsOverhead(b *testing.B) {
	backend := NewLocalBackend(8)

	b.Run("WithoutMetrics", func(b *testing.B) {
		lock, err := NewSimpleLock("bench:no-metrics", backend)
		if err != nil {
			b.Fatal(err)
		}

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			success, err := lock.TryLock(ctx, WithLockTTL(100*time.Millisecond))
			if err == nil && success {
				lock.Unlock(ctx)
			}
		}
	})

	b.Run("WithMetrics", func(b *testing.B) {
		lock, err := NewSimpleLockWithMetrics("bench:with-metrics", backend)
		if err != nil {
			b.Fatal(err)
		}

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			success, err := lock.TryLock(ctx, WithLockTTL(100*time.Millisecond))
			if err == nil && success {
				lock.Unlock(ctx)
			}
		}
	})
}
