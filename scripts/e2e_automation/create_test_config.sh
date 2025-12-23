#!/bin/bash
# 从 e2e_test_plan_manual.md 生成测试配置文件

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_PLAN_MD="$PROJECT_ROOT/doc/tiup/e2e_test_plan_manual.md"
OUTPUT_FILE="${1:-$SCRIPT_DIR/test_config.json}"

if [ ! -f "$TEST_PLAN_MD" ]; then
    echo "Error: Test plan file not found: $TEST_PLAN_MD"
    exit 1
fi

# 使用 Python 解析 Markdown 并生成 JSON 配置
python3 <<EOF
import re
import json
import sys

def parse_test_plan(md_file):
    """解析测试计划 Markdown 文件，生成测试配置"""
    with open(md_file, 'r', encoding='utf-8') as f:
        content = f.read()
    
    tests = []
    current_phase = None
    current_test = None
    collecting_command = False
    command_lines = []
    
    lines = content.split('\n')
    i = 0
    
    while i < len(lines):
        line = lines[i]
        
        # Phase 标题
        phase_match = re.match(r'^### Phase (\d+):\s*(.+)$', line)
        if phase_match:
            current_phase = phase_match.group(1)
            i += 1
            continue
        
        # Test 标题
        test_match = re.match(r'^#### Test (\d+\.\d+):\s*(.+)$', line)
        if test_match:
            if current_test:
                tests.append(current_test)
            
            test_id = test_match.group(1)
            test_name = test_match.group(2).strip()
            current_test = {
                'id': f"test_{test_id}",
                'name': test_name,
                'phase': current_phase,
                'command': '',
                'checkpoints': []
            }
            i += 1
            continue
        
        # 命令块
        if line.strip() == '```bash':
            collecting_command = True
            command_lines = []
            i += 1
            continue
        
        if collecting_command:
            if line.strip() == '```':
                if current_test:
                    current_test['command'] = '\n'.join(command_lines).strip()
                collecting_command = False
                command_lines = []
            else:
                command_lines.append(line)
            i += 1
            continue
        
        # 验证点
        checkpoint_match = re.match(r'^-\s+\[\s*\]\s+(.+)$', line)
        if checkpoint_match and current_test:
            checkpoint_text = checkpoint_match.group(1).strip()
            checkpoint_cmd = ''
            
            # 检查下一行是否是命令块
            j = i + 1
            if j < len(lines) and lines[j].strip() == '```bash':
                j += 1
                cmd_lines = []
                while j < len(lines) and lines[j].strip() != '```':
                    cmd_lines.append(lines[j])
                    j += 1
                checkpoint_cmd = '\n'.join(cmd_lines).strip()
                i = j + 1
            else:
                i += 1
            
            current_test['checkpoints'].append({
                'text': checkpoint_text,
                'command': checkpoint_cmd
            })
            continue
        
        i += 1
    
    # 添加最后一个测试
    if current_test:
        tests.append(current_test)
    
    return tests

def main():
    md_file = "$TEST_PLAN_MD"
    output_file = "$OUTPUT_FILE"
    
    tests = parse_test_plan(md_file)
    
    config = {
        'version': '1.0',
        'description': 'E2E test configuration generated from e2e_test_plan_manual.md',
        'tests': tests
    }
    
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(config, f, indent=2, ensure_ascii=False)
    
    print(f"Generated test config with {len(tests)} tests")
    print(f"Output: {output_file}")

if __name__ == '__main__':
    main()
EOF

echo "Test config created: $OUTPUT_FILE"

