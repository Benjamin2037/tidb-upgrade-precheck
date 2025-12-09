# Deployment Guide

This document provides deployment guides for integrating `tidb-upgrade-precheck` with different cluster management tools.

## Overview

`tidb-upgrade-precheck` can be integrated with various cluster management tools to provide upgrade risk assessment capabilities. The deployment model varies depending on the target platform.

## Integration Options

### TiUP Integration

TiUP is the recommended deployment method for on-premises TiDB clusters. The precheck tool is packaged as a standalone TiUP component (`upgrade-precheck`) that is automatically installed and updated.

**Status**: ✅ Fully Supported

- **[TiUP Deployment Guide](./tiup_deployment.md)** - Complete deployment and packaging guide for TiUP integration

**Key Features:**
- Standalone TiUP component (`upgrade-precheck`)
- Automatic installation and updates with TiUP
- Complete knowledge base packaged with component
- Persistent runtime data storage (logs, user configurations)
- Integration with `tiup-cluster` for upgrade workflows

### TiDB Operator Integration

TiDB Operator integration allows the precheck tool to be used in Kubernetes environments for TiDB clusters managed by TiDB Operator.

**Status**: 🚧 Planned (TBD)

- **[TiDB Operator Deployment Guide](./tidb_operator_deployment.md)** - Deployment guide for TiDB Operator integration (Coming Soon)

**Planned Features:**
- Kubernetes-native deployment
- Integration with TiDB Operator upgrade workflows
- ConfigMap/Secret-based knowledge base management
- CRD-based precheck configuration

**Last Updated**: 2025

