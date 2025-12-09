# TiUP 集成设计文档

## 概述

本文档详细说明了如何在 TiUP 中集成 [tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134) 功能。集成的目标是使 TiUP 用户能够在执行 TiDB 集群升级之前，利用 [tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134) 工具进行全面的兼容性检查。

## 设计原则

1. **库集成方式**：将[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)作为Go库集成到TiUP中，而不是作为一个独立的二进制工具调用。
2. **复用现有基础设施**：充分利用TiUP已有的集群连接和信息收集功能。
3. **无缝用户体验**：提供与TiUP现有命令风格一致的用户界面。
4. **模块化设计**：保持[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)的独立性，使其可以被其他工具集成。

## 集成架构

```
┌─────────────┐    ┌──────────────────────────┐    ┌──────────────────────┐
│             │    │                          │    │                      │
│    TiUP     │───▶│  tidb-upgrade-precheck   │───▶│   TiDB Cluster       │
│             │    │        (library)         │    │                      │
└─────────────┘    └──────────────────────────┘    └──────────────────────┘
                         │                                  │
                         ▼                                  ▼
               ┌──────────────────────┐         ┌─────────────────────────┐
               │   Knowledge Base     │         │   Runtime Information   │
               │ (parameters-history  │         │    (Configuration &     │
               │   upgrade_logic)     │         │      Variables)         │
               └──────────────────────┘         └─────────────────────────┘
```

## 实现细节

### 1. 依赖管理

在 TiUP 项目的 `go.mod` 文件中添加[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)作为依赖：

```go
require (
    github.com/pingcap/tidb-upgrade-precheck v0.1.0
    // ... 其他依赖
)
```

### 2. 命令行接口设计

在 TiUP 中添加一个新的子命令 `upgrade-precheck`：

```bash
tiup cluster upgrade-precheck <cluster-name> [flags]
```

支持的参数包括：
- `--target-version`: 目标升级版本
- `--format`: 报告格式 (text, json, markdown, html)
- `--output-dir`: 报告输出目录

### 3. 核心集成流程

#### 步骤1：获取集群信息
```go
// 使用 TiUP 现有的集群操作功能获取集群拓扑
topo := cluster.GetTopology(clusterName)
endpoints := convertToEndpoint(topo)
```

#### 步骤2：收集运行时配置
```go
// 利用 tidb-upgrade-precheck 的运行时收集功能
collector := runtime.NewCollector()
snapshot, err := collector.Collect(endpoints)
```

#### 步骤3：执行兼容性检查
```go
// 执行预检查分析
reportData := precheck.FromClusterSnapshot(snapshot)
```

#### 步骤4：生成报告
```go
// 生成并输出报告
generator := report.NewGenerator()
options := &report.Options{
    Format:    report.TextFormat,
    OutputDir: "./",
}
reportPath, err := generator.Generate(reportData, options)
```

### 4. 数据流映射

| TiUP 数据 | tidb-upgrade-precheck 映射 |
|-----------|-----------------------------|
| Cluster Topology | runtime.ClusterEndpoints |
| Connection Info | TiDBAddr, TiKVAddrs, PDAddrs |
| Cluster Name | 用于报告标识 |
| Version Info | SourceVersion, TargetVersion |

## 使用示例

### 基本用法
```bash
tiup cluster upgrade-precheck my-cluster --target-version v7.5.0
```

### 指定输出格式
```bash
tiup cluster upgrade-precheck my-cluster --target-version v7.5.0 --format json --output-dir /tmp
```

## 错误处理

集成应妥善处理以下类型的错误：

1. **集群连接错误**
   - TiDB/PD/TiKV节点无法访问
   - 认证失败
   
2. **数据收集错误**
   - 配置参数无法获取
   - 系统变量读取失败
   
3. **分析过程错误**
   - 版本信息无效
   - 内部处理异常
   
4. **报告生成错误**
   - 输出目录无权限
   - 磁盘空间不足

## 配置管理

TiUP 应将[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)的配置与其自身配置集成：

1. **连接配置**：复用TiUP的SSH密钥和连接设置
2. **超时配置**：使用TiUP全局超时设置
3. **日志配置**：集成到TiUP的日志系统中

## 测试策略

### 单元测试
1. 测试数据转换逻辑
2. 测试错误处理路径
3. 测试命令行参数解析

### 集成测试
1. 测试与真实集群的端到端流程
2. 测试各种报告格式的生成
3. 测试边界条件和异常情况

## 发布与维护

### 版本同步
- 定期更新[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)依赖版本
- 保持与TiDB版本发布节奏同步

### 向后兼容
- 确保API变更不会破坏现有集成
- 提供迁移指南和废弃警告

## 性能考虑

1. **并发收集**：并行收集多个组件的信息
2. **连接复用**：复用已有连接避免重复建立
3. **缓存机制**：对于大型集群，适当缓存部分不变信息
4. **资源限制**：控制并发数量避免对集群造成过大压力

## 安全考虑

1. **权限最小化**：只请求执行检查所需的最小权限
2. **数据保护**：敏感配置信息不应记录在日志中
3. **传输加密**：确保与集群组件通信的安全性
4. **输出审查**：报告中不应包含敏感信息

## 未来扩展

1. **增量检查**：支持只检查自上次检查以来的变更
2. **自定义规则**：允许用户添加自定义检查规则
3. **历史对比**：支持与历史检查结果进行对比
4. **自动化建议**：基于检查结果提供自动化的修复建议