# 参数当前值与默认值差异的原因说明

## 问题描述

在升级预检查报告中，您可能会发现某些参数的**当前值**与**源版本默认值**不一致，即使您没有手动修改过这些参数。本文档解释出现这种情况的常见原因。

## 主要原因

### 1. **TiKV 自动调优（Auto-Tune）**

TiKV 会根据系统资源（特别是 CPU 核心数）自动调整某些参数，这些参数不是用户配置的，而是 TiKV 在运行时根据系统环境自动计算的。

**典型参数：**
- `backup.auto-tune-remain-threads`: 根据 CPU 核心数自动调整备份线程数
- `backup.num-threads`: 可能根据系统资源自动调整
- 其他以 `auto-tune` 开头的参数

**示例场景：**
- **知识库生成环境**：在 macOS ARM64 上使用 `tiup playground` 生成，系统有 8 个 CPU 核心
  - 默认值：`backup.auto-tune-remain-threads = 2`
  
- **实际运行环境**：在 Linux AMD64 上运行，系统有 4 个 CPU 核心
  - 当前值：`backup.auto-tune-remain-threads = 1`（TiKV 根据 4 核自动调整为 1）

**这是正常行为**，TiKV 会根据实际系统资源自动优化参数，无需用户干预。

### 2. **知识库生成环境 vs 实际运行环境不同**

知识库通常在开发/测试环境（如 macOS）中生成，而实际集群在生产环境（如 Linux）中运行，环境差异可能导致参数值不同。

**环境差异包括：**
- **操作系统**：macOS vs Linux
- **CPU 架构**：ARM64 vs AMD64
- **系统资源**：CPU 核心数、内存大小不同
- **部署方式**：`tiup playground` vs `tiup cluster`

**示例：**
- `version_compile_machine`: 知识库中可能是 `arm64`，实际运行环境是 `amd64`
- `version_compile_os`: 知识库中可能是 `darwin`，实际运行环境是 `linux`

这些参数反映的是**编译时的平台信息**，不是用户配置，已被自动过滤。

### 3. **部署工具自动调整**

TiUP 等部署工具可能会根据系统资源自动调整某些参数，以确保集群在给定硬件上正常运行。

**可能被自动调整的参数：**
- 内存相关参数（根据系统内存大小）
- 线程数相关参数（根据 CPU 核心数）
- 存储相关参数（根据磁盘容量）

### 4. **路径相关参数**

部署路径相关的参数会因为部署环境不同而不同，这些差异是正常的。

**典型参数：**
- `data-dir`: 数据目录路径
- `log-file`: 日志文件路径
- `deploy-dir`: 部署目录路径

这些参数已被自动过滤，不会出现在报告中。

## 如何处理

### 对于自动调优参数

如果参数名称包含 `auto-tune` 或 `auto_tune`，且：
- 当前值 ≠ 源默认值
- 但源默认值 = 目标默认值

这通常表示 TiKV 根据系统资源自动调整了参数，**这是正常行为，无需处理**。

### 对于环境相关参数

如果参数反映的是环境信息（如编译平台、路径等），这些差异是正常的，已被自动过滤。

### 对于其他参数

如果参数既不是自动调优参数，也不是环境相关参数，且当前值 ≠ 源默认值，建议：

1. **检查是否在配置文件中显式设置了该参数**
2. **检查部署工具是否自动调整了该参数**
3. **确认该参数是否会影响升级后的行为**

## 代码中的处理

在 `rule_tikv_consistency.go` 中，对于源默认值 = 目标默认值但当前值不同的情况，会添加以下提示：

```
Note: Source and target defaults are the same. The current value may be auto-tuned by TiKV based on system resources (e.g., CPU cores).
```

这帮助用户理解差异可能是由自动调优引起的。

## 建议

1. **自动调优参数**：如果源默认值 = 目标默认值，当前值的差异通常是由自动调优引起的，可以忽略
2. **环境相关参数**：已被自动过滤，不会出现在报告中
3. **其他参数**：如果源默认值 ≠ 目标默认值，需要关注升级后的默认值变化

## 相关代码

- `pkg/analyzer/rules/rule_tikv_consistency.go`: TiKV 一致性检查规则
- `pkg/analyzer/rules/rule_user_modified_params.go`: 用户修改参数检测规则
- `pkg/analyzer/rules/rule_upgrade_differences.go`: 升级差异检测规则

