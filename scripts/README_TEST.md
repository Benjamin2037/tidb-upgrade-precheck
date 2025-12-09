# generate_knowledge.sh Test Suite

## Overview

This document describes the test suite for `generate_knowledge.sh`, which generates knowledge base files for TiDB, PD, TiKV, and TiFlash components. The script supports:

- Automatic repository cloning with sparse checkout
- Dynamic LTS version detection from git tags
- Version-specific file path management
- Fallback path loading from Go code
- Global `upgrade_logic.json` generation from master branch
- Bootstrap version extraction and storage

## Test Script

The test suite is located at `scripts/generate_knowledge_test.sh`.

## Running Tests

```bash
cd scripts
./generate_knowledge_test.sh
```

## Key Features

- **LTS Version Detection**: Automatically detects LTS versions (v6.5.x, v7.1.x, v7.5.x, v8.1.x, v8.5.x) from git tags
- **Version Filtering**: Supports `--start-from` and `--stop-at` options to filter versions
- **Skip Existing**: `--skip-existing` flag to skip versions that already have knowledge base
- **Force Regeneration**: `--force` flag to delete and recreate knowledge directory
- **Concurrent Generation**: `--max-concurrent` option to control parallel processing
- **Global Upgrade Logic**: Generates `upgrade_logic.json` once from master branch before processing versions
- **Bootstrap Version**: Extracts and stores `bootstrap_version` in each `tidb/defaults.json`

## Test Cases

### Test 1: get_required_files Function
**Purpose**: Verify that the `get_required_files` function exists and can be called.

**Steps**:
1. Source the script to load functions
2. Verify `get_required_files` function exists
3. Attempt to call the function

**Expected Result**: Function exists and can be called (may fail if Go tools unavailable, but function should exist).

### Test 2: get_required_files Function
**Purpose**: Verify that the `get_required_files` function exists and can get paths from Go code.

**Steps**:
1. Source the script to load functions
2. Verify `get_required_files` function exists
3. Attempt to call the function

**Expected Result**: Function exists and attempts to load paths from generated JSON or Go code.

### Test 3: Knowledge Directory Creation and Cleanup
**Purpose**: Verify that the script properly creates and cleans up the knowledge directory.

**Steps**:
1. Create an existing knowledge directory with files
2. Run the script with `--force` flag
3. Verify that the old directory was removed and a new one was created

**Expected Result**: Old knowledge directory is removed and a fresh one is created when `--force` is used.

### Test 4: Version Detection from Git Tags
**Purpose**: Verify that the script can detect LTS versions from git repository tags.

**Steps**:
1. Create a mock git repository with version tags (v6.5.0, v7.1.0, etc.)
2. Run the script pointing to the mock repository
3. Verify that version detection messages appear in output

**Expected Result**: Script detects version groups and individual versions from git tags.

### Test 5: Repository Auto-Clone Logic
**Purpose**: Verify that the `ensure_repo` function exists and handles repository cloning logic.

**Steps**:
1. Source the script to load functions
2. Verify `ensure_repo` function exists

**Expected Result**: Function exists and can handle repository cloning (actual cloning is mocked in tests).

### Test 6: All Repositories Exist Locally
**Purpose**: Verify that the script works correctly when all four repositories (TiDB, PD, TiKV, TiFlash) are present locally.

**Steps**:
1. Create mock git repositories for TiDB, PD, TiKV, and TiFlash
2. Run the script with all repositories specified
3. Verify that all repositories are detected and used

**Expected Result**: Script executes successfully and all repositories are recognized as local.

### Test 7: Partial Repositories (Only TiDB Exists)
**Purpose**: Verify that the script handles partial repository availability gracefully.

**Steps**:
1. Create only TiDB repository
2. Run the script with all repositories specified (PD and TiKV missing)
3. Verify that the script continues with available repositories

**Expected Result**: Script shows warnings for missing repositories but continues execution with TiDB.

### Test 8: No Repositories Exist (Should Fail Gracefully)
**Purpose**: Verify that the script fails appropriately when no repositories are available.

**Steps**:
1. Create no repositories
2. Run the script with non-existent repository paths
3. Verify that the script exits with an appropriate error message

**Expected Result**: Script exits with error message indicating at least one repository must be available.

### Test 9: Environment Variable Override
**Purpose**: Verify that custom environment variables override default paths.

**Steps**:
1. Create repositories with custom names
2. Set environment variables to point to custom paths
3. Run the script
4. Verify that custom paths are used

