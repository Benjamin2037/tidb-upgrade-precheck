#!/bin/bash
# Fully generate defaults.json and upgrade_logic.json for all patch versions
# Usage: Execute in tidb-upgrade-precheck/scripts directory
set -e

TIDB_REPO=${TIDB_REPO:-../tidb}
KNOWLEDGE_DIR=${KNOWLEDGE_DIR:-../knowledge}

if [ ! -d "$TIDB_REPO" ]; then
  echo "Please clone tidb source code to $TIDB_REPO"
  exit 1
fi

mkdir -p "$KNOWLEDGE_DIR"

# Use the new kb-generator tool for full collection
cd ..
go run cmd/kb-generator/main.go --all --repo="$TIDB_REPO" --out="$KNOWLEDGE_DIR"

echo "Full generation completed!"