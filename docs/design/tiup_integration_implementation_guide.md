# TiUP Integration Implementation Guide

## 概述

本文档提供了在 TiUP 中集成 tidb-upgrade-precheck 功能的详细实施指南。该指南旨在帮助开发者在 TiUP 项目中实现预检查功能，以便在升级 TiDB 集群之前自动执行兼容性检查。

## 集成架构

TiUP 将通过 Go 模块依赖的方式集成 tidb-upgrade-precheck 库，而不是通过外部命令调用。这种集成方式具有以下优势：

1. 更好的性能（避免进程间通信开销）
2. 更紧密的集成（可以直接使用 TiUP 的连接和认证信息）
3. 更好的错误处理（统一的错误处理机制）
4. 更好的用户体验（一致的命令行接口）

## 实施步骤

### 1. 添加依赖

首先，在 TiUP 项目的 `go.mod` 文件中添加 tidb-upgrade-precheck 依赖：

```bash
go get github.com/pingcap/tidb-upgrade-precheck@latest
```

或者直接在 `go.mod` 文件中添加：

```go
require (
    github.com/pingcap/tidb-upgrade-precheck v0.1.0
    // ... 其他依赖
)
```

### 2. 扩展命令行参数

在 TiUP 的 `upgrade` 命令中添加预检查相关的参数。通常在 `cmd/cluster/command/upgrade.go` 文件中：

```go
// 添加预检查相关参数
cmd.Flags().Bool("precheck", false, "Perform compatibility check and ask user for confirmation")
cmd.Flags().Bool("without-precheck", false, "Skip compatibility check and proceed directly to upgrade")
```

### 3. 实现预检查逻辑

在 `upgrade` 命令的执行函数中添加预检查逻辑。通常在 `cmd/cluster/command/upgrade.go` 文件中：

```go
import (
    "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
)

func upgradeCluster(clusterName, version string, opt operator.Options) error {
    // 获取集群信息
    clusterInst, err := cc.GetClusterManager().GetCluster(clusterName)
    if err != nil {
        return err
    }
    
    topo := clusterInst.Topology
    
    // 检查是否跳过预检查
    withoutPrecheck, err := cmd.Flags().GetBool("without-precheck")
    if err != nil {
        return err
    }
    
    // 检查是否明确要求执行预检查
    explicitPrecheck, err := cmd.Flags().GetBool("precheck")
    if err != nil {
        return err
    }
    
    // 如果明确要求执行预检查或者使用默认行为（既没有明确要求也不跳过）
    if explicitPrecheck || (!explicitPrecheck && !withoutPrecheck) {
        // 执行预检查
        if err := runPrecheck(topo, clusterName, version); err != nil {
            return fmt.Errorf("precheck failed: %v", err)
        }
        
        // 如果不是明确要求执行预检查，则询问用户是否继续
        if !explicitPrecheck {
            if !askUserConfirmation() {
                fmt.Println("Upgrade cancelled by user.")
                return nil
            }
        }
    }
    
    // 继续执行现有的升级逻辑
    // ... 现有的升级代码 ...
    return nil
}
```

### 4. 实现 runPrecheck 函数

创建一个函数来执行预检查。可以在 `cmd/cluster/command/upgrade.go` 或者创建一个新的文件如 `cmd/cluster/command/precheck.go`：

```go
func runPrecheck(topo *spec.Specification, clusterName, targetVersion string) error {
    // 将 TiUP 拓扑转换为 endpoints 格式
    endpoints := convertToEndpoint(topo)
    
    // 初始化收集器
    collector := runtime.NewCollector()
    
    // 收集集群快照
    snapshot, err := collector.Collect(endpoints)
    if err != nil {
        return fmt.Errorf("failed to collect cluster information: %v", err)
    }
    
    // 设置版本信息
    snapshot.SourceVersion = getCurrentVersion(topo) // 实现此函数以获取当前版本
    snapshot.TargetVersion = targetVersion
    
    // 运行预检查分析
    reportData := precheck.FromClusterSnapshot(snapshot)
    
    // 生成报告
    generator := report.NewGenerator()
    options := &report.Options{
        Format:    report.TextFormat,
        OutputDir: "./",
    }
    
    reportPath, err := generator.Generate(reportData, options)
    if err != nil {
        return fmt.Errorf("failed to generate precheck report: %v", err)
    }
    
    fmt.Printf("Precheck report generated: %s\n", reportPath)
    
    return nil
}
```

