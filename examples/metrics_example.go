package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	lock "github.com/wenpiner/xailock"
)

func main() {
	fmt.Println("=== xAILock 监控指标示例 ===")

	// 创建本地后端
	backend := lock.NewLocalBackend(8)

	// 演示1: 基本监控功能
	fmt.Println("\n1. 基本监控功能演示")
	demonstrateBasicMetrics(backend)

	// 演示2: 并发监控
	fmt.Println("\n2. 并发监控演示")
	demonstrateConcurrentMetrics(backend)

	// 演示3: 不同锁类型的监控
	fmt.Println("\n3. 不同锁类型监控演示")
	demonstrateDifferentLockTypes(backend)

	// 演示4: 错误监控
	fmt.Println("\n4. 错误监控演示")
	demonstrateErrorMetrics(backend)

	// 演示5: 全局监控统计
	fmt.Println("\n5. 全局监控统计")
	demonstrateGlobalMetrics()
}

// demonstrateBasicMetrics 演示基本监控功能
func demonstrateBasicMetrics(backend lock.Backend) {
	// 创建带监控的简单锁
	simpleLock, err := lock.NewSimpleLockWithMetrics("demo:simple", backend)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// 执行一些锁操作
	fmt.Println("执行锁操作...")

	// TryLock操作
	success, err := simpleLock.TryLock(ctx, lock.WithLockTTL(2*time.Second))
	if err != nil {
		log.Printf("TryLock错误: %v", err)
	} else if success {
		fmt.Println("✓ TryLock成功")
		time.Sleep(100 * time.Millisecond) // 模拟业务逻辑

		// 延长锁
		extended, err := simpleLock.Extend(ctx, 3*time.Second)
		if err != nil {
			log.Printf("Extend错误: %v", err)
		} else if extended {
			fmt.Println("✓ 锁延长成功")
		}

		// 释放锁
		err = simpleLock.Unlock(ctx)
		if err != nil {
			log.Printf("Unlock错误: %v", err)
		} else {
			fmt.Println("✓ Unlock成功")
		}
	}

	// 获取监控统计
	stats := lock.GetGlobalMetrics().GetStats()
	fmt.Printf("锁操作统计:\n")
	fmt.Printf("  - TryAcquire次数: %d (成功: %d, 失败: %d)\n",
		stats.LockOperations.TryAcquireCount,
		stats.LockOperations.TryAcquireSuccess,
		stats.LockOperations.TryAcquireFailed)
	fmt.Printf("  - Release次数: %d (成功: %d, 失败: %d)\n",
		stats.LockOperations.ReleaseCount,
		stats.LockOperations.ReleaseSuccess,
		stats.LockOperations.ReleaseFailed)
	fmt.Printf("  - Extend次数: %d (成功: %d, 失败: %d)\n",
		stats.LockOperations.ExtendCount,
		stats.LockOperations.ExtendSuccess,
		stats.LockOperations.ExtendFailed)
}

