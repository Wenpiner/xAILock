# RLock 监控系统使用指南

## 概述

RLock提供了全面的监控系统，帮助您了解锁的使用情况、性能表现和潜在问题。监控系统设计为低开销、高性能，适合在生产环境中使用。

## 快速开始

### 创建带监控的锁

```go
import "github.com/wenpiner/rlock"

// 创建带监控的简单锁
lock, err := lock.NewSimpleLockWithMetrics("my-lock", backend)

// 创建带监控的读写锁
rwLock, err := lock.NewRWLockWithMetrics("my-rw-lock", backend)

// 创建带监控的公平锁
fairLock, err := lock.NewFairLockWithMetrics("my-fair-lock", backend)
```

### 获取监控数据

```go
// 获取全局监控统计
stats := lock.GetGlobalMetrics().GetStats()

// 打印基本统计信息
fmt.Printf("锁操作成功率: %.2f%%\n", 
    float64(stats.LockOperations.TryAcquireSuccess) / 
    float64(stats.LockOperations.TryAcquireCount) * 100)
```

## 监控指标详解

### 1. 锁操作统计 (LockOperationStats)

```go
type LockOperationStats struct {
    // 获取锁统计
    AcquireCount         int64         // Lock() 调用次数
    AcquireSuccess       int64         // Lock() 成功次数
    AcquireFailed        int64         // Lock() 失败次数
    AverageAcquireTime   time.Duration // 平均获取时间
    
    // 尝试获取锁统计
    TryAcquireCount      int64         // TryLock() 调用次数
    TryAcquireSuccess    int64         // TryLock() 成功次数
    TryAcquireFailed     int64         // TryLock() 失败次数
    AverageTryAcquireTime time.Duration // 平均尝试获取时间
    
    // 释放锁统计
    ReleaseCount         int64         // Unlock() 调用次数
    ReleaseSuccess       int64         // Unlock() 成功次数
    ReleaseFailed        int64         // Unlock() 失败次数
    AverageReleaseTime   time.Duration // 平均释放时间
    
    // 延长锁统计
    ExtendCount          int64         // Extend() 调用次数
    ExtendSuccess        int64         // Extend() 成功次数
    ExtendFailed         int64         // Extend() 失败次数
    AverageExtendTime    time.Duration // 平均延长时间
}
```

**使用示例:**
```go
stats := lock.GetGlobalMetrics().GetStats()
ops := stats.LockOperations

fmt.Printf("锁操作统计:\n")
fmt.Printf("  获取锁: %d次 (成功: %d, 失败: %d)\n", 
    ops.AcquireCount, ops.AcquireSuccess, ops.AcquireFailed)
fmt.Printf("  平均获取时间: %v\n", ops.AverageAcquireTime)
fmt.Printf("  成功率: %.2f%%\n", 
    float64(ops.AcquireSuccess)/float64(ops.AcquireCount)*100)
```

### 2. 并发统计 (ConcurrencyStats)

```go
type ConcurrencyStats struct {
    MaxConcurrentLocks   int64         // 最大并发锁数量
    CurrentLocks         int64         // 当前持有的锁数量
    MaxQueueLength       int64         // 最大队列长度
    AverageWaitTime      time.Duration // 平均等待时间
    ContentionRate       float64       // 竞争率 (0-1)
}
```

**使用示例:**
```go
concurrency := stats.ConcurrencyStats

fmt.Printf("并发统计:\n")
fmt.Printf("  当前锁数: %d (历史最大: %d)\n", 
    concurrency.CurrentLocks, concurrency.MaxConcurrentLocks)
fmt.Printf("  竞争率: %.2f%%\n", concurrency.ContentionRate*100)
```

### 3. 错误统计 (ErrorStats)

```go
type ErrorStats struct {
    TotalErrors      int64            // 总错误数
    ErrorsByType     map[string]int64 // 按错误类型分组
    ErrorsByCode     map[string]int64 // 按错误代码分组
    ErrorsBySeverity map[string]int64 // 按严重程度分组
    ErrorsByBackend  map[string]int64 // 按后端类型分组
}
```

**使用示例:**
```go
errors := stats.ErrorStats

fmt.Printf("错误统计:\n")
fmt.Printf("  总错误数: %d\n", errors.TotalErrors)
for errType, count := range errors.ErrorsByType {
    fmt.Printf("  %s: %d次\n", errType, count)
}
```

### 4. 资源统计 (ResourceStats)

```go
type ResourceStats struct {
    CurrentMemoryUsage   int64 // 当前内存使用 (bytes)
    MaxMemoryUsage       int64 // 最大内存使用 (bytes)
    CurrentConnections   int64 // 当前连接数
    CurrentGoroutines    int64 // 当前协程数
}
```

### 5. 看门狗统计 (WatchdogStats)

```go
type WatchdogStats struct {
    ActiveWatchdogs      int64         // 活跃看门狗数量
    TotalExtends         int64         // 总续约次数
    SuccessfulExtends    int64         // 成功续约次数
    FailedExtends        int64         // 失败续约次数
    AverageExtendTime    time.Duration // 平均续约时间
}
```

### 6. 分片统计 (ShardingStats)

