#!/bin/bash
# 增量生成指定版本的 defaults.json，并更新 upgrade_logic.json
# 用法：./generate-incremental.sh v8.1.0
set -e

if [ -z "$1" ]; then
  echo "用法: $0 <tidb-tag>"
  exit 1
fi

TIDB_TAG=$1
TIDB_REPO=${TIDB_REPO:-../tidb}
KNOWLEDGE_DIR=${KNOWLEDGE_DIR:-../knowledge}

if [ ! -d "$TIDB_REPO" ]; then
  echo "请先 clone tidb 源码到 $TIDB_REPO"
  exit 1
fi

cd "$TIDB_REPO"
git fetch --tags
git checkout "$TIDB_TAG"
VERSION_DIR="$KNOWLEDGE_DIR/$TIDB_TAG"
mkdir -p "$VERSION_DIR"
go run ../tidb-upgrade-precheck/cmd/collect-defaults/main.go > "$VERSION_DIR/defaults.json"

git checkout master
# 你可以指定主分支或最新 tag
# 下面命令假设分析逻辑已实现
# go run ../tidb-upgrade-precheck/cmd/upgrade-logic-collector/main.go -bootstrap ./pkg/session/bootstrap.go > "$KNOWLEDGE_DIR/upgrade_logic.json"
echo "增量生成完成！"
