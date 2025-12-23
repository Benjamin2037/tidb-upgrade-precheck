#!/bin/bash
# E2E 自动化测试脚本
# 用于在虚拟机上自动执行 e2e 测试计划中的所有测试用例

set -euo pipefail

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_RESULTS_DIR="${TEST_RESULTS_DIR:-$PROJECT_ROOT/test_results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_RUN_ID="e2e_${TIMESTAMP}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# 创建测试结果目录
mkdir -p "$TEST_RESULTS_DIR/$TEST_RUN_ID"
mkdir -p "$TEST_RESULTS_DIR/$TEST_RUN_ID/logs"
mkdir -p "$TEST_RESULTS_DIR/$TEST_RUN_ID/reports"
mkdir -p "$TEST_RESULTS_DIR/$TEST_RUN_ID/artifacts"

# 测试结果文件
RESULTS_JSON="$TEST_RESULTS_DIR/$TEST_RUN_ID/results.json"
SUMMARY_JSON="$TEST_RESULTS_DIR/$TEST_RUN_ID/summary.json"

# 初始化结果文件
cat > "$RESULTS_JSON" <<EOF
{
  "test_run_id": "$TEST_RUN_ID",
  "start_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "environment": {
    "hostname": "$(hostname)",
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "go_version": "$(go version 2>/dev/null || echo 'N/A')"
  },
  "tests": []
}
EOF

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# 测试执行函数
run_test() {
    local test_id="$1"
    local test_name="$2"
    local test_command="$3"
    local test_checkpoints="$4"  # JSON array of checkpoint objects
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    local test_log="$TEST_RESULTS_DIR/$TEST_RUN_ID/logs/${test_id}.log"
    local test_result="running"
    local test_duration=0
    local start_time=$(date +%s)
    local checkpoint_results=()
    
    log_info "Running test: $test_name (ID: $test_id)"
    
    # 执行测试命令
    if eval "$test_command" > "$test_log" 2>&1; then
        test_result="passed"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        log_info "✓ Test $test_id passed"
    else
        test_result="failed"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log_error "✗ Test $test_id failed (see $test_log)"
    fi
    
    local end_time=$(date +%s)
    test_duration=$((end_time - start_time))
    
    # 执行验证点检查
    if [ -n "$test_checkpoints" ] && [ "$test_checkpoints" != "null" ]; then
        local checkpoint_count=$(echo "$test_checkpoints" | jq '. | length' 2>/dev/null || echo "0")
        for ((i=0; i<checkpoint_count; i++)); do
            local checkpoint=$(echo "$test_checkpoints" | jq -r ".[$i]")
            local cp_text=$(echo "$checkpoint" | jq -r '.text // .' 2>/dev/null || echo "$checkpoint")
            local cp_command=$(echo "$checkpoint" | jq -r '.command // ""' 2>/dev/null || echo "")
            
            local cp_result="not_checked"
            if [ -n "$cp_command" ] && [ "$cp_command" != "null" ]; then
                if eval "$cp_command" > "$test_log.checkpoint.$i" 2>&1; then
                    cp_result="passed"
                else
                    cp_result="failed"
                fi
            fi
            
            checkpoint_results+=("{\"text\": $(echo "$cp_text" | jq -R .), \"command\": $(echo "$cp_command" | jq -R .), \"result\": \"$cp_result\"}")
        done
    fi
    
    # 记录测试结果
    local checkpoint_json=$(IFS=,; echo "[${checkpoint_results[*]}]")
    jq --arg test_id "$test_id" \
       --arg test_name "$test_name" \
       --arg result "$test_result" \
       --arg duration "$test_duration" \
       --arg log_file "$test_log" \
       --argjson checkpoints "$checkpoint_json" \
       '.tests += [{
         "id": $test_id,
         "name": $test_name,
         "result": $result,
         "duration": ($duration | tonumber),
         "log_file": $log_file,
         "checkpoints": $checkpoints
       }]' "$RESULTS_JSON" > "$RESULTS_JSON.tmp" && mv "$RESULTS_JSON.tmp" "$RESULTS_JSON"
}

