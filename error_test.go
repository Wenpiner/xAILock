package xailock

import (
	"errors"
	"testing"
	"time"
)

// TestLockError 测试基本的锁错误功能
func TestLockError(t *testing.T) {
	// 测试基本错误创建
	err := NewLockError(ErrCodeLockNotFound, "测试错误", SeverityError)
	
	if err.Code != ErrCodeLockNotFound {
		t.Errorf("期望错误代码 %s, 得到 %s", ErrCodeLockNotFound, err.Code)
	}
	
	if err.Message != "测试错误" {
		t.Errorf("期望错误消息 '测试错误', 得到 '%s'", err.Message)
	}
	
	if err.Severity != SeverityError {
		t.Errorf("期望错误严重程度 %v, 得到 %v", SeverityError, err.Severity)
	}
	
	// 测试错误字符串格式
	expected := "[LOCK_NOT_FOUND] 测试错误"
	if err.Error() != expected {
		t.Errorf("期望错误字符串 '%s', 得到 '%s'", expected, err.Error())
	}
}

// TestLockErrorWithCause 测试带原始错误的锁错误
func TestLockErrorWithCause(t *testing.T) {
	originalErr := errors.New("原始错误")
	err := NewLockErrorWithCause(ErrCodeNetworkError, "网络错误", SeverityError, originalErr)
	
	// 测试错误链
	if !errors.Is(err, originalErr) {
		t.Error("错误链不正确，应该包含原始错误")
	}
	
	// 测试Unwrap
	if errors.Unwrap(err) != originalErr {
		t.Error("Unwrap应该返回原始错误")
	}
	
	// 测试错误字符串包含原始错误
	expected := "[NETWORK_ERROR] 网络错误: 原始错误"
	if err.Error() != expected {
		t.Errorf("期望错误字符串 '%s', 得到 '%s'", expected, err.Error())
	}
}

// TestLockErrorContext 测试错误上下文
func TestLockErrorContext(t *testing.T) {
	err := NewLockError(ErrCodeLockNotAcquired, "锁获取失败", SeverityError).
		WithContext("key", "test-key").
		WithContext("timeout", "5s")
	
	if err.Context["key"] != "test-key" {
		t.Errorf("期望上下文 key='test-key', 得到 '%v'", err.Context["key"])
	}
	
	if err.Context["timeout"] != "5s" {
		t.Errorf("期望上下文 timeout='5s', 得到 '%v'", err.Context["timeout"])
	}
}

// TestErrorSeverity 测试错误严重程度
func TestErrorSeverity(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityError, "ERROR"},
		{SeverityCritical, "CRITICAL"},
	}
	
	for _, test := range tests {
		if test.severity.String() != test.expected {
			t.Errorf("期望严重程度字符串 '%s', 得到 '%s'", test.expected, test.severity.String())
		}
	}
}

// TestErrorClassification 测试错误分类
func TestErrorClassification(t *testing.T) {
	// 测试临时错误
	tempErr := NewLockError(ErrCodeAcquireTimeout, "超时", SeverityWarning)
	if !tempErr.IsTemporary() {
		t.Error("获取锁超时应该是临时错误")
	}
	
	// 测试可重试错误
	retryErr := NewLockError(ErrCodeLockNotAcquired, "锁获取失败", SeverityError)
	if !retryErr.IsRetryable() {
		t.Error("锁获取失败应该是可重试错误")
	}
	
	// 测试非临时错误
	permErr := NewLockError(ErrCodeInvalidConfig, "配置错误", SeverityError)
	if permErr.IsTemporary() {
		t.Error("配置错误不应该是临时错误")
	}
}

// TestErrorUtilityFunctions 测试错误工具函数
func TestErrorUtilityFunctions(t *testing.T) {
	lockErr := NewLockError(ErrCodeLockNotFound, "锁不存在", SeverityError)
	
	// 测试IsLockError
	if !IsLockError(lockErr) {
		t.Error("IsLockError应该返回true")
	}
	
	regularErr := errors.New("普通错误")
	if IsLockError(regularErr) {
		t.Error("IsLockError对普通错误应该返回false")
	}
	
	// 测试GetLockError
	if GetLockError(lockErr) != lockErr {
		t.Error("GetLockError应该返回原始锁错误")
	}
	
	if GetLockError(regularErr) != nil {
		t.Error("GetLockError对普通错误应该返回nil")
	}
	
	// 测试IsErrorCode
	if !IsErrorCode(lockErr, ErrCodeLockNotFound) {
		t.Error("IsErrorCode应该正确识别错误代码")
	}
	
	if IsErrorCode(lockErr, ErrCodeLockExpired) {
		t.Error("IsErrorCode不应该匹配错误的代码")
	}
}

