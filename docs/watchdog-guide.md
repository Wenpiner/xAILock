# RLock 看门狗机制使用指南

## 概述

看门狗(Watchdog)机制是RLock提供的自动续约功能，用于防止长时间运行的任务因锁过期而被意外中断。看门狗会在后台定期延长锁的生存时间，确保任务执行期间锁始终有效。

## 核心特性

- **自动续约**: 后台自动延长锁的TTL，无需手动干预
- **智能管理**: 自动检测锁的状态，避免无效续约
- **资源清理**: 任务完成后自动停止看门狗，释放资源
- **错误处理**: 续约失败时提供详细的错误信息
- **性能监控**: 集成监控系统，跟踪续约成功率和性能

## 快速开始

### 基本使用

```go
import (
    "context"
    "time"
    "github.com/wenpiner/rlock"
)

func main() {
    // 创建锁
    lock := lock.NewSimpleLock("long-task", backend)
    ctx := context.Background()
    
    // 启用看门狗获取锁
    err := lock.Lock(ctx,
        lock.WithLockTTL(30*time.Second),        // 锁的初始TTL
        lock.WithWatchdog(true),                 // 启用看门狗
        lock.WithWatchdogInterval(10*time.Second)) // 续约间隔
    if err != nil {
        panic(err)
    }
    defer lock.Unlock(ctx)
    
    // 长时间运行的任务
    // 看门狗会自动续约，确保锁不会过期
    time.Sleep(2 * time.Minute)
    
    fmt.Println("任务完成，锁在整个过程中保持有效")
}
```

### 手动管理看门狗

```go
func manualWatchdogExample() {
    lock := lock.NewSimpleLock("manual-watchdog", backend)
    ctx := context.Background()
    
    // 先获取锁
    err := lock.Lock(ctx, lock.WithLockTTL(30*time.Second))
    if err != nil {
        panic(err)
    }
    defer lock.Unlock(ctx)
    
    // 手动启动看门狗
    watchdog, err := lock.StartWatchdog(ctx, &lock.WatchdogConfig{
        Interval:    10 * time.Second,
        ExtendBy:    30 * time.Second,
        MaxFailures: 3,
    })
    if err != nil {
        panic(err)
    }
    defer watchdog.Stop()
    
    // 执行长时间任务
    performLongRunningTask()
}
```

## 配置选项

### WatchdogConfig 结构

```go
type WatchdogConfig struct {
    Interval    time.Duration // 续约间隔 (默认: 锁TTL的1/3)
    ExtendBy    time.Duration // 每次续约延长的时间 (默认: 锁TTL)
    MaxFailures int           // 最大连续失败次数 (默认: 3)
    OnFailure   func(error)   // 失败回调函数
    OnSuccess   func()        // 成功回调函数
}
```

### 配置示例

```go
// 自定义看门狗配置
config := &lock.WatchdogConfig{
    Interval:    5 * time.Second,   // 每5秒续约一次
    ExtendBy:    60 * time.Second,  // 每次延长60秒
    MaxFailures: 5,                 // 最多允许5次连续失败
    OnFailure: func(err error) {
        log.Error("看门狗续约失败", "error", err)
    },
    OnSuccess: func() {
        log.Debug("看门狗续约成功")
    },
}

watchdog, err := lock.StartWatchdog(ctx, config)
```

## 使用场景

### 1. 长时间数据处理

```go
func processLargeDataset() {
    lock := lock.NewSimpleLock("data-processing", redisBackend)
    ctx := context.Background()
    
    // 数据处理可能需要几小时
    err := lock.Lock(ctx,
        lock.WithLockTTL(10*time.Minute),        // 初始TTL 10分钟
        lock.WithWatchdog(true),                 // 启用看门狗
        lock.WithWatchdogInterval(3*time.Minute)) // 每3分钟续约
    if err != nil {
        return err
    }
    defer lock.Unlock(ctx)
    
    // 处理大量数据
    for _, batch := range dataBatches {
        processBatch(batch)
        // 看门狗确保锁在整个过程中有效
    }
}
```

### 2. 定时任务执行

```go
func scheduledTask() {
    lock := lock.NewSimpleLock("scheduled-job", redisBackend)
    ctx := context.Background()
    
    // 防止任务重复执行
    success, err := lock.TryLock(ctx,
        lock.WithLockTTL(1*time.Hour),           // 任务最长执行1小时
        lock.WithWatchdog(true),                 // 启用看门狗
        lock.WithWatchdogInterval(15*time.Minute)) // 每15分钟续约
    if err != nil || !success {
        log.Info("任务已在其他节点执行")
        return
    }
    defer lock.Unlock(ctx)
    
    // 执行定时任务
    executeScheduledWork()
}
```

### 3. 分布式锁保护的服务

```go
func criticalService() {
    lock := lock.NewSimpleLock("critical-service", redisBackend)
    ctx := context.Background()
    
    // 服务启动时获取锁
    err := lock.Lock(ctx,
        lock.WithLockTTL(5*time.Minute),
        lock.WithWatchdog(true),
        lock.WithWatchdogInterval(1*time.Minute))
    if err != nil {
        log.Fatal("无法获取服务锁", "error", err)
    }
    defer lock.Unlock(ctx)
    
    // 启动服务
    server := startServer()
    defer server.Shutdown()
    
    // 服务运行期间，看门狗保持锁有效
    waitForShutdownSignal()
}
```

## 监控和调试

### 看门狗状态监控