```go
type ShardingStats struct {
    ShardCount       int64   // 分片数量
    MaxShardLoad     int64   // 最大分片负载
    MinShardLoad     int64   // 最小分片负载
    LoadStandardDev  float64 // 负载标准差
    BalanceRatio     float64 // 负载均衡比率
}
```

## 性能影响

### 监控开销测试

```bash
# 运行性能对比测试
go test -run TestMetricsPerformanceImpact -v

# 运行基准测试
go test -bench=BenchmarkMetricsOverhead -benchmem
```

### 开销分析

| 指标 | 不带监控 | 带监控 | 开销 |
|------|----------|--------|------|
| 执行时间 | 1,465 ns/op | 1,638 ns/op | **+11.8%** |
| 内存分配 | 328 B/op | 328 B/op | **0%** |
| 分配次数 | 11 allocs/op | 11 allocs/op | **0%** |

**结论**: 监控系统的性能开销极低，完全适合生产环境使用。

## 最佳实践

### 1. 监控数据收集

```go
// 定期收集监控数据
func collectMetrics() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := lock.GetGlobalMetrics().GetStats()
        
        // 发送到监控系统 (如 Prometheus)
        sendToMonitoring(stats)
        
        // 检查异常情况
        if stats.ErrorStats.TotalErrors > threshold {
            log.Warn("锁错误数量过高", "count", stats.ErrorStats.TotalErrors)
        }
    }
}
```

### 2. 告警规则

```go
func checkAlerts(stats lock.MetricsStats) {
    // 成功率过低告警
    if stats.LockOperations.TryAcquireCount > 0 {
        successRate := float64(stats.LockOperations.TryAcquireSuccess) / 
                      float64(stats.LockOperations.TryAcquireCount)
        if successRate < 0.95 {
            alert("锁获取成功率过低", successRate)
        }
    }
    
    // 平均响应时间过高告警
    if stats.LockOperations.AverageAcquireTime > 100*time.Millisecond {
        alert("锁获取时间过长", stats.LockOperations.AverageAcquireTime)
    }
    
    // 并发数过高告警
    if stats.ConcurrencyStats.CurrentLocks > 1000 {
        alert("并发锁数量过高", stats.ConcurrencyStats.CurrentLocks)
    }
}
```

### 3. 监控重置

```go
// 定期重置监控数据，避免数据累积过多
func resetMetricsPeriodically() {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        // 保存当前数据
        stats := lock.GetGlobalMetrics().GetStats()
        archiveStats(stats)
        
        // 重置监控数据
        lock.GetGlobalMetrics().Reset()
        log.Info("监控数据已重置")
    }
}
```

## 与外部监控系统集成

### Prometheus 集成示例

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    lockAcquireTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rlock_acquire_total",
            Help: "Total number of lock acquire attempts",
        },
        []string{"type", "backend", "result"},
    )
    
    lockAcquireDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "rlock_acquire_duration_seconds",
            Help: "Lock acquire duration in seconds",
        },
        []string{"type", "backend"},
    )
)

func exportToPrometheus() {
    stats := lock.GetGlobalMetrics().GetStats()
    
    // 导出计数器
    lockAcquireTotal.WithLabelValues("simple", "local", "success").
        Add(float64(stats.LockOperations.AcquireSuccess))
    
    // 导出直方图
    lockAcquireDuration.WithLabelValues("simple", "local").
        Observe(stats.LockOperations.AverageAcquireTime.Seconds())
}
```

## 故障排查

### 常见问题

1. **锁获取成功率低**
   - 检查锁的TTL设置是否合理
   - 分析并发竞争情况
   - 考虑使用公平锁减少饥饿

2. **平均响应时间高**
   - 检查网络延迟 (Redis锁)
   - 分析锁竞争程度
   - 考虑使用本地锁或分片优化

3. **内存使用持续增长**
   - 检查是否有锁泄漏
   - 验证Unlock调用是否正确
   - 监控看门狗的清理机制

### 调试工具

```go
// 打印详细的监控报告
func printDetailedReport() {
    stats := lock.GetGlobalMetrics().GetStats()
    
    fmt.Printf("=== RLock 监控报告 ===\n")
    fmt.Printf("锁操作: 成功 %d / 总计 %d (%.2f%%)\n",
        stats.LockOperations.AcquireSuccess,
        stats.LockOperations.AcquireCount,
        float64(stats.LockOperations.AcquireSuccess)/float64(stats.LockOperations.AcquireCount)*100)
    
    fmt.Printf("平均响应时间: %v\n", stats.LockOperations.AverageAcquireTime)
    fmt.Printf("当前并发锁: %d\n", stats.ConcurrencyStats.CurrentLocks)
    fmt.Printf("错误总数: %d\n", stats.ErrorStats.TotalErrors)
    
    if len(stats.ErrorStats.ErrorsByType) > 0 {
        fmt.Printf("错误分布:\n")
        for errType, count := range stats.ErrorStats.ErrorsByType {
            fmt.Printf("  %s: %d\n", errType, count)
        }
    }
}
```

---

通过合理使用RLock的监控系统，您可以：
- 实时了解锁的使用情况
- 及时发现性能问题
- 优化锁的配置和使用方式
- 确保系统的稳定性和可靠性