// TestErrorComparison 测试错误比较
func TestErrorComparison(t *testing.T) {
	err1 := NewLockError(ErrCodeLockNotFound, "锁不存在", SeverityError)
	err2 := NewLockError(ErrCodeLockNotFound, "另一个锁不存在", SeverityWarning)
	err3 := NewLockError(ErrCodeLockExpired, "锁过期", SeverityInfo)
	
	// 测试相同错误代码的比较
	if !errors.Is(err1, err2) {
		t.Error("相同错误代码的错误应该被认为是相等的")
	}
	
	// 测试不同错误代码的比较
	if errors.Is(err1, err3) {
		t.Error("不同错误代码的错误不应该被认为是相等的")
	}
}

// TestPreDefinedErrors 测试预定义错误的向后兼容性
func TestPreDefinedErrors(t *testing.T) {
	// 测试预定义错误仍然可以正常使用
	if ErrLockNotFound.Error() == "" {
		t.Error("预定义错误应该有错误消息")
	}
	
	// 测试预定义错误是LockError类型
	if !IsLockError(ErrLockNotFound) {
		t.Error("预定义错误应该是LockError类型")
	}
	
	// 测试错误代码正确
	if !IsErrorCode(ErrLockNotFound, ErrCodeLockNotFound) {
		t.Error("预定义错误的错误代码应该正确")
	}
}

// TestContextualErrorCreation 测试上下文错误创建函数
func TestContextualErrorCreation(t *testing.T) {
	// 测试NewLockNotFoundError
	err := NewLockNotFoundError("test-key")
	if !IsErrorCode(err, ErrCodeLockNotFound) {
		t.Error("NewLockNotFoundError应该创建正确的错误代码")
	}
	if err.Context["key"] != "test-key" {
		t.Error("NewLockNotFoundError应该设置正确的key上下文")
	}
	
	// 测试NewAcquireTimeoutError
	timeout := 5 * time.Second
	timeoutErr := NewAcquireTimeoutError("test-key", timeout)
	if !IsErrorCode(timeoutErr, ErrCodeAcquireTimeout) {
		t.Error("NewAcquireTimeoutError应该创建正确的错误代码")
	}
	if timeoutErr.Context["key"] != "test-key" {
		t.Error("NewAcquireTimeoutError应该设置正确的key上下文")
	}
	if timeoutErr.Context["timeout"] != timeout.String() {
		t.Error("NewAcquireTimeoutError应该设置正确的timeout上下文")
	}
	
	// 测试NewInvalidConfigError
	configErr := NewInvalidConfigError("LockTTL", -1, "必须大于0")
	if !IsErrorCode(configErr, ErrCodeInvalidConfig) {
		t.Error("NewInvalidConfigError应该创建正确的错误代码")
	}
	if configErr.Context["field"] != "LockTTL" {
		t.Error("NewInvalidConfigError应该设置正确的field上下文")
	}
}

// TestWrapError 测试错误包装函数
func TestWrapError(t *testing.T) {
	originalErr := errors.New("网络连接失败")
	
	// 测试WrapNetworkError
	networkErr := WrapNetworkError(originalErr, "lock_acquire")
	if !IsErrorCode(networkErr, ErrCodeNetworkError) {
		t.Error("WrapNetworkError应该创建网络错误代码")
	}
	if !errors.Is(networkErr, originalErr) {
		t.Error("WrapNetworkError应该保持错误链")
	}
	if networkErr.Context["operation"] != "lock_acquire" {
		t.Error("WrapNetworkError应该设置正确的operation上下文")
	}
	
	// 测试WrapConnectionError
	connErr := WrapConnectionError(originalErr, "Redis")
	if !IsErrorCode(connErr, ErrCodeConnectionFailed) {
		t.Error("WrapConnectionError应该创建连接失败错误代码")
	}
	if connErr.Context["backend"] != "Redis" {
		t.Error("WrapConnectionError应该设置正确的backend上下文")
	}
}
