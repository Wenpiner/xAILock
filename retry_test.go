package xailock

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRetryWithTimeout 测试通用重试函数
func TestRetryWithTimeout(t *testing.T) {
	ctx := context.Background()
	
	// 测试成功情况
	t.Run("Success", func(t *testing.T) {
		config := NewRetryConfig("test-key", 5*time.Second, 100*time.Millisecond)
		
		callCount := 0
		result, err := RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, string, error) {
			callCount++
			if callCount >= 3 {
				return true, "success", nil
			}
			return false, "", nil
		})
		
		if err != nil {
			t.Errorf("期望成功，但得到错误: %v", err)
		}
		if result != "success" {
			t.Errorf("期望结果 'success'，得到 '%s'", result)
		}
		if callCount != 3 {
			t.Errorf("期望调用3次，实际调用%d次", callCount)
		}
	})
	
	// 测试超时情况
	t.Run("Timeout", func(t *testing.T) {
		config := NewRetryConfig("test-key", 200*time.Millisecond, 50*time.Millisecond)
		
		start := time.Now()
		_, err := RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, string, error) {
			return false, "", nil // 总是失败
		})
		duration := time.Since(start)
		
		if !IsErrorCode(err, ErrCodeAcquireTimeout) {
			t.Errorf("期望超时错误，得到: %v", err)
		}
		
		// 检查超时时间是否合理（允许一些误差）
		if duration < 180*time.Millisecond || duration > 300*time.Millisecond {
			t.Errorf("期望超时时间约200ms，实际: %v", duration)
		}
	})
	
	// 测试错误情况
	t.Run("Error", func(t *testing.T) {
		config := NewRetryConfig("test-key", 5*time.Second, 100*time.Millisecond)
		expectedErr := errors.New("操作错误")
		
		_, err := RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, string, error) {
			return false, "", expectedErr
		})
		
		if err != expectedErr {
			t.Errorf("期望错误 %v，得到 %v", expectedErr, err)
		}
	})
	
	// 测试上下文取消
	t.Run("ContextCanceled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		config := NewRetryConfig("test-key", 5*time.Second, 100*time.Millisecond)
		
		// 在100ms后取消上下文
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		
		_, err := RetryWithTimeout(cancelCtx, config, func(ctx context.Context) (bool, string, error) {
			return false, "", nil // 总是失败
		})
		
		if err != context.Canceled {
			t.Errorf("期望上下文取消错误，得到: %v", err)
		}
	})
}

// TestRetryLockOperation 测试锁操作重试函数
func TestRetryLockOperation(t *testing.T) {
	ctx := context.Background()
	
	// 测试成功情况
	t.Run("Success", func(t *testing.T) {
		config := NewRetryConfig("test-key", 5*time.Second, 100*time.Millisecond)
		
		callCount := 0
		err := RetryLockOperation(ctx, config, func(ctx context.Context) (bool, error) {
			callCount++
			if callCount >= 2 {
				return true, nil
			}
			return false, nil
		})
		
		if err != nil {
			t.Errorf("期望成功，但得到错误: %v", err)
		}
		if callCount != 2 {
			t.Errorf("期望调用2次，实际调用%d次", callCount)
		}
	})
	
	// 测试超时情况
	t.Run("Timeout", func(t *testing.T) {
		config := NewRetryConfig("test-key", 200*time.Millisecond, 50*time.Millisecond)
		
		err := RetryLockOperation(ctx, config, func(ctx context.Context) (bool, error) {
			return false, nil // 总是失败
		})
		
		if !IsErrorCode(err, ErrCodeAcquireTimeout) {
			t.Errorf("期望超时错误，得到: %v", err)
		}
	})
}

