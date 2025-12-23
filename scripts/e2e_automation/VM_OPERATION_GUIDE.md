# 虚拟机 E2E 自动化测试操作指南

本文档提供在虚拟机上执行 E2E 自动化测试的完整操作步骤。

## 前置条件

- 已创建并启动 Linux 虚拟机（Ubuntu/Debian/CentOS）
- 虚拟机可以访问互联网（用于克隆代码）
- 已配置 SSH 访问（可选，用于从本地下载结果）

## 步骤 1: 连接到虚拟机

```bash
# 如果使用 SSH 连接
ssh user@vm-ip-address

# 或者直接在虚拟机控制台操作
```

## 步骤 2: 准备环境

### 2.1 安装基础依赖

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y git curl wget tar bash ca-certificates rsync sudo \
    build-essential golang openssh-server jq python3 python3-pip \
    vim net-tools

# CentOS/RHEL
sudo yum install -y git curl wget tar bash ca-certificates rsync sudo \
    gcc gcc-c++ make golang openssh-server jq python3 python3-pip \
    vim net-tools
```

### 2.2 创建工作目录

```bash
mkdir -p ~/workspace/sourcecode
cd ~/workspace/sourcecode
```

### 2.3 克隆代码仓库

```bash
# 克隆 tidb-upgrade-precheck
git clone https://github.com/Benjamin2037/tidb-upgrade-precheck.git
# 或者使用 SSH（如果已配置）
# git clone git@github.com:Benjamin2037/tidb-upgrade-precheck.git

# 克隆 tiup（如果需要）
git clone https://github.com/pingcap/tiup.git
# 或者使用 SSH
# git clone git@github.com:pingcap/tiup.git
```

## 步骤 3: 构建二进制文件

### 3.1 构建 tidb-upgrade-precheck

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
GOWORK=off make build

# 验证构建成功
ls -lh bin/upgrade-precheck
./bin/upgrade-precheck --help | head -5
```

### 3.2 构建 tiup-cluster（如果需要）

```bash
cd ~/workspace/sourcecode/tiup
GOWORK=off go build -ldflags '-w -s' -o bin/tiup-cluster ./components/cluster

# 验证构建成功
ls -lh bin/tiup-cluster
./bin/tiup-cluster --help | head -5
```

## 步骤 4: 设置环境变量

```bash
# 编辑 ~/.bashrc
cat >> ~/.bashrc <<'EOF'

# TiDB Upgrade Precheck E2E Test Environment
export TIDB_UPGRADE_PRECHECK_BIN=$HOME/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=$HOME/workspace/sourcecode/tidb-upgrade-precheck/knowledge
export PATH=$PATH:$HOME/workspace/sourcecode/tiup/bin
export WORKSPACE=$HOME/workspace/sourcecode
EOF

# 使环境变量生效
source ~/.bashrc

# 验证环境变量
echo "TIDB_UPGRADE_PRECHECK_BIN: $TIDB_UPGRADE_PRECHECK_BIN"
echo "TIDB_UPGRADE_PRECHECK_KB: $TIDB_UPGRADE_PRECHECK_KB"
```

## 步骤 5: 生成知识库（如果还没有）

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 生成知识库（示例：生成 v7.5.0 到 v8.5.0）
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.5.0

# 验证知识库存在
ls -d knowledge/v7.5/v7.5.0
ls -d knowledge/v8.5/v8.5.0
```

## 步骤 6: 准备测试集群（如果需要）

```bash
# 如果还没有测试集群，可以使用 tiup playground 创建一个
cd ~/workspace/sourcecode/tiup
./bin/tiup playground v7.5.0 --db 1 --pd 1 --kv 1 --tiflash 0

# 或者使用已有的集群
# 确保集群名称是 e2e-test-cluster
```

## 步骤 7: 生成测试配置

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 从测试计划文档生成测试配置
bash scripts/e2e_automation/create_test_config.sh

# 验证配置文件已生成
ls -lh scripts/e2e_automation/test_config.json
```

## 步骤 8: 运行自动化测试

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 运行所有测试
bash scripts/e2e_automation/run_e2e_tests.sh

