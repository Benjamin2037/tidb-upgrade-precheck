de# TiDB 参数采集详细设计文档

## 1. 概述

本文档详细描述了 TiDB 升级预检查工具中的参数采集模块设计与实现。该模块负责从不同版本的 TiDB 源代码中提取系统变量和配置参数的默认值，为升级风险评估提供基础数据。

## 2. 设计目标

1. **多版本兼容性**：支持从 TiDB v6.x 到最新版本的参数采集
2. **零侵入性**：不修改 TiDB 源代码，通过外部工具采集参数
3. **准确性**：确保采集到的参数值与实际版本完全一致
4. **可扩展性**：易于添加对新版本的支持
5. **效率性**：避免重复采集已处理过的版本

## 3. 核心组件

### 3.1 参数采集工具文件

为了支持不同版本的 TiDB 源码结构差异，我们提供了多个版本特定的采集工具：

- `export_defaults.go` - 适用于最新版本（使用 pkg 目录结构）
- `export_defaults_v6.go` - 适用于 v6.x 版本
- `export_defaults_v71.go` - 适用于 v7.0 - v7.4 版本
- `export_defaults_v75plus.go` - 适用于 v7.5+ 和 v8.x 版本
- `export_defaults_legacy.go` - 适用于更早版本（无 pkg 目录）

这些工具文件位于 `tools/upgrade-precheck/` 目录下。

### 3.2 版本路由机制

根据目标版本自动选择合适的采集工具：

```go
func selectToolByVersion(tag string) string {
    version := strings.TrimPrefix(tag, "v")
    parts := strings.Split(version, ".")
    
    if len(parts) < 2 {
        return "export_defaults.go"
    }
    
    major, _ := strconv.Atoi(parts[0])
    minor, _ := strconv.Atoi(parts[1])
    
    switch {
    case major == 6:
        return "export_defaults_v6.go"
    case major == 7:
        if minor < 5 {
            return "export_defaults_v71.go"
        } else {
            return "export_defaults_v75plus.go"
        }
    case major >= 8:
        return "export_defaults_v75plus.go"
    default:
        return "export_defaults.go"
    }
}
```

### 3.3 版本管理机制

为了避免重复采集相同版本，系统实现了版本管理机制：

- 使用 `knowledge/generated_versions.json` 文件记录已采集的版本
- 每次采集前检查版本是否已存在
- 支持强制重新采集所有版本

## 4. 采集流程

### 4.1 单版本采集流程

1. 接收目标版本标签和 TiDB 源码路径
2. 检查该版本是否已采集过
3. 创建临时目录用于克隆源码
4. 克隆 TiDB 源码到临时目录
5. 切换到目标版本标签
6. 根据版本选择合适的采集工具
7. 将采集工具复制到临时目录
8. 运行采集工具导出参数
9. 将结果保存到 `knowledge/<version>/defaults.json`
10. 记录版本信息到版本管理文件

### 4.2 全量采集流程

1. 获取所有 LTS 版本标签
2. 对每个版本执行单版本采集流程
3. 聚合所有版本的参数历史
4. 生成全局参数历史文件 `knowledge/tidb/parameters-history.json`

### 4.3 增量采集流程

1. 根据起始和结束版本确定版本范围
2. 对范围内的每个版本执行单版本采集流程

## 5. 输出格式

### 5.1 defaults.json

每个版本的参数默认值保存在 `knowledge/<version>/defaults.json` 文件中：

```json
{
  "sysvars": {
    "variable_name": "default_value",
    ...
  },
  "config": {
    "config_item": "default_value",
    ...
  },
  "bootstrap_version": 0
}
```

### 5.2 parameters-history.json

聚合所有版本的参数历史保存在 `knowledge/tidb/parameters-history.json` 文件中：

```json
{
  "component": "tidb",
  "parameters": [
    {
      "name": "parameter_name",
      "type": "parameter_type",
      "history": [
        {
          "version": 93,
          "default": "value",
          "scope": "unknown",
          "description": "unknown",
          "dynamic": false
        },
        ...
      ]
    },
    ...
  ]
}
```

## 6. 技术实现细节

### 6.1 临时克隆机制

为了避免工作区状态干扰，所有采集操作都在临时克隆的仓库中进行：

```go
tempDir, err := ioutil.TempDir("", "tidb_upgrade_precheck")
if err != nil {
    return fmt.Errorf("failed to create temp directory: %v", err)
}
defer os.RemoveAll(tempDir)
```

### 6.2 动态导入机制

通过 Go 的运行时导入机制获取参数默认值：

```go
// Collect config default values
cfg := config.GetGlobalConfig()
cfgMap := make(map[string]interface{})
data, _ := json.Marshal(cfg)
json.Unmarshal(data, &cfgMap)

// Collect all user-visible sysvars
sysvars := make(map[string]interface{})
for _, sv := range variable.GetSysVars() {
    if sv.Hidden || sv.Scope == variable.ScopeNone {
        continue
    }
    sysvars[sv.Name] = sv.Value
}
```

## 7. 使用指南

### 7.1 环境准备

确保已安装以下依赖：

- Go 1.18+
- Git
- TiDB 源码（默认位于 `../tidb`）

### 7.2 全量采集

```bash
# 采集所有未采集的 LTS 版本
make collect

# 或者
go run cmd/kb-generator/main.go --all
```

### 7.3 采集所有版本（包括已采集的）

```bash
# 采集所有 LTS 版本，包括已采集的版本
make collect-all

# 或者
go run cmd/kb-generator/main.go --all --skip-generated=false
```

### 7.4 增量采集

```bash
# 采集指定版本范围
go run cmd/kb-generator/main.go --from-tag=v7.5.0 --to-tag=v8.1.0
```

### 7.5 参数历史聚合

```bash
# 聚合所有版本的参数历史
make aggregate

# 或者
go run cmd/kb-generator/main.go --aggregate
```

### 7.6 清理采集记录

```bash
# 清理版本采集记录
make clean-generated

# 或者手动删除
rm knowledge/generated_versions.json
```

## 8. 故障处理

### 8.1 版本采集失败

如果某个版本采集失败，系统会记录警告并继续处理其他版本，不会中断整个采集过程。

### 8.2 重复采集

通过版本管理机制避免重复采集，提高效率。

### 8.3 路径问题

确保 TiDB 源码路径正确，工具会验证路径有效性。

## 9. 扩展性考虑

### 9.1 添加新版本支持

1. 创建新的版本特定采集工具文件
2. 更新版本路由逻辑
3. 测试新版本采集功能

### 9.2 并行采集

未来可以考虑实现并行采集以提高性能。

### 9.3 更多元数据采集

可以扩展工具以采集参数的更多元数据，如描述、作用域等。