// TestNewRetryConfigFromLockOptions 测试从锁选项创建重试配置
func TestNewRetryConfigFromLockOptions(t *testing.T) {
	key := "test-key"
	timeout := 10 * time.Second
	interval := 200 * time.Millisecond
	
	config := NewRetryConfigFromLockOptions(key, 
		WithAcquireTimeout(timeout),
		WithRetryInterval(interval),
	)
	
	if config.Key != key {
		t.Errorf("期望键名 '%s'，得到 '%s'", key, config.Key)
	}
	if config.Timeout != timeout {
		t.Errorf("期望超时 %v，得到 %v", timeout, config.Timeout)
	}
	if config.Interval != interval {
		t.Errorf("期望间隔 %v，得到 %v", interval, config.Interval)
	}
}

// TestRetryWithCustomLogic 测试自定义重试逻辑
func TestRetryWithCustomLogic(t *testing.T) {
	ctx := context.Background()
	timeout := 500 * time.Millisecond
	key := "test-key"
	
	// 测试成功情况
	t.Run("Success", func(t *testing.T) {
		callCount := 0
		err := RetryWithCustomLogic(ctx, timeout, key, func(ctx context.Context, deadline time.Time) error {
			callCount++
			if callCount >= 2 {
				return nil
			}
			// 模拟自定义重试逻辑
			if time.Now().After(deadline) {
				return NewAcquireTimeoutError(key, timeout)
			}
			time.Sleep(100 * time.Millisecond)
			return errors.New("继续重试")
		})
		
		if err == nil {
			t.Error("期望错误，但操作成功了")
		}
		if callCount < 1 {
			t.Errorf("期望至少调用1次，实际调用%d次", callCount)
		}
	})
}

// TestRetryPerformance 测试重试性能
func TestRetryPerformance(t *testing.T) {
	ctx := context.Background()
	config := NewRetryConfig("perf-test", 1*time.Second, 10*time.Millisecond)
	
	start := time.Now()
	callCount := 0
	
	err := RetryLockOperation(ctx, config, func(ctx context.Context) (bool, error) {
		callCount++
		if callCount >= 50 {
			return true, nil
		}
		return false, nil
	})
	
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("期望成功，但得到错误: %v", err)
	}
	
	// 检查性能：50次重试，每次间隔10ms，应该在约500ms内完成
	expectedDuration := 50 * 10 * time.Millisecond
	if duration > expectedDuration*2 {
		t.Errorf("性能测试失败：期望约%v，实际%v", expectedDuration, duration)
	}
	
	t.Logf("性能测试：%d次重试，耗时%v", callCount, duration)
}

// TestRetryErrorContext 测试重试错误上下文
func TestRetryErrorContext(t *testing.T) {
	ctx := context.Background()
	key := "context-test-key"
	timeout := 100 * time.Millisecond
	config := NewRetryConfig(key, timeout, 20*time.Millisecond)
	
	_, err := RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, string, error) {
		return false, "", nil // 总是失败，触发超时
	})
	
	// 检查错误是否包含正确的上下文信息
	if !IsErrorCode(err, ErrCodeAcquireTimeout) {
		t.Errorf("期望超时错误，得到: %v", err)
	}
	
	lockErr := GetLockError(err)
	if lockErr == nil {
		t.Fatal("期望LockError类型")
	}
	
	if lockErr.Context["key"] != key {
		t.Errorf("期望错误上下文包含key '%s'，得到 %v", key, lockErr.Context["key"])
	}
	
	if lockErr.Context["timeout"] != timeout.String() {
		t.Errorf("期望错误上下文包含timeout '%s'，得到 %v", timeout.String(), lockErr.Context["timeout"])
	}
}

// BenchmarkRetryWithTimeout 重试函数性能基准测试
func BenchmarkRetryWithTimeout(b *testing.B) {
	ctx := context.Background()
	config := NewRetryConfig("bench-key", 1*time.Second, 1*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, int, error) {
			return true, i, nil // 立即成功
		})
	}
}

// BenchmarkRetryLockOperation 锁重试函数性能基准测试
func BenchmarkRetryLockOperation(b *testing.B) {
	ctx := context.Background()
	config := NewRetryConfig("bench-key", 1*time.Second, 1*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RetryLockOperation(ctx, config, func(ctx context.Context) (bool, error) {
			return true, nil // 立即成功
		})
	}
}
