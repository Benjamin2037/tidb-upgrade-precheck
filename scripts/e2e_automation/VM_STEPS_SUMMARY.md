# 虚拟机操作步骤总结

## 📋 完整操作流程

### 1️⃣ 连接虚拟机
```bash
ssh user@vm-ip-address
```

### 2️⃣ 安装依赖
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y git curl wget tar bash ca-certificates rsync sudo \
    build-essential golang openssh-server jq python3 python3-pip vim net-tools
```

### 3️⃣ 克隆代码
```bash
mkdir -p ~/workspace/sourcecode
cd ~/workspace/sourcecode
git clone https://github.com/Benjamin2037/tidb-upgrade-precheck.git
git clone https://github.com/pingcap/tiup.git
```

### 4️⃣ 构建二进制
```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
GOWORK=off make build

cd ../tiup
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster
```

### 5️⃣ 设置环境变量
```bash
cat >> ~/.bashrc <<'EOF'
export TIDB_UPGRADE_PRECHECK_BIN=$HOME/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=$HOME/workspace/sourcecode/tidb-upgrade-precheck/knowledge
export PATH=$PATH:$HOME/workspace/sourcecode/tiup/bin
export WORKSPACE=$HOME/workspace/sourcecode
EOF

source ~/.bashrc
```

### 6️⃣ 生成知识库（如需要）
```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.5.0
```

### 7️⃣ 生成测试配置
```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/create_test_config.sh
```

### 8️⃣ 运行自动化测试
```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/run_e2e_tests.sh
```

### 9️⃣ 查看结果
```bash
# 启动 HTTP 服务器
cd ~/workspace/sourcecode/tidb-upgrade-precheck
python3 -m http.server 8000

# 访问: http://vm-ip:8000/doc/tiup/e2e_test_plan_manual.html
```

### 🔟 下载结果到本地
```bash
# 在本地机器执行
scp -r user@vm-ip:~/workspace/sourcecode/tidb-upgrade-precheck/test_results/e2e_* ./
scp user@vm-ip:~/workspace/sourcecode/tidb-upgrade-precheck/doc/tiup/e2e_test_plan_manual.html ./
```

## 🎯 关键文件位置

- **测试结果**: `~/workspace/sourcecode/tidb-upgrade-precheck/test_results/e2e_YYYYMMDD_HHMMSS/`
- **测试计划 HTML**: `~/workspace/sourcecode/tidb-upgrade-precheck/doc/tiup/e2e_test_plan_manual.html`
- **测试配置**: `~/workspace/sourcecode/tidb-upgrade-precheck/scripts/e2e_automation/test_config.json`
- **测试日志**: `~/workspace/sourcecode/tidb-upgrade-precheck/test_results/e2e_*/logs/`

## ⚡ 快速命令

```bash
# 一键运行测试
cd ~/workspace/sourcecode/tidb-upgrade-precheck && \
bash scripts/e2e_automation/create_test_config.sh && \
bash scripts/e2e_automation/run_e2e_tests.sh

# 查看最新测试结果统计
cd test_results/$(ls -t test_results/ | grep e2e_ | head -1) && \
cat summary.json | jq '.statistics'
```

