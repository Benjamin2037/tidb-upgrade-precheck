# TiDB 参数采集操作步骤指南

## 1. 环境准备

### 1.1 系统要求

- Go 1.18 或更高版本
- Git
- 能够访问 TiDB 源码仓库

### 1.2 目录结构

tidb 和 tidb-upgrade-precheck 两个代码仓库需要放在同一个上级目录下。
例如，如果上级目录是 `sourcecode`，则目录结构应如下所示：

```
sourcecode/
├── tidb/                    # TiDB 源码仓库
└── tidb-upgrade-precheck/   # 本项目
```

要设置此结构，可以按照以下步骤操作：

```bash
# 创建 sourcecode 目录（如果不存在）
mkdir -p sourcecode

# 切换到 sourcecode 目录
cd sourcecode

# 克隆 TiDB 仓库（包含所有 tags 和 branches）
git clone --bare https://github.com/pingcap/tidb.git tidb-bare
git clone tidb-bare tidb
cd tidb
git remote set-url origin https://github.com/pingcap/tidb.git
cd ..

# 克隆 tidb-upgrade-precheck 仓库
git clone https://github.com/pingcap/tidb-upgrade-precheck.git
```

或者，如果您已经有一个 TiDB 克隆，可以获取所有 tags：

```bash
cd tidb
git fetch --all --tags
```

这种结构很重要，因为参数采集工具默认期望在同级目录中找到 TiDB 源码。
如果您的目录结构不同，可以在命令中使用 `--repo` 参数指定 TiDB 源码路径。

### 1.3 环境验证

设置目录结构后，验证两个仓库是否正确克隆：

```bash
# 检查两个目录是否存在
ls -la sourcecode/

# 验证 TiDB 仓库
cd sourcecode/tidb
git status

# 验证 tidb-upgrade-precheck 仓库
cd ../tidb-upgrade-precheck
git status
```

## 2. 构建工具

### 2.1 使用 Make 命令构建

```bash
# 进入 tidb-upgrade-precheck 目录
cd tidb-upgrade-precheck

# 构建 kb-generator 工具
make build
```

构建后的工具将位于 `bin/kb-generator`。

### 2.2 直接运行（无需构建）

也可以直接使用 `go run` 命令运行工具，无需预先构建。

## 3. 参数采集操作

### 3.1 全量采集（推荐）

全量采集会自动处理所有 LTS 版本，跳过已经采集过的版本：

```bash
# 方法1：使用 Make 命令
make collect

# 方法2：直接运行
go run cmd/kb-generator/main.go --all

# 方法3：指定 TiDB 源码路径
go run cmd/kb-generator/main.go --all --repo=/path/to/tidb
```

### 3.2 强制全量采集

强制采集所有版本，包括已经采集过的版本：

```bash
# 方法1：使用 Make 命令
make collect-all

# 方法2：直接运行
go run cmd/kb-generator/main.go --all --skip-generated=false
```

### 3.3 单版本采集

采集指定的单个版本：

```bash
go run cmd/kb-generator/main.go --tag=v8.1.0
```

### 3.4 增量采集

采集指定版本范围内的所有版本：

```bash
go run cmd/kb-generator/main.go --from-tag=v7.5.0 --to-tag=v8.1.0
```

### 3.5 参数历史聚合

将所有版本的参数信息聚合到一个全局历史文件中：

```
# 方法1：使用 Make 命令
make aggregate

# 方法2：直接运行
go run cmd/kb-generator/main.go --aggregate
```

## 4. 输出文件说明

采集完成后，会在 `knowledge` 目录下生成以下文件：

### 4.1 版本特定参数文件

每个版本的参数默认值保存在对应的目录中：

```
knowledge/
├── v6.5.0/
│   └── defaults.json
├── v7.1.0/
│   └── defaults.json
├── v7.5.0/
│   └── defaults.json
└── v8.1.0/
    └── defaults.json
```

### 4.2 参数历史文件

聚合所有版本的参数历史：

```
knowledge/
└── tidb/
    └── parameters-history.json
```

### 4.3 版本管理文件

记录已采集的版本信息：

```
knowledge/
└── generated_versions.json
```

## 5. 验证采集结果

### 5.1 检查输出目录

```bash
ls -la knowledge/
```

### 5.2 检查特定版本文件

```bash
# 查看 v8.1.0 版本的参数
cat knowledge/v8.1.0/defaults.json | jq '.sysvars | keys | length'
```

### 5.3 检查参数历史

```bash
# 查看聚合的参数历史
cat knowledge/tidb/parameters-history.json | jq '.parameters | length'
```

## 6. 常见问题处理

### 6.1 采集过程中断

如果采集过程中断，可以重新运行相同命令，系统会自动跳过已成功采集的版本。

### 6.2 版本采集失败

如果某些版本采集失败，系统会输出警告信息并继续处理其他版本。可以单独重新运行失败的版本：

```bash
go run cmd/kb-generator/main.go --tag=<failed_version>
```

### 6.3 清理采集记录

如果需要重新采集所有版本，可以清理采集记录：

```bash
# 方法1：使用 Make 命令
make clean-generated

# 方法2：直接删除文件
rm knowledge/generated_versions.json
```

### 6.4 Git 权限问题

确保对 TiDB 源码仓库有读取权限，并且 Git 配置正确。

## 7. 高级用法

### 7.1 自定义 TiDB 路径

如果 TiDB 源码不在默认位置，可以使用 `--repo` 参数指定：

```bash
go run cmd/kb-generator/main.go --all --repo=/custom/path/to/tidb
```

### 7.2 调试模式

可以通过增加日志输出来调试采集过程：

```bash
go run cmd/kb-generator/main.go --all --verbose
```

### 7.3 并行处理

目前工具是串行处理各个版本，未来可以考虑实现并行处理以提高效率。

## 8. 最佳实践

### 8.1 定期更新

建议定期运行全量采集，以确保包含最新的 LTS 版本。

### 8.2 版本控制

`knowledge` 目录中的文件不需要加入版本控制，因为它们可以通过工具重新生成。

### 8.3 自动化集成

可以将采集过程集成到 CI/CD 流程中，自动更新参数数据库。

## 9. 性能优化建议

### 9.1 网络优化

确保 TiDB 源码仓库克隆速度快，可以考虑使用本地镜像。

### 9.2 存储优化

确保有足够的磁盘空间用于临时克隆和输出文件。

### 9.3 并行处理

对于大量版本的采集，可以考虑实现并行处理机制。