# TiUP Implementation Steps for tidb-upgrade-precheck Integration

## 概述

本文档提供了在 TiUP 项目中实现 tidb-upgrade-precheck 集成的具体步骤。按照这些步骤，开发者可以在 TiUP 中实现预检查功能。

## 实施前准备

1. 确保你有 TiUP 项目的访问权限
2. 确保你已经熟悉 TiUP 的代码结构
3. 确保 tidb-upgrade-precheck 项目已经发布并且可以作为 Go 模块使用

## 具体实施步骤

### 步骤 1: 添加依赖

1. 进入 TiUP 项目根目录
2. 运行以下命令添加 tidb-upgrade-precheck 依赖：
   ```bash
   go get github.com/pingcap/tidb-upgrade-precheck@latest
   ```
   
   或者手动编辑 `go.mod` 文件，添加以下内容：
   ```go
   require (
       github.com/pingcap/tidb-upgrade-precheck v0.1.0
       // ... 其他依赖
   )
   ```

3. 运行 `go mod tidy` 更新依赖

### 步骤 2: 扩展命令行参数

1. 找到 TiUP 中 `upgrade` 命令的实现文件，通常位于 `cmd/cluster/command/upgrade.go`
2. 在命令定义部分添加新的参数：
   ```go
   // 添加预检查相关参数
   cmd.Flags().Bool("precheck", false, "Perform compatibility check and ask user for confirmation")
   cmd.Flags().Bool("without-precheck", false, "Skip compatibility check and proceed directly to upgrade")
   ```

### 步骤 3: 实现预检查逻辑

1. 在 `upgrade` 命令的执行函数中添加预检查逻辑：
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

### 步骤 4: 实现 runPrecheck 函数

1. 创建 `runPrecheck` 函数来执行预检查：
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

### 步骤 5: 实现辅助函数

1. 实现将 TiUP 拓扑转换为 endpoints 的函数：
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
   ```

2. 实现获取当前集群版本的函数：
   ```go
   // 获取当前集群版本
   func getCurrentVersion(topo *spec.Specification) string {
       // 实现获取当前版本的逻辑
       // 这可能涉及查询集群或检查组件版本
       return "unknown"
   }
   ```

3. 实现用户确认函数：
   ```go
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

### 步骤 6: 错误处理和日志记录

1. 确保在整个预检查过程中有适当的错误处理和日志记录：
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

### 步骤 7: 测试实现

1. 为新函数创建单元测试：
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

2. 创建集成测试以验证与真实集群的端到端流程：
   ```go
   func TestPrecheckIntegration(t *testing.T) {
       // 设置测试集群
       // 执行预检查
       // 验证结果
   }
   ```

## 验证实施结果

1. 编译 TiUP 项目：
   ```bash
   make
   ```

2. 测试新添加的命令行参数：
   ```bash
   ./bin/tiup-cluster upgrade --help
   ```

3. 在测试集群上运行预检查：
   ```bash
   ./bin/tiup-cluster upgrade my-cluster v7.5.0 --precheck
   ```

## 注意事项

1. **向后兼容性**：确保新功能不会影响现有 TiUP 用户的使用体验
2. **性能考虑**：预检查不应显著增加升级命令的执行时间
3. **网络稳定性**：在网络不稳定的环境中应有适当的重试机制
4. **权限要求**：预检查所需的权限应与升级命令保持一致
5. **安全性**：密码等敏感信息不应出现在日志或错误消息中

## 故障排除

1. 如果遇到依赖问题，尝试运行 `go mod tidy`
2. 如果遇到编译错误，检查导入路径是否正确
3. 如果遇到运行时错误，检查日志输出以定位问题

## 结论

通过以上步骤，你可以在 TiUP 中成功集成 tidb-upgrade-precheck 功能。这种集成将为 TiUP 用户提供在升级 TiDB 集群之前进行兼容性检查的能力，有助于降低升级风险并提高系统的稳定性。