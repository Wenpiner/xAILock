package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchdogBasicFunctionality(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-watchdog-basic", backend)
	require.NoError(t, err)

	// 创建看门狗
	config := &LockConfig{
		LockTTL:          2 * time.Second,
		WatchdogInterval: 500 * time.Millisecond,
		EnableWatchdog:   true,
	}

	extendableLock := lock.(ExtendableLock)
	watchdog := NewWatchdog(extendableLock, config.WatchdogInterval, config.LockTTL)

	ctx := context.Background()

	// 获取锁
	err = lock.Lock(ctx, WithLockTTL(2*time.Second))
	require.NoError(t, err)

	// 启动看门狗
	err = watchdog.Start(ctx)
	require.NoError(t, err)
	assert.True(t, watchdog.IsRunning())

	// 等待一段时间，让看门狗执行几次续约
	time.Sleep(1500 * time.Millisecond)

	// 检查锁仍然有效
	locked, err := lock.IsLocked(ctx)
	require.NoError(t, err)
	assert.True(t, locked)

	// 检查看门狗统计信息
	stats := watchdog.GetStats()
	assert.True(t, stats.ExtendCount > 0, "看门狗应该执行了续约操作")
	assert.Equal(t, int64(0), stats.FailedExtends, "不应该有失败的续约")

	// 停止看门狗
	err = watchdog.Stop()
	require.NoError(t, err)
	assert.False(t, watchdog.IsRunning())

	// 释放锁
	err = lock.Unlock(ctx)
	require.NoError(t, err)
}

func TestWatchdogWithLockExpiration(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-watchdog-expiration", backend)
	require.NoError(t, err)

	ctx := context.Background()

	// 获取锁，设置较短的TTL
	err = lock.Lock(ctx, WithLockTTL(1*time.Second))
	require.NoError(t, err)

	// 不启动看门狗，等待锁过期
	time.Sleep(1500 * time.Millisecond)

	// 检查锁已过期
	locked, err := lock.IsLocked(ctx)
	require.NoError(t, err)
	assert.False(t, locked, "锁应该已经过期")
}

func TestWatchdogManager(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-watchdog-manager", backend)
	require.NoError(t, err)

	ctx := context.Background()
	manager := NewWatchdogManager()

	// 获取锁
	err = lock.Lock(ctx, WithLockTTL(2*time.Second))
	require.NoError(t, err)

	extendableLock := lock.(ExtendableLock)

	// 通过管理器启动看门狗
	err = manager.StartWatchdog(ctx, extendableLock, 500*time.Millisecond, 2*time.Second)
	require.NoError(t, err)

	// 检查看门狗是否存在
	watchdog, exists := manager.GetWatchdog(lock.GetKey())
	assert.True(t, exists)
	assert.True(t, watchdog.IsRunning())

	// 等待一段时间
	time.Sleep(1200 * time.Millisecond)

	// 检查统计信息
	stats := watchdog.GetStats()
	assert.True(t, stats.ExtendCount > 0)

	// 停止看门狗
	err = manager.StopWatchdog(lock.GetKey())
	require.NoError(t, err)

	// 检查看门狗已停止
	_, exists = manager.GetWatchdog(lock.GetKey())
	assert.False(t, exists)

	// 释放锁
	err = lock.Unlock(ctx)
	require.NoError(t, err)
}

func TestLockWithWatchdog(t *testing.T) {
	// 创建带看门狗的锁
	backend := NewLocalBackend(32)
	lockWithWatchdog, err := NewSimpleLockWithWatchdog("test-lock-with-watchdog", backend,
		WithLockTTL(2*time.Second),
		WithWatchdogInterval(500*time.Millisecond),
		WithWatchdog(true))
	require.NoError(t, err)

	ctx := context.Background()

	// 获取锁
	err = lockWithWatchdog.Lock(ctx)
	require.NoError(t, err)

	// 启动看门狗
	err = lockWithWatchdog.Start(ctx)
	require.NoError(t, err)
	assert.True(t, lockWithWatchdog.IsRunning())

	// 等待一段时间，让看门狗工作
	time.Sleep(1500 * time.Millisecond)

	// 检查锁仍然有效
	locked, err := lockWithWatchdog.IsLocked(ctx)
	require.NoError(t, err)
	assert.True(t, locked)

	// 检查统计信息
	stats := lockWithWatchdog.GetStats()
	assert.True(t, stats.ExtendCount > 0)
	assert.Equal(t, int64(0), stats.FailedExtends)

	// 停止看门狗
	err = lockWithWatchdog.Stop()
	require.NoError(t, err)

	// 释放锁
	err = lockWithWatchdog.Unlock(ctx)
	require.NoError(t, err)
}

