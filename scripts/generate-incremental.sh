#!/bin/bash
# Incrementally generate defaults.json for specified version and update upgrade_logic.json
# Usage: ./generate-incremental.sh v7.5.0 v8.1.0
set -e

if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Usage: $0 <from-tidb-tag> <to-tidb-tag>"
  echo "Example: $0 v7.5.0 v8.1.0"
  exit 1
fi

FROM_TAG=$1
TO_TAG=$2
TIDB_REPO=${TIDB_REPO:-../tidb}
KNOWLEDGE_DIR=${KNOWLEDGE_DIR:-../knowledge}

if [ ! -d "$TIDB_REPO" ]; then
  echo "Please clone tidb source code to $TIDB_REPO"
  exit 1
fi

# Use the new kb-generator tool for incremental collection
cd ..
go run cmd/kb-generator/main.go --from-tag="$FROM_TAG" --to-tag="$TO_TAG" --repo="$TIDB_REPO" --out="$KNOWLEDGE_DIR"

echo "Incremental generation completed!"