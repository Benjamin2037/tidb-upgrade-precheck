# TiUP Cluster Upgrade E2E 测试计划（手动执行）

本文档提供详细的端到端测试计划，使用真实的 `tiup cluster upgrade` 命令测试完整的升级场景。

## 测试目标

1. 验证 `tiup cluster upgrade --precheck` 命令正常工作
2. 验证 `tiup cluster upgrade` 默认行为（自动运行 precheck）
3. 验证所有 precheck 相关参数正常工作
4. 验证报告生成和显示
5. 验证完整升级流程中的 precheck 集成
6. 验证错误处理和边界情况

---

## 前置条件

### 1. 环境准备

```bash
# 1.1 检查目录结构
cd /Users/benjamin2037/Desktop/workspace/sourcecode
ls -d tiup tidb-upgrade-precheck

# 1.2 构建 tidb-upgrade-precheck
cd tidb-upgrade-precheck
GOWORK=off make build
ls -lh bin/upgrade-precheck

# 1.3 构建 TiUP cluster 组件
cd ../tiup
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster
ls -lh bin/tiup-cluster

# 1.4 设置环境变量
# 注意：这些环境变量只在当前 shell 会话中有效
# 如果需要在新的终端窗口中使用，需要重新执行 export 命令
# 或者可以将这些 export 命令添加到 ~/.zshrc 或 ~/.bashrc 中使其永久生效
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# 验证环境变量（在同一 shell 会话中）
echo "Binary: $TIDB_UPGRADE_PRECHECK_BIN"
echo "KB: $TIDB_UPGRADE_PRECHECK_KB"
ls -lh "$TIDB_UPGRADE_PRECHECK_BIN"
ls -d "$TIDB_UPGRADE_PRECHECK_KB"
```

**验证点**（执行完上述操作步骤后验证）:
- [ ] tidb-upgrade-precheck 二进制存在且可执行（验证 1.2 步骤的编译结果）
  ```bash
  ls -lh /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
  /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck --help | head -5
  ```
- [ ] tiup-cluster 二进制存在且可执行（验证 1.3 步骤的编译结果）
  ```bash
  ls -lh /Users/benjamin2037/Desktop/workspace/sourcecode/tiup/bin/tiup-cluster
  /Users/benjamin2037/Desktop/workspace/sourcecode/tiup/bin/tiup-cluster --help | head -5
  ```
- [ ] 环境变量正确设置（**注意：必须先执行 1.4 步骤的 export 命令，或在同一个 shell 会话中执行**）
  ```bash
  # 如果环境变量未设置，先执行以下命令：
  export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
  export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge
  
  # 然后验证环境变量
  echo "TIDB_UPGRADE_PRECHECK_BIN: $TIDB_UPGRADE_PRECHECK_BIN"
  echo "TIDB_UPGRADE_PRECHECK_KB: $TIDB_UPGRADE_PRECHECK_KB"
  [ -n "$TIDB_UPGRADE_PRECHECK_BIN" ] && [ -n "$TIDB_UPGRADE_PRECHECK_KB" ] && echo "✓ Environment variables set" || echo "✗ Environment variables not set"
  ```
- [ ] 知识库目录存在（**注意：如果环境变量未设置，使用绝对路径验证**）
  ```bash
  # 如果环境变量已设置，使用：
  ls -d "$TIDB_UPGRADE_PRECHECK_KB" && echo "✓ KB directory exists" || echo "✗ KB directory not found"
  
  # 如果环境变量未设置，使用绝对路径：
  ls -d /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge && echo "✓ KB directory exists" || echo "✗ KB directory not found"
  ```

### 1.5 远程 Linux VM 快速准备（在有 systemd 的 Linux 上运行 tiup cluster）

```bash
# 1.5.1 安装基础依赖（Debian/Ubuntu）
sudo apt update
sudo apt install -y git curl wget tar bash ca-certificates rsync sudo     build-essential golang openssh-server
sudo systemctl enable --now ssh

# 1.5.2 克隆代码
mkdir -p ~/workspace && cd ~/workspace
git clone git@github.com:Benjamin2037/tidb-upgrade-precheck.git
git clone git@github.com:Benjamin2037/tiup.git

# 1.5.3 构建 tidb-upgrade-precheck
cd ~/workspace/tidb-upgrade-precheck
GOWORK=off go build -o bin/upgrade-precheck ./cmd/precheck

# 1.5.4 构建 tiup-cluster（使用本地 precheck 作为 replace）
cd ~/workspace/tiup
git checkout precheck-dev-qwen
sed -i 's|replace github.com/pingcap/tidb-upgrade-precheck .*|replace github.com/pingcap/tidb-upgrade-precheck => ../tidb-upgrade-precheck|' go.mod
GOWORK=off go mod tidy
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster

# 1.5.5 设置环境变量（当前 shell 会话）
export TIDB_UPGRADE_PRECHECK_BIN=~/workspace/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=~/workspace/tidb-upgrade-precheck/knowledge
export PATH=~/workspace/tiup/bin:$PATH

# 1.5.6 准备拓扑文件（/tmp/e2e-test-topology.yaml，使用当前登录用户）
cat > /tmp/e2e-test-topology.yaml <<'EOF'
global:
  user: "<你的登录用户名>"
  ssh_port: 22
  deploy_dir: "/data/tidb-deploy"
  data_dir: "/data/tidb-data"

pd_servers:
  - host: 127.0.0.1
    name: pd1
    client_port: 2379
    peer_port: 2380

tidb_servers:
  - host: 127.0.0.1
    port: 4000
    status_port: 10080

tikv_servers:
  - host: 127.0.0.1
    port: 20160
    status_port: 20180

tiflash_servers:
  - host: 127.0.0.1
    data_dir: "/data/tidb-data/tiflash-9000"
    tcp_port: 9000
    http_port: 8123
    flash_service_port: 3930
    flash_proxy_port: 20170
    flash_proxy_status_port: 20292
EOF

# 确保 /data 可写
sudo mkdir -p /data && sudo chown -R $(whoami):$(whoami) /data

# 1.5.7 部署与启动
tiup_bin=~/workspace/tiup/bin/tiup-cluster
$tiup_bin deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y
$tiup_bin start e2e-test-cluster
$tiup_bin display e2e-test-cluster

# 1.5.8 升级前检查（显式指定 precheck）
$tiup_bin upgrade e2e-test-cluster v8.5.2 --precheck   --precheck-bin $TIDB_UPGRADE_PRECHECK_BIN   --precheck-kb $TIDB_UPGRADE_PRECHECK_KB   --yes
```

**验证点（远程 VM）**：
- [ ] `bin/upgrade-precheck` 可执行  
- [ ] `bin/tiup-cluster` 可执行  
- [ ] 环境变量已设置（`echo $TIDB_UPGRADE_PRECHECK_BIN`）  
- [ ] `/data/tidb-deploy` 与 `/data/tidb-data` 可写  
- [ ] `tiup-cluster deploy/start/display` 正常，无 systemd 缺失错误  
- [ ] `tiup-cluster upgrade --precheck` 运行成功或输出预期检查结果  

### 2. 知识库准备

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

# 检查知识库
ls -d knowledge/v7.5/v7.5.1 knowledge/v8.5/v8.5.4 2>/dev/null || echo "需要生成知识库"

# 如果不存在，生成知识库（可以混合使用两种方式）

# 重要：先确保 upgrade_logic.json 已生成（TiDB 模块的强制变更逻辑）
# upgrade_logic.json 是全局文件，包含所有历史版本的升级逻辑，需要从 master/main 分支提取
# 方式 A: 使用 generate_knowledge.sh 的 --force 模式会自动生成（推荐）
# bash scripts/generate_knowledge.sh --force --serial --start-from=v7.5.1 --stop-at=v7.5.1

# 方式 B: 单独生成 upgrade_logic.json（如果需要强制重新生成）
# 注意：需要先切换到 TiDB 仓库的 master/main 分支
# cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb
# git checkout master  # 或 git checkout main
# cd ../tidb-upgrade-precheck
# mkdir -p knowledge/tidb
# GOWORK=off go run cmd/generate_upgrade_logic/main.go \
#   --tidb-repo=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb \
#   --output=knowledge/tidb/upgrade_logic.json

