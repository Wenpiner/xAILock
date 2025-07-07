# RLock 文档中心

欢迎来到RLock分布式锁库的文档中心！这里包含了所有相关的技术文档、使用指南和项目报告。

## 📚 文档导航

### 🚀 快速开始
- [项目主页](../README.md) - 项目概述、特性介绍和快速开始指南

### 📖 核心文档
- [重构计划书](REFACTOR_PLAN.md) - 详细的重构计划和进度跟踪
- [文档更新总结](DOCUMENTATION_UPDATE_SUMMARY.md) - 文档更新记录

### 📋 项目报告 (按Phase顺序)
- [Phase 1: 接口统一化报告](reports/phase-1-interface-unification-report.md) - 接口设计统一化
- [Phase 2: API文档报告](reports/phase-2-api-documentation-report.md) - 完整的API参考文档
- [Phase 3: 监控系统报告](reports/phase-3-monitoring-system-report.md) - 监控功能实现报告
- [Phase 4: 项目完成总结](reports/phase-4-project-completion-summary.md) - 整体重构成果总结

### 🛠️ 功能指南
- [监控系统指南](monitoring-guide.md) - 完整的监控功能使用指南
- [看门狗机制指南](watchdog-guide.md) - 自动续约功能详细说明

### 💻 开发日志
- [AI开发日志](../AIs/) - 详细的开发过程记录和技术决策

### 📝 使用示例
- [监控示例](../examples/metrics_example.go) - 监控功能完整演示
- [基础示例](../examples/) - 各种锁类型使用示例

## 🎯 文档分类

### 按用户类型

#### 👨‍💻 开发者
- **新手**: [项目主页](../README.md) → [API文档](reports/phase-2-api-documentation-report.md)
- **进阶**: [监控系统指南](monitoring-guide.md) → [看门狗机制指南](watchdog-guide.md)
- **专家**: [重构计划书](REFACTOR_PLAN.md) → [项目完成总结](reports/phase-4-project-completion-summary.md)

#### 🏗️ 架构师
- [项目完成总结](reports/phase-4-project-completion-summary.md) - 技术架构和设计决策
- [重构计划书](REFACTOR_PLAN.md) - 系统演进过程
- [AI开发日志](../AIs/) - 详细的技术实现过程

#### 🔧 运维人员
- [监控系统指南](monitoring-guide.md) - 监控配置和故障排查
- [项目主页](../README.md) - 性能特性和部署要求

### 按功能模块

#### 🔒 核心锁功能
- [API文档](reports/phase-2-api-documentation-report.md) - 锁接口和使用方法
- [项目主页](../README.md) - 功能特性和使用示例

#### 📊 监控系统
- [监控系统指南](monitoring-guide.md) - 完整使用指南
- [监控示例](../examples/metrics_example.go) - 代码示例
- [监控系统报告](reports/phase-3-monitoring-system-report.md) - 实现细节

#### 🐕 看门狗机制
- [看门狗机制指南](watchdog-guide.md) - 详细使用说明
- [项目主页](../README.md) - 基本使用示例

#### 🚀 性能优化
- [项目完成总结](reports/phase-4-project-completion-summary.md) - 性能测试结果
- [重构计划书](REFACTOR_PLAN.md) - 优化策略和实现

## 📈 文档更新记录

| 日期 | 文档 | 更新内容 |
|------|------|----------|
| 2025-07-07 | 全部文档 | 统一整理到docs文件夹，Phase报告移至reports子目录 |
| 2025-07-07 | [项目完成总结](reports/project-completion-summary.md) | 新增项目完成总结 |
| 2025-07-07 | [监控系统指南](monitoring-guide.md) | 新增监控功能完整指南 |
| 2025-07-07 | [看门狗机制指南](watchdog-guide.md) | 新增看门狗功能详细说明 |
| 2025-07-07 | [项目主页](../README.md) | 更新监控和性能信息 |

## 🔍 快速查找

### 常见问题
- **如何开始使用?** → [项目主页](../README.md)
- **API怎么调用?** → [API文档](API_DOCUMENTATION.md)
- **如何监控锁性能?** → [监控系统指南](monitoring-guide.md)
- **如何防止锁过期?** → [看门狗机制指南](watchdog-guide.md)
- **性能表现如何?** → [项目完成总结](reports/project-completion-summary.md)

### 技术细节
- **架构设计** → [项目完成总结](reports/project-completion-summary.md)
- **重构过程** → [重构计划书](REFACTOR_PLAN.md)
- **实现细节** → [AI开发日志](../AIs/)
- **测试覆盖** → [Phase 4.2完成报告](reports/phase-4.2-completion-report.md)

## 📞 获取帮助

如果您在使用过程中遇到问题：

1. **查阅文档**: 首先查看相关的使用指南
2. **查看示例**: 参考[使用示例](../examples/)中的代码
3. **检查日志**: 查看[AI开发日志](../AIs/)了解实现细节
4. **性能问题**: 参考[监控系统指南](monitoring-guide.md)进行诊断

## 🤝 贡献文档

欢迎为RLock文档做出贡献！

### 文档规范
- 使用Markdown格式
- 包含代码示例
- 提供清晰的使用说明
- 保持与代码同步更新

### 文档结构
```
docs/
├── README.md                    # 文档导航 (本文件)
├── API_DOCUMENTATION.md         # API参考文档
├── REFACTOR_PLAN.md            # 重构计划书
├── monitoring-guide.md         # 监控系统指南
├── watchdog-guide.md           # 看门狗机制指南
├── reports/                    # 项目报告目录
│   ├── INTERFACE_UNIFICATION_COMPLETION_REPORT.md  # Phase 1完成报告
│   ├── phase-4.2-completion-report.md              # Phase 4.2完成报告
│   └── project-completion-summary.md               # 项目完成总结
└── *.md                        # 其他技术文档
```

---

**RLock文档中心** - 让分布式锁更简单、更高效、更可靠！ 🚀

最后更新: 2025-07-07
