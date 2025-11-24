## 1. Environment Preparation

### 1.1 System Requirements

- Go 1.18 or higher
- Git
- Access to TiDB source code repository

### 1.2 Directory Structure

Ensure the project directory structure is as follows:

```
sourcecode/
├── tidb/                    # TiDB source code repository
└── tidb-upgrade-precheck/   # This project# TiDB Parameter Collection Operation Guide


```

If the directory structure is different, you can specify the TiDB source code path using the `--repo` parameter in commands.

## 2. Building the Tool

### 2.1 Using Make Command to Build

```bash
# Enter tidb-upgrade-precheck directory
cd tidb-upgrade-precheck

# Build kb-generator tool
make build
```

The built tool will be located at `bin/kb-generator`.

### 2.2 Direct Execution (No Build Required)

You can also directly use the `go run` command to run the tool without pre-building.

## 3. Parameter Collection Operations

### 3.1 Full Collection (Recommended)

Full collection automatically processes all LTS versions, skipping already collected versions:

```bash
# Method 1: Using Make command
make collect

# Method 2: Direct execution
go run cmd/kb-generator/main.go --all

# Method 3: Specify TiDB source code path
go run cmd/kb-generator/main.go --all --repo=/path/to/tidb
```

### 3.2 Force Full Collection

Force collection of all versions, including already collected versions:

```bash
# Method 1: Using Make command
make collect-all

# Method 2: Direct execution
go run cmd/kb-generator/main.go --all --skip-generated=false
```

### 3.3 Single Version Collection

Collect a specified single version:

```bash
go run cmd/kb-generator/main.go --tag=v8.1.0
```

### 3.4 Incremental Collection

Collect all versions within a specified version range:

```bash
go run cmd/kb-generator/main.go --from-tag=v7.5.0 --to-tag=v8.1.0
```

### 3.5 Parameter History Aggregation

Aggregate parameter information from all versions into a global history file:

```bash
# Method 1: Using Make command
make aggregate

# Method 2: Direct execution
go run cmd/kb-generator/main.go --aggregate
```

## 4. Output File Description

After collection is complete, the following files will be generated in the `knowledge` directory:

### 4.1 Version-Specific Parameter Files

Default parameter values for each version are saved in the corresponding directory:

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

### 4.2 Parameter History File

Aggregate parameter history across all versions:

```
knowledge/
└── tidb/
    └── parameters-history.json
```

### 4.3 Version Management File

Record information about collected versions:

```
knowledge/
└── generated_versions.json
```

## 5. Verifying Collection Results

### 5.1 Check Output Directory

```bash
ls -la knowledge/
```

### 5.2 Check Specific Version Files

```bash
# View parameters for v8.1.0 version
cat knowledge/v8.1.0/defaults.json | jq '.sysvars | keys | length'
```

### 5.3 Check Parameter History

```bash
# View aggregated parameter history
cat knowledge/tidb/parameters-history.json | jq '.parameters | length'
```

## 6. Common Issue Handling

### 6.1 Collection Process Interrupted

If the collection process is interrupted, you can re-run the same command, and the system will automatically skip successfully collected versions.

### 6.2 Version Collection Failed

If some versions fail to collect, the system will output warning information and continue processing other versions. You can re-run the failed version separately:

```bash
go run cmd/kb-generator/main.go --tag=<failed_version>
```

### 6.3 Clean Collection Records

If you need to recollect all versions, you can clean the collection records:

```bash
# Method 1: Using Make command
make clean-generated

# Method 2: Direct file deletion
rm knowledge/generated_versions.json
```

### 6.4 Git Permission Issues

Ensure you have read access to the TiDB source code repository and that Git configuration is correct.

## 7. Advanced Usage

### 7.1 Custom TiDB Path

If the TiDB source code is not in the default location, you can specify it using the `--repo` parameter:

```bash
go run cmd/kb-generator/main.go --all --repo=/custom/path/to/tidb
```

### 7.2 Debug Mode

You can increase log output to debug the collection process:

```bash
go run cmd/kb-generator/main.go --all --verbose
```

### 7.3 Parallel Processing

Currently, the tool processes versions serially. Parallel processing can be considered in the future to improve efficiency.

## 8. Best Practices

### 8.1 Regular Updates

It is recommended to run full collection regularly to ensure the latest LTS versions are included.

### 8.2 Version Control

Files in the `knowledge` directory do not need to be added to version control, as they can be regenerated through the tool.

### 8.3 Automation Integration

The collection process can be integrated into CI/CD pipelines to automatically update the parameter database.

## 9. Performance Optimization Recommendations

### 9.1 Network Optimization

Ensure fast cloning of the TiDB source code repository. Consider using a local mirror.

### 9.2 Storage Optimization

Ensure sufficient disk space for temporary clones and output files.

### 9.3 Parallel Processing

For collecting large numbers of versions, consider implementing a parallel processing mechanism.