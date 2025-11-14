# tidb-upgrade-precheck

`tidb-upgrade-precheck` 提供一个轻量的 Go 库和 CLI，用于在执行 TiDB 集群升级前统一做参数与配置校验。该项目旨在被 TiUP CLI 与 TiOperator 共用，从而共享同一套规则和风险模型。

## 主要特性

- **统一抽象**：定义 `Snapshot`、`Rule`、`Report` 等核心数据结构，适配不同上层组件的输入输出。
- **可扩展的规则体系**：内置基础校验规则，可按需注册自定义规则或加载外部知识库（如 `paramguard` 生成的 JSON）。
- **结构化报告**：将检查结果输出为结构化的 `Report`，便于终端展示、API 返回或落库审计。
- **示例 CLI**：`cmd/precheck` 提供参考实现，可直接读取快照 JSON 运行所有规则。

## 快速开始

```bash
# 安装 CLI
go install ./cmd/precheck

# 运行示例
precheck --snapshot examples/minimal_snapshot.json
```

## 项目结构

```
cmd/precheck        # 参考 CLI
pkg/precheck        # 核心引擎与公共类型
pkg/rules           # 内置规则实现
examples            # 示例输入
```

## 未来计划

- 接入 `paramguard` 生成的知识库，自动化加载版本化的强制修改与默认值。
- 提供 Kubernetes 适配器，支持 TiOperator 内嵌调用。
- 扩充规则体系（硬件、依赖项、配置冲突等）。

欢迎提交 Issue 或 PR 共同完善。