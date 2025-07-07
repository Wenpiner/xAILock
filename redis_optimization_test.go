package lock

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisOptimizationPerformance 测试Redis脚本优化的性能提升
func TestRedisOptimizationPerformance(t *testing.T) {
	// 跳过如果没有Redis连接
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // 使用测试数据库
	})
	
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping performance test")
	}
	defer client.Close()
	
	// 清理测试数据
	defer func() {
		client.FlushDB(ctx)
	}()
	
	t.Run("SimpleLock Performance", func(t *testing.T) {
		testSimpleLockPerformance(t, client)
	})
	
	t.Run("RWLock Performance", func(t *testing.T) {
		testRWLockPerformance(t, client)
	})
	
	t.Run("FairLock Performance", func(t *testing.T) {
		testFairLockPerformance(t, client)
	})
}

func testSimpleLockPerformance(t *testing.T, client redis.UniversalClient) {
	const numOperations = 1000
	const numGoroutines = 10
	
	// 测试优化后的性能
	start := time.Now()
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations/numGoroutines; j++ {
				key := fmt.Sprintf("perf_test_simple_%d_%d", id, j)
				lock := newRedisSimpleLock(key, client)
				
				// 获取锁
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				success, err := lock.TryLock(ctx, WithValue(fmt.Sprintf("holder_%d_%d", id, j)))
				if err != nil {
					t.Errorf("TryLock failed: %v", err)
					cancel()
					continue
				}
				
				if success {
					// 检查状态（优化后的方法）
					_, err = lock.IsLocked(ctx)
					if err != nil {
						t.Errorf("IsLocked failed: %v", err)
					}
					
					// 获取TTL（优化后的方法）
					_, err = lock.GetTTL(ctx)
					if err != nil {
						t.Errorf("GetTTL failed: %v", err)
					}
					
					// 释放锁
					err = lock.Unlock(ctx)
					if err != nil {
						t.Errorf("Unlock failed: %v", err)
					}
				}
				cancel()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	opsPerSecond := float64(numOperations) / duration.Seconds()
	t.Logf("SimpleLock Performance: %d operations in %v (%.2f ops/sec)", 
		numOperations, duration, opsPerSecond)
	
	// 性能基准：应该能达到至少1000 ops/sec
	if opsPerSecond < 1000 {
		t.Logf("Warning: Performance may be suboptimal (%.2f ops/sec < 1000 ops/sec)", opsPerSecond)
	}
}

func testRWLockPerformance(t *testing.T, client redis.UniversalClient) {
	const numOperations = 500
	const numGoroutines = 10
	
	start := time.Now()
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations/numGoroutines; j++ {
				key := fmt.Sprintf("perf_test_rw_%d_%d", id, j)
				rwLock := newRedisRWLock(key, client)
				
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				
				// 测试读锁
				success, err := rwLock.TryRLock(ctx, WithLockTTL(30*time.Second))
				if err != nil {
					t.Errorf("TryRLock failed: %v", err)
					cancel()
					continue
				}
				
				if success {
					// 检查读锁数量（优化后的方法）
					_, err = rwLock.GetReadCount(ctx)
					if err != nil {
						t.Errorf("GetReadCount failed: %v", err)
					}
					
					// 检查写锁状态（优化后的方法）
					_, err = rwLock.IsWriteLocked(ctx)
					if err != nil {
						t.Errorf("IsWriteLocked failed: %v", err)
					}
					
					// 释放读锁
					err = rwLock.RUnlock(ctx)
					if err != nil {
						t.Errorf("RUnlock failed: %v", err)
					}
				}
				cancel()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	opsPerSecond := float64(numOperations) / duration.Seconds()
	t.Logf("RWLock Performance: %d operations in %v (%.2f ops/sec)", 
		numOperations, duration, opsPerSecond)
}

func testFairLockPerformance(t *testing.T, client redis.UniversalClient) {
	const numOperations = 200
	const numGoroutines = 5
	
	start := time.Now()
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations/numGoroutines; j++ {
				key := fmt.Sprintf("perf_test_fair_%d_%d", id, j)
				fairLock := newRedisFairLock(key, client)
				
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				
				// 尝试获取公平锁（优化后的方法）
				success, err := fairLock.TryLock(ctx, 
					WithValue(fmt.Sprintf("holder_%d_%d", id, j)),
					WithLockTTL(30*time.Second))
				if err != nil {
					t.Errorf("TryLock failed: %v", err)
					cancel()
					continue
				}
				
				if success {
					// 检查锁状态（优化后的方法）
					_, err = fairLock.IsLocked(ctx)
					if err != nil {
						t.Errorf("IsLocked failed: %v", err)
					}
					
					// 获取队列长度（优化后的方法）
					_, err = fairLock.GetQueueLength(ctx)
					if err != nil {
						t.Errorf("GetQueueLength failed: %v", err)
					}
					
					// 释放锁
					err = fairLock.Unlock(ctx)
					if err != nil {
						t.Errorf("Unlock failed: %v", err)
					}
				}
				cancel()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	opsPerSecond := float64(numOperations) / duration.Seconds()
	t.Logf("FairLock Performance: %d operations in %v (%.2f ops/sec)", 
		numOperations, duration, opsPerSecond)
}

// BenchmarkRedisScriptOptimization 基准测试Redis脚本优化
func BenchmarkRedisScriptOptimization(b *testing.B) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		b.Skip("Redis not available, skipping benchmark")
	}
	defer client.Close()
	defer client.FlushDB(ctx)
	
	b.Run("SimpleLock", func(b *testing.B) {
		lock := newRedisSimpleLock("bench_simple", client)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			success, err := lock.TryLock(ctx, WithValue(fmt.Sprintf("holder_%d", i)))
			if err != nil {
				b.Fatal(err)
			}
			if success {
				_ = lock.Unlock(ctx)
			}
		}
	})
	
	b.Run("FairLock", func(b *testing.B) {
		fairLock := newRedisFairLock("bench_fair", client)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			success, err := fairLock.TryLock(ctx, WithValue(fmt.Sprintf("holder_%d", i)))
			if err != nil {
				b.Fatal(err)
			}
			if success {
				_ = fairLock.Unlock(ctx)
			}
		}
	})
}