# 方式 1: 使用 generate_knowledge.sh 脚本生成 v7.5.1（批量生成方式）
# bash scripts/generate_knowledge.sh --serial --start-from=v7.5.1 --stop-at=v7.5.1

# 方式 2: 直接使用 kb_generator 生成 v8.5.4（单个版本，使用 --version 参数）
# GOWORK=off go run cmd/kb_generator/main.go --version=v8.5.4 --components=tidb,pd,tikv,tiflash

# 或者反过来：
# 方式 2: 使用 kb_generator 生成 v7.5.1
# GOWORK=off go run cmd/kb_generator/main.go --version=v7.5.1 --components=tidb,pd,tikv,tiflash
# 方式 1: 使用 generate_knowledge.sh 生成 v8.5.4
# bash scripts/generate_knowledge.sh --serial --start-from=v8.5.4 --stop-at=v8.5.4
```

**验证点**:
- [ ] 源版本知识库存在（v7.5.1）
  ```bash
  ls -d knowledge/v7.5/v7.5.1 && echo "✓ Source version KB exists" || echo "✗ Source version KB not found"
  ```
- [ ] 目标版本知识库存在（v8.5.4）
  ```bash
  ls -d knowledge/v8.5/v8.5.4 && echo "✓ Target version KB exists" || echo "✗ Target version KB not found"
  ```
- [ ] 所有组件 defaults.json 文件存在
  ```bash
  for version in "v7.5/v7.5.1" "v8.5/v8.5.4"; do
    for comp in tidb pd tikv tiflash; do
      if [ -f "knowledge/$version/$comp/defaults.json" ]; then
        echo "✓ $version/$comp/defaults.json"
      else
        echo "✗ $version/$comp/defaults.json MISSING"
      fi
    done
  done
  ```
- [ ] TiDB upgrade_logic.json 文件存在（包含强制变更逻辑）
  ```bash
  if [ -f "knowledge/tidb/upgrade_logic.json" ]; then
    echo "✓ upgrade_logic.json exists"
    echo "  File size: $(ls -lh knowledge/tidb/upgrade_logic.json | awk '{print $5}')"
    echo "  Forced changes count: $(cat knowledge/tidb/upgrade_logic.json | grep -o '"type":"force"' | wc -l | tr -d ' ')"
  else
    echo "✗ upgrade_logic.json MISSING"
    echo "  Run: GOWORK=off go run cmd/generate_upgrade_logic/main.go --tidb-repo=<tidb-repo-path> --output=knowledge/tidb/upgrade_logic.json"
  fi
  ```

### 3. 测试集群准备

**重要提示**: 
- 以下脚本可以直接复制粘贴到终端执行
- 如果遇到错误，脚本会显示错误信息但不会退出 shell 会话
- 建议：可以将脚本保存为文件执行，例如：`bash /tmp/deploy-cluster.sh`

```bash
# 3.1 创建测试拓扑文件
# 注意：此拓扑文件只包含核心组件（pd、tidb、tikv、tiflash）
# 不包含任何监控组件（monitoring_servers、grafana_servers、alertmanager_servers、node_exporter、blackbox_exporter）
# 这样可以避免在 macOS ARM64 平台上遇到监控组件不支持的问题
# 对于 E2E 测试，核心组件已经足够，不需要监控组件
cat > /tmp/e2e-test-topology.yaml << 'EOF'
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/tmp/tidb-deploy"
  data_dir: "/tmp/tidb-data"

pd_servers:
  - host: 127.0.0.1
    name: pd1
    client_port: 2379
    peer_port: 2380

tidb_servers:
  - host: 127.0.0.1
    port: 4000
    status_port: 10080

tikv_servers:
  - host: 127.0.0.1
    port: 20160
    status_port: 20180

tiflash_servers:
  - host: 127.0.0.1
    data_dir: "/tmp/tidb-data/tiflash-9000"
    tcp_port: 9000
    http_port: 8123
    flash_service_port: 3930
    flash_proxy_port: 20170
    flash_proxy_status_port: 20292
    # 注意：TiFlash 不使用 status_port 字段，已移除

# 注意：
# 1. 只包含核心组件：pd、tidb、tikv、tiflash（用于 E2E 测试已足够）
# 2. 不包含任何监控组件：
#    - 不包含 monitoring_servers（node_exporter、blackbox_exporter）
#    - 不包含 grafana_servers
#    - 不包含 alertmanager_servers
# 3. 这样可以避免在 macOS ARM64 平台上遇到监控组件不支持的问题
# 4. 对于 E2E 测试，核心组件已经足够，不需要监控组件
# 5. 如果仍遇到监控组件下载错误，请参考"故障排除 - 问题 0"
EOF

# 3.2 部署测试集群（如果不存在）
# 注意：确保使用本地构建的 tiup-cluster 二进制，或确保系统 tiup 已正确配置
# 注意：拓扑文件只包含核心组件（pd、tidb、tikv、tiflash），不包含任何监控组件
CLUSTER_NAME="e2e-test-cluster"

# 检查集群是否已存在
echo "检查集群 $CLUSTER_NAME 是否存在..."
if tiup cluster list 2>/dev/null | grep -q "^$CLUSTER_NAME"; then
    echo "✓ 集群 $CLUSTER_NAME 已存在，跳过部署"
