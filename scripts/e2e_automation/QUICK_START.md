# E2E 自动化测试快速开始

## 一键执行脚本

创建并运行以下脚本可以快速完成所有设置：

```bash
#!/bin/bash
# quick_setup.sh - 快速设置脚本

set -e

echo "=== 步骤 1: 安装依赖 ==="
sudo apt-get update
sudo apt-get install -y git curl wget tar bash ca-certificates rsync sudo \
    build-essential golang openssh-server jq python3 python3-pip vim net-tools

echo "=== 步骤 2: 创建工作目录 ==="
mkdir -p ~/workspace/sourcecode
cd ~/workspace/sourcecode

echo "=== 步骤 3: 克隆代码 ==="
[ -d tidb-upgrade-precheck ] || git clone https://github.com/Benjamin2037/tidb-upgrade-precheck.git
[ -d tiup ] || git clone https://github.com/pingcap/tiup.git

echo "=== 步骤 4: 构建二进制文件 ==="
cd tidb-upgrade-precheck
GOWORK=off make build

cd ../tiup
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster

echo "=== 步骤 5: 设置环境变量 ==="
cat >> ~/.bashrc <<'EOF'
export TIDB_UPGRADE_PRECHECK_BIN=$HOME/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=$HOME/workspace/sourcecode/tidb-upgrade-precheck/knowledge
export PATH=$PATH:$HOME/workspace/sourcecode/tiup/bin
export WORKSPACE=$HOME/workspace/sourcecode
EOF

source ~/.bashrc

echo "=== 步骤 6: 生成测试配置 ==="
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/create_test_config.sh

echo "=== 完成！现在可以运行测试了 ==="
echo "运行: bash scripts/e2e_automation/run_e2e_tests.sh"
```

## 最小化操作步骤

如果你已经准备好了环境，只需要：

```bash
# 1. 进入项目目录
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 2. 生成测试配置（首次运行）
bash scripts/e2e_automation/create_test_config.sh

# 3. 运行测试
bash scripts/e2e_automation/run_e2e_tests.sh

# 4. 查看结果
python3 -m http.server 8000
# 访问: http://vm-ip:8000/doc/tiup/e2e_test_plan_manual.html
```

## 下载结果到本地

```bash
# 在本地机器执行
scp -r user@vm-ip:~/workspace/sourcecode/tidb-upgrade-precheck/test_results/e2e_* ./
scp user@vm-ip:~/workspace/sourcecode/tidb-upgrade-precheck/doc/tiup/e2e_test_plan_manual.html ./
```

