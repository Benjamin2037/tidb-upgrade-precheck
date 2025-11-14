# Upgrade Metadata Notes

This document captures the current status of the bootstrap upgrade metadata work so that
`tidb-upgrade-precheck` can consume it consistently.

## What changed upstream (tidb)
- Added a helper (`registerGlobalSysVarChange`) in `pkg/session/upgrade.go` to register every
  bootstrap step that mutates `mysql.global_variables`.
- Populated the registry with entries covering versions 2 through 217; the generated
  metadata lives in `tools/upgrade-metadata/upgrade_changes.json` and is refreshed with
  `go generate ./pkg/session/upgradecatalog`.
- Extended `pkg/session/upgrade_test.go` with assertions that the registry entries have the
  expected defaults (scope, risk level, force flag, optional hints, etc.).

## Validating the metadata
Run the upstream session tests to make sure the registry remains consistent:

```bash
cd /path/to/tidb
go test ./pkg/session -count=0
```

Regenerate the JSON if you touch the registration list:

```bash
cd /path/to/tidb
go generate ./pkg/session/upgradecatalog
```

## Consuming the metadata in tidb-upgrade-precheck
- `cmd/precheck` 新增了 `-upgrade-metadata` 参数，指向 tidb 仓库生成的
  `tools/upgrade-metadata/upgrade_changes.json`，即可启用新的规则。
- 规则 `core.forced-global-sysvars` 会读取知识库中的 bootstrap 映射，并列出
  升级区间内所有强制性的全局系统变量变更（以 warning 呈现，可选提示会作为建议输出）。
- 单元测试 `pkg/rules/forced_sysvars_test.go` 提供了最小化样例，便于后续扩展。

## Next steps for precheck
- 将规则输出整合进最终报告/前端展示流程，帮助 DBA 提前评估风险。
- 根据需要扩展知识库覆盖更多 TiDB 版本（保证 bootstrap 映射最新）。
- 后续可以考虑对非强制变更给出提示级别的提醒或关联配置项差异分析。