else
    echo "集群 $CLUSTER_NAME 不存在，开始部署..."
    
    # 验证拓扑文件是否存在
    if [ ! -f "/tmp/e2e-test-topology.yaml" ]; then
        echo "✗ 错误: 拓扑文件不存在: /tmp/e2e-test-topology.yaml"
        echo "  请先执行 3.1 步骤创建拓扑文件"
        # 注意：不使用 exit，避免退出 shell 会话
    else
        echo "✓ 拓扑文件存在: /tmp/e2e-test-topology.yaml"
        
        # 清理可能存在的残留数据
        if [ -d "$HOME/.tiup/storage/cluster/clusters/$CLUSTER_NAME" ]; then
            echo "清理残留的集群数据..."
            rm -rf "$HOME/.tiup/storage/cluster/clusters/$CLUSTER_NAME"
        fi
        
        # 检测平台
        PLATFORM=$(uname -m)
        OS=$(uname -s)
        if [ "$OS" = "Darwin" ] && [ "$PLATFORM" = "arm64" ]; then
            echo "检测到 macOS ARM64 平台"
            echo "拓扑文件已配置为只包含核心组件，不包含监控组件"
        fi
        
        # 检查端口是否被占用
        echo "检查端口是否被占用..."
        PORTS_IN_USE=""
        for port in 4000 2379 20160 9000; do
            if lsof -i :$port > /dev/null 2>&1; then
                PORTS_IN_USE="$PORTS_IN_USE $port"
            fi
        done
        if [ -n "$PORTS_IN_USE" ]; then
            echo "⚠ 警告: 以下端口被占用: $PORTS_IN_USE"
            echo "  如果这些端口被其他 TiDB 集群占用，请先停止它们"
            echo "  或者修改拓扑文件中的端口配置"
        else
            echo "✓ 所有端口可用"
        fi
        
        # 使用本地构建的 tiup-cluster（推荐）
        DEPLOY_LOG="/tmp/deploy-${CLUSTER_NAME}-$(date +%Y%m%d-%H%M%S).log"
        if [ -f "./bin/tiup-cluster" ]; then
            echo "使用本地构建的 tiup-cluster 部署..."
            echo "部署日志: $DEPLOY_LOG"
            echo "执行命令: ./bin/tiup-cluster deploy $CLUSTER_NAME v7.5.1 /tmp/e2e-test-topology.yaml -y"
            
            # 执行部署命令并捕获输出
            ./bin/tiup-cluster deploy "$CLUSTER_NAME" v7.5.1 /tmp/e2e-test-topology.yaml -y > "$DEPLOY_LOG" 2>&1
            DEPLOY_EXIT_CODE=$?
            
            echo "部署命令退出码: $DEPLOY_EXIT_CODE"
            
            # 显示部署日志的最后部分（包含错误信息）
            echo ""
            echo "=== 部署日志（最后 50 行）==="
            tail -50 "$DEPLOY_LOG"
            echo "=============================="
            echo ""
            
            # 等待一下，让 TiUP 完成所有操作
            echo "等待 TiUP 完成操作..."
            sleep 3
            
            # 检查集群是否真正创建成功（这是最可靠的检查方式）
            echo "检查集群是否创建成功..."
            if tiup cluster list 2>/dev/null | grep -q "^$CLUSTER_NAME"; then
                echo "✓ 集群部署成功（集群已创建）"
                echo "集群列表:"
                tiup cluster list | grep "^$CLUSTER_NAME"
            else
                echo "✗ 集群部署失败：集群未创建"
                echo ""
                echo "  可能的原因："
                echo "  1. 部署过程中出现错误（即使拓扑文件只包含核心组件）"
                echo "  2. 端口被占用"
                echo "  3. 权限问题"
                echo "  4. TiUP 版本问题"
                echo ""
                echo "  排查步骤："
                echo "  1. 查看完整部署日志: cat $DEPLOY_LOG"
                echo "  2. 检查端口是否被占用: lsof -i :4000 -i :2379 -i :20160 -i :9000"
                echo "  3. 检查 TiUP 日志: ls -lt ~/.tiup/logs/ | head -5"
                echo "  4. 尝试手动部署（见下方手动部署步骤）"
                echo "  5. 参考'故障排除 - 问题 0'获取更多帮助"
                echo ""
                echo "⚠ 警告: 集群部署失败，请解决上述问题后重新执行部署命令"
                # 注意：不使用 exit，避免退出 shell 会话
            fi
        else
            # 如果本地二进制不存在，使用系统安装的 tiup
            echo "警告: 本地 tiup-cluster 不存在，使用系统 tiup"
            echo "部署日志: $DEPLOY_LOG"
            echo "执行命令: tiup cluster deploy $CLUSTER_NAME v7.5.1 /tmp/e2e-test-topology.yaml -y"
            
            tiup cluster deploy "$CLUSTER_NAME" v7.5.1 /tmp/e2e-test-topology.yaml -y > "$DEPLOY_LOG" 2>&1
            DEPLOY_EXIT_CODE=$?
            
            echo "部署命令退出码: $DEPLOY_EXIT_CODE"
            
            # 显示部署日志的最后部分
            echo ""
            echo "=== 部署日志（最后 50 行）==="
            tail -50 "$DEPLOY_LOG"
            echo "=============================="
            echo ""
            
            # 等待一下
            echo "等待 TiUP 完成操作..."
            sleep 3
            
            # 检查集群是否真正创建成功
            echo "检查集群是否创建成功..."
            if tiup cluster list 2>/dev/null | grep -q "^$CLUSTER_NAME"; then
                echo "✓ 集群部署成功（集群已创建）"
                echo "集群列表:"
                tiup cluster list | grep "^$CLUSTER_NAME"
            else
                echo "✗ 集群部署失败：集群未创建"
                echo ""
                echo "  排查步骤："
                echo "  1. 查看完整部署日志: cat $DEPLOY_LOG"
                echo "  2. 检查端口是否被占用: lsof -i :4000 -i :2379 -i :20160 -i :9000"
                echo "  3. 尝试手动部署（见下方手动部署步骤）"
                echo "  4. 参考'故障排除 - 问题 0'获取更多帮助"
                echo ""
                echo "⚠ 警告: 集群部署失败，请解决上述问题后重新执行部署命令"
                # 注意：不使用 exit，避免退出 shell 会话
            fi
        fi
        
        # 保留日志文件以便调试（不自动删除）
        echo ""
        echo "部署日志已保存到: $DEPLOY_LOG"
        echo "如需查看完整日志: cat $DEPLOY_LOG"
    fi
fi

# 手动部署步骤（如果自动部署失败，可以手动执行）
echo ""
echo "=========================================="
echo "手动部署步骤（如果自动部署失败）"
echo "=========================================="
echo "如果自动部署失败，可以手动执行以下命令："
echo ""
echo "1. 检查拓扑文件:"
echo "   cat /tmp/e2e-test-topology.yaml"
echo ""
echo "2. 手动部署集群:"
echo "   cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup"
echo "   ./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y"
echo ""
echo "3. 检查部署结果:"
echo "   tiup cluster list"
echo ""
echo "4. 如果部署成功，继续执行 3.3 步骤启动集群"
echo "=========================================="

# 3.3 启动集群
echo "启动集群 $CLUSTER_NAME..."

# 先检查集群是否存在
if ! tiup cluster list 2>/dev/null | grep -q "^$CLUSTER_NAME"; then
    echo "✗ 错误: 集群 $CLUSTER_NAME 不存在，无法启动"
    echo "  请先执行 3.2 步骤部署集群"
    echo "  或者手动执行: cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup && ./bin/tiup-cluster deploy $CLUSTER_NAME v7.5.1 /tmp/e2e-test-topology.yaml -y"
    echo ""
    echo "⚠ 警告: 集群不存在，请先部署集群"
    # 注意：不使用 exit，避免退出 shell 会话
else
    echo "✓ 集群存在，开始启动..."
    tiup cluster start "$CLUSTER_NAME"
    
    # 检查启动是否成功
    if [ $? -eq 0 ]; then
        echo "✓ 集群启动命令执行成功"
    else
        echo "✗ 集群启动失败，请检查错误信息"
        echo "⚠ 警告: 集群启动失败，请检查集群状态: tiup cluster display $CLUSTER_NAME"
        # 注意：不使用 exit，避免退出 shell 会话
    fi
fi

# 3.4 等待集群就绪（约 1-2 分钟）
echo "等待集群就绪..."
sleep 30

# 3.5 验证集群状态
echo "验证集群状态..."

# 先检查集群是否存在
if ! tiup cluster list 2>/dev/null | grep -q "^$CLUSTER_NAME"; then
    echo "✗ 错误: 集群 $CLUSTER_NAME 不存在"
    echo "  请先执行 3.2 步骤部署集群"
    echo "  或者手动执行: cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup && ./bin/tiup-cluster deploy $CLUSTER_NAME v7.5.1 /tmp/e2e-test-topology.yaml -y"
    echo ""
    echo "⚠ 警告: 集群不存在，请先部署集群"
    # 注意：不使用 exit，避免退出 shell 会话
else
    echo "✓ 集群存在，显示集群状态..."
    tiup cluster display "$CLUSTER_NAME"
    
    # 检查 display 命令是否成功
    if [ $? -ne 0 ]; then
        echo "✗ 错误: 无法显示集群状态"
        echo "  可能的原因："
        echo "  1. 集群未启动: tiup cluster start $CLUSTER_NAME"
        echo "  2. 集群组件异常"
        echo ""
        echo "⚠ 警告: 集群状态显示失败，请检查集群状态"
        # 注意：不使用 exit，避免退出 shell 会话
    else
        echo "✓ 集群状态显示成功"
    fi
