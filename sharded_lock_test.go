package xailock

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestShardedLockBasicFunctionality 测试分片锁基本功能
func TestShardedLockBasicFunctionality(t *testing.T) {
	factory := NewLocalLockFactory(16)

	// 测试简单锁
	t.Run("SimpleLock", func(t *testing.T) {
		lock, err := factory.CreateSimpleLock("test-simple")
		if err != nil {
			t.Fatalf("创建简单锁失败: %v", err)
		}
		ctx := context.Background()

		// 获取锁
		err = lock.Lock(ctx, WithLockTTL(time.Second*5))
		if err != nil {
			t.Fatalf("获取锁失败: %v", err)
		}

		// 检查锁状态
		locked, err := lock.IsLocked(ctx)
		if err != nil {
			t.Fatalf("检查锁状态失败: %v", err)
		}
		if !locked {
			t.Fatal("锁应该被持有")
		}

		// 释放锁
		err = lock.Unlock(ctx)
		if err != nil {
			t.Fatalf("释放锁失败: %v", err)
		}

		// 再次检查锁状态
		locked, err = lock.IsLocked(ctx)
		if err != nil {
			t.Fatalf("检查锁状态失败: %v", err)
		}
		if locked {
			t.Fatal("锁应该已被释放")
		}
	})

	// 测试读写锁
	t.Run("RWLock", func(t *testing.T) {
		lock, err := factory.CreateRWLock("test-rw")
		if err != nil {
			t.Fatalf("创建读写锁失败: %v", err)
		}
		ctx := context.Background()

		// 获取读锁
		err = lock.RLock(ctx, WithLockTTL(time.Second*5))
		if err != nil {
			t.Fatalf("获取读锁失败: %v", err)
		}

		// 检查读锁数量
		count, err := lock.GetReadCount(ctx)
		if err != nil {
			t.Fatalf("获取读锁数量失败: %v", err)
		}
		if count != 1 {
			t.Fatalf("读锁数量应该为1，实际为%d", count)
		}

		// 释放读锁
		err = lock.RUnlock(ctx)
		if err != nil {
			t.Fatalf("释放读锁失败: %v", err)
		}
	})

	// 测试公平锁
	t.Run("FairLock", func(t *testing.T) {
		lock, err := factory.CreateFairLock("test-fair")
		if err != nil {
			t.Fatalf("创建公平锁失败: %v", err)
		}
		ctx := context.Background()

		// 获取锁
		err = lock.Lock(ctx, WithLockTTL(time.Second*5), WithValue("test-value"))
		if err != nil {
			t.Fatalf("获取公平锁失败: %v", err)
		}

		// 检查锁状态
		locked, err := lock.IsLocked(ctx)
		if err != nil {
			t.Fatalf("检查锁状态失败: %v", err)
		}
		if !locked {
			t.Fatal("锁应该被持有")
		}

		// 检查队列长度
		queueLen, err := lock.GetQueueLength(ctx)
		if err != nil {
			t.Fatalf("获取队列长度失败: %v", err)
		}
		if queueLen != 1 {
			t.Fatalf("队列长度应该为1，实际为%d", queueLen)
		}

		// 释放锁
		err = lock.Unlock(ctx)
		if err != nil {
			t.Fatalf("释放公平锁失败: %v", err)
		}
	})
}

// TestShardedLockConcurrency 测试分片锁并发性能
func TestShardedLockConcurrency(t *testing.T) {
	const (
		numGoroutines = 100
		numOperations = 1000
		shardCount    = 16
	)

	factory := NewLocalLockFactory(shardCount)

	t.Run("SimpleLockConcurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount int64

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("test-key-%d", j%100) // 100个不同的键
					lock, err := factory.CreateSimpleLock(key)
					if err != nil {
						continue
					}
					ctx := context.Background()

					// 尝试获取锁
					success, err := lock.TryLock(ctx, WithLockTTL(time.Millisecond*100))
					if err != nil {
						continue
					}

					if success {
						// 模拟一些工作
						time.Sleep(time.Microsecond * 10)

						// 释放锁
						lock.Unlock(ctx)
						successCount++
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("分片锁并发测试完成:")
		t.Logf("- 协程数: %d", numGoroutines)
		t.Logf("- 每协程操作数: %d", numOperations)
		t.Logf("- 分片数: %d", shardCount)
		t.Logf("- 总耗时: %v", duration)
		t.Logf("- 成功获取锁次数: %d", successCount)
		t.Logf("- 平均每秒操作数: %.2f", float64(numGoroutines*numOperations)/duration.Seconds())
	})
}

// TestShardDistribution 测试分片分布均匀性
func TestShardDistribution(t *testing.T) {
	const (
		numKeys    = 10000
		shardCount = 16
	)

	manager := GetGlobalSimpleLockManager(shardCount)

	// 统计每个分片的键数量
	shardCounts := make([]int, shardCount)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		shardIndex := manager.getShardIndex(key)
		shardCounts[shardIndex]++
	}

	// 计算分布统计
	var total, min, max int
	min = numKeys
	for _, count := range shardCounts {
		total += count
		if count < min {
			min = count
		}
		if count > max {
			max = count
		}
	}

	average := float64(total) / float64(shardCount)
	variance := 0.0
	for _, count := range shardCounts {
		diff := float64(count) - average
		variance += diff * diff
	}
	variance /= float64(shardCount)
	stdDev := variance // 简化计算，不开平方根

	t.Logf("分片分布统计:")
	t.Logf("- 总键数: %d", numKeys)
	t.Logf("- 分片数: %d", shardCount)
	t.Logf("- 平均每分片: %.2f", average)
	t.Logf("- 最小分片键数: %d", min)
	t.Logf("- 最大分片键数: %d", max)
	t.Logf("- 标准差: %.2f", stdDev)

	// 检查分布是否相对均匀（允许20%的偏差）
	expectedPerShard := numKeys / shardCount
	tolerance := float64(expectedPerShard) * 0.2

	for i, count := range shardCounts {
		if float64(count) < float64(expectedPerShard)-tolerance ||
			float64(count) > float64(expectedPerShard)+tolerance {
			t.Errorf("分片%d的键数量%d超出预期范围[%.0f, %.0f]",
				i, count,
				float64(expectedPerShard)-tolerance,
				float64(expectedPerShard)+tolerance)
		}
	}
}

// TestMemoryUsage 测试内存使用情况
func TestMemoryUsage(t *testing.T) {
	const numLocks = 10000

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	factory := NewLocalLockFactory(16)
	locks := make([]Lock, numLocks)

	// 创建大量锁
	for i := 0; i < numLocks; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		lock, err := factory.CreateSimpleLock(key)
		if err != nil {
			t.Fatalf("创建锁失败: %v", err)
		}
		locks[i] = lock
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	memUsed := m2.Alloc - m1.Alloc
	avgMemPerLock := float64(memUsed) / float64(numLocks)

	t.Logf("内存使用统计:")
	t.Logf("- 创建锁数量: %d", numLocks)
	t.Logf("- 总内存使用: %d bytes", memUsed)
	t.Logf("- 平均每锁内存: %.2f bytes", avgMemPerLock)

	// 清理锁引用
	locks = nil
	runtime.GC()

	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	t.Logf("- 清理后内存: %d bytes", m3.Alloc)
}
