# 在 VM 上编译预检查工具

## 问题说明

如果遇到 `cannot execute binary file: Exec format error` 错误，说明二进制文件是为不同的架构编译的（比如在 macOS 上编译，但在 Linux VM 上运行）。

## 解决方法：在 VM 上重新编译

### 方法1：使用 Makefile（推荐）

```bash
# 在 VM 上进入项目目录
cd /path/to/tidb-upgrade-precheck

# 编译
make upgrade_precheck

# 编译后的二进制文件在 bin/ 目录下
./bin/upgrade-precheck --help
```

### 方法2：直接使用 go build

```bash
# 在 VM 上进入项目目录
cd /path/to/tidb-upgrade-precheck

# 编译
go build -o bin/upgrade-precheck ./cmd/precheck

# 运行
./bin/upgrade-precheck --help
```

### 方法3：交叉编译（在本地为 Linux 编译）

如果你在 macOS 上，想为 Linux VM 编译：

```bash
# 在 macOS 上交叉编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o bin/upgrade-precheck-linux ./cmd/precheck

# 然后传输到 VM
scp bin/upgrade-precheck-linux user@vm:/path/to/vm/
```

## 运行并收集日志

编译完成后，在 VM 上运行并收集日志：

```bash
# 运行并保存日志
./bin/upgrade-precheck \
  --source-version v7.5.1 \
  --target-version v8.1.0 \
  --topology-file topology.yaml \
  --output-format html \
  --output-dir ./reports \
  > precheck.log 2>&1

# 查看关键日志
grep -E "ERROR|WARNING|DEBUG.*loadKBFromRequirements|DEBUG.*GetSourceDefault" precheck.log
```

## 检查编译环境

如果编译失败，检查 Go 环境：

```bash
# 检查 Go 版本（需要 Go 1.18+）
go version

# 检查 Go 环境
go env

# 如果 Go 未安装，安装 Go
# Ubuntu/Debian:
sudo apt-get update
sudo apt-get install golang-go

# CentOS/RHEL:
sudo yum install golang
```