### 5. 实现辅助函数

实现必要的辅助函数：

```go
// 将 TiUP 拓扑转换为 endpoints
func convertToEndpoint(topo *spec.Specification) runtime.ClusterEndpoints {
    endpoints := runtime.ClusterEndpoints{}
    
    // 获取 TiDB 地址
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentTiDB {
            inst := comp.Instances()[0] // 简化处理，获取第一个实例
            endpoints.TiDBAddr = fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort())
            break
        }
    }
    
    // 获取 TiKV 地址
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentTiKV {
            for _, inst := range comp.Instances() {
                endpoints.TiKVAddrs = append(endpoints.TiKVAddrs, 
                    fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()))
            }
        }
    }
    
    // 获取 PD 地址
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentPD {
            for _, inst := range comp.Instances() {
                endpoints.PDAddrs = append(endpoints.PDAddrs, 
                    fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()))
            }
        }
    }
    
    return endpoints
}

// 获取当前集群版本
func getCurrentVersion(topo *spec.Specification) string {
    // 实现获取当前版本的逻辑
    // 这可能涉及查询集群或检查组件版本
    return "unknown"
}

// 询问用户确认
func askUserConfirmation() bool {
    reader := bufio.NewReader(os.Stdin)
    fmt.Print("Precheck completed. Do you want to continue with the upgrade? (Y/n): ")
    response, err := reader.ReadString('\n')
    if err != nil {
        return false
    }
    
    response = strings.ToLower(strings.TrimSpace(response))
    return response == "y" || response == "yes" || response == ""
}
```

### 6. 错误处理和日志记录

确保在整个预检查过程中有适当的错误处理和日志记录：

```go
func runPrecheckWithLogging(...) error {
    log.Infof("Starting precheck for cluster upgrade from %s to %s", 
        snapshot.SourceVersion, snapshot.TargetVersion)
    
    // ... 预检查逻辑 ...
    
    if err != nil {
        log.Errorf("Precheck failed: %v", err)
        return err
    }
    
    log.Infof("Precheck completed successfully")
    return nil
}
```

## 测试考虑

### 单元测试

为新函数创建单元测试：

```go
func TestConvertToEndpoint(t *testing.T) {
    // 创建模拟拓扑
    topo := createMockTopology()
    
    // 转换为 endpoints
    endpoints := convertToEndpoint(topo)
    
    // 验证结果
    assert.NotEmpty(t, endpoints.TiDBAddr)
    assert.NotEmpty(t, endpoints.TiKVAddrs)
    assert.NotEmpty(t, endpoints.PDAddrs)
}
```

### 集成测试

创建集成测试以验证与真实集群的端到端流程：

```go
func TestPrecheckIntegration(t *testing.T) {
    // 设置测试集群
    // 执行预检查
    // 验证结果
}
```

## 注意事项

1. **向后兼容性**：确保新功能不会影响现有 TiUP 用户的使用体验
2. **性能考虑**：预检查不应显著增加升级命令的执行时间
3. **网络稳定性**：在网络不稳定的环境中应有适当的重试机制
4. **权限要求**：预检查所需的权限应与升级命令保持一致
5. **安全性**：密码等敏感信息不应出现在日志或错误消息中

## 未来扩展

1. 支持更多的报告格式（HTML、JSON等）
2. 支持自定义检查规则
3. 支持增量检查功能
4. 添加历史对比功能
5. 集成自动化建议功能

## 结论

通过以上步骤，可以在 TiUP 中成功集成 tidb-upgrade-precheck 功能。这种集成将为 TiUP 用户提供在升级 TiDB 集群之前进行兼容性检查的能力，有助于降低升级风险并提高系统的稳定性。