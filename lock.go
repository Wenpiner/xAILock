package xailock

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// 结构化错误类型定义
// =============================================================================

// ErrorCode 错误代码类型
type ErrorCode string

// 错误代码常量定义
const (
	// 锁操作相关错误
	ErrCodeLockNotAcquired     ErrorCode = "LOCK_NOT_ACQUIRED"
	ErrCodeLockNotFound        ErrorCode = "LOCK_NOT_FOUND"
	ErrCodeLockAlreadyReleased ErrorCode = "LOCK_ALREADY_RELEASED"
	ErrCodeLockExpired         ErrorCode = "LOCK_EXPIRED"
	ErrCodeAcquireTimeout      ErrorCode = "ACQUIRE_TIMEOUT"

	// 配置相关错误
	ErrCodeInvalidConfig      ErrorCode = "INVALID_CONFIG"
	ErrCodeUnsupportedBackend ErrorCode = "UNSUPPORTED_BACKEND"

	// 看门狗相关错误
	ErrCodeWatchdogStopped ErrorCode = "WATCHDOG_STOPPED"
	ErrCodeWatchdogFailed  ErrorCode = "WATCHDOG_FAILED"

	// 操作相关错误
	ErrCodeInvalidOperation ErrorCode = "INVALID_OPERATION"

	// 网络和连接相关错误
	ErrCodeNetworkError     ErrorCode = "NETWORK_ERROR"
	ErrCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"

	// 内部错误
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// ErrorSeverity 错误严重程度
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (s ErrorSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// LockError 结构化锁错误类型
type LockError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Severity  ErrorSeverity          `json:"severity"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Cause     error                  `json:"-"` // 原始错误，不序列化
	Timestamp time.Time              `json:"timestamp"`
}

// Error 实现 error 接口
func (e *LockError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *LockError) Unwrap() error {
	return e.Cause
}

// Is 支持错误比较
func (e *LockError) Is(target error) bool {
	if t, ok := target.(*LockError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext 添加上下文信息
func (e *LockError) WithContext(key string, value interface{}) *LockError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithCause 设置原始错误
func (e *LockError) WithCause(cause error) *LockError {
	e.Cause = cause
	return e
}

// IsTemporary 判断是否为临时错误
func (e *LockError) IsTemporary() bool {
	switch e.Code {
	case ErrCodeAcquireTimeout, ErrCodeNetworkError, ErrCodeConnectionFailed:
		return true
	default:
		return false
	}
}

// IsRetryable 判断是否可重试
func (e *LockError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeLockNotAcquired, ErrCodeAcquireTimeout, ErrCodeNetworkError:
		return true
	default:
		return false
	}
}

// =============================================================================
// 错误创建工具函数
// =============================================================================

// NewLockError 创建新的锁错误
func NewLockError(code ErrorCode, message string, severity ErrorSeverity) *LockError {
	return &LockError{
		Code:      code,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}
}

// NewLockErrorWithCause 创建带原始错误的锁错误
func NewLockErrorWithCause(code ErrorCode, message string, severity ErrorSeverity, cause error) *LockError {
	return &LockError{
		Code:      code,
		Message:   message,
		Severity:  severity,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// 预定义错误实例（向后兼容）
// =============================================================================

var (
	ErrLockNotAcquired     = NewLockError(ErrCodeLockNotAcquired, "锁获取失败", SeverityError)
	ErrLockNotFound        = NewLockError(ErrCodeLockNotFound, "锁不存在", SeverityError)
	ErrLockAlreadyReleased = NewLockError(ErrCodeLockAlreadyReleased, "锁已释放", SeverityWarning)
	ErrUnsupportedBackend  = NewLockError(ErrCodeUnsupportedBackend, "不支持的后端类型", SeverityError)
	ErrInvalidConfig       = NewLockError(ErrCodeInvalidConfig, "无效的配置", SeverityError)
	ErrWatchdogStopped     = NewLockError(ErrCodeWatchdogStopped, "看门狗已停止", SeverityWarning)
	ErrLockExpired         = NewLockError(ErrCodeLockExpired, "锁已过期", SeverityInfo)
	ErrAcquireTimeout      = NewLockError(ErrCodeAcquireTimeout, "获取锁超时", SeverityWarning)
)

// =============================================================================
// 错误检查和处理工具函数
// =============================================================================

// IsLockError 检查是否为锁错误
func IsLockError(err error) bool {
	var lockErr *LockError
	return errors.As(err, &lockErr)
}

// GetLockError 获取锁错误，如果不是锁错误则返回nil
func GetLockError(err error) *LockError {
	var lockErr *LockError
	if errors.As(err, &lockErr) {
		return lockErr
	}
	return nil
}

// IsErrorCode 检查错误是否为指定的错误代码
func IsErrorCode(err error, code ErrorCode) bool {
	if lockErr := GetLockError(err); lockErr != nil {
		return lockErr.Code == code
	}
	return false
}

// IsTemporaryError 检查是否为临时错误
func IsTemporaryError(err error) bool {
	if lockErr := GetLockError(err); lockErr != nil {
		return lockErr.IsTemporary()
	}
	return false
}

// IsRetryableError 检查是否为可重试错误
func IsRetryableError(err error) bool {
	if lockErr := GetLockError(err); lockErr != nil {
		return lockErr.IsRetryable()
	}
	return false
}

// WrapError 包装底层错误为锁错误
func WrapError(err error, code ErrorCode, message string, severity ErrorSeverity) *LockError {
	return NewLockErrorWithCause(code, message, severity, err)
}

// WrapNetworkError 包装网络错误
func WrapNetworkError(err error, operation string) *LockError {
	return NewLockErrorWithCause(ErrCodeNetworkError,
		fmt.Sprintf("网络错误: %s", operation), SeverityError, err).
		WithContext("operation", operation)
}

// WrapConnectionError 包装连接错误
func WrapConnectionError(err error, backend string) *LockError {
	return NewLockErrorWithCause(ErrCodeConnectionFailed,
		fmt.Sprintf("连接失败: %s", backend), SeverityError, err).
		WithContext("backend", backend)
}

// NewLockNotAcquiredError 创建锁获取失败错误（带上下文）
func NewLockNotAcquiredError(key string, reason string) *LockError {
	return NewLockError(ErrCodeLockNotAcquired, "锁获取失败", SeverityError).
		WithContext("key", key).
		WithContext("reason", reason)
}

// NewLockNotFoundError 创建锁不存在错误（带上下文）
func NewLockNotFoundError(key string) *LockError {
	return NewLockError(ErrCodeLockNotFound, "锁不存在", SeverityError).
		WithContext("key", key)
}

// NewAcquireTimeoutError 创建获取锁超时错误（带上下文）
func NewAcquireTimeoutError(key string, timeout time.Duration) *LockError {
	return NewLockError(ErrCodeAcquireTimeout, "获取锁超时", SeverityWarning).
		WithContext("key", key).
		WithContext("timeout", timeout.String())
}

// NewInvalidConfigError 创建无效配置错误（带上下文）
func NewInvalidConfigError(field string, value interface{}, reason string) *LockError {
	return NewLockError(ErrCodeInvalidConfig, "无效的配置", SeverityError).
		WithContext("field", field).
		WithContext("value", value).
		WithContext("reason", reason)
}

// LockOption 统一的锁选项接口
// 所有锁类型都使用这个统一的选项接口
type LockOption interface {
	// Apply 将选项应用到配置中
	Apply(config *LockConfig)
}

// LockConfig 统一的锁配置结构
// 包含所有锁类型可能需要的配置项
type LockConfig struct {
	// 基础配置 - 所有锁类型都支持
	AcquireTimeout   time.Duration // 获取锁的超时时间
	LockTTL          time.Duration // 锁的生存时间
	EnableWatchdog   bool          // 是否启用看门狗
	WatchdogInterval time.Duration // 看门狗续约间隔
	RetryInterval    time.Duration // 重试间隔
	Value            string        // 锁的值（持有者标识）

	// 特定锁类型配置
	QueueTTL time.Duration // 公平锁队列TTL（仅公平锁使用）
}

// 选项函数类型定义
type optionFunc func(*LockConfig)

// Apply 实现 LockOption 接口
func (f optionFunc) Apply(config *LockConfig) {
	f(config)
}

// 向后兼容性支持
// 为了保持向后兼容性，我们保留原有的类型别名
// 在后续版本中可以标记为 deprecated 并最终移除

// Option 向后兼容的类型别名
// Deprecated: 使用 LockOption 替代
type Option = LockOption

// RWOption 向后兼容的类型别名
// Deprecated: 使用 LockOption 替代
type RWOption = LockOption

// FairOption 向后兼容的类型别名
// Deprecated: 使用 LockOption 替代
type FairOption = LockOption

// =============================================================================
// 通用选项函数 - 所有锁类型都可以使用
// =============================================================================

// WithAcquireTimeout 设置获取锁的超时时间
func WithAcquireTimeout(timeout time.Duration) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.AcquireTimeout = timeout
	})
}

// WithLockTTL 设置锁的生存时间
func WithLockTTL(ttl time.Duration) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.LockTTL = ttl
	})
}

// WithWatchdog 设置是否启用看门狗
func WithWatchdog(enable bool) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.EnableWatchdog = enable
	})
}

// WithWatchdogInterval 设置看门狗续约间隔
func WithWatchdogInterval(interval time.Duration) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.WatchdogInterval = interval
	})
}

// WithRetryInterval 设置重试间隔
func WithRetryInterval(interval time.Duration) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.RetryInterval = interval
	})
}

// WithValue 设置锁的值（持有者标识）
func WithValue(value string) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.Value = value
	})
}

// =============================================================================
// 特定锁类型选项函数
// =============================================================================

// WithQueueTTL 设置公平锁队列TTL（仅公平锁使用）
func WithQueueTTL(ttl time.Duration) LockOption {
	return optionFunc(func(config *LockConfig) {
		config.QueueTTL = ttl
	})
}

// =============================================================================
// 配置应用和默认值设置
// =============================================================================

// ApplyLockOptions 应用选项并设置默认值
// 这是统一的选项应用函数，替代原来的三个函数
func ApplyLockOptions(opts []LockOption) *LockConfig {
	// 设置默认值
	config := &LockConfig{
		AcquireTimeout:   DefaultAcquireTimeout,
		LockTTL:          DefaultLockTTL,
		EnableWatchdog:   true,
		WatchdogInterval: DefaultWatchdogInterval,
		RetryInterval:    DefaultRetryInterval,
		QueueTTL:         DefaultQueueTTL,
		Value:            generateUniqueValue(),
	}

	// 应用用户提供的选项
	for _, opt := range opts {
		opt.Apply(config)
	}

	// 智能设置看门狗间隔
	// 如果用户没有自定义看门狗间隔，则设置为锁TTL的1/3
	if config.WatchdogInterval == DefaultWatchdogInterval {
		config.WatchdogInterval = config.LockTTL / 3
	}

	return config
}

// =============================================================================
// 配置验证和便利方法
// =============================================================================

// Validate 验证配置的有效性
func (c *LockConfig) Validate() error {
	if c.AcquireTimeout <= 0 {
		return NewInvalidConfigError("AcquireTimeout", c.AcquireTimeout, "必须大于0")
	}
	if c.LockTTL <= 0 {
		return NewInvalidConfigError("LockTTL", c.LockTTL, "必须大于0")
	}
	if c.RetryInterval <= 0 {
		return NewInvalidConfigError("RetryInterval", c.RetryInterval, "必须大于0")
	}
	if c.EnableWatchdog && c.WatchdogInterval <= 0 {
		return NewInvalidConfigError("WatchdogInterval", c.WatchdogInterval, "启用看门狗时必须大于0")
	}
	if c.QueueTTL < 0 { // QueueTTL 可以为0，表示不设置过期
		return NewInvalidConfigError("QueueTTL", c.QueueTTL, "不能小于0")
	}
	return nil
}

// Clone 克隆配置
func (c *LockConfig) Clone() *LockConfig {
	return &LockConfig{
		AcquireTimeout:   c.AcquireTimeout,
		LockTTL:          c.LockTTL,
		EnableWatchdog:   c.EnableWatchdog,
		WatchdogInterval: c.WatchdogInterval,
		RetryInterval:    c.RetryInterval,
		Value:            c.Value,
		QueueTTL:         c.QueueTTL,
	}
}

// String 返回配置的字符串表示
func (c *LockConfig) String() string {
	return fmt.Sprintf("LockConfig{AcquireTimeout:%v, LockTTL:%v, EnableWatchdog:%v, WatchdogInterval:%v, RetryInterval:%v, QueueTTL:%v, Value:%s}",
		c.AcquireTimeout, c.LockTTL, c.EnableWatchdog, c.WatchdogInterval, c.RetryInterval, c.QueueTTL, c.Value)
}

// Lock 基本锁接口
type Lock interface {
	// Lock 加锁，使用统一的选项系统
	Lock(ctx context.Context, opts ...LockOption) error
	// TryLock 尝试加锁，立即返回结果
	TryLock(ctx context.Context, opts ...LockOption) (bool, error)
	// Unlock 解锁
	Unlock(ctx context.Context) error
	// Extend 续约锁，延长TTL
	Extend(ctx context.Context, ttl time.Duration) (bool, error)
	// IsLocked 检查锁是否被持有
	IsLocked(ctx context.Context) (bool, error)
	// GetTTL 获取锁的剩余时间
	GetTTL(ctx context.Context) (time.Duration, error)
	// GetKey 获取锁的键名
	GetKey() string
	// GetValue 获取锁的值（持有者标识）
	GetValue() string
}

// RWLock 读写锁接口
type RWLock interface {
	// RLock 获取读锁
	RLock(ctx context.Context, opts ...LockOption) error
	// TryRLock 尝试获取读锁
	TryRLock(ctx context.Context, opts ...LockOption) (bool, error)
	// RUnlock 释放读锁
	RUnlock(ctx context.Context) error

	// WLock 获取写锁
	WLock(ctx context.Context, opts ...LockOption) error
	// TryWLock 尝试获取写锁
	TryWLock(ctx context.Context, opts ...LockOption) (bool, error)
	// WUnlock 释放写锁
	WUnlock(ctx context.Context) error

	// GetReadCount 获取当前读锁数量
	GetReadCount(ctx context.Context) (int, error)
	// IsWriteLocked 检查是否有写锁
	IsWriteLocked(ctx context.Context) (bool, error)
	// GetKey 获取锁的键名
	GetKey() string
}

// FairLock 公平锁接口，确保先到先得
type FairLock interface {
	// Lock 获取公平锁
	Lock(ctx context.Context, opts ...LockOption) error
	// TryLock 尝试获取公平锁
	TryLock(ctx context.Context, opts ...LockOption) (bool, error)
	// Unlock 释放锁
	Unlock(ctx context.Context) error
	// Extend 续约锁
	Extend(ctx context.Context, ttl time.Duration) (bool, error)
	// IsLocked 检查锁是否被持有
	IsLocked(ctx context.Context) (bool, error)
	// GetTTL 获取锁的剩余时间
	GetTTL(ctx context.Context) (time.Duration, error)
	// GetQueuePosition 获取在等待队列中的位置，0表示当前持有锁
	GetQueuePosition(ctx context.Context) (int, error)
	// GetQueueLength 获取等待队列长度
	GetQueueLength(ctx context.Context) (int, error)
	// GetQueueInfo 获取队列详细信息
	GetQueueInfo(ctx context.Context) ([]string, error)
	// GetKey 获取锁的键名
	GetKey() string
	// GetValue 获取锁的值
	GetValue() string
}

// Watchdog 看门狗接口，用于自动续约
type Watchdog interface {
	// Start 启动看门狗
	Start(ctx context.Context) error
	// Stop 停止看门狗
	Stop() error
	// IsRunning 检查看门狗是否在运行
	IsRunning() bool
	// GetStats 获取看门狗统计信息
	GetStats() WatchdogStats
}

// WatchdogStats 看门狗统计信息
type WatchdogStats struct {
	StartTime        time.Time     // 启动时间
	ExtendCount      int64         // 续约次数
	FailedExtends    int64         // 失败的续约次数
	LastExtendTime   time.Time     // 最后续约时间
	LastExtendResult bool          // 最后续约结果
	Interval         time.Duration // 续约间隔
}

// LockWithWatchdog 带看门狗的锁接口
type LockWithWatchdog interface {
	Lock
	Watchdog
}

// LockType 锁类型
type LockType int

const (
	// TypeSimple 简单互斥锁
	TypeSimple LockType = iota
	// TypeFair 公平锁（FIFO）
	TypeFair
	// TypeRW 读写锁
	TypeRW
)

func (lt LockType) String() string {
	switch lt {
	case TypeSimple:
		return "Simple"
	case TypeFair:
		return "Fair"
	case TypeRW:
		return "ReadWrite"
	default:
		return "Unknown"
	}
}

// Backend 后端存储
type Backend interface {
	Type() string
}

// =============================================================================
// 类型安全的工厂接口设计
// =============================================================================

// LockFactory 锁工厂接口，提供类型安全的锁创建方法
type LockFactory interface {
	// CreateSimpleLock 创建简单锁
	CreateSimpleLock(key string) (Lock, error)
	// CreateRWLock 创建读写锁
	CreateRWLock(key string) (RWLock, error)
	// CreateFairLock 创建公平锁
	CreateFairLock(key string) (FairLock, error)
	// GetBackendType 获取后端类型
	GetBackendType() string
}

// RedisBackend Redis后端
type RedisBackend struct {
	Client redis.UniversalClient
}

func (r *RedisBackend) Type() string {
	return "Redis"
}

// LocalBackend 本地内存后端
type LocalBackend struct {
	// ShardCount 分片数量，用于减少锁竞争
	ShardCount int
}

func (l *LocalBackend) Type() string {
	return "Local"
}

// =============================================================================
// 具体工厂实现
// =============================================================================

// RedisLockFactory Redis锁工厂，提供类型安全的Redis锁创建
type RedisLockFactory struct {
	client redis.UniversalClient
}

// NewRedisLockFactory 创建Redis锁工厂
func NewRedisLockFactory(client redis.UniversalClient) LockFactory {
	return &RedisLockFactory{
		client: client,
	}
}

// CreateSimpleLock 创建Redis简单锁
func (f *RedisLockFactory) CreateSimpleLock(key string) (Lock, error) {
	return newRedisSimpleLock(key, f.client), nil
}

// CreateRWLock 创建Redis读写锁
func (f *RedisLockFactory) CreateRWLock(key string) (RWLock, error) {
	return newRedisRWLock(key, f.client), nil
}

// CreateFairLock 创建Redis公平锁
func (f *RedisLockFactory) CreateFairLock(key string) (FairLock, error) {
	return newRedisFairLock(key, f.client), nil
}

// GetBackendType 获取后端类型
func (f *RedisLockFactory) GetBackendType() string {
	return "Redis"
}

// LocalLockFactory 本地锁工厂，提供类型安全的本地锁创建
type LocalLockFactory struct {
	backend *LocalBackend
}

// NewLocalLockFactory 创建本地锁工厂
func NewLocalLockFactory(shardCount int) LockFactory {
	if shardCount <= 0 {
		shardCount = DefaultShardCount
	}
	return &LocalLockFactory{
		backend: &LocalBackend{ShardCount: shardCount},
	}
}

// CreateSimpleLock 创建本地简单锁
func (f *LocalLockFactory) CreateSimpleLock(key string) (Lock, error) {
	return newLocalSimpleLock(key, f.backend), nil
}

// CreateRWLock 创建本地读写锁
func (f *LocalLockFactory) CreateRWLock(key string) (RWLock, error) {
	return newLocalRWLock(key, f.backend), nil
}

// CreateFairLock 创建本地公平锁
func (f *LocalLockFactory) CreateFairLock(key string) (FairLock, error) {
	return newLocalFairLock(key, f.backend), nil
}

// GetBackendType 获取后端类型
func (f *LocalLockFactory) GetBackendType() string {
	return "Local"
}

// NewRedisBackend 创建Redis后端
func NewRedisBackend(client redis.UniversalClient) *RedisBackend {
	return &RedisBackend{
		Client: client,
	}
}

// NewLocalBackend 创建本地后端
func NewLocalBackend(shardCount int) *LocalBackend {
	if shardCount <= 0 {
		shardCount = DefaultShardCount
	}
	return &LocalBackend{
		ShardCount: shardCount,
	}
}

// 默认配置常量
var (
	DefaultAcquireTimeout   = 10 * time.Second
	DefaultLockTTL          = 30 * time.Second
	DefaultWatchdogInterval = 10 * time.Second
	DefaultRetryInterval    = 100 * time.Millisecond
	DefaultQueueTTL         = 5 * time.Minute
	DefaultShardCount       = 32
)

// 生成唯一值，用于标识锁的持有者
func generateUniqueValue() string {
	hostname, _ := os.Hostname()

	// 生成随机字节
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	}

	return fmt.Sprintf("%s-%x", hostname, b)
}

// =============================================================================
// 向后兼容的工厂函数（已弃用）
// =============================================================================

// NewSimpleLock 创建简单锁实例
// Deprecated: 使用 LockFactory.CreateSimpleLock 替代，以获得更好的类型安全性
func NewSimpleLock(key string, backend Backend) (Lock, error) {
	var factory LockFactory
	switch backend.Type() {
	case "Redis":
		redisBackend, ok := backend.(*RedisBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewRedisLockFactory(redisBackend.Client)
	case "Local":
		localBackend, ok := backend.(*LocalBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewLocalLockFactory(localBackend.ShardCount)
	default:
		return nil, ErrUnsupportedBackend
	}
	return factory.CreateSimpleLock(key)
}

// NewRWLock 创建读写锁实例
// Deprecated: 使用 LockFactory.CreateRWLock 替代，以获得更好的类型安全性
func NewRWLock(key string, backend Backend) (RWLock, error) {
	var factory LockFactory
	switch backend.Type() {
	case "Redis":
		redisBackend, ok := backend.(*RedisBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewRedisLockFactory(redisBackend.Client)
	case "Local":
		localBackend, ok := backend.(*LocalBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewLocalLockFactory(localBackend.ShardCount)
	default:
		return nil, ErrUnsupportedBackend
	}
	return factory.CreateRWLock(key)
}

// NewFairLock 创建公平锁实例
// Deprecated: 使用 LockFactory.CreateFairLock 替代，以获得更好的类型安全性
func NewFairLock(key string, backend Backend) (FairLock, error) {
	var factory LockFactory
	switch backend.Type() {
	case "Redis":
		redisBackend, ok := backend.(*RedisBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewRedisLockFactory(redisBackend.Client)
	case "Local":
		localBackend, ok := backend.(*LocalBackend)
		if !ok {
			return nil, ErrUnsupportedBackend
		}
		factory = NewLocalLockFactory(localBackend.ShardCount)
	default:
		return nil, ErrUnsupportedBackend
	}
	return factory.CreateFairLock(key)
}

// ===== 重试逻辑抽象 =====

// RetryOperation 定义重试操作的函数签名
// 返回 (success bool, result T, err error)
// - success: 操作是否成功，如果为true则停止重试
// - result: 操作结果，仅在success为true时有效
// - err: 操作错误，如果非nil则立即返回错误
type RetryOperation[T any] func(ctx context.Context) (success bool, result T, err error)

// RetryConfig 重试配置
type RetryConfig struct {
	// Timeout 总超时时间
	Timeout time.Duration
	// Interval 重试间隔
	Interval time.Duration
	// Key 锁键名，用于错误上下文
	Key string
}

// RetryWithTimeout 通用重试函数，支持泛型返回类型
func RetryWithTimeout[T any](ctx context.Context, config RetryConfig, operation RetryOperation[T]) (T, error) {
	var zero T
	deadline := time.Now().Add(config.Timeout)

	for {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// 检查是否超时
		if time.Now().After(deadline) {
			return zero, NewAcquireTimeoutError(config.Key, config.Timeout)
		}

		// 执行操作
		success, result, err := operation(ctx)
		if err != nil {
			return zero, err
		}
		if success {
			return result, nil
		}

		// 等待重试间隔
		time.Sleep(config.Interval)
	}
}

// RetryLockOperation 专门用于锁操作的重试函数
// 简化了RetryWithTimeout的使用，专门针对返回bool的锁操作
func RetryLockOperation(ctx context.Context, config RetryConfig, operation func(ctx context.Context) (bool, error)) error {
	_, err := RetryWithTimeout(ctx, config, func(ctx context.Context) (bool, bool, error) {
		success, err := operation(ctx)
		return success, success, err
	})
	return err
}

// NewRetryConfig 创建重试配置的便捷函数
func NewRetryConfig(key string, timeout, interval time.Duration) RetryConfig {
	return RetryConfig{
		Timeout:  timeout,
		Interval: interval,
		Key:      key,
	}
}

// NewRetryConfigFromLockOptions 从锁选项创建重试配置
func NewRetryConfigFromLockOptions(key string, opts ...LockOption) RetryConfig {
	config := ApplyLockOptions(opts)
	return RetryConfig{
		Timeout:  config.AcquireTimeout,
		Interval: config.RetryInterval,
		Key:      key,
	}
}

// RetryWithCustomLogic 支持自定义重试逻辑的重试函数
// 用于处理复杂的重试场景，如公平锁的队列管理
func RetryWithCustomLogic(ctx context.Context, timeout time.Duration, key string, operation func(ctx context.Context, deadline time.Time) error) error {
	deadline := time.Now().Add(timeout)
	return operation(ctx, deadline)
}

// =============================================================================
// 带看门狗的锁创建函数
// =============================================================================

// NewSimpleLockWithWatchdog 创建带看门狗的简单锁
func NewSimpleLockWithWatchdog(key string, backend Backend, opts ...LockOption) (LockWithWatchdog, error) {
	lock, err := NewSimpleLock(key, backend)
	if err != nil {
		return nil, err
	}
	config := ApplyLockOptions(opts)
	return NewLockWithWatchdog(lock, config), nil
}

// NewRWLockWithWatchdog 创建带看门狗的读写锁
// 注意：看门狗只对写锁有效，读锁不支持看门狗
func NewRWLockWithWatchdog(key string, backend Backend, opts ...LockOption) (RWLock, error) {
	// 读写锁的看门狗实现比较复杂，暂时返回普通读写锁
	// TODO: 实现读写锁的看门狗支持
	return NewRWLock(key, backend)
}

// NewFairLockWithWatchdog 创建带看门狗的公平锁
func NewFairLockWithWatchdog(key string, backend Backend, opts ...LockOption) (LockWithWatchdog, error) {
	fairLock, err := NewFairLock(key, backend)
	if err != nil {
		return nil, err
	}

	// 将FairLock适配为Lock接口
	lockAdapter := &fairLockAdapter{fairLock}
	config := ApplyLockOptions(opts)
	return NewLockWithWatchdog(lockAdapter, config), nil
}

// fairLockAdapter 公平锁适配器，将FairLock适配为Lock接口
type fairLockAdapter struct {
	FairLock
}

// 确保fairLockAdapter实现Lock接口
var _ Lock = (*fairLockAdapter)(nil)
