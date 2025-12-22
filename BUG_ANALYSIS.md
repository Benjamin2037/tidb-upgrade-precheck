# Bug 分析：raftdb 参数在源版本知识库中存在但 GetSourceDefault 返回 nil

## 问题描述

以下参数在 v7.5.1 知识库中存在，但在报告中显示为"New"（`sourceDefault == nil`）：
- `raftdb.defaultcf.titan.min-blob-size`
- `raftdb.info-log-keep-log-file-num`
- `raftdb.info-log-level`
- `raftdb.info-log-max-size`

## 验证结果

1. **知识库文件验证**：这些参数在 `knowledge/v7.5/v7.5.1/tikv/defaults.json` 中确实存在
2. **参数值**：
   - `raftdb.defaultcf.titan.min-blob-size`: `{"value": "1KiB", "type": "string"}`
   - `raftdb.info-log-keep-log-file-num`: `{"value": 10, "type": "float"}`
   - `raftdb.info-log-level`: `{"value": "info", "type": "string"}`
   - `raftdb.info-log-max-size`: `{"value": "1GiB", "type": "string"}`

## 可能的原因

### 1. 知识库加载问题

在 `pkg/analyzer/analyzer.go` 的 `loadKBFromRequirements` 函数中：
- 第269-270行：直接将整个 `v`（即 `{"value": "1KiB", "type": "string"}`）存储到 `defaults[comp][k]` 中
- 这应该是正确的，因为 `extractValueFromDefault` 函数能够正确处理这种格式

### 2. GetSourceDefault 查找问题

在 `pkg/analyzer/rules/context.go` 的 `GetSourceDefault` 函数中：
- 第116-119行：从 `ctx.SourceDefaults[component][paramName]` 中查找参数
- 如果找不到，返回 `nil`

### 3. 参数名称匹配问题

可能的问题：
- 参数名称在知识库中存储的格式与查找时使用的格式不一致
- 大小写、连字符等问题

## 需要检查的点

1. **知识库加载时是否正确加载了这些参数**
   - 检查 `loadKBFromRequirements` 函数是否正确处理了所有参数
   - 检查是否有参数被过滤或跳过

2. **SourceDefaults 中是否包含这些参数**
   - 在运行时检查 `SourceDefaults["tikv"]` 中是否包含这些参数
   - 检查参数名称是否完全匹配

3. **extractValueFromDefault 是否正确处理**
   - 检查 `extractValueFromDefault` 函数是否能正确处理 `{"value": "1KiB", "type": "string"}` 格式

## 建议的修复方案

1. **添加调试日志**：在 `GetSourceDefault` 函数中添加日志，记录查找的参数名称和结果
2. **验证知识库加载**：在 `loadKBFromRequirements` 函数中添加日志，记录加载的参数数量
3. **检查参数名称**：确保参数名称在知识库和查找时完全一致

## 临时解决方案

如果确认这些参数在源版本知识库中存在，但 `GetSourceDefault` 返回 `nil`，可以考虑：
1. 在步骤1中，如果 `sourceDefault == nil` 但参数在目标版本中存在，检查源版本知识库文件是否真的包含该参数
2. 如果确认存在，记录警告日志，并继续处理（不标记为"New"）