fi
```

**验证点**:
- [ ] 集群部署成功（**如果集群不存在，请先执行 3.2 步骤部署集群**）
  ```bash
  # 检查集群是否存在
  if tiup cluster list 2>/dev/null | grep -q "^e2e-test-cluster"; then
    echo "✓ Cluster deployed"
  else
    echo "✗ Cluster not deployed"
    echo "  请执行以下命令部署集群："
    echo "  cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup"
    echo "  ./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y"
  fi
  ```
- [ ] 集群启动成功（**如果集群未启动，请先执行 3.3 步骤启动集群**）
  ```bash
  # 先检查集群是否存在
  if ! tiup cluster list 2>/dev/null | grep -q "^e2e-test-cluster"; then
    echo "✗ Cluster not found, please deploy first"
    echo "  执行部署: cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup && ./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y"
    # 注意：不使用 exit，避免退出 shell 会话
  else
    # 启动集群（如果未启动）
    tiup cluster start e2e-test-cluster
    
    # 等待并检查状态
    sleep 10
    if tiup cluster display e2e-test-cluster 2>/dev/null | grep -i "status.*up" > /dev/null; then
      echo "✓ Cluster started"
    else
      echo "✗ Cluster not started or components not ready"
      echo "  检查集群状态: tiup cluster display e2e-test-cluster"
    fi
  fi
  ```
- [ ] 所有组件状态为 "Up"（**如果集群不存在，请先部署和启动集群**）
  ```bash
  # 检查集群是否存在
  if ! tiup cluster list 2>/dev/null | grep -q "^e2e-test-cluster"; then
    echo "✗ Cluster not found, please deploy and start cluster first"
    echo "  执行部署: cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup && ./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y"
    # 注意：不使用 exit，避免退出 shell 会话
  else
    # 检查组件状态
    if tiup cluster display e2e-test-cluster 2>/dev/null | grep -E "tidb|tikv|pd" | grep -v "Up" > /dev/null; then
      echo "✗ Some components not up"
      echo "  检查详细状态: tiup cluster display e2e-test-cluster"
    else
      echo "✓ All components up"
    fi
  fi
  ```
- [ ] TiDB 端口 4000 可访问（**如果端口不可访问，请检查集群是否已启动**）
  ```bash
  if nc -z 127.0.0.1 4000 2>/dev/null; then
    echo "✓ TiDB port 4000 accessible"
    mysql -h 127.0.0.1 -P 4000 -u root -e "SELECT VERSION();" 2>&1 | head -1
  else
    echo "✗ TiDB port 4000 not accessible"
    echo "  请检查："
    echo "  1. 集群是否已启动: tiup cluster start e2e-test-cluster"
    echo "  2. TiDB 组件状态: tiup cluster display e2e-test-cluster"
  fi
  ```
- [ ] PD 端口 2379 可访问（**如果端口不可访问，请检查集群是否已启动**）
  ```bash
  if nc -z 127.0.0.1 2379 2>/dev/null; then
    echo "✓ PD port 2379 accessible"
    curl -s http://127.0.0.1:2379/pd/api/v1/version 2>&1 | head -1
  else
    echo "✗ PD port 2379 not accessible"
    echo "  请检查："
    echo "  1. 集群是否已启动: tiup cluster start e2e-test-cluster"
    echo "  2. PD 组件状态: tiup cluster display e2e-test-cluster"
  fi
  ```

---

## 测试阶段

### Phase 1: Precheck-Only 模式测试

**目标**: 验证 `tiup cluster upgrade --precheck` 命令正常工作

#### Test 1.1: 基本 Precheck-Only 测试

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# 执行 precheck-only
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] 命令执行成功
  ```bash
  echo $?  # 应该返回 0
  ```
- [ ] 显示 "Running parameter precheck..."
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "running parameter precheck"
  ```
- [ ] Precheck 执行完成
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "precheck\|report generated"
  ```
- [ ] 报告显示在 stdout
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -E "Report|Summary|Findings" | head -5
  ```
- [ ] 命令退出，**不执行升级**
  ```bash
  # 检查集群版本未改变
  tiup cluster display e2e-test-cluster | grep "Cluster version"
  # 应该仍然是 v7.5.1，不是 v8.5.4
  ```
- [ ] 退出码为 0（如果 precheck 成功）
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
  echo "Exit code: $?"
  ```

**检查输出**:
- [ ] 包含 "Running parameter precheck..."
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep "Running parameter precheck"
  ```
- [ ] 包含 "Executing: .../upgrade-precheck ..."
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep "Executing:"
  ```
- [ ] 包含报告内容或报告路径
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -E "Report generated|report.*\.(txt|html|md|json)"
  ```
- [ ] 不包含升级相关操作
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "upgrading\|upgrade.*start\|component.*upgrade" && echo "✗ Found upgrade operations" || echo "✓ No upgrade operations"
  ```

#### Test 1.2: Precheck-Only with Text Format

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output text
```

**验证点**:
- [ ] 报告格式为 text
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output text 2>&1 | head -20
  ```
- [ ] 报告内容完整
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output text 2>&1 | grep -E "Source Version|Target Version|Check Results|Summary" | head -10
  ```
- [ ] 报告显示在 stdout
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output text 2>&1 | wc -l
  # 应该有多行输出
  ```

#### Test 1.3: Precheck-Only with HTML Format

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output html
```

**验证点**:
- [ ] 报告格式为 HTML
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html 2>&1 | grep -i "html\|report generated"
  ```
- [ ] 报告文件生成在 `~/.tiup/storage/cluster/upgrade_precheck/reports/`
  ```bash
  ls -lh ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html
  ```
- [ ] 报告文件名格式为 `upgrade_precheck_report_*.html`
  ```bash
  ls ~/.tiup/storage/cluster/upgrade_precheck/reports/ | grep "upgrade_precheck_report_.*\.html"
  ```
- [ ] 报告内容完整且格式正确
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  echo "Report file: $REPORT_FILE"
  head -50 "$REPORT_FILE"
  # 检查 HTML 结构
  grep -E "<html|<head|<body|</html>" "$REPORT_FILE" | head -5
  ```

**检查报告文件**:
```bash
# 列出所有 HTML 报告
ls -lh ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html

# 查看最新报告内容
REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
cat "$REPORT_FILE" | head -50

# 在浏览器中打开（macOS）
open "$REPORT_FILE"
```

#### Test 1.4: Precheck-Only with Markdown Format

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output markdown
```

**验证点**:
- [ ] 报告格式为 markdown
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output markdown 2>&1 | grep -i "markdown\|report generated"
  ```
- [ ] 报告文件生成
  ```bash
  ls -lh ~/.tiup/storage/cluster/upgrade_precheck/reports/*.md
  ```
- [ ] Markdown 格式正确
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.md | head -1)
  echo "Report file: $REPORT_FILE"
  head -30 "$REPORT_FILE"
  # 检查 Markdown 语法
  grep -E "^#|^##|^\*|^-" "$REPORT_FILE" | head -10
  ```

#### Test 1.5: Precheck-Only with Custom Output File

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output html \
  --precheck-output-file /tmp/precheck-report.html
```

**验证点**:
- [ ] 报告保存到指定文件
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-output-file /tmp/precheck-report.html
  ls -lh /tmp/precheck-report.html
  ```
- [ ] 文件内容完整
  ```bash
  wc -l /tmp/precheck-report.html
  head -50 /tmp/precheck-report.html
  ```
- [ ] 文件格式正确
  ```bash
  file /tmp/precheck-report.html
  grep -E "<html|<head|<body" /tmp/precheck-report.html | head -3
  ```

---

### Phase 2: 完整升级流程测试（默认行为）

**目标**: 验证 `tiup cluster upgrade` 默认行为（自动运行 precheck）

#### Test 2.1: 默认升级流程（自动 Precheck）

```bash
# 注意：这会实际执行升级，需要确认
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4
```

**验证点**:
- [ ] 自动运行 precheck（不显示 --precheck 标志）
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -i "running parameter precheck"
  ```
- [ ] 显示 precheck 报告
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -E "Report|Summary|Findings" | head -5
  ```
- [ ] 询问用户确认
  ```bash
  # 注意：这个测试需要交互，可以查看输出中是否包含确认提示
  echo "n" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -i "confirm\|continue\|backup"
  ```
- [ ] 如果确认，执行升级
  ```bash
  # 注意：这会实际执行升级，请谨慎使用
  # echo "y" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4
  # 升级后检查版本
  # tiup cluster display e2e-test-cluster | grep "Cluster version"
  ```
- [ ] 如果取消，不执行升级
  ```bash
  echo "n" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | tail -5
  # 检查集群版本未改变
  tiup cluster display e2e-test-cluster | grep "Cluster version"
  # 应该仍然是 v7.5.1
  ```

**预期流程验证**:
1. 显示 "Running parameter precheck..."
   ```bash
   ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep "Running parameter precheck"
   ```
2. 显示 precheck 报告
   ```bash
   ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -A 10 "Report generated\|Summary"
   ```
3. 显示确认提示
   ```bash
   echo "n" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -i "backup.*data.*read.*report.*confirm"
   ```
4. 用户确认后执行升级（需要手动确认）
   ```bash
   # 手动执行并输入 'y' 确认
   # ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4
   ```
5. 升级完成后集群版本为 v8.5.4
   ```bash
   tiup cluster display e2e-test-cluster | grep "Cluster version"
   # 应该显示 v8.5.4
   ```

#### Test 2.2: 默认升级流程 with HTML Report

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck-output html
```

