# TiUP Cluster Upgrade 命令集成设计

## 概述

本文档描述了如何在现有的 `tiup cluster upgrade` 命令中集成[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)功能，使得在执行 TiDB 集群升级之前自动进行兼容性检查。

## 设计目标

1. 保留现有的 `tiup cluster upgrade` 命令行为
2. 在升级前自动执行兼容性检查
3. 根据检查结果提供用户决策支持
4. 提供灵活的配置选项控制检查行为

## 集成方案

### 1. 命令行参数扩展

在现有的 `tiup cluster upgrade` 命令中增加以下参数：

```
--precheck                  执行兼容性检查并展示结果，然后询问用户是否继续升级
--without-precheck          跳过兼容性检查，直接执行升级
--precheck-only             仅执行预检查，不执行实际升级
--precheck-fail-severity    预检查发现问题时的失败级别 (默认: "error")
--precheck-format           预检查报告格式 (text,json,markdown,html)
--precheck-output-dir       预检查报告输出目录 (默认: "./")
```

### 2. 执行流程

```
┌─────────────────────────────────────────────┐
│        tiup cluster upgrade 命令           │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│           解析命令行参数                     │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│     是否指定 --without-precheck?           │
└─────────────────────────────────────────────┘
                    │ 是
            ┌───────▼───────┐
            │   执行升级     │
            └───────┬───────┘
                    │ 否
                    ▼
┌─────────────────────────────────────────────┐
│      是否指定 --precheck?                   │
└─────────────────────────────────────────────┘
        │ 否(默认情况)        │ 是
        ▼                    ▼
┌─────────────────────────────────────────────┐
│        收集集群运行时信息                    │
│  (使用TiUP现有集群连接基础设施)              │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│        调用 tidb-upgrade-precheck 库        │
│         执行兼容性分析                       │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│        生成并展示预检查报告                  │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│      根据检查结果和配置决定是否继续          │
│  (检查是否存在高于阈值的严重问题)            │
└─────────────────────────────────────────────┘
            │ 有问题且未强制忽略
     ┌──────▼──────┐
     │  提示用户   │
     │  是否继续?  │ ◄──┐
     └──────┬──────┘    │
            │ 是       │
    ┌───────▼──────┐   │
    │   执行升级    │   │
    └───────┬──────┘   │
            │ 否       │
      ┌─────▼─────┐    │
      │   退出     │ ───┘
      └───────────┘
```

### 3. 核心实现逻辑

#### 3.1 集群信息收集

利用 TiUP 现有的集群连接和信息收集能力：

```go
// 使用 TiUP 现有的集群操作接口
clusterInst, err := cc.GetClusterManager().GetCluster(clusterName)
if err != nil {
    return err
}

// 获取集群拓扑信息
topo := clusterInst.Topology

// 构建 tidb-upgrade-precheck 所需的端点信息
endpoints := runtime.ClusterEndpoints{
    TiDBAddr:  getTiDBAddress(topo),
    TiKVAddrs: getTiKVAddresses(topo),
    PDAddrs:   getPDAddresses(topo),
}
```

#### 3.2 调用预检查库

```go
// 初始化运行时收集器
collector := runtime.NewCollector()

// 收集集群快照
snapshot, err := collector.Collect(endpoints)
if err != nil {
    return fmt.Errorf("收集集群信息失败: %v", err)
}

// 设置版本信息
snapshot.SourceVersion = currentVersion
snapshot.TargetVersion = targetVersion

// 执行预检查分析
reportData := precheck.FromClusterSnapshot(snapshot)
```

#### 3.3 报告生成与展示

```go
// 生成报告
generator := report.NewGenerator()
options := &report.Options{
    Format:    report.Format(precheckFormat),
    OutputDir: precheckOutputDir,
}

reportPath, err := generator.Generate(reportData, options)
if err != nil {
    return fmt.Errorf("生成预检查报告失败: %v", err)
}

// 展示报告摘要到控制台
printReportSummary(reportData)

// 如果启用了 --precheck-only，则在此处退出
if precheckOnly {
    fmt.Printf("预检查完成，报告已保存至: %s\n", reportPath)
    return nil
}
```

#### 3.4 决策逻辑