```go
func monitorWatchdog(watchdog lock.Watchdog) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        status := watchdog.GetStatus()
        
        fmt.Printf("看门狗状态:\n")
        fmt.Printf("  运行状态: %v\n", status.IsRunning)
        fmt.Printf("  续约次数: %d\n", status.ExtendCount)
        fmt.Printf("  失败次数: %d\n", status.FailureCount)
        fmt.Printf("  最后续约: %v\n", status.LastExtendTime)
        fmt.Printf("  下次续约: %v\n", status.NextExtendTime)
        
        if status.LastError != nil {
            fmt.Printf("  最后错误: %v\n", status.LastError)
        }
    }
}
```

### 全局看门狗管理

```go
func globalWatchdogStats() {
    manager := lock.GetGlobalWatchdogManager()
    stats := manager.GetStats()
    
    fmt.Printf("全局看门狗统计:\n")
    fmt.Printf("  活跃看门狗: %d\n", stats.ActiveWatchdogs)
    fmt.Printf("  总续约次数: %d\n", stats.TotalExtends)
    fmt.Printf("  成功续约: %d\n", stats.SuccessfulExtends)
    fmt.Printf("  失败续约: %d\n", stats.FailedExtends)
    fmt.Printf("  平均续约时间: %v\n", stats.AverageExtendTime)
}
```

## 最佳实践

### 1. 合理设置续约间隔

```go
// 推荐配置
lockTTL := 30 * time.Second
watchdogInterval := lockTTL / 3  // TTL的1/3，确保有足够的续约机会

err := lock.Lock(ctx,
    lock.WithLockTTL(lockTTL),
    lock.WithWatchdog(true),
    lock.WithWatchdogInterval(watchdogInterval))
```

### 2. 处理续约失败

```go
config := &lock.WatchdogConfig{
    Interval:    10 * time.Second,
    MaxFailures: 3,
    OnFailure: func(err error) {
        // 记录错误
        log.Error("看门狗续约失败", "error", err)
        
        // 发送告警
        sendAlert("Watchdog failure", err.Error())
        
        // 可以选择停止任务或采取其他措施
        if isNetworkError(err) {
            // 网络错误，可能是临时的，继续尝试
            return
        } else {
            // 其他错误，可能需要停止任务
            stopCurrentTask()
        }
    },
}
```

### 3. 优雅关闭

```go
func gracefulShutdown() {
    lock := lock.NewSimpleLock("service", backend)
    ctx := context.Background()
    
    err := lock.Lock(ctx,
        lock.WithLockTTL(5*time.Minute),
        lock.WithWatchdog(true))
    if err != nil {
        return
    }
    
    // 设置信号处理
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Info("收到关闭信号，开始优雅关闭")
        
        // 停止看门狗和释放锁
        lock.Unlock(ctx)
        
        // 其他清理工作
        cleanup()
        
        os.Exit(0)
    }()
    
    // 主要业务逻辑
    runMainService()
}
```

### 4. 错误恢复策略

```go
func robustWatchdog() {
    lock := lock.NewSimpleLock("robust-task", backend)
    ctx := context.Background()
    
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := lock.Lock(ctx,
            lock.WithLockTTL(30*time.Second),
            lock.WithWatchdog(true))
        if err != nil {
            log.Warn("获取锁失败，重试中", "attempt", attempt+1, "error", err)
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        
        // 成功获取锁，执行任务
        err = executeTask()
        lock.Unlock(ctx)
        
        if err == nil {
            break // 任务成功完成
        }
        
        log.Warn("任务执行失败，重试中", "attempt", attempt+1, "error", err)
    }
}
```

## 性能考虑

### 续约频率优化

```go
// 根据任务特性调整续约频率
func optimizeWatchdogInterval(taskDuration time.Duration) time.Duration {
    if taskDuration < 1*time.Minute {
        // 短任务：频繁续约
        return 10 * time.Second
    } else if taskDuration < 10*time.Minute {
        // 中等任务：适中频率
        return 30 * time.Second
    } else {
        // 长任务：较低频率
        return 2 * time.Minute
    }
}
```

### 资源使用监控

```go
func monitorWatchdogResources() {
    stats := lock.GetGlobalMetrics().GetStats()
    
    if stats.WatchdogStats.ActiveWatchdogs > 100 {
        log.Warn("活跃看门狗数量过多", "count", stats.WatchdogStats.ActiveWatchdogs)
    }
    
    if stats.WatchdogStats.FailedExtends > stats.WatchdogStats.TotalExtends/10 {
        log.Warn("看门狗失败率过高", 
            "failed", stats.WatchdogStats.FailedExtends,
            "total", stats.WatchdogStats.TotalExtends)
    }
}
```

## 故障排查

### 常见问题

1. **续约失败率高**
   - 检查网络连接稳定性
   - 验证Redis服务器状态
   - 调整续约间隔和超时设置

2. **看门狗无法启动**
   - 确认锁已成功获取
   - 检查看门狗配置参数
   - 验证上下文是否已取消

3. **内存泄漏**
   - 确保正确调用Stop()方法
   - 检查是否有未释放的看门狗实例
   - 监控全局看门狗管理器状态

### 调试工具

```go
// 启用详细日志
func enableWatchdogDebugLogging() {
    config := &lock.WatchdogConfig{
        Interval: 10 * time.Second,
        OnSuccess: func() {
            log.Debug("看门狗续约成功", "time", time.Now())
        },
        OnFailure: func(err error) {
            log.Error("看门狗续约失败", "error", err, "time", time.Now())
        },
    }
    
    watchdog, err := lock.StartWatchdog(ctx, config)
    if err != nil {
        log.Fatal("启动看门狗失败", "error", err)
    }
    defer watchdog.Stop()
}
```

---

通过合理使用看门狗机制，您可以：
- 确保长时间任务的锁不会意外过期
- 提高分布式系统的可靠性
- 简化锁的生命周期管理
- 获得详细的续约监控数据