# 或者指定自定义配置文件
# bash scripts/e2e_automation/run_e2e_tests.sh /path/to/custom_test_config.json
```

### 测试执行过程

测试脚本会：
1. 自动执行所有测试用例
2. 收集测试结果和日志
3. 生成 JSON 格式的测试结果
4. 生成独立的 HTML 测试报告
5. **自动更新测试计划 HTML，集成测试结果**

### 查看测试进度

测试运行过程中，你可以：
- 查看实时输出了解测试进度
- 在另一个终端查看测试日志：
  ```bash
  tail -f test_results/e2e_*/logs/*.log
  ```

## 步骤 9: 查看测试结果

### 9.1 查看测试结果目录

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 列出所有测试运行
ls -lht test_results/

# 进入最新的测试结果目录
cd test_results/e2e_$(ls -t test_results/ | grep e2e_ | head -1 | cut -d'_' -f2-)
```

### 9.2 查看测试摘要

```bash
# 查看测试摘要（JSON）
cat summary.json | jq .

# 或者查看简要统计
cat summary.json | jq '.statistics'
```

### 9.3 查看测试计划 HTML（集成结果）

```bash
# 在浏览器中打开（如果虚拟机有图形界面）
xdg-open doc/tiup/e2e_test_plan_manual.html

# 或者使用 Python 启动简单 HTTP 服务器
cd ~/workspace/sourcecode/tidb-upgrade-precheck
python3 -m http.server 8000

# 然后在本地浏览器访问: http://vm-ip-address:8000/doc/tiup/e2e_test_plan_manual.html
```

### 9.4 查看独立测试报告

```bash
# 查看独立的测试报告 HTML
python3 -m http.server 8001

# 访问: http://vm-ip-address:8001/test_results/e2e_YYYYMMDD_HHMMSS/report.html
```

## 步骤 10: 下载测试结果到本地

### 方法 1: 使用 scp（推荐）

在**本地机器**上执行：

```bash
# 下载整个测试结果目录
scp -r user@vm-ip:/home/user/workspace/sourcecode/tidb-upgrade-precheck/test_results/e2e_* ./local_test_results/

# 下载更新后的测试计划 HTML
scp user@vm-ip:/home/user/workspace/sourcecode/tidb-upgrade-precheck/doc/tiup/e2e_test_plan_manual.html ./local_test_results/
```

### 方法 2: 使用 rsync

在**本地机器**上执行：

```bash
# 同步测试结果
rsync -avz user@vm-ip:/home/user/workspace/sourcecode/tidb-upgrade-precheck/test_results/ ./local_test_results/

# 同步测试计划 HTML
rsync -avz user@vm-ip:/home/user/workspace/sourcecode/tidb-upgrade-precheck/doc/tiup/e2e_test_plan_manual.html ./local_test_results/
```

### 方法 3: 打包下载

在**虚拟机**上执行：

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck

# 打包测试结果
tar -czf e2e_test_results.tar.gz test_results/e2e_* doc/tiup/e2e_test_plan_manual.html

# 查看打包文件
ls -lh e2e_test_results.tar.gz
```

然后在**本地机器**上：

```bash
# 下载打包文件
scp user@vm-ip:/home/user/workspace/sourcecode/tidb-upgrade-precheck/e2e_test_results.tar.gz ./

# 解压
tar -xzf e2e_test_results.tar.gz
```

## 步骤 11: 在本地查看结果

### 11.1 打开测试计划 HTML

```bash
# macOS
open local_test_results/e2e_test_plan_manual.html

# Linux
xdg-open local_test_results/e2e_test_plan_manual.html

# Windows
start local_test_results/e2e_test_plan_manual.html
```

### 11.2 查看测试结果

在 HTML 页面中，你可以：
- **查看测试摘要**：页面顶部显示总体统计
- **查看每个测试的状态**：每个测试项显示执行状态徽章
- **查看验证点结果**：每个验证点显示执行结果
- **查看日志**：点击"查看日志"链接查看详细日志
- **继续手动测试**：使用复选框和备注功能跟踪手动测试进度

## 故障排查

### 问题 1: 测试执行失败

```bash
# 查看测试日志
cat test_results/e2e_*/logs/test_X.X.log

# 检查环境变量
echo $TIDB_UPGRADE_PRECHECK_BIN
echo $TIDB_UPGRADE_PRECHECK_KB

# 验证二进制文件
ls -lh $TIDB_UPGRADE_PRECHECK_BIN
ls -d $TIDB_UPGRADE_PRECHECK_KB
```

### 问题 2: 知识库不存在

```bash
# 检查知识库目录
ls -d knowledge/v*/v*

# 如果不存在，重新生成
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.5.0
```

### 问题 3: 测试配置生成失败

```bash
# 检查测试计划文档是否存在
ls -lh doc/tiup/e2e_test_plan_manual.md

# 手动运行配置生成脚本查看错误
bash scripts/e2e_automation/create_test_config.sh
```

### 问题 4: HTML 页面无法显示结果

```bash
# 检查测试结果文件是否存在
ls -lh test_results/e2e_*/results.json
ls -lh test_results/e2e_*/summary.json

# 手动重新生成 HTML
python3 scripts/generate_test_plan_html.py \
    --input doc/tiup/e2e_test_plan_manual.md \
    --results test_results/e2e_*/results.json \
    --summary test_results/e2e_*/summary.json \
    --output doc/tiup/e2e_test_plan_manual.html
```

## 快速参考命令

```bash
# 完整流程（一键执行）
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/create_test_config.sh
bash scripts/e2e_automation/run_e2e_tests.sh

# 查看最新测试结果
cd test_results/$(ls -t test_results/ | grep e2e_ | head -1)
cat summary.json | jq '.statistics'

# 启动 HTTP 服务器查看 HTML
python3 -m http.server 8000
# 访问: http://vm-ip:8000/doc/tiup/e2e_test_plan_manual.html
```

## 注意事项

1. **环境变量**：确保每次新开终端时都 source ~/.bashrc 或重新设置环境变量
2. **知识库**：确保知识库已生成，否则测试会失败
3. **测试集群**：确保测试集群已部署并运行（如果测试需要）
4. **网络连接**：确保虚拟机可以访问互联网（用于克隆代码和下载依赖）
5. **磁盘空间**：确保有足够的磁盘空间存储测试结果和日志

## 下一步

测试完成后，你可以：
1. 分析失败的测试用例
2. 查看详细日志找出问题原因
3. 修复问题后重新运行测试
4. 将测试结果分享给团队
5. 将测试结果集成到 CI/CD 流程中

