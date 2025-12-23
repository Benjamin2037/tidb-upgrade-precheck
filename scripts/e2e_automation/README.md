# E2E 自动化测试框架

这个目录包含了将手动 E2E 测试自动化的完整框架，支持在虚拟机上运行测试、收集结果并生成可视化报告。

## 目录结构

```
e2e_automation/
├── README.md                    # 本文档
├── setup_vm.sh                  # 虚拟机环境准备脚本
├── run_e2e_tests.sh             # 主测试执行脚本
├── create_test_config.sh        # 从测试计划生成测试配置
├── generate_test_report.py     # 生成 HTML 测试报告
└── test_config.json             # 测试配置文件（自动生成）
```

## 快速开始

### 1. 准备虚拟机环境

在虚拟机或远程服务器上运行环境准备脚本：

```bash
# 克隆代码
cd ~/workspace/sourcecode
git clone https://github.com/Benjamin2037/tidb-upgrade-precheck.git

# 运行环境准备脚本
cd tidb-upgrade-precheck
bash scripts/e2e_automation/setup_vm.sh
```

这个脚本会：
- 检测操作系统并安装依赖
- 克隆/更新 tidb-upgrade-precheck 和 tiup 代码
- 构建必要的二进制文件
- 设置环境变量
- 验证环境配置

### 2. 生成测试配置

从测试计划文档生成测试配置文件：

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/create_test_config.sh
```

这会从 `doc/tiup/e2e_test_plan_manual.md` 解析测试用例并生成 `test_config.json`。

### 3. 运行测试

执行自动化测试：

```bash
cd ~/workspace/sourcecode/tidb-upgrade-precheck
bash scripts/e2e_automation/run_e2e_tests.sh
```

测试结果会保存在 `test_results/e2e_YYYYMMDD_HHMMSS/` 目录下。

### 4. 查看测试报告

测试完成后，会生成 HTML 报告：

```bash
# 在本地浏览器中打开
open test_results/e2e_YYYYMMDD_HHMMSS/report.html

# 或使用 Python 简单 HTTP 服务器
cd test_results/e2e_YYYYMMDD_HHMMSS
python3 -m http.server 8000
# 然后在浏览器中访问 http://your-vm-ip:8000/report.html
```

## 测试配置文件格式

`test_config.json` 的格式如下：

```json
{
  "version": "1.0",
  "description": "E2E test configuration",
  "tests": [
    {
      "id": "test_1.1",
      "name": "基本 Precheck-Only 测试",
      "phase": "1",
      "command": "./bin/tiup-cluster upgrade e2e-test-cluster v8.5.4 --precheck",
      "checkpoints": [
        {
          "text": "命令执行成功",
          "command": "echo $? | grep -q '^0$'"
        },
        {
          "text": "报告文件生成",
          "command": "ls -f ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html"
        }
      ]
    }
  ]
}
```

## 测试结果结构

每次测试运行会生成以下结构：

```
test_results/
└── e2e_YYYYMMDD_HHMMSS/
    ├── results.json          # 详细测试结果（JSON）
    ├── summary.json          # 测试摘要（JSON）
    ├── report.html           # HTML 可视化报告
    ├── logs/                 # 测试日志
    │   ├── test_1.1.log
    │   ├── test_1.2.log
    │   └── ...
    ├── reports/              # 测试生成的报告文件
    └── artifacts/            # 其他测试产物
```

## 下载测试结果

### 方法 1: 使用 scp

```bash
# 从虚拟机下载测试结果
scp -r user@vm-ip:/path/to/tidb-upgrade-precheck/test_results/e2e_YYYYMMDD_HHMMSS ./test_results/
```

### 方法 2: 使用 rsync

```bash
# 同步测试结果
rsync -avz user@vm-ip:/path/to/tidb-upgrade-precheck/test_results/ ./test_results/
```

### 方法 3: 打包下载

在虚拟机上：

```bash
cd test_results
tar -czf e2e_YYYYMMDD_HHMMSS.tar.gz e2e_YYYYMMDD_HHMMSS/
```

然后下载 tar.gz 文件。

## 报告展示

HTML 报告包含：

1. **测试摘要**
   - 总测试数、通过数、失败数、跳过数
   - 通过率
   - 测试持续时间

2. **按阶段分组的测试结果**
   - 每个测试的状态（通过/失败）
   - 测试执行时间
   - 验证点检查结果
   - 日志文件链接

3. **详细信息**
   - 每个测试的命令
   - 验证点的执行结果
   - 错误日志链接

## 自定义测试配置

你可以手动编辑 `test_config.json` 来：

- 添加新的测试用例
- 修改测试命令
- 调整验证点
- 跳过某些测试

然后运行：

```bash
bash scripts/e2e_automation/run_e2e_tests.sh test_config.json
```

## 持续集成

可以将此框架集成到 CI/CD 流程中：

```yaml
# GitHub Actions 示例
- name: Run E2E Tests
  run: |
    bash scripts/e2e_automation/setup_vm.sh
    bash scripts/e2e_automation/create_test_config.sh
    bash scripts/e2e_automation/run_e2e_tests.sh
    
- name: Upload Test Results
  uses: actions/upload-artifact@v3
  with:
    name: e2e-test-results
    path: test_results/
```

## 故障排查

### 测试失败

1. 查看测试日志：`test_results/e2e_YYYYMMDD_HHMMSS/logs/test_X.X.log`
2. 检查环境变量是否正确设置
3. 确认二进制文件存在且可执行
4. 验证知识库是否已生成

### 环境问题

1. 重新运行 `setup_vm.sh` 来修复环境
2. 检查依赖是否完整安装
3. 确认网络连接正常（用于克隆代码）

### 报告生成失败

1. 确保 Python3 已安装
2. 检查 JSON 文件格式是否正确
3. 查看 Python 脚本的错误输出

## 扩展功能

### 添加新的测试用例

1. 在 `e2e_test_plan_manual.md` 中添加测试用例
2. 运行 `create_test_config.sh` 重新生成配置
3. 或手动编辑 `test_config.json`

### 自定义报告格式

修改 `generate_test_report.py` 来：
- 添加新的统计信息
- 改变报告样式
- 添加图表和可视化

### 集成其他测试工具

可以扩展框架来支持：
- 性能测试
- 压力测试
- 兼容性测试

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个自动化测试框架！

