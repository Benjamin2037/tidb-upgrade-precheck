
# TiDB Upgrade Precheck Report Usage Guide

## Overview

- Supports automated risk scanning of TiDB cluster parameters, configuration, and compatibility before upgrade.
- Supports multiple report output formats: text, markdown, html.
- Can be invoked via both tiup and tidb-upgrade-precheck CLI, suitable for automation and manual review scenarios.

## tiup Usage Examples

### 1. Generate a risk report only (no upgrade)

```bash
tiup cluster upgrade precheck <cluster-name> <target-version> --precheck-output markdown --precheck-output-file report.md
```
- Generates a Markdown risk report and saves it as report.md.
- Use --precheck-output html to generate a web report.

### 2. Pre-upgrade check with manual confirmation

```bash
tiup cluster upgrade <cluster-name> <target-version> --precheck
```
- Outputs the risk report first, then proceeds with upgrade after user confirmation.

### 3. Skip risk check (not recommended)

```bash
tiup cluster upgrade <cluster-name> <target-version> --without-precheck
```

## tidb-upgrade-precheck CLI Example

```bash
precheck --snapshot examples/minimal_snapshot.json --report-format html --report-dir ./out
```
- Directly analyzes a snapshot file for risks and outputs an HTML report to the specified directory.

## Common Parameter Descriptions

- `--precheck-output`: Report format, supports text, markdown, html.
- `--precheck-output-file`: Report output file path.
- `--report-format`, `--report-dir`: For tidb-upgrade-precheck CLI only.

## Typical Scenarios

- Pre-upgrade risk assessment and archiving
- Integration into automated O&M workflows
- Change review and compliance traceability

---

For rule extension, custom report templates, or integration into your own system, please refer to the source code and README.
