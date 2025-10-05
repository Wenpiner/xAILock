package xailock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisLockFactory 测试Redis锁工厂
func TestRedisLockFactory(t *testing.T) {
	// 创建模拟Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// 创建Redis锁工厂
	factory := NewRedisLockFactory(client)

	// 测试工厂类型
	if factory.GetBackendType() != "Redis" {
		t.Errorf("Expected backend type 'Redis', got '%s'", factory.GetBackendType())
	}

	// 测试创建简单锁
	simpleLock, err := factory.CreateSimpleLock("test:simple")
	if err != nil {
		t.Fatalf("Failed to create simple lock: %v", err)
	}
	if simpleLock == nil {
		t.Fatal("Simple lock is nil")
	}
	if simpleLock.GetKey() != "test:simple" {
		t.Errorf("Expected key 'test:simple', got '%s'", simpleLock.GetKey())
	}

	// 测试创建读写锁
	rwLock, err := factory.CreateRWLock("test:rw")
	if err != nil {
		t.Fatalf("Failed to create RW lock: %v", err)
	}
	if rwLock == nil {
		t.Fatal("RW lock is nil")
	}
	if rwLock.GetKey() != "test:rw" {
		t.Errorf("Expected key 'test:rw', got '%s'", rwLock.GetKey())
	}

	// 测试创建公平锁
	fairLock, err := factory.CreateFairLock("test:fair")
	if err != nil {
		t.Fatalf("Failed to create fair lock: %v", err)
	}
	if fairLock == nil {
		t.Fatal("Fair lock is nil")
	}
	if fairLock.GetKey() != "test:fair" {
		t.Errorf("Expected key 'test:fair', got '%s'", fairLock.GetKey())
	}
}

// TestLocalLockFactory 测试本地锁工厂
func TestLocalLockFactory(t *testing.T) {
	// 创建本地锁工厂
	factory := NewLocalLockFactory(16)

	// 测试工厂类型
	if factory.GetBackendType() != "Local" {
		t.Errorf("Expected backend type 'Local', got '%s'", factory.GetBackendType())
	}

	// 测试创建简单锁
	simpleLock, err := factory.CreateSimpleLock("test:simple")
	if err != nil {
		t.Fatalf("Failed to create simple lock: %v", err)
	}
	if simpleLock == nil {
		t.Fatal("Simple lock is nil")
	}
	if simpleLock.GetKey() != "test:simple" {
		t.Errorf("Expected key 'test:simple', got '%s'", simpleLock.GetKey())
	}

	// 测试创建读写锁
	rwLock, err := factory.CreateRWLock("test:rw")
	if err != nil {
		t.Fatalf("Failed to create RW lock: %v", err)
	}
	if rwLock == nil {
		t.Fatal("RW lock is nil")
	}
	if rwLock.GetKey() != "test:rw" {
		t.Errorf("Expected key 'test:rw', got '%s'", rwLock.GetKey())
	}

	// 测试创建公平锁
	fairLock, err := factory.CreateFairLock("test:fair")
	if err != nil {
		t.Fatalf("Failed to create fair lock: %v", err)
	}
	if fairLock == nil {
		t.Fatal("Fair lock is nil")
	}
	if fairLock.GetKey() != "test:fair" {
		t.Errorf("Expected key 'test:fair', got '%s'", fairLock.GetKey())
	}
}

// TestFactoryTypeSafety 测试工厂模式的类型安全性
func TestFactoryTypeSafety(t *testing.T) {
	// 测试本地锁工厂
	localFactory := NewLocalLockFactory(8)
	
	// 创建锁并测试基本功能
	lock, err := localFactory.CreateSimpleLock("test:safety")
	if err != nil {
		t.Fatalf("Failed to create lock: %v", err)
	}

	ctx := context.Background()
	
	// 测试锁操作
	err = lock.Lock(ctx, WithLockTTL(time.Second))
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// 检查锁状态
	locked, err := lock.IsLocked(ctx)
	if err != nil {
		t.Fatalf("Failed to check lock status: %v", err)
	}
	if !locked {
		t.Error("Lock should be acquired")
	}

	// 释放锁
	err = lock.Unlock(ctx)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}
}

// TestBackwardCompatibility 测试向后兼容性
func TestBackwardCompatibility(t *testing.T) {
	// 测试旧的工厂函数仍然工作
	backend := NewLocalBackend(8)
	
	// 测试旧的NewSimpleLock函数
	lock, err := NewSimpleLock("test:compat", backend)
	if err != nil {
		t.Fatalf("Failed to create lock with old API: %v", err)
	}
	if lock == nil {
		t.Fatal("Lock is nil")
	}
	if lock.GetKey() != "test:compat" {
		t.Errorf("Expected key 'test:compat', got '%s'", lock.GetKey())
	}

	// 测试旧的NewRWLock函数
	rwLock, err := NewRWLock("test:rw:compat", backend)
	if err != nil {
		t.Fatalf("Failed to create RW lock with old API: %v", err)
	}
	if rwLock == nil {
		t.Fatal("RW lock is nil")
	}

	// 测试旧的NewFairLock函数
	fairLock, err := NewFairLock("test:fair:compat", backend)
	if err != nil {
		t.Fatalf("Failed to create fair lock with old API: %v", err)
	}
	if fairLock == nil {
		t.Fatal("Fair lock is nil")
	}
}

// BenchmarkFactoryCreation 基准测试工厂创建性能
func BenchmarkFactoryCreation(b *testing.B) {
	factory := NewLocalLockFactory(16)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock, err := factory.CreateSimpleLock("bench:test")
		if err != nil {
			b.Fatalf("Failed to create lock: %v", err)
		}
		_ = lock
	}
}
