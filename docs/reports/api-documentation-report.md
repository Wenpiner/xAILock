# xAILock API 文档

## 📋 目录

- [核心接口](#核心接口)
- [锁类型](#锁类型)
- [配置选项](#配置选项)
- [后端支持](#后端支持)
- [错误处理](#错误处理)
- [性能优化](#性能优化)

## 🔌 核心接口

### Lock 接口 (简单锁)

```go
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
```

### RWLock 接口 (读写锁)

```go
type RWLock interface {
    // 读锁操作
    RLock(ctx context.Context, opts ...LockOption) error
    TryRLock(ctx context.Context, opts ...LockOption) (bool, error)
    RUnlock(ctx context.Context) error
    
    // 写锁操作
    WLock(ctx context.Context, opts ...LockOption) error
    TryWLock(ctx context.Context, opts ...LockOption) (bool, error)
    WUnlock(ctx context.Context) error
    
    // 状态查询
    GetReadCount(ctx context.Context) (int, error)
    IsWriteLocked(ctx context.Context) (bool, error)
    GetKey() string
}
```

### FairLock 接口 (公平锁)

```go
type FairLock interface {
    // 基础锁操作
    Lock(ctx context.Context, opts ...LockOption) error
    TryLock(ctx context.Context, opts ...LockOption) (bool, error)
    Unlock(ctx context.Context) error
    Extend(ctx context.Context, ttl time.Duration) (bool, error)
    
    // 状态查询
    IsLocked(ctx context.Context) (bool, error)
    GetTTL(ctx context.Context) (time.Duration, error)
    
    // 队列管理
    GetQueuePosition(ctx context.Context) (int, error)
    GetQueueLength(ctx context.Context) (int, error)
    GetQueueInfo(ctx context.Context) ([]string, error)
    
    GetKey() string
    GetValue() string
}
```

## 🏭 锁类型

### SimpleLock (简单锁)

**特点**: 基础的互斥锁，同时只能有一个持有者

**创建方式**:
```go
// 本地锁
lock := lock.NewSimpleLock("my-lock", localBackend)

// Redis锁
lock := lock.NewSimpleLock("my-lock", redisBackend)

// 带监控的锁
lock, err := lock.NewSimpleLockWithMetrics("my-lock", backend)
```

**使用示例**:
```go
ctx := context.Background()

// 获取锁
err := lock.Lock(ctx, 
    lock.WithTimeout(30*time.Second),
    lock.WithTTL(60*time.Second))
if err != nil {
    log.Fatal(err)
}
defer lock.Unlock(ctx)

// 执行业务逻辑
doWork()
```

### RWLock (读写锁)

**特点**: 支持多个读者或单个写者，读写互斥

**创建方式**:
```go
rwLock := lock.NewRWLock("my-rw-lock", backend)
```

**使用示例**:
```go
// 读操作
err := rwLock.RLock(ctx, lock.WithTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
defer rwLock.RUnlock(ctx)

data := readData()

// 写操作
err = rwLock.WLock(ctx, lock.WithTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
defer rwLock.WUnlock(ctx)

writeData(newData)
```

### FairLock (公平锁)

**特点**: 先到先得，防止锁饥饿现象

**创建方式**:
```go
fairLock := lock.NewFairLock("my-fair-lock", backend)
```

**使用示例**:
```go
// 获取公平锁
err := fairLock.Lock(ctx, 
    lock.WithTimeout(30*time.Second),
    lock.WithQueueTTL(120*time.Second))
if err != nil {
    log.Fatal(err)
}
defer fairLock.Unlock(ctx)

// 查看队列状态
position, _ := fairLock.GetQueuePosition(ctx)
length, _ := fairLock.GetQueueLength(ctx)
fmt.Printf("队列位置: %d, 队列长度: %d\n", position, length)
```

## ⚙️ 配置选项

### 统一选项系统

所有锁类型都使用相同的选项系统：

```go
type LockConfig struct {
    AcquireTimeout   time.Duration // 获取锁超时时间
    LockTTL          time.Duration // 锁的生存时间
    EnableWatchdog   bool          // 是否启用看门狗
    WatchdogInterval time.Duration // 看门狗续约间隔
    RetryInterval    time.Duration // 重试间隔
    Value            string        // 锁的值（持有者标识）
    QueueTTL         time.Duration // 队列TTL（仅公平锁）
}
```

### 选项函数

```go
// 基础选项
func WithTimeout(timeout time.Duration) LockOption
func WithTTL(ttl time.Duration) LockOption
func WithRetryInterval(interval time.Duration) LockOption
func WithValue(value string) LockOption

// 看门狗选项
func WithWatchdog(enabled bool) LockOption
func WithWatchdogInterval(interval time.Duration) LockOption

// 公平锁专用选项
func WithQueueTTL(ttl time.Duration) LockOption
```

### 使用示例

```go
// 基础配置
err := lock.Lock(ctx,
    lock.WithTimeout(30*time.Second),    // 30秒获取超时
    lock.WithTTL(60*time.Second),        // 60秒锁TTL
    lock.WithRetryInterval(100*time.Millisecond)) // 100ms重试间隔

// 启用看门狗
err := lock.Lock(ctx,
    lock.WithTTL(30*time.Second),
    lock.WithWatchdog(true),             // 启用看门狗
    lock.WithWatchdogInterval(10*time.Second)) // 10秒续约间隔

// 自定义锁值
err := lock.Lock(ctx,
    lock.WithValue("custom-holder-id"),  // 自定义持有者ID
    lock.WithTTL(60*time.Second))
```

## 🔧 后端支持

### LocalBackend (本地内存锁)

**特点**: 
- 基于内存的锁实现
- 支持分片优化，提升并发性能
- 适用于单机场景

**创建方式**:
```go
backend := lock.NewLocalBackend(
    lock.WithShardCount(64),  // 64个分片
)
```

**性能表现**:
```
BenchmarkShardedLock-12    726,101    1,644 ns/op    48 B/op    1 allocs/op
```

### RedisBackend (Redis分布式锁)

**特点**:
- 基于Redis的分布式锁
- 使用Lua脚本保证原子性
- 支持集群和哨兵模式

**创建方式**:
```go
// 单机Redis
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})
backend := lock.NewRedisBackend(client)

// Redis集群
client := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{"localhost:7000", "localhost:7001"},
})
backend := lock.NewRedisBackend(client)
```

**性能表现**:
```
SimpleLock:  5,965 ops/sec  (207μs/op)
RWLock:      7,791 ops/sec  (128μs/op)  
FairLock:    5,427 ops/sec  (231μs/op)
```

## 🚨 错误处理

### 结构化错误类型

```go
type LockError struct {
    Code      ErrorCode     // 错误代码
    Message   string        // 错误消息
    Severity  ErrorSeverity // 严重程度
    Context   map[string]interface{} // 错误上下文
    Cause     error         // 原始错误
    Timestamp time.Time     // 错误时间
}
```

### 错误代码

```go
const (
    ErrCodeTimeout          ErrorCode = "TIMEOUT"
    ErrCodeLockNotFound     ErrorCode = "LOCK_NOT_FOUND"
    ErrCodeInvalidOperation ErrorCode = "INVALID_OPERATION"
    ErrCodeNetworkError     ErrorCode = "NETWORK_ERROR"
    ErrCodeBackendError     ErrorCode = "BACKEND_ERROR"
)
```

### 错误处理示例

```go
err := lock.Lock(ctx, lock.WithTimeout(5*time.Second))
if err != nil {
    if lockErr, ok := err.(*lock.LockError); ok {
        switch lockErr.Code {
        case lock.ErrCodeTimeout:
            log.Warn("获取锁超时", "key", lock.GetKey())
        case lock.ErrCodeNetworkError:
            log.Error("网络错误", "error", lockErr.Cause)
        default:
            log.Error("未知错误", "error", lockErr)
        }
    }
    return err
}
```

## 🚀 性能优化

### 本地锁分片机制

**原理**: 使用哈希分片将锁分散到多个分片中，减少锁竞争

**实现**:
```go
type ShardedLockManager[T LockData] struct {
    shards    []*LockShard[T]
    shardMask uint64
    hasher    hash.Hash64
    pool      *LockPool[T]
}
```

**优势**:
- 并发性能提升至726,101 ops/sec
- 分片分布均匀，标准差仅24.25
- 支持对象池，减少GC压力

### Redis脚本优化

**优化效果**:
| 锁类型 | 操作 | 优化前 | 优化后 | 改进 |
|--------|------|--------|--------|------|
| 简单锁 | 状态查询 | 2次网络调用 | 1次 | **50%减少** |
| 读写锁 | 状态查询 | 2次网络调用 | 1次 | **50%减少** |
| 公平锁 | TryLock | 5-6次网络调用 | 1次 | **83%减少** |

**核心脚本**:
- SimpleLockAcquireScript: 原子获取简单锁
- RWLockAcquireReadScript: 原子获取读锁
- FairLockTryAcquireScript: 公平锁原子获取

---

**文档版本**: v2.0  
**最后更新**: 2025-07-07  
**API兼容性**: 100%向后兼容