// demonstrateConcurrentMetrics 演示并发监控
func demonstrateConcurrentMetrics(backend lock.Backend) {
	const goroutines = 10
	const operations = 50

	var wg sync.WaitGroup

	fmt.Printf("启动%d个协程，每个执行%d次锁操作...\n", goroutines, operations)

	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			lockKey := fmt.Sprintf("demo:concurrent:%d", id)
			concurrentLock, err := lock.NewSimpleLockWithMetrics(lockKey, backend)
			if err != nil {
				log.Printf("创建锁失败: %v", err)
				return
			}

			ctx := context.Background()

			for j := 0; j < operations; j++ {
				success, err := concurrentLock.TryLock(ctx, lock.WithLockTTL(100*time.Millisecond))
				if err == nil && success {
					time.Sleep(time.Millisecond) // 模拟业务逻辑
					concurrentLock.Unlock(ctx)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// 获取并发统计
	stats := lock.GetGlobalMetrics().GetStats()
	fmt.Printf("并发测试完成 (耗时: %v):\n", duration)
	fmt.Printf("  - 总操作次数: %d\n", stats.LockOperations.TryAcquireCount)
	fmt.Printf("  - 成功操作: %d\n", stats.LockOperations.TryAcquireSuccess)
	fmt.Printf("  - 失败操作: %d\n", stats.LockOperations.TryAcquireFailed)
	fmt.Printf("  - 平均操作时间: %v\n", stats.LockOperations.AverageTryAcquireTime)
	fmt.Printf("  - 当前协程数: %d\n", stats.ResourceStats.CurrentGoroutines)
}

// demonstrateDifferentLockTypes 演示不同锁类型的监控
func demonstrateDifferentLockTypes(backend lock.Backend) {
	ctx := context.Background()

	// 读写锁监控
	fmt.Println("测试读写锁监控...")
	rwLock, err := lock.NewRWLockWithMetrics("demo:rw", backend)
	if err != nil {
		log.Printf("创建读写锁失败: %v", err)
		return
	}

	// 读锁操作
	success, err := rwLock.TryRLock(ctx, lock.WithLockTTL(1*time.Second))
	if err == nil && success {
		fmt.Println("✓ 读锁获取成功")
		count, _ := rwLock.GetReadCount(ctx)
		fmt.Printf("  当前读锁数量: %d\n", count)
		rwLock.RUnlock(ctx)
	}

	// 写锁操作
	success, err = rwLock.TryWLock(ctx, lock.WithLockTTL(1*time.Second))
	if err == nil && success {
		fmt.Println("✓ 写锁获取成功")
		isWriteLocked, _ := rwLock.IsWriteLocked(ctx)
		fmt.Printf("  写锁状态: %v\n", isWriteLocked)
		rwLock.WUnlock(ctx)
	}

	// 公平锁监控
	fmt.Println("测试公平锁监控...")
	fairLock, err := lock.NewFairLockWithMetrics("demo:fair", backend)
	if err != nil {
		log.Printf("创建公平锁失败: %v", err)
		return
	}

	success, err = fairLock.TryLock(ctx, lock.WithLockTTL(1*time.Second))
	if err == nil && success {
		fmt.Println("✓ 公平锁获取成功")
		position, _ := fairLock.GetQueuePosition(ctx)
		length, _ := fairLock.GetQueueLength(ctx)
		fmt.Printf("  队列位置: %d, 队列长度: %d\n", position, length)
		fairLock.Unlock(ctx)
	}
}

// demonstrateErrorMetrics 演示错误监控
func demonstrateErrorMetrics(backend lock.Backend) {
	ctx := context.Background()

	// 创建一个锁并故意制造一些错误场景
	testLock, err := lock.NewSimpleLockWithMetrics("demo:error", backend)
	if err != nil {
		log.Printf("创建锁失败: %v", err)
		return
	}

	// 先获取锁
	success, err := testLock.TryLock(ctx, lock.WithLockTTL(1*time.Second))
	if err == nil && success {
		fmt.Println("✓ 锁获取成功")

		// 尝试重复获取（应该失败）
		success2, err2 := testLock.TryLock(ctx, lock.WithLockTTL(1*time.Second))
		if err2 != nil || !success2 {
			fmt.Println("✓ 重复获取锁失败（预期行为）")
		}

		// 释放锁
		testLock.Unlock(ctx)

		// 尝试重复释放（可能产生错误）
		err3 := testLock.Unlock(ctx)
		if err3 != nil {
			fmt.Printf("✓ 重复释放锁产生错误: %v\n", err3)
		}
	}

	// 获取错误统计
	stats := lock.GetGlobalMetrics().GetStats()
	fmt.Printf("错误统计:\n")
	fmt.Printf("  - 总错误数: %d\n", stats.ErrorStats.TotalErrors)
	if len(stats.ErrorStats.ErrorsByType) > 0 {
		fmt.Printf("  - 按类型分组:\n")
		for errType, count := range stats.ErrorStats.ErrorsByType {
			fmt.Printf("    %s: %d\n", errType, count)
		}
	}
	if len(stats.ErrorStats.ErrorsByCode) > 0 {
		fmt.Printf("  - 按错误码分组:\n")
		for errCode, count := range stats.ErrorStats.ErrorsByCode {
			fmt.Printf("    %s: %d\n", errCode, count)
		}
	}
}

// demonstrateGlobalMetrics 演示全局监控统计
func demonstrateGlobalMetrics() {
	stats := lock.GetGlobalMetrics().GetStats()

	fmt.Println("=== 全局监控统计报告 ===")

	// 锁操作统计
	fmt.Printf("\n📊 锁操作统计:\n")
	fmt.Printf("  获取锁: %d次 (成功: %d, 失败: %d, 平均耗时: %v)\n",
		stats.LockOperations.AcquireCount,
		stats.LockOperations.AcquireSuccess,
		stats.LockOperations.AcquireFailed,
		stats.LockOperations.AverageAcquireTime)
	fmt.Printf("  尝试获取: %d次 (成功: %d, 失败: %d, 平均耗时: %v)\n",
		stats.LockOperations.TryAcquireCount,
		stats.LockOperations.TryAcquireSuccess,
		stats.LockOperations.TryAcquireFailed,
		stats.LockOperations.AverageTryAcquireTime)
	fmt.Printf("  释放锁: %d次 (成功: %d, 失败: %d, 平均耗时: %v)\n",
		stats.LockOperations.ReleaseCount,
		stats.LockOperations.ReleaseSuccess,
		stats.LockOperations.ReleaseFailed,
		stats.LockOperations.AverageReleaseTime)
	fmt.Printf("  延长锁: %d次 (成功: %d, 失败: %d, 平均耗时: %v)\n",
		stats.LockOperations.ExtendCount,
		stats.LockOperations.ExtendSuccess,
		stats.LockOperations.ExtendFailed,
		stats.LockOperations.AverageExtendTime)

	// 并发统计
	fmt.Printf("\n🔄 并发统计:\n")
	fmt.Printf("  最大并发锁数: %d\n", stats.ConcurrencyStats.MaxConcurrentLocks)
	fmt.Printf("  当前锁数: %d\n", stats.ConcurrencyStats.CurrentLocks)
	fmt.Printf("  最大队列长度: %d\n", stats.ConcurrencyStats.MaxQueueLength)
	fmt.Printf("  平均等待时间: %v\n", stats.ConcurrencyStats.AverageWaitTime)

	// 资源统计
	fmt.Printf("\n💾 资源统计:\n")
	fmt.Printf("  当前内存使用: %d bytes\n", stats.ResourceStats.CurrentMemoryUsage)
	fmt.Printf("  最大内存使用: %d bytes\n", stats.ResourceStats.MaxMemoryUsage)
	fmt.Printf("  当前连接数: %d\n", stats.ResourceStats.CurrentConnections)
	fmt.Printf("  当前协程数: %d\n", stats.ResourceStats.CurrentGoroutines)

	// 看门狗统计
	if stats.WatchdogStats.TotalExtends > 0 {
		fmt.Printf("\n🐕 看门狗统计:\n")
		fmt.Printf("  活跃看门狗: %d\n", stats.WatchdogStats.ActiveWatchdogs)
		fmt.Printf("  总续约次数: %d (成功: %d, 失败: %d)\n",
			stats.WatchdogStats.TotalExtends,
			stats.WatchdogStats.SuccessfulExtends,
			stats.WatchdogStats.FailedExtends)
		fmt.Printf("  平均续约时间: %v\n", stats.WatchdogStats.AverageExtendTime)
	}

	// 分片统计
	if stats.ShardingStats.ShardCount > 0 {
		fmt.Printf("\n🔀 分片统计:\n")
		fmt.Printf("  分片数量: %d\n", stats.ShardingStats.ShardCount)
		fmt.Printf("  最大分片负载: %d\n", stats.ShardingStats.MaxShardLoad)
		fmt.Printf("  最小分片负载: %d\n", stats.ShardingStats.MinShardLoad)
		fmt.Printf("  负载标准差: %.2f\n", stats.ShardingStats.LoadStandardDev)
	}

	// 错误统计
	if stats.ErrorStats.TotalErrors > 0 {
		fmt.Printf("\n❌ 错误统计:\n")
		fmt.Printf("  总错误数: %d\n", stats.ErrorStats.TotalErrors)
		if len(stats.ErrorStats.ErrorsByType) > 0 {
			fmt.Printf("  按类型分组: %v\n", stats.ErrorStats.ErrorsByType)
		}
	}

	fmt.Println("\n=== 监控报告结束 ===")
}