# 加载测试配置
load_test_config() {
    local config_file="$1"
    if [ ! -f "$config_file" ]; then
        log_error "Test config file not found: $config_file"
        exit 1
    fi
    
    # 从 JSON 配置文件中读取测试用例
    local test_count=$(jq '.tests | length' "$config_file")
    log_info "Loaded $test_count tests from $config_file"
    
    for ((i=0; i<test_count; i++)); do
        local test_id=$(jq -r ".tests[$i].id" "$config_file")
        local test_name=$(jq -r ".tests[$i].name" "$config_file")
        local test_command=$(jq -r ".tests[$i].command" "$config_file")
        local test_checkpoints=$(jq -c ".tests[$i].checkpoints // []" "$config_file")
        
        run_test "$test_id" "$test_name" "$test_command" "$test_checkpoints"
    done
}

# 生成测试摘要
generate_summary() {
    local end_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    local total_duration=$(jq '[.tests[].duration] | add' "$RESULTS_JSON" 2>/dev/null || echo "0")
    
    cat > "$SUMMARY_JSON" <<EOF
{
  "test_run_id": "$TEST_RUN_ID",
  "start_time": "$(jq -r '.start_time' "$RESULTS_JSON")",
  "end_time": "$end_time",
  "total_duration": $total_duration,
  "statistics": {
    "total": $TOTAL_TESTS,
    "passed": $PASSED_TESTS,
    "failed": $FAILED_TESTS,
    "skipped": $SKIPPED_TESTS,
    "pass_rate": $(awk "BEGIN {printf \"%.2f\", ($PASSED_TESTS / ($TOTAL_TESTS + 0.0001)) * 100}")
  },
  "results_file": "$RESULTS_JSON"
}
EOF
    
    log_info "Test summary generated: $SUMMARY_JSON"
    log_info "Total: $TOTAL_TESTS, Passed: $PASSED_TESTS, Failed: $FAILED_TESTS, Skipped: $SKIPPED_TESTS"
}

# 主函数
main() {
    log_info "Starting E2E test automation"
    log_info "Test run ID: $TEST_RUN_ID"
    log_info "Results directory: $TEST_RESULTS_DIR/$TEST_RUN_ID"
    
    # 检查前置条件
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed. Please install jq first."
        exit 1
    fi
    
    # 加载测试配置
    local config_file="${1:-$SCRIPT_DIR/test_config.json}"
    if [ ! -f "$config_file" ]; then
        log_warn "Test config file not found: $config_file"
        log_info "Creating sample test config..."
        "$SCRIPT_DIR/create_test_config.sh" "$config_file"
    fi
    
    load_test_config "$config_file"
    
    # 生成摘要
    generate_summary
    
    # 生成 HTML 报告
    log_info "Generating HTML report..."
    python3 "$SCRIPT_DIR/generate_test_report.py" \
        --results "$RESULTS_JSON" \
        --summary "$SUMMARY_JSON" \
        --output "$TEST_RESULTS_DIR/$TEST_RUN_ID/report.html"
    
    # 更新测试计划 HTML，集成测试结果
    log_info "Updating test plan HTML with test results..."
    python3 "$PROJECT_ROOT/scripts/generate_test_plan_html.py" \
        --input "$PROJECT_ROOT/doc/tiup/e2e_test_plan_manual.md" \
        --results "$RESULTS_JSON" \
        --summary "$SUMMARY_JSON" \
        --output "$PROJECT_ROOT/doc/tiup/e2e_test_plan_manual.html"
    
    log_info "E2E test automation completed"
    log_info "Results: $TEST_RESULTS_DIR/$TEST_RUN_ID"
    log_info "HTML Report: $TEST_RESULTS_DIR/$TEST_RUN_ID/report.html"
    log_info "Updated Test Plan: $PROJECT_ROOT/doc/tiup/e2e_test_plan_manual.html"
    
    # 返回退出码
    if [ $FAILED_TESTS -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# 执行主函数
main "$@"