**验证点**:
- [ ] 自动运行 precheck
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck-output html 2>&1 | grep -i "running parameter precheck"
  ```
- [ ] HTML 报告生成
  ```bash
  ls -lh ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html
  ```
- [ ] 报告路径显示
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck-output html 2>&1 | grep -E "Report generated|report.*\.html"
  ```
- [ ] 询问用户确认
  ```bash
  echo "n" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck-output html 2>&1 | grep -i "confirm\|continue"
  ```

#### Test 2.3: 默认升级流程 with Skip Confirm

```bash
# 注意：这会直接执行升级，不询问确认
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 -y
```

**验证点**:
- [ ] 自动运行 precheck
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 -y 2>&1 | grep -i "running parameter precheck"
  ```
- [ ] 显示报告
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 -y 2>&1 | grep -E "Report|Summary" | head -5
  ```
- [ ] 跳过确认，直接执行升级
  ```bash
  # 注意：这会实际执行升级
  # ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 -y 2>&1 | grep -i "confirm" && echo "✗ Still asking for confirmation" || echo "✓ Skipped confirmation"
  ```
- [ ] 升级完成
  ```bash
  # 升级后检查
  tiup cluster display e2e-test-cluster | grep "Cluster version"
  # 应该显示 v8.5.4
  ```

---

### Phase 3: 凭证和认证测试

**目标**: 验证 TiDB 凭证处理

#### Test 3.1: 默认用户（root）

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-user root
```

**验证点**:
- [ ] 使用 root 用户连接
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-user root 2>&1 | grep -i "tidb-user\|connecting"
  ```
- [ ] 连接成功
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-user root 2>&1 | grep -i "error\|failed\|connection" && echo "✗ Connection failed" || echo "✓ Connection successful"
  ```
- [ ] 配置收集成功
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-user root 2>&1 | grep -i "collecting\|configuration\|report generated" && echo "✓ Config collection successful" || echo "✗ Config collection failed"
  ```

#### Test 3.2: 自定义用户

```bash
# 如果集群中有其他用户
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-user admin
```

**验证点**:
- [ ] 使用指定用户连接
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-user admin 2>&1 | grep -i "tidb-user.*admin\|connecting.*admin"
  ```
- [ ] 连接成功或显示适当错误
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-user admin 2>&1 | grep -E "error|failed|success|connection" | head -3
  ```

#### Test 3.3: 密码提示

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-prompt
```

**验证点**:
- [ ] 提示输入密码
  ```bash
  # 注意：这个测试需要交互，可以检查帮助信息或代码
  ./bin/tiup-cluster upgrade --help | grep -i "password-prompt"
  ```
- [ ] 接受密码输入
  ```bash
  # 手动测试：执行命令并输入密码
  # ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-password-prompt
  ```
- [ ] 使用密码连接
  ```bash
  # 手动测试后检查连接是否成功
  # 查看输出中是否有连接错误
  ```

#### Test 3.4: 密码文件

```bash
echo "your-password" > /tmp/tidb-password.txt
chmod 600 /tmp/tidb-password.txt

./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-file /tmp/tidb-password.txt

# 清理
rm /tmp/tidb-password.txt
```

**验证点**:
- [ ] 从文件读取密码
  ```bash
  echo "test-password" > /tmp/tidb-password.txt
  chmod 600 /tmp/tidb-password.txt
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-password-file /tmp/tidb-password.txt 2>&1 | grep -i "password.*file\|reading.*password"
  ```
- [ ] 使用密码连接
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-password-file /tmp/tidb-password.txt 2>&1 | grep -E "error|failed|success|connection" | head -3
  ```
- [ ] 文件权限检查（如果实现）
  ```bash
  chmod 644 /tmp/tidb-password.txt
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-tidb-password-file /tmp/tidb-password.txt 2>&1 | grep -i "permission\|security\|readable" && echo "✓ Permission check exists" || echo "⚠ Permission check not implemented"
  rm /tmp/tidb-password.txt
  ```

---

### Phase 4: 高风险参数配置测试

**目标**: 验证高风险参数配置功能

#### Test 4.1: 创建高风险参数配置

```bash
cat > /tmp/high_risk_params.json << 'EOF'
{
  "tidb": {
    "config": {},
    "system_variables": {
      "tidb_enable_async_commit": {
        "severity": "error",
        "description": "Async commit may cause data inconsistency",
        "from_version": "v7.0.0",
        "to_version": "",
        "check_modified": true,
        "allowed_values": []
      }
    }
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {}
  },
  "tiflash": {
    "config": {}
  }
}
EOF
```

#### Test 4.2: Precheck with High-Risk Config

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-high-risk-params-config /tmp/high_risk_params.json
```

**验证点**:
- [ ] 配置文件正确加载
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-high-risk-params-config /tmp/high_risk_params.json 2>&1 | grep -i "loading.*high.*risk\|config.*loaded"
  ```
- [ ] 高风险参数规则执行
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-high-risk-params-config /tmp/high_risk_params.json 2>&1 | grep -i "high.*risk\|checking.*parameter"
  ```
- [ ] 检查结果包含在报告中
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-high-risk-params-config /tmp/high_risk_params.json 2>&1 | grep -i "tidb_enable_async_commit\|high.*risk.*finding"
  ```
- [ ] 如果发现问题，报告显示相应警告/错误
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-high-risk-params-config /tmp/high_risk_params.json 2>&1 | grep -E "error|warning|severity" | head -5
  ```

#### Test 4.3: Default High-Risk Config Location

```bash
# 复制到默认位置
cp /tmp/high_risk_params.json ~/.tiup/high_risk_params.json

# 不指定 --precheck-high-risk-params-config
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] 自动从默认位置加载配置
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "loading.*high.*risk\|~/.tiup/high_risk_params.json"
  ```
- [ ] 高风险参数规则执行
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "high.*risk\|checking.*parameter"
  ```

---

### Phase 5: 错误处理和边界情况

**目标**: 验证错误处理和边界情况

#### Test 5.1: 跳过 Precheck

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --without-precheck
```

**验证点**:
- [ ] Precheck 被跳过
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --without-precheck 2>&1 | grep -i "precheck" && echo "✗ Precheck not skipped" || echo "✓ Precheck skipped"
  ```
- [ ] 直接执行升级
  ```bash
  # 注意：这会实际执行升级
  # ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --without-precheck 2>&1 | grep -i "upgrading\|upgrade.*start"
  ```
- [ ] 不显示 precheck 相关信息
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --without-precheck 2>&1 | grep -i "running parameter precheck\|precheck.*report" && echo "✗ Precheck info shown" || echo "✓ No precheck info"
  ```

#### Test 5.2: Precheck 失败但继续升级

```bash
# 模拟 precheck 失败（例如：临时移除知识库）
mv "$TIDB_UPGRADE_PRECHECK_KB" "${TIDB_UPGRADE_PRECHECK_KB}.bak"

./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4