**Expected Result**: Script uses custom paths specified via environment variables.

### Test 10: Script Output Format and Structure
**Purpose**: Verify that the script produces expected output format.

**Steps**:
1. Create mock repositories
2. Run the script
3. Verify that expected output messages appear

**Expected Result**: Script produces expected output including "Starting knowledge base generation" and "Full knowledge base generation completed" messages.

### Test 11: Global Upgrade Logic Generation
**Purpose**: Verify that `upgrade_logic.json` is generated once from master branch.

**Steps**:
1. Create mock TiDB repository with master branch
2. Run the script
3. Verify that `upgrade_logic.json` is generated before version processing
4. Verify that it's generated from master branch

**Expected Result**: `upgrade_logic.json` is generated once globally from master branch.

### Test 12: Skip Existing Versions
**Purpose**: Verify that `--skip-existing` flag correctly skips versions that already have knowledge base.

**Steps**:
1. Create knowledge base for a specific version
2. Run the script with `--skip-existing` flag
3. Verify that existing version is skipped

**Expected Result**: Existing versions are skipped when `--skip-existing` is used.

### Test 13: Force Regeneration
**Purpose**: Verify that `--force` flag deletes and recreates knowledge directory.

**Steps**:
1. Create existing knowledge directory with files
2. Run the script with `--force` flag
3. Verify that directory is deleted and recreated

**Expected Result**: Knowledge directory is completely recreated when `--force` is used.

### Test 14: Bootstrap Version Extraction
**Purpose**: Verify that `bootstrap_version` is extracted and stored in `tidb/defaults.json`.

**Steps**:
1. Run the script to generate knowledge base
2. Check that `tidb/defaults.json` contains `bootstrap_version` field
3. Verify that the value is correct

**Expected Result**: All `tidb/defaults.json` files contain `bootstrap_version` field.

## Test Implementation Details

### Mock Components

The test suite uses several mock components:

1. **Mock kb-generator**: A Go program that validates command-line arguments without actually generating knowledge base files. This allows testing the shell script logic without requiring actual TiDB/PD/TiKV source code.

2. **Mock Git Repositories**: Test repositories created with `git init` and populated with version tags. These simulate real repositories for version detection testing.

### Test Isolation

Each test:
- Runs in its own temporary directory
- Creates isolated mock repositories
- Cleans up after itself
- Does not interfere with other tests

### Test Output

The test suite provides:
- Color-coded output (green for pass, red for fail)
- Detailed error messages for failed tests
- Summary statistics at the end

### CI/CD Integration

These tests can be integrated into CI/CD pipelines:

```yaml
- name: Test generate_knowledge.sh
  run: |
    cd scripts
    ./generate_knowledge_test.sh
```

## Usage Examples

### Generate All LTS Versions
```bash
bash scripts/generate_knowledge.sh
```

### Generate Specific Versions
```bash
bash scripts/generate_knowledge.sh --versions=versions.txt
```

### Skip Existing Versions
```bash
bash scripts/generate_knowledge.sh --skip-existing
```

### Force Regeneration
```bash
bash scripts/generate_knowledge.sh --force
```

### Generate with Concurrency Control
```bash
bash scripts/generate_knowledge.sh --max-concurrent=3
```

### Generate from Specific Version
```bash
bash scripts/generate_knowledge.sh --start-from=v7.5.0
```

### Generate up to Specific Version
```bash
bash scripts/generate_knowledge.sh --stop-at=v8.1.0
```

## Troubleshooting

### Test Failures

If tests fail:
1. Check that the test script has execute permissions: `chmod +x generate_knowledge_test.sh`
2. Ensure git is installed (required for mock repository creation)
3. Check test output for specific error messages
4. Verify that Go is installed (for mock kb-generator)

### Limitations

Current test limitations:
- Does not test actual GitHub cloning (uses mock repositories)
- Does not test actual sparse checkout (uses local repositories)
- Mock kb-generator does not actually generate knowledge base files
- Version detection tests use mock repositories with limited tags

### Future Improvements

Potential improvements to the test suite:
- Integration tests with actual repositories (requires TiDB/PD/TiKV source)
- Performance tests for large knowledge base generation
- Cross-platform compatibility tests (Linux, macOS, Windows)
- Edge case testing (special characters in paths, very long paths, etc.)
- Testing actual sparse checkout functionality
- Testing actual GitHub API interactions
- Testing bootstrap_version extraction accuracy
- Testing upgrade_logic.json content validation
