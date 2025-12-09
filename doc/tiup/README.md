# TiUP Integration

This directory contains design and implementation resources for integrating tidb-upgrade-precheck with TiUP.

## Overview

TiUP integration allows users to perform compatibility checks before upgrading TiDB clusters directly through TiUP commands. The integration supports two usage modes:

1. **Standalone Precheck Command**: `tiup upgrade-precheck <cluster-name> <version>`
2. **Integrated into Upgrade Command**: `tiup cluster upgrade <cluster-name> <version> --precheck`

## Quick Reference

- **[TiUP Integration and Deployment Guide](../tiup_deployment.md)** - Complete integration, deployment and packaging guide
- **[TiUP Implementation Guide](./tiup_implementation_guide.md)** - Step-by-step implementation instructions

## Related Documents

- [Deployment Guide](../deployment.md) - Overview of all deployment options
- [System Design Overview](../design.md) - High-level system architecture

---

**Last Updated**: 2025