# 恢复知识库
mv "${TIDB_UPGRADE_PRECHECK_KB}.bak" "$TIDB_UPGRADE_PRECHECK_KB"
```

**验证点**:
- [ ] Precheck 失败显示警告
  ```bash
  mv "$TIDB_UPGRADE_PRECHECK_KB" "${TIDB_UPGRADE_PRECHECK_KB}.bak"
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -i "precheck.*fail\|warning.*precheck"
  mv "${TIDB_UPGRADE_PRECHECK_KB}.bak" "$TIDB_UPGRADE_PRECHECK_KB"
  ```
- [ ] 仍然询问用户确认
  ```bash
  mv "$TIDB_UPGRADE_PRECHECK_KB" "${TIDB_UPGRADE_PRECHECK_KB}.bak"
  echo "n" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 2>&1 | grep -i "confirm\|continue"
  mv "${TIDB_UPGRADE_PRECHECK_KB}.bak" "$TIDB_UPGRADE_PRECHECK_KB"
  ```
- [ ] 用户可以选择继续升级
  ```bash
  # 手动测试：即使 precheck 失败，用户仍可以确认继续
  # mv "$TIDB_UPGRADE_PRECHECK_KB" "${TIDB_UPGRADE_PRECHECK_KB}.bak"
  # echo "y" | ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4
  # mv "${TIDB_UPGRADE_PRECHECK_KB}.bak" "$TIDB_UPGRADE_PRECHECK_KB"
  ```
- [ ] 升级可以正常执行
  ```bash
  # 手动测试后验证升级是否完成
  # tiup cluster display e2e-test-cluster | grep "Cluster version"
  ```

#### Test 5.3: 无效集群名称

```bash
./bin/tiup-cluster upgrade non-existent-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] 显示清晰的错误信息
  ```bash
  ./bin/tiup-cluster upgrade non-existent-cluster v8.5.4 --precheck 2>&1 | grep -i "error\|not found\|cluster.*exist"
  ```
- [ ] 不执行 precheck
  ```bash
  ./bin/tiup-cluster upgrade non-existent-cluster v8.5.4 --precheck 2>&1 | grep -i "running parameter precheck" && echo "✗ Precheck executed" || echo "✓ Precheck not executed"
  ```
- [ ] 退出码非零
  ```bash
  ./bin/tiup-cluster upgrade non-existent-cluster v8.5.4 --precheck
  echo "Exit code: $?"
  # 应该返回非零值
  ```

#### Test 5.4: 无效目标版本

```bash
./bin/tiup-cluster upgrade e2e-test-cluster invalid-version --precheck
```

**验证点**:
- [ ] 显示版本错误信息
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster invalid-version --precheck 2>&1 | grep -i "version.*invalid\|version.*not found\|invalid.*version"
  ```
- [ ] 或显示知识库缺失错误
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster invalid-version --precheck 2>&1 | grep -i "knowledge.*base.*not found\|knowledge.*base.*missing"
  ```
- [ ] 不执行 precheck
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster invalid-version --precheck 2>&1 | grep -i "running parameter precheck" && echo "✗ Precheck executed" || echo "✓ Precheck not executed"
  ```

#### Test 5.5: 缺少环境变量（二进制路径）

```bash
unset TIDB_UPGRADE_PRECHECK_BIN
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] TiUP 尝试从其他位置查找二进制
  ```bash
  unset TIDB_UPGRADE_PRECHECK_BIN
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "trying\|looking.*for\|searching"
  export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
  ```
- [ ] 或显示清晰的错误信息
  ```bash
  unset TIDB_UPGRADE_PRECHECK_BIN
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "error.*binary\|not found\|cannot.*find"
  export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
  ```
- [ ] 错误信息包含解决建议
  ```bash
  unset TIDB_UPGRADE_PRECHECK_BIN
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "set.*TIDB_UPGRADE_PRECHECK_BIN\|environment.*variable\|solution"
  export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
  ```

#### Test 5.6: 缺少环境变量（知识库路径）

```bash
unset TIDB_UPGRADE_PRECHECK_KB
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] TiUP 尝试从其他位置查找知识库
  ```bash
  unset TIDB_UPGRADE_PRECHECK_KB
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "trying\|looking.*for\|searching.*knowledge"
  export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge
  ```
- [ ] 或显示清晰的错误信息
  ```bash
  unset TIDB_UPGRADE_PRECHECK_KB
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "error.*knowledge\|knowledge.*not found\|cannot.*find.*knowledge"
  export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge
  ```
- [ ] 错误信息包含解决建议
  ```bash
  unset TIDB_UPGRADE_PRECHECK_KB
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "set.*TIDB_UPGRADE_PRECHECK_KB\|environment.*variable\|solution"
  export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge
  ```

---

### Phase 6: 报告验证

**目标**: 验证报告内容和格式

#### Test 6.1: 验证 Text 报告内容

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output text > /tmp/precheck-text.txt

# 检查报告内容
cat /tmp/precheck-text.txt
```

**验证点**:
- [ ] 包含源版本信息
  ```bash
  cat /tmp/precheck-text.txt | grep -i "source.*version\|from.*version" | head -2
  ```
- [ ] 包含目标版本信息
  ```bash
  cat /tmp/precheck-text.txt | grep -i "target.*version\|to.*version" | head -2
  ```
- [ ] 包含配置检查结果
  ```bash
  cat /tmp/precheck-text.txt | grep -i "config.*check\|configuration.*result" | head -3
  ```
- [ ] 包含系统变量检查结果
  ```bash
  cat /tmp/precheck-text.txt | grep -i "system.*variable\|variable.*check" | head -3
  ```
- [ ] 包含兼容性检查结果
  ```bash
  cat /tmp/precheck-text.txt | grep -i "compatibility\|compatible" | head -3
  ```
- [ ] 包含风险等级（如果有）
  ```bash
  cat /tmp/precheck-text.txt | grep -E "risk|severity|error|warning" | head -5
  ```

#### Test 6.2: 验证 HTML 报告内容

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output html

# 检查报告文件
REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
echo "Report file: $REPORT_FILE"

# 在浏览器中打开
open "$REPORT_FILE"
```

**验证点**:
- [ ] HTML 格式正确
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  python3 -c "import html.parser; parser = html.parser.HTMLParser(); open('$REPORT_FILE').read()" 2>&1 && echo "✓ Valid HTML" || echo "✗ Invalid HTML"
  ```
- [ ] 可以在浏览器中正常显示
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  open "$REPORT_FILE"  # macOS
  # 手动检查浏览器显示是否正常
  ```
- [ ] 包含所有检查结果
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  grep -i "source.*version\|target.*version\|check.*result\|summary" "$REPORT_FILE" | head -5
  ```
- [ ] 样式正确
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  grep -E "<style|\.css" "$REPORT_FILE" | head -3
  ```
- [ ] 链接和交互正常（如果有）
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  grep -E "<a href|onclick|button" "$REPORT_FILE" | head -3
  ```

#### Test 6.3: 验证 JSON 报告内容

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output json

# 检查报告文件
REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.json | head -1)
echo "Report file: $REPORT_FILE"

# 验证 JSON 格式
python3 -m json.tool "$REPORT_FILE" > /dev/null && echo "✓ JSON valid" || echo "✗ JSON invalid"

# 查看内容
cat "$REPORT_FILE" | python3 -m json.tool | head -50
```

**验证点**:
- [ ] JSON 格式有效
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.json | head -1)
  python3 -m json.tool "$REPORT_FILE" > /dev/null 2>&1 && echo "✓ Valid JSON" || echo "✗ Invalid JSON"
  ```
- [ ] 包含所有必需字段
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.json | head -1)
  python3 -c "import json; d=json.load(open('$REPORT_FILE')); print('Fields:', list(d.keys()))"
  ```
- [ ] 结构符合预期
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.json | head -1)
  python3 -c "import json; d=json.load(open('$REPORT_FILE')); print('Has source_version:', 'source_version' in d); print('Has target_version:', 'target_version' in d); print('Has check_results:', 'check_results' in d)"
  ```
- [ ] 数据完整
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.json | head -1)
  python3 -c "import json; d=json.load(open('$REPORT_FILE')); print('Source version:', d.get('source_version')); print('Target version:', d.get('target_version')); print('Check results count:', len(d.get('check_results', [])))"
  ```

---

### Phase 7: 集成验证

**目标**: 验证 TiUP 和 precheck 的完整集成

#### Test 7.1: 验证拓扑文件生成

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck

# 检查拓扑文件
TOPOLOGY_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -1)
echo "Topology file: $TOPOLOGY_FILE"
cat "$TOPOLOGY_FILE"
```

**验证点**:
- [ ] 拓扑文件生成
  ```bash
  ls -lh ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml
  ```
