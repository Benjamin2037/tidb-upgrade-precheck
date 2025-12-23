#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
生成 E2E 测试结果 HTML 报告
"""

import json
import sys
import argparse
from datetime import datetime
from pathlib import Path


def load_json(file_path):
    """加载 JSON 文件"""
    with open(file_path, 'r', encoding='utf-8') as f:
        return json.load(f)


def format_duration(seconds):
    """格式化持续时间"""
    if seconds < 60:
        return f"{seconds:.2f}s"
    elif seconds < 3600:
        return f"{seconds // 60:.0f}m {seconds % 60:.0f}s"
    else:
        hours = seconds // 3600
        minutes = (seconds % 3600) // 60
        secs = seconds % 60
        return f"{hours:.0f}h {minutes:.0f}m {secs:.0f}s"


def generate_html_report(results, summary, output_file):
    """生成 HTML 报告"""
    
    # 统计数据
    stats = summary.get('statistics', {})
    total = stats.get('total', 0)
    passed = stats.get('passed', 0)
    failed = stats.get('failed', 0)
    skipped = stats.get('skipped', 0)
    pass_rate = stats.get('pass_rate', 0)
    
    # 测试列表
    tests = results.get('tests', [])
    
    # 按阶段分组
    tests_by_phase = {}
    for test in tests:
        phase = test.get('phase', 'unknown')
        if phase not in tests_by_phase:
            tests_by_phase[phase] = []
        tests_by_phase[phase].append(test)
    
    html = f'''<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>E2E 测试报告 - {summary.get('test_run_id', 'Unknown')}</title>
    <style>
        * {{
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }}
        
        body {{
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }}
        
        .container {{
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 30px;
        }}
        
        .header {{
            border-bottom: 3px solid #3498db;
            padding-bottom: 20px;
            margin-bottom: 30px;
        }}
        
        h1 {{
            color: #2c3e50;
            margin-bottom: 10px;
        }}
        
        .metadata {{
            display: flex;
            gap: 30px;
            margin-top: 15px;
            flex-wrap: wrap;
        }}
        
        .metadata-item {{
            display: flex;
            flex-direction: column;
        }}
        
        .metadata-label {{
            font-size: 12px;
            color: #666;
            text-transform: uppercase;
        }}
        
        .metadata-value {{
            font-size: 18px;
            font-weight: bold;
            color: #2c3e50;
        }}
        
        .stats {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }}
        
        .stat-card {{
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            text-align: center;
            border-left: 4px solid #3498db;
        }}
        
        .stat-card.passed {{
            border-left-color: #27ae60;
        }}
        
        .stat-card.failed {{
            border-left-color: #e74c3c;
        }}
        
        .stat-card.skipped {{
            border-left-color: #95a5a6;
        }}
        
        .stat-value {{
            font-size: 36px;
            font-weight: bold;
            margin-bottom: 5px;
        }}
        
        .stat-label {{
            font-size: 14px;
            color: #666;
            text-transform: uppercase;
        }}
        
        .phase {{
            margin-bottom: 30px;
            border: 1px solid #ddd;
            border-radius: 6px;
            padding: 20px;
            background: #fafafa;
        }}
        
        .phase-header {{
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
            padding-bottom: 10px;
            border-bottom: 2px solid #e0e0e0;
        }}
        
        .phase-title {{
            font-size: 20px;
            font-weight: bold;
            color: #34495e;
        }}
        
        .test {{
            margin-bottom: 15px;
            padding: 15px;
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 4px;
            border-left: 4px solid #95a5a6;
        }}
        
        .test.passed {{
            border-left-color: #27ae60;
        }}
        
        .test.failed {{
            border-left-color: #e74c3c;
        }}
        
        .test-header {{
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }}
        
        .test-name {{
            font-size: 16px;
            font-weight: bold;
            color: #2c3e50;
        }}
        
        .test-id {{
            font-size: 12px;
            color: #666;
            font-family: monospace;
        }}
        
        .test-status {{
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: bold;
            text-transform: uppercase;
        }}
        
        .test-status.passed {{
            background: #d4edda;
            color: #155724;
        }}
        
        .test-status.failed {{
            background: #f8d7da;
            color: #721c24;
        }}
        
        .test-details {{
            margin-top: 10px;
            font-size: 14px;
        }}
        
        .test-detail-item {{
            margin: 5px 0;
            color: #666;
        }}
        
        .checkpoints {{
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #e0e0e0;
        }}
        
        .checkpoint {{
            margin: 10px 0;
            padding: 10px;
            background: #f9f9f9;
            border-left: 3px solid #95a5a6;
            border-radius: 3px;
        }}
        
        .checkpoint.passed {{
            border-left-color: #27ae60;
        }}
        
        .checkpoint.failed {{
            border-left-color: #e74c3c;
        }}
        
        .checkpoint-text {{
            font-weight: 500;
            margin-bottom: 5px;
        }}
        
        .log-link {{
            color: #3498db;
            text-decoration: none;
            font-size: 12px;
        }}
        
        .log-link:hover {{
            text-decoration: underline;
        }}
        
        .command-block {{
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 10px;
            border-radius: 4px;
            margin: 5px 0;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 12px;
            overflow-x: auto;
        }}
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>E2E 测试报告</h1>
            <div class="metadata">
                <div class="metadata-item">
                    <span class="metadata-label">Test Run ID</span>
                    <span class="metadata-value">{summary.get('test_run_id', 'Unknown')}</span>
                </div>
                <div class="metadata-item">
                    <span class="metadata-label">Start Time</span>
                    <span class="metadata-value">{summary.get('start_time', 'Unknown')}</span>
                </div>
                <div class="metadata-item">
                    <span class="metadata-label">End Time</span>
                    <span class="metadata-value">{summary.get('end_time', 'Unknown')}</span>
                </div>
                <div class="metadata-item">
                    <span class="metadata-label">Duration</span>
                    <span class="metadata-value">{format_duration(summary.get('total_duration', 0))}</span>
                </div>
            </div>
        </div>
        
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">{total}</div>
                <div class="stat-label">Total Tests</div>
            </div>
            <div class="stat-card passed">
                <div class="stat-value">{passed}</div>
                <div class="stat-label">Passed</div>
            </div>
            <div class="stat-card failed">
                <div class="stat-value">{failed}</div>
                <div class="stat-label">Failed</div>
            </div>
            <div class="stat-card skipped">
                <div class="stat-value">{skipped}</div>
                <div class="stat-label">Skipped</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{pass_rate:.1f}%</div>
                <div class="stat-label">Pass Rate</div>
            </div>
        </div>
'''
    
    # 按阶段生成测试结果
    for phase_num in sorted(tests_by_phase.keys(), key=lambda x: float(x) if x.replace('.', '').isdigit() else 999):
        phase_tests = tests_by_phase[phase_num]
        phase_passed = sum(1 for t in phase_tests if t.get('result') == 'passed')
        phase_failed = sum(1 for t in phase_tests if t.get('result') == 'failed')
        
        html += f'''
        <div class="phase">
            <div class="phase-header">
                <div class="phase-title">Phase {phase_num}</div>
                <div>
                    <span style="color: #27ae60;">✓ {phase_passed}</span>
                    <span style="color: #e74c3c;">✗ {phase_failed}</span>
                </div>
            </div>
'''
        
        for test in phase_tests:
            test_id = test.get('id', 'unknown')
            test_name = test.get('name', 'Unknown Test')
            test_result = test.get('result', 'unknown')
            test_duration = test.get('duration', 0)
            test_log = test.get('log_file', '')
            checkpoints = test.get('checkpoints', [])
            
            html += f'''
            <div class="test {test_result}">
                <div class="test-header">
                    <div>
                        <div class="test-name">{test_name}</div>
                        <div class="test-id">{test_id}</div>
                    </div>
                    <div>
                        <span class="test-status {test_result}">{test_result}</span>
                    </div>
                </div>
                <div class="test-details">
                    <div class="test-detail-item">Duration: {format_duration(test_duration)}</div>
                    {f'<div class="test-detail-item"><a href="{test_log}" class="log-link">View Log</a></div>' if test_log else ''}
                </div>
'''
            
            if checkpoints:
                html += '<div class="checkpoints">'
                for cp in checkpoints:
                    cp_text = cp.get('text', '')
                    cp_result = cp.get('result', 'not_checked')
                    cp_command = cp.get('command', '')
                    
                    html += f'''
                    <div class="checkpoint {cp_result}">
                        <div class="checkpoint-text">{cp_text}</div>
                        {f'<div class="command-block">{cp_command}</div>' if cp_command else ''}
                        <div style="font-size: 12px; color: #666; margin-top: 5px;">Status: {cp_result}</div>
                    </div>
'''
                html += '</div>'
            
            html += '</div>'
        
        html += '</div>'
    
    html += '''
    </div>
</body>
</html>
'''
    
    # 写入文件
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(html)
    
    print(f"HTML report generated: {output_file}")


def main():
    parser = argparse.ArgumentParser(description='Generate E2E test HTML report')
    parser.add_argument('--results', required=True, help='Test results JSON file')
    parser.add_argument('--summary', required=True, help='Test summary JSON file')
    parser.add_argument('--output', required=True, help='Output HTML file')
    
    args = parser.parse_args()
    
    results = load_json(args.results)
    summary = load_json(args.summary)
    
    generate_html_report(results, summary, args.output)


if __name__ == '__main__':
    main()

