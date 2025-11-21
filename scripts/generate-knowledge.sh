#!/bin/bash
# 全量生成所有 patch 版本的 defaults.json 和 upgrade_logic.json
# 用法：在 tidb-upgrade-precheck/scripts 目录下执行
set -e

TIDB_REPO=${TIDB_REPO:-../tidb}
KNOWLEDGE_DIR=${KNOWLEDGE_DIR:-../knowledge}

if [ ! -d "$TIDB_REPO" ]; then
  echo "请先 clone tidb 源码到 $TIDB_REPO"
  exit 1
fi

mkdir -p "$KNOWLEDGE_DIR"
cd "$TIDB_REPO"

tags=$(git tag --list 'v*' | sort -V)
for tag in $tags; do
  echo "[全量] 处理 $tag ..."
  git checkout "$tag"
  VERSION_DIR="$KNOWLEDGE_DIR/$tag"
  mkdir -p "$VERSION_DIR"
  go run ../tidb-upgrade-precheck/cmd/collect-defaults/main.go > "$VERSION_DIR/defaults.json"
done

echo "[全量] 生成全局 upgrade_logic.json ..."
git checkout master
# 你可以指定主分支或最新 tag
# 下面命令假设分析逻辑已实现
# go run ../tidb-upgrade-precheck/cmd/upgrade-logic-collector/main.go -bootstrap ./pkg/session/bootstrap.go > "$KNOWLEDGE_DIR/upgrade_logic.json"
echo "全量生成完成！"