- [ ] 包含集群信息
  ```bash
  TOPOLOGY_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -1)
  grep -E "cluster|tidb_servers|tikv_servers|pd_servers" "$TOPOLOGY_FILE" | head -5
  ```
- [ ] 包含源版本信息
  ```bash
  TOPOLOGY_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -1)
  grep -i "version\|v7.5.1" "$TOPOLOGY_FILE" | head -3
  ```
- [ ] 包含所有组件配置
  ```bash
  TOPOLOGY_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -1)
  grep -E "tidb_servers|tikv_servers|pd_servers" "$TOPOLOGY_FILE" | head -5
  ```
- [ ] YAML 格式正确
  ```bash
  TOPOLOGY_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -1)
  python3 -c "import yaml; yaml.safe_load(open('$TOPOLOGY_FILE'))" 2>&1 && echo "✓ Valid YAML" || echo "✗ Invalid YAML"
  ```

#### Test 7.2: 验证命令参数传递

```bash
# 启用详细日志（如果支持）
export TIUP_LOG_LEVEL=debug

./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output html \
  --precheck-tidb-user root 2>&1 | grep -i "executing\|command"
```

**验证点**:
- [ ] 命令包含 --topology-file
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--topology-file"
  ```
- [ ] 命令包含 --target-version
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--target-version.*v8.5.4"
  ```
- [ ] 命令包含 --format
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--format.*html"
  ```
- [ ] 命令包含 --output-dir
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--output-dir"
  ```
- [ ] 命令包含 --source-version
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--source-version"
  ```
- [ ] 命令包含 --tidb-user
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | grep -- "--tidb-user.*root"
  ```
- [ ] 所有参数值正确
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html --precheck-tidb-user root 2>&1 | grep "Executing:" | head -1
  # 手动检查所有参数值是否正确
  ```

#### Test 7.3: 验证工作目录

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "knowledge\|working"
```

**验证点**:
- [ ] 工作目录设置正确
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "working.*directory\|knowledge.*base.*path"
  ```
- [ ] 知识库路径可访问
  ```bash
  ls -d "$TIDB_UPGRADE_PRECHECK_KB" && echo "✓ KB path accessible" || echo "✗ KB path not accessible"
  ```
- [ ] Precheck 可以找到知识库
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck 2>&1 | grep -i "knowledge.*base.*found\|using.*knowledge" && echo "✓ KB found" || echo "✗ KB not found"
  ```

#### Test 7.4: 验证报告路径提取

```bash
./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 \
  --precheck \
  --precheck-output html 2>&1 | grep -i "report generated"
```

**验证点**:
- [ ] 报告路径从输出中提取
  ```bash
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html 2>&1 | grep -E "Report generated|report.*\.html"
  ```
- [ ] 报告文件存在于提取的路径
  ```bash
  REPORT_PATH=$(./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck --precheck-output html 2>&1 | grep -oE "Report generated:.*\.html" | cut -d: -f2 | tr -d ' ')
  if [ -n "$REPORT_PATH" ]; then
    ls -lh "$REPORT_PATH" && echo "✓ Report file exists" || echo "✗ Report file not found"
  fi
  ```
- [ ] 报告文件可读
  ```bash
  REPORT_FILE=$(ls -t ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -1)
  [ -r "$REPORT_FILE" ] && echo "✓ Report file readable" || echo "✗ Report file not readable"
  cat "$REPORT_FILE" | head -10
  ```

---

### Phase 8: 性能测试

**目标**: 验证 precheck 性能

#### Test 8.1: 测量 Precheck 执行时间

```bash
time ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] Precheck 在合理时间内完成（< 2 分钟，小集群）
  ```bash
  time ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
  # 检查 real 时间是否 < 2 分钟
  ```
- [ ] 没有明显的性能问题
  ```bash
  # 在另一个终端监控资源使用
  # top -pid $(pgrep -f "tiup-cluster") -l 1
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
  # 检查是否有明显的延迟或卡顿
  ```

#### Test 8.2: 资源使用

```bash
# 监控资源使用（在另一个终端）
# top -pid $(pgrep -f "tiup-cluster")

./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck
```

**验证点**:
- [ ] 内存使用合理
  ```bash
  # 在另一个终端执行
  # ps aux | grep "tiup-cluster\|upgrade-precheck" | grep -v grep | awk '{print "Memory:", $6/1024 "MB"}'
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck &
  PID=$!
  sleep 5
  ps -p $PID -o rss= | awk '{print "Memory usage:", $1/1024 "MB"}'
  wait $PID
  ```
- [ ] CPU 使用合理
  ```bash
  # 在另一个终端执行
  # top -pid $(pgrep -f "tiup-cluster") -l 1 | grep "CPU usage"
  ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck &
  PID=$!
  sleep 5
  ps -p $PID -o %cpu= | awk '{print "CPU usage:", $1 "%"}'
  wait $PID
  ```
- [ ] 没有资源泄漏
  ```bash
  # 执行多次 precheck，检查资源使用是否稳定
  for i in {1..3}; do
    echo "Run $i:"
    ./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck > /dev/null 2>&1
    sleep 2
  done
  # 检查进程是否都正常退出
  ps aux | grep "tiup-cluster\|upgrade-precheck" | grep -v grep && echo "✗ Process still running" || echo "✓ All processes exited"
  ```

---

## 测试检查清单

### 前置条件
- [ ] TiUP 已安装或构建
- [ ] tidb-upgrade-precheck 已构建
- [ ] 知识库已生成
- [ ] 测试集群已部署并运行
- [ ] 环境变量已设置

### Phase 1: Precheck-Only 模式
- [ ] Test 1.1: 基本 Precheck-Only 测试
- [ ] Test 1.2: Text 格式
- [ ] Test 1.3: HTML 格式
- [ ] Test 1.4: Markdown 格式
- [ ] Test 1.5: 自定义输出文件

### Phase 2: 完整升级流程
- [ ] Test 2.1: 默认升级流程
- [ ] Test 2.2: HTML 报告
- [ ] Test 2.3: 跳过确认

### Phase 3: 凭证和认证
- [ ] Test 3.1: 默认用户
- [ ] Test 3.2: 自定义用户
- [ ] Test 3.3: 密码提示
- [ ] Test 3.4: 密码文件

### Phase 4: 高风险参数配置
- [ ] Test 4.1: 创建配置文件
- [ ] Test 4.2: 使用自定义配置
- [ ] Test 4.3: 使用默认配置位置

### Phase 5: 错误处理
- [ ] Test 5.1: 跳过 Precheck
- [ ] Test 5.2: Precheck 失败但继续
- [ ] Test 5.3: 无效集群名称
- [ ] Test 5.4: 无效目标版本
- [ ] Test 5.5: 缺少二进制路径
- [ ] Test 5.6: 缺少知识库路径

### Phase 6: 报告验证
- [ ] Test 6.1: Text 报告内容
- [ ] Test 6.2: HTML 报告内容
- [ ] Test 6.3: JSON 报告内容

### Phase 7: 集成验证
- [ ] Test 7.1: 拓扑文件生成
- [ ] Test 7.2: 命令参数传递
- [ ] Test 7.3: 工作目录
- [ ] Test 7.4: 报告路径提取

### Phase 8: 性能测试
- [ ] Test 8.1: 执行时间
- [ ] Test 8.2: 资源使用

---

## 测试记录模板

### 测试执行记录

**测试日期**: _______________
**测试人员**: _______________
**测试环境**: _______________

#### Phase 1: Precheck-Only 模式

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 1.1: 基本测试 | [ ] | |
| Test 1.2: Text 格式 | [ ] | |
| Test 1.3: HTML 格式 | [ ] | |
| Test 1.4: Markdown 格式 | [ ] | |
| Test 1.5: 自定义输出文件 | [ ] | |

#### Phase 2: 完整升级流程

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 2.1: 默认升级流程 | [ ] | |
| Test 2.2: HTML 报告 | [ ] | |
| Test 2.3: 跳过确认 | [ ] | |

#### Phase 3: 凭证和认证

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 3.1: 默认用户 | [ ] | |
| Test 3.2: 自定义用户 | [ ] | |
| Test 3.3: 密码提示 | [ ] | |
| Test 3.4: 密码文件 | [ ] | |

#### Phase 4: 高风险参数配置

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 4.1: 创建配置文件 | [ ] | |
| Test 4.2: 使用自定义配置 | [ ] | |
| Test 4.3: 使用默认配置位置 | [ ] | |

#### Phase 5: 错误处理

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 5.1: 跳过 Precheck | [ ] | |
| Test 5.2: Precheck 失败但继续 | [ ] | |
| Test 5.3: 无效集群名称 | [ ] | |
| Test 5.4: 无效目标版本 | [ ] | |
| Test 5.5: 缺少二进制路径 | [ ] | |
| Test 5.6: 缺少知识库路径 | [ ] | |

#### Phase 6: 报告验证

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 6.1: Text 报告内容 | [ ] | |
| Test 6.2: HTML 报告内容 | [ ] | |
| Test 6.3: JSON 报告内容 | [ ] | |

#### Phase 7: 集成验证

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 7.1: 拓扑文件生成 | [ ] | |
| Test 7.2: 命令参数传递 | [ ] | |
| Test 7.3: 工作目录 | [ ] | |
| Test 7.4: 报告路径提取 | [ ] | |

#### Phase 8: 性能测试

| 测试项 | 状态 | 备注 |
|--------|------|------|
| Test 8.1: 执行时间 | [ ] | |
| Test 8.2: 资源使用 | [ ] | |

### 发现的问题

1. **问题描述**: 
   - **复现步骤**: 
   - **预期行为**: 
   - **实际行为**: 
   - **严重程度**: [ ] 严重 [ ] 中等 [ ] 轻微

2. **问题描述**: 
   - **复现步骤**: 
   - **预期行为**: 
   - **实际行为**: 
   - **严重程度**: [ ] 严重 [ ] 中等 [ ] 轻微

### 测试结论

- [ ] 所有测试通过
- [ ] 部分测试通过
- [ ] 测试失败

**总体评价**: 

---

## 清理步骤

测试完成后，清理测试环境：

```bash
# 1. 停止测试集群（如果需要）
tiup cluster stop e2e-test-cluster

