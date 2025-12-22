# 如何收集预检查工具的调试日志

## 重要提示

如果遇到 `cannot execute binary file: Exec format error` 错误，说明二进制文件架构不匹配。需要在 VM 上重新编译：

```bash
# 在 VM 上编译
cd /path/to/tidb-upgrade-precheck
make upgrade_precheck
# 或
go build -o bin/upgrade-precheck ./cmd/precheck
```

详细说明请参考 `BUILD_ON_VM.md`。

## 正确的运行命令

**注意：参数名是 `--format` 而不是 `--output-format`**

```bash
# 正确的命令
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --format html \
  --output-dir ./reports \
  > precheck.log 2>&1
```

## 日志输出位置

所有调试日志都输出到**标准输出（stdout）**和**标准错误（stderr）**，没有专门的日志文件。

## 收集日志的方法

### 方法1：重定向所有输出到文件（推荐）

```bash
# 同时捕获 stdout 和 stderr 到同一个文件
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --format html \
  --output-dir ./reports \
  > precheck.log 2>&1
```

或者分别保存：

```bash
# stdout 和 stderr 分别保存
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --format html \
  --output-dir ./reports \
  > precheck_stdout.log 2> precheck_stderr.log
```

### 方法2：使用 tee 同时显示和保存

```bash
# 同时显示在终端和保存到文件
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --format html \
  --output-dir ./reports \
  2>&1 | tee precheck.log
```

### 方法3：如果通过 TiUP 运行

```bash
# TiUP 通常会捕获输出，但可以手动重定向
tiup cluster upgrade-precheck <cluster-name> v8.1.0 \
  > precheck.log 2>&1
```

## 关键调试日志标识

查找以下关键日志信息：

### 知识库加载相关
- `[DEBUG] Using knowledge base path: ...` - 知识库路径
- `[DEBUG loadKBFromRequirements] Loaded X config defaults for component tikv` - 加载的参数数量
- `[WARNING loadKBFromRequirements] Critical parameter 'xxx' not found` - 参数未找到警告
- `[ERROR loadKBFromRequirements] Parameter 'xxx' exists in source KB but was not loaded!` - 参数存在但未加载的错误

### 组件和参数查找相关
- `[DEBUG GetSourceDefault] Component 'xxx' not found in SourceDefaults` - 组件未找到
- `[DEBUG GetSourceDefault] Parameter 'xxx' not found in component 'xxx'` - 参数未找到
- `[DEBUG GetSourceDefault] Component 'xxx' has X parameters` - 组件参数数量

### 规则执行相关
- `[ERROR rule_upgrade_differences] sourceDefaults[tikv] is nil or empty!` - 源知识库未加载
- `[DEBUG rule_upgrade_differences] sourceDefaults[tikv] has X parameters` - 源知识库参数数量

## 日志文件位置

运行后，日志文件会在你执行命令的目录下：
- `precheck.log` - 如果使用重定向
- `precheck_stdout.log` - 标准输出
- `precheck_stderr.log` - 标准错误

## 快速检查命令

```bash
# 运行并保存日志
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --format html \
  --output-dir ./reports \
  > precheck_$(date +%Y%m%d_%H%M%S).log 2>&1

# 查看日志中的关键错误
grep -E "ERROR|WARNING" precheck_*.log

# 查看知识库加载相关日志
grep -E "loadKBFromRequirements|GetSourceDefault" precheck_*.log

# 查看特定参数的日志
grep "raftdb.defaultcf.titan.min-blob-size" precheck_*.log
```