func TestWatchdogErrorHandling(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-watchdog-error", backend)
	require.NoError(t, err)

	extendableLock := lock.(ExtendableLock)
	watchdog := NewWatchdog(extendableLock, 100*time.Millisecond, 1*time.Second)

	ctx := context.Background()

	// 测试重复启动
	err = watchdog.Start(ctx)
	require.NoError(t, err)

	err = watchdog.Start(ctx)
	assert.Error(t, err, "重复启动应该返回错误")

	// 停止看门狗
	err = watchdog.Stop()
	require.NoError(t, err)

	// 测试重复停止
	err = watchdog.Stop()
	assert.Error(t, err, "重复停止应该返回错误")
}

func TestWatchdogWithCancelledContext(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-watchdog-cancel", backend)
	require.NoError(t, err)

	// 获取锁
	ctx := context.Background()
	err = lock.Lock(ctx, WithLockTTL(5*time.Second))
	require.NoError(t, err)

	// 创建可取消的上下文
	cancelCtx, cancel := context.WithCancel(ctx)

	extendableLock := lock.(ExtendableLock)
	watchdog := NewWatchdog(extendableLock, 200*time.Millisecond, 2*time.Second)

	// 启动看门狗
	err = watchdog.Start(cancelCtx)
	require.NoError(t, err)

	// 等待一段时间
	time.Sleep(500 * time.Millisecond)

	// 取消上下文
	cancel()

	// 等待看门狗停止
	time.Sleep(300 * time.Millisecond)

	// 看门狗应该仍然标记为运行中，但实际上已经停止工作
	// 这是因为取消上下文不会自动调用Stop()方法
	assert.True(t, watchdog.IsRunning())

	// 手动停止看门狗
	err = watchdog.Stop()
	require.NoError(t, err)

	// 释放锁
	err = lock.Unlock(ctx)
	require.NoError(t, err)
}

func TestGlobalWatchdogManager(t *testing.T) {
	// 创建本地简单锁
	backend := NewLocalBackend(32)
	lock, err := NewSimpleLock("test-global-manager", backend)
	require.NoError(t, err)

	ctx := context.Background()

	// 获取锁
	err = lock.Lock(ctx, WithLockTTL(2*time.Second))
	require.NoError(t, err)

	extendableLock := lock.(ExtendableLock)

	// 使用全局管理器启动看门狗
	err = StartWatchdogForLock(ctx, extendableLock, 300*time.Millisecond, 2*time.Second)
	require.NoError(t, err)

	// 等待一段时间
	time.Sleep(800 * time.Millisecond)

	// 获取统计信息
	stats, exists := GetWatchdogStats(lock.GetKey())
	assert.True(t, exists)
	assert.True(t, stats.ExtendCount > 0)

	// 停止看门狗
	err = StopWatchdogForLock(lock.GetKey())
	require.NoError(t, err)

	// 释放锁
	err = lock.Unlock(ctx)
	require.NoError(t, err)
}

func TestWatchdogWithRedisLock(t *testing.T) {
	// 这个测试需要Redis服务器运行在localhost:6379
	// 会自动通过Docker启动Redis容器

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Redis服务器不可用: %v", err)
	}

	// 创建Redis后端
	backend := NewRedisBackend(client)
	lock, err := NewSimpleLock("test-redis-watchdog", backend)
	require.NoError(t, err)

	// 获取锁
	err = lock.Lock(ctx, WithLockTTL(2*time.Second))
	require.NoError(t, err)

	// 创建看门狗
	extendableLock := lock.(ExtendableLock)
	watchdog := NewWatchdog(extendableLock, 500*time.Millisecond, 2*time.Second)

	// 启动看门狗
	err = watchdog.Start(ctx)
	require.NoError(t, err)

	// 等待一段时间，让看门狗工作
	time.Sleep(3 * time.Second)

	// 检查锁仍然有效
	locked, err := lock.IsLocked(ctx)
	require.NoError(t, err)
	assert.True(t, locked)

	// 检查统计信息
	stats := watchdog.GetStats()
	assert.True(t, stats.ExtendCount > 0)
	t.Logf("Redis看门狗统计: 续约次数=%d, 失败次数=%d", stats.ExtendCount, stats.FailedExtends)

	// 停止看门狗
	err = watchdog.Stop()
	require.NoError(t, err)

	// 释放锁
	err = lock.Unlock(ctx)
	require.NoError(t, err)
}