# 2. 销毁测试集群（如果需要）
tiup cluster destroy e2e-test-cluster -y

# 3. 清理测试文件
rm -f /tmp/e2e-test-topology.yaml
rm -f /tmp/high_risk_params.json
rm -f /tmp/precheck-*.txt
rm -f /tmp/precheck-report.html

# 4. 清理报告文件（可选）
rm -rf ~/.tiup/storage/cluster/upgrade_precheck/reports/*
rm -rf ~/.tiup/storage/cluster/upgrade_precheck/tmp/*

# 5. 取消设置环境变量（如果需要）
unset TIDB_UPGRADE_PRECHECK_BIN
unset TIDB_UPGRADE_PRECHECK_KB
```

---

## 故障排除

### 问题 0: macOS ARM64 平台部署失败（监控组件不支持）

**症状**: 
- 部署时出现 `Error: component node_exporter doesn't support platform darwin/arm64`
- 部署命令失败，集群未创建
- `tiup cluster list` 显示集群不存在

**原因**: 
TiUP 在 macOS ARM64 平台上不支持 `node_exporter` 和 `blackbox_exporter` 组件。即使拓扑文件中不包含这些组件，TiUP 在某些情况下仍可能尝试下载它们。

**解决方案**:

**方案 1: 使用不包含 TiFlash 的拓扑文件（推荐，先测试基本功能）**
```bash
# 创建不包含 TiFlash 的拓扑文件（TiFlash 可能触发监控组件下载）
# 对于 E2E 测试，pd、tidb、tikv 三个核心组件已经足够
cat > /tmp/e2e-test-topology-no-tiflash.yaml << 'EOF'
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/tmp/tidb-deploy"
  data_dir: "/tmp/tidb-data"

pd_servers:
  - host: 127.0.0.1
    name: pd1
    client_port: 2379
    peer_port: 2380

tidb_servers:
  - host: 127.0.0.1
    port: 4000
    status_port: 10080

tikv_servers:
  - host: 127.0.0.1
    port: 20160
    status_port: 20180
EOF

# 使用不包含 TiFlash 的拓扑文件部署
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology-no-tiflash.yaml -y

# 如果仍然失败，说明 TiUP 会强制下载监控组件
# 这种情况下，需要修改 TiUP 源码或使用 Linux 平台
```

**方案 2: 检查并清理后重试**
```bash
# 1. 检查是否有残留的集群数据
tiup cluster list
ls -la ~/.tiup/storage/cluster/clusters/

# 2. 如果有残留数据，清理
rm -rf ~/.tiup/storage/cluster/clusters/e2e-test-cluster

# 3. 重新部署
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
./bin/tiup-cluster deploy e2e-test-cluster v7.5.1 /tmp/e2e-test-topology.yaml -y
```

**方案 3: 修改 TiUP 源码跳过监控组件（高级方案）**

如果方案 1 和 2 都不行，可以修改 TiUP 源码来跳过监控组件的下载：

```bash
# 1. 找到 TiUP cluster 组件的部署代码
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
grep -r "node_exporter\|blackbox_exporter" components/cluster/

# 2. 修改部署逻辑，在下载组件时跳过不支持的平台
# 具体修改位置需要根据代码结构确定

# 3. 重新编译
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster
```

**方案 4: 使用 Linux 平台（推荐，如果方案 1-3 都不可行）**

如果必须在 macOS ARM64 上测试，可以考虑：
- 使用 Docker 运行 Linux 容器
- 使用虚拟机运行 Linux
- 在 Linux 服务器上执行测试

**方案 5: 临时 workaround - 修改 TiUP 组件清单（实验性）**

如果 TiUP 从远程下载组件清单，可以尝试修改本地缓存：

```bash
# 1. 找到组件清单文件
find ~/.tiup -name "*manifest*" -o -name "*component*" | grep -i "node_exporter\|blackbox"

# 2. 修改清单文件，移除 darwin/arm64 平台的下载要求
# 注意：这是实验性方案，可能不工作
```

**验证部署是否成功**:
```bash
# 检查集群是否真正创建
tiup cluster list | grep "^e2e-test-cluster"

# 如果存在，尝试启动
tiup cluster start e2e-test-cluster

# 检查状态
tiup cluster display e2e-test-cluster
```

### 问题 1: 集群启动失败

**症状**: `tiup cluster start` 失败

**解决方案**:
```bash
# 检查集群状态
tiup cluster display e2e-test-cluster

# 检查日志
tiup cluster audit e2e-test-cluster

# 尝试重启
tiup cluster restart e2e-test-cluster
```

### 问题 2: Precheck 找不到知识库

**症状**: "knowledge base not found"

**解决方案**:
```bash
# 检查环境变量
echo $TIDB_UPGRADE_PRECHECK_KB

# 检查知识库目录
ls -la $TIDB_UPGRADE_PRECHECK_KB

# 重新设置环境变量
export TIDB_UPGRADE_PRECHECK_KB=/path/to/knowledge
```

### 问题 3: Precheck 找不到二进制

**症状**: "binary not found"

**解决方案**:
```bash
# 检查环境变量
echo $TIDB_UPGRADE_PRECHECK_BIN

# 检查二进制文件
ls -la $TIDB_UPGRADE_PRECHECK_BIN

# 重新设置环境变量
export TIDB_UPGRADE_PRECHECK_BIN=/path/to/upgrade-precheck
```

### 问题 4: 报告未生成

**症状**: 报告文件不存在

**解决方案**:
```bash
# 检查输出目录
ls -la ~/.tiup/storage/cluster/upgrade_precheck/reports/

# 检查权限
ls -ld ~/.tiup/storage/cluster/upgrade_precheck/reports/

# 检查日志
cat ~/.tiup/logs/tiup-cluster-debug-*.log | grep -i "report\|error"
```

---

**最后更新**: 2025-12-15

**版本**: 1.0