```go
// 检查是否有超过阈值的问题
hasBlockingIssues := hasIssuesAboveSeverity(reportData, precheckFailSeverity)
if hasBlockingIssues {
    if forceUpgrade {
        fmt.Println("警告: 检测到严重问题，但由于使用了 --force 参数将继续升级")
    } else {
        // 询问用户是否继续
        continueUpgrade := askUserConfirmation()
        if !continueUpgrade {
            fmt.Println("升级已取消")
            return nil
        }
    }
}
```

### 4. 用户体验

#### 4.1 默认行为

默认情况下，在执行升级前会自动运行预检查并询问用户：

```bash
tiup cluster upgrade my-cluster v7.5.0
# 自动执行预检查，展示结果并询问用户是否继续升级
```

#### 4.2 明确执行预检查

用户可以明确指定执行预检查：

```bash
tiup cluster upgrade my-cluster v7.5.0 --precheck
# 明确执行预检查并询问是否继续升级
```

#### 4.3 跳过预检查

用户也可以选择跳过预检查直接升级：

```bash
tiup cluster upgrade my-cluster v7.5.0 --without-precheck
# 直接执行升级，跳过预检查
```

#### 4.4 仅执行预检查

用户可以选择只执行预检查而不进行升级：

```bash
tiup cluster upgrade my-cluster v7.5.0 --precheck-only
# 只执行预检查并将报告保存到指定目录
```

#### 4.5 控制失败阈值

用户可以控制什么级别的问题会导致升级中断：

```bash
tiup cluster upgrade my-cluster v7.5.0 --precheck-fail-severity warning
# 当检测到 warning 级别及以上问题时中断升级
```

### 5. 错误处理

#### 5.1 预检查执行失败

如果预检查本身执行失败（如无法连接到集群），根据错误严重程度决定是否继续升级：

```bash
tiup cluster upgrade my-cluster v7.5.0 --precheck-strict=false
# 预检查失败时仍然尝试继续升级
```

#### 5.2 报告生成失败

如果预检查成功但报告生成失败，应该继续升级流程并给出警告：

```go
if err := generateReport(...); err != nil {
    fmt.Printf("警告: 生成预检查报告失败: %v\n", err)
    fmt.Println("将继续执行升级...")
}
```

### 6. 配置选项详解

| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `--precheck` | boolean | false | 明确执行预检查并询问用户是否继续升级 |
| `--without-precheck` | boolean | false | 跳过预检查直接执行升级 |
| `--precheck-only` | boolean | false | 是否只执行预检查，不执行升级 |
| `--precheck-fail-severity` | string | "error" | 预检查发现问题时的失败级别 ("info", "warning", "error") |
| `--precheck-format` | string | "text" | 预检查报告格式 ("text", "json", "markdown", "html") |
| `--precheck-output-dir` | string | "./" | 预检查报告输出目录 |
| `--precheck-strict` | boolean | true | 预检查执行失败时是否中断升级 |

### 7. 实施步骤

#### 7.1 第一阶段：基础集成

1. 在 `tiup` 项目中添加[tidb-upgrade-precheck](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/precheck/analyzer.go#L134-L134)依赖
2. 扩展 `upgrade` 命令的参数
3. 实现基础的预检查调用逻辑
4. 添加简单的报告展示功能

#### 7.2 第二阶段：用户体验优化

1. 实现丰富的报告展示功能
2. 添加用户交互确认机制
3. 优化错误处理和日志输出
4. 添加更多配置选项

#### 7.3 第三阶段：高级功能

1. 实现增量检查功能
2. 添加历史对比功能
3. 集成自动化建议功能
4. 支持自定义检查规则

### 8. 注意事项

1. **向后兼容性**：确保默认行为不会影响现有用户的使用习惯
2. **性能影响**：预检查不应显著增加升级命令的执行时间
3. **网络依赖**：在网络不稳定的环境中应有适当的重试机制
4. **权限要求**：预检查所需权限应与升级命令保持一致
5. **错误恢复**：如果预检查失败，应能够安全地继续升级流程

### 9. 测试策略

#### 9.1 单元测试

1. 测试参数解析逻辑
2. 测试决策逻辑（不同严重级别的处理）
3. 测试报告生成功能

#### 9.2 集成测试

1. 测试与真实集群的端到端流程
2. 测试各种异常场景（网络故障、权限不足等）
3. 测试不同配置选项的行为

#### 9.3 回归测试

1. 确保现有升级功能不受影响
2. 验证与不同版本 TiDB 集群的兼容性