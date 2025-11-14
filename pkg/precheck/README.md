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
- `cmd/precheck` exposes a new `-upgrade-metadata` flag. Point it to TiDB's
  `tools/upgrade-metadata/upgrade_changes.json` and the additional rules become active.
- The `core.forced-global-sysvars` rule consults the embedded bootstrap mapping and
  lists every forced global system variable change across the upgrade window (reported
  as warnings, with optional hints surfaced as suggestions).
- The unit test `pkg/rules/forced_sysvars_test.go` contains a minimal fixture that can be
  extended as the rule evolves.

## Next steps for precheck
- Integrate the rule output with the final report and UI flows so DBAs can assess risk
  before executing upgrades.
- Expand the knowledge base to cover additional TiDB releases and keep the bootstrap
  mapping up to date.
- Consider adding informational hints for non-forced changes or correlating related
  configuration differences in future iterations.
