#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
生成交互式 E2E 测试计划 HTML 文件
"""

import re
import json

def parse_markdown(content):
    """解析 Markdown 文件，提取测试计划结构"""
    lines = content.split('\n')
    
    # 前置条件
    prerequisites = []
    current_prerequisite = None
    
    # 测试阶段
    phases = []
    current_phase = None
    current_test = None
    current_section = None
    collecting_command = False
    command_lines = []
    in_prerequisite = False
    
    i = 0
    while i < len(lines):
        line = lines[i]
        
        # 前置条件部分
        if line.startswith('### 1. 环境准备') or line.startswith('### 2. 知识库准备') or line.startswith('### 3. 测试集群准备'):
            if current_phase:
                phases.append(current_phase)
            current_phase = None
            in_prerequisite = True
            current_prerequisite = {
                'title': line.replace('### ', ''),
                'command': '',
                'checkpoints': []
            }
            prerequisites.append(current_prerequisite)
            i += 1
            continue
        
        # 如果遇到 Phase，说明前置条件部分结束
        if re.match(r'^### Phase \d+:', line):
            in_prerequisite = False
            current_prerequisite = None
        
        # Phase 标题
        if re.match(r'^### Phase \d+:', line):
            if current_phase:
                if current_test:
                    current_phase['tests'].append(current_test)
                phases.append(current_phase)
            current_phase = {
                'title': line.replace('### ', ''),
                'description': '',
                'tests': []
            }
            i += 1
            if i < len(lines) and lines[i].startswith('**目标**:'):
                current_phase['description'] = lines[i].replace('**目标**:', '').strip()
                i += 1
            continue
        
        # Test 标题
        if re.match(r'^#### Test \d+\.\d+:', line):
            if current_phase:
                if current_test:
                    current_phase['tests'].append(current_test)
                match = re.search(r'Test (\d+\.\d+)', line)
                test_id = match.group(1) if match else '0.0'
                current_test = {
                    'id': test_id,
                    'title': line.replace('#### ', ''),
                    'command': '',
                    'checkpoints': [],
                    'output_checks': []
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
                if in_prerequisite and current_prerequisite:
                    current_prerequisite['command'] = '\n'.join(command_lines).strip()
                elif current_test:
                    current_test['command'] = '\n'.join(command_lines).strip()
                collecting_command = False
                command_lines = []
            else:
                command_lines.append(line)
            i += 1
            continue
        
        # 验证点
        if line.strip().startswith('- [ ]'):
            checkpoint_text = line.replace('- [ ]', '').strip()
            # 检查是否包含命令块
            checkpoint_obj = {
                'text': checkpoint_text,
                'command': ''
            }
            # 检查下一行是否是命令块
            j = i + 1
            if j < len(lines) and lines[j].strip() == '```bash':
                j += 1
                cmd_lines = []
                while j < len(lines) and lines[j].strip() != '```':
                    cmd_lines.append(lines[j])
                    j += 1
                checkpoint_obj['command'] = '\n'.join(cmd_lines).strip()
                i = j + 1
            else:
                i += 1
            
            if in_prerequisite and current_prerequisite:
                current_prerequisite['checkpoints'].append(checkpoint_obj)
            elif current_test:
                current_test['checkpoints'].append(checkpoint_obj)
            continue
        
        # 检查输出
        if line.strip() == '**检查输出**:':
            current_section = 'output_checks'
            i += 1
            continue
        
        i += 1
    
    # 添加最后一个
    if current_phase:
        if current_test:
            current_phase['tests'].append(current_test)
        phases.append(current_phase)
    
    return prerequisites, phases

def generate_html(prerequisites, phases):
    """生成 HTML 内容"""
    
    html = '''<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TiUP Cluster Upgrade E2E 测试计划</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 30px;
        }
        
        h1 {
            color: #2c3e50;
            border-bottom: 3px solid #3498db;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        
        h2 {
            color: #34495e;
            margin-top: 30px;
            margin-bottom: 15px;
            padding: 10px;
            background: #ecf0f1;
            border-left: 4px solid #3498db;
        }
        
        h3 {
            color: #555;
            margin-top: 25px;
            margin-bottom: 15px;
            cursor: pointer;
            padding: 10px;
            background: #f8f9fa;
            border-radius: 4px;
            transition: background 0.2s;
        }
        
        h3:hover {
            background: #e9ecef;
        }
        
        h4 {
            color: #666;
            margin-top: 20px;
            margin-bottom: 10px;
            padding: 8px;
            background: #f0f0f0;
            border-left: 3px solid #95a5a6;
        }
        
        .phase {
            margin-bottom: 30px;
            border: 1px solid #ddd;
            border-radius: 6px;
            padding: 20px;
            background: #fafafa;
        }
        
        .test {
            margin-bottom: 20px;
            padding: 15px;
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 4px;
        }
        
        .test-header {
            display: flex;
            align-items: center;
            gap: 10px;
            margin-bottom: 15px;
        }
        
        .test-checkbox {
            width: 20px;
            height: 20px;
            cursor: pointer;
        }
        
        .command-block {
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 15px;
            border-radius: 4px;
            margin: 10px 0;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 13px;
            line-height: 1.5;
            overflow-x: auto;
            position: relative;
        }
        
        .copy-btn {
            position: absolute;
            top: 10px;
            right: 10px;
            background: #3498db;
            color: white;
            border: none;
            padding: 5px 10px;
            border-radius: 3px;
            cursor: pointer;
            font-size: 12px;
        }
        
        .copy-btn:hover {
            background: #2980b9;
        }
        
        .checkpoint {
            margin: 10px 0;
            padding: 10px;
            background: #f9f9f9;
            border-left: 3px solid #95a5a6;
            border-radius: 3px;
        }
        
        .checkpoint-item {
            display: flex;
            align-items: flex-start;
            gap: 10px;
            margin: 8px 0;
        }
        
        .checkpoint-checkbox {
            width: 18px;
            height: 18px;
            margin-top: 3px;
            cursor: pointer;
        }
        
        .checkpoint-text {
            flex: 1;
        }
        
        .remark-input {
            width: 100%;
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-top: 5px;
            font-size: 13px;
            min-height: 60px;
        }
        
        .remark-display {
            margin-top: 5px;
            padding: 8px;
            background: #fff3cd;
            border-left: 3px solid #ffc107;
            border-radius: 3px;
            font-size: 13px;
        }
        
        .progress-bar {
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 4px;
            background: #e0e0e0;
            z-index: 1000;
        }
        
        .progress-fill {
            height: 100%;
            background: #3498db;
            transition: width 0.3s;
        }
        
        .stats {
            position: fixed;
            top: 10px;
            right: 20px;
            background: white;
            padding: 15px;
            border-radius: 6px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
            z-index: 999;
        }
        
        .stats-item {
            margin: 5px 0;
            font-size: 14px;
        }
        
        .stats-number {
            font-weight: bold;
            color: #3498db;
        }
        
        .collapsible {
            cursor: pointer;
        }
        
        .collapsible::before {
            content: '▼ ';
            display: inline-block;
            transition: transform 0.2s;
        }
        
        .collapsible.collapsed::before {
            transform: rotate(-90deg);
        }
        
        .collapsed-content {
            display: block;
        }
        
        .save-btn {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: #27ae60;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
        }
        
        .save-btn:hover {
            background: #229954;
        }
        
        .save-indicator {
            position: fixed;
            bottom: 70px;
            right: 20px;
            background: #27ae60;
            color: white;
            padding: 8px 16px;
            border-radius: 4px;
            font-size: 12px;
            opacity: 0;
            transition: opacity 0.3s;
        }
        
        .save-indicator.show {
            opacity: 1;
        }
        
        code {
            background: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="progress-bar">
        <div class="progress-fill" id="progressFill"></div>
    </div>
    
    <div class="stats" id="stats">
        <div class="stats-item">总测试: <span class="stats-number" id="totalTests">0</span></div>
        <div class="stats-item">已完成: <span class="stats-number" id="completedTests">0</span></div>
        <div class="stats-item">进度: <span class="stats-number" id="progressPercent">0%</span></div>
    </div>
    
    <div class="container">
        <h1>TiUP Cluster Upgrade E2E 测试计划（手动执行）</h1>
        <p style="margin-bottom: 20px; color: #666;">本文档提供详细的端到端测试计划，使用真实的 <code>tiup cluster upgrade</code> 命令测试完整的升级场景。</p>
        
        <h2>测试目标</h2>
        <ul style="margin-left: 20px; margin-bottom: 20px;">
            <li>验证 <code>tiup cluster upgrade --precheck</code> 命令正常工作</li>
            <li>验证 <code>tiup cluster upgrade</code> 默认行为（自动运行 precheck）</li>
            <li>验证所有 precheck 相关参数正常工作</li>
            <li>验证报告生成和显示</li>
            <li>验证完整升级流程中的 precheck 集成</li>
            <li>验证错误处理和边界情况</li>
        </ul>
        
        <h2>前置条件</h2>
'''
    
    # 生成前置条件
    for prep_idx, prep in enumerate(prerequisites, 1):
        prep_id = f"prep-{prep_idx}"
        html += f'''
        <div class="phase">
            <h3 class="collapsible" onclick="toggleSection(this)">{prep['title']}</h3>
            <div class="collapsed-content">
'''
        
        # 显示命令
        if prep.get('command'):
            html += f'''
                <div class="command-block">
                    <button class="copy-btn" onclick="copyCommand(this)">复制</button>
                    <pre>{prep['command']}</pre>
                </div>
'''
        
        # 显示验证点
        if prep.get('checkpoints'):
            html += '<h4>验证点</h4>\n'
            for cp_idx, checkpoint in enumerate(prep['checkpoints'], 1):
                cp_id = f"{prep_id}-cp{cp_idx}"
                if isinstance(checkpoint, dict):
                    checkpoint_text = checkpoint.get('text', '')
                    checkpoint_cmd = checkpoint.get('command', '')
                else:
                    checkpoint_text = checkpoint
                    checkpoint_cmd = ''
                
                cmd_html = ''
                if checkpoint_cmd:
                    cmd_html = f'''
                            <div class="command-block" style="margin-top: 5px;">
                                <button class="copy-btn" onclick="copyCommand(this)">复制</button>
                                <pre>{checkpoint_cmd}</pre>
                            </div>
'''
                
                html += f'''
                <div class="checkpoint">
                    <div class="checkpoint-item">
                        <input type="checkbox" class="checkpoint-checkbox" data-id="{cp_id}" onchange="updateProgress()">
                        <div class="checkpoint-text">
                            <strong>{checkpoint_text}</strong>
                            {cmd_html}
                            <textarea class="remark-input" placeholder="添加备注..." data-id="{cp_id}-remark" onblur="saveRemark(this)" style="margin-top: 5px;"></textarea>
                            <div class="remark-display" data-id="{cp_id}-remark-display" style="display: none;"></div>
                        </div>
                    </div>
                </div>
'''
        
        html += '</div></div>\n'
    
    # 生成测试阶段
    html += '<h2>测试阶段</h2>\n'
    
    for phase_idx, phase in enumerate(phases, 1):
        phase_id = f"phase-{phase_idx}"
        html += f'''
        <div class="phase">
            <h3 class="collapsible" onclick="toggleSection(this)">{phase['title']}</h3>
            <p style="margin: 10px 0; color: #666;"><strong>目标:</strong> {phase['description']}</p>
            <div class="collapsed-content">
'''
        
        for test in phase['tests']:
            test_id = f"{phase_id}-test-{test['id']}"
            html += f'''
                <div class="test">
                    <div class="test-header">
                        <input type="checkbox" class="test-checkbox" data-id="{test_id}" onchange="updateProgress()">
                        <h4>{test['title']}</h4>
                    </div>
'''
            
            if test['command']:
                html += f'''
                    <div class="command-block">
                        <button class="copy-btn" onclick="copyCommand(this)">复制</button>
                        <pre>{test['command']}</pre>
                    </div>
'''
            
            html += f'''
                    <textarea class="remark-input" placeholder="添加测试备注..." data-id="{test_id}-remark" onblur="saveRemark(this)"></textarea>
                    <div class="remark-display" data-id="{test_id}-remark-display" style="display: none;"></div>
'''
            
            if test['checkpoints']:
                html += '<h4>验证点</h4>\n'
                for cp_idx, checkpoint in enumerate(test['checkpoints']):
                    cp_id = f"{test_id}-cp{cp_idx+1}"
                    # 处理验证点（可能是字符串或字典）
                    if isinstance(checkpoint, dict):
                        checkpoint_text = checkpoint.get('text', '')
                        checkpoint_cmd = checkpoint.get('command', '')
                    else:
                        # 尝试从字符串中提取命令
                        cmd_match = re.search(r'```bash\n(.*?)```', checkpoint, re.DOTALL)
                        checkpoint_text = checkpoint
                        checkpoint_cmd = ''
                        if cmd_match:
                            checkpoint_cmd = cmd_match.group(1).strip()
                            checkpoint_text = checkpoint.replace(f'```bash\n{checkpoint_cmd}```', '').strip()
                    
                    cmd_html = ''
                    if checkpoint_cmd:
                        cmd_html = f'''
                            <div class="command-block" style="margin-top: 5px;">
                                <button class="copy-btn" onclick="copyCommand(this)">复制</button>
                                <pre>{checkpoint_cmd}</pre>
                            </div>
'''
                    
                    html += f'''
                    <div class="checkpoint">
                        <div class="checkpoint-item">
                            <input type="checkbox" class="checkpoint-checkbox" data-id="{cp_id}" onchange="updateProgress()">
                            <div class="checkpoint-text">
                                <strong>{checkpoint_text}</strong>
                                {cmd_html}
                                <textarea class="remark-input" placeholder="添加备注..." data-id="{cp_id}-remark" onblur="saveRemark(this)" style="margin-top: 5px;"></textarea>
                                <div class="remark-display" data-id="{cp_id}-remark-display" style="display: none;"></div>
                            </div>
                        </div>
                    </div>
'''
            
            html += '</div>\n'
        
        html += '</div></div>\n'
    
    # JavaScript
    html += '''
    </div>
    
    <button class="save-btn" onclick="saveAll()">💾 保存进度</button>
    <div class="save-indicator" id="saveIndicator">已保存</div>
    
    <script>
        // 初始化
        document.addEventListener('DOMContentLoaded', function() {
            loadProgress();
            updateProgress();
        });
        
        // 切换折叠
        function toggleSection(element) {
            const content = element.nextElementSibling;
            if (content && content.classList.contains('collapsed-content')) {
                element.classList.toggle('collapsed');
                content.style.display = content.style.display === 'none' ? 'block' : 'none';
            } else {
                // 查找下一个兄弟元素
                let next = element.nextElementSibling;
                while (next && !next.classList.contains('collapsed-content')) {
                    next = next.nextElementSibling;
                }
                if (next) {
                    element.classList.toggle('collapsed');
                    next.style.display = next.style.display === 'none' ? 'block' : 'none';
                }
            }
        }
        
        // 复制命令
        function copyCommand(btn) {
            const pre = btn.nextElementSibling;
            const text = pre.textContent;
            navigator.clipboard.writeText(text).then(() => {
                const originalText = btn.textContent;
                btn.textContent = '已复制!';
                setTimeout(() => {
                    btn.textContent = originalText;
                }, 2000);
            }).catch(() => {
                // 降级方案
                const textarea = document.createElement('textarea');
                textarea.value = text;
                document.body.appendChild(textarea);
                textarea.select();
                document.execCommand('copy');
                document.body.removeChild(textarea);
                btn.textContent = '已复制!';
                setTimeout(() => {
                    btn.textContent = '复制';
                }, 2000);
            });
        }
        
        // 保存备注
        function saveRemark(textarea) {
            const remarkId = textarea.dataset.id;
            const remark = textarea.value.trim();
            localStorage.setItem(`remark_${remarkId}`, remark);
            
            const display = document.querySelector(`[data-id="${remarkId}-display"]`);
            if (remark) {
                display.textContent = remark;
                display.style.display = 'block';
                textarea.style.display = 'none';
            } else {
                display.style.display = 'none';
                textarea.style.display = 'block';
            }
        }
        
        // 加载备注
        function loadRemark(remarkId) {
            const remark = localStorage.getItem(`remark_${remarkId}`);
            if (remark) {
                const textarea = document.querySelector(`[data-id="${remarkId}"]`);
                const display = document.querySelector(`[data-id="${remarkId}-display"]`);
                if (textarea && display) {
                    textarea.value = remark;
                    display.textContent = remark;
                    display.style.display = 'block';
                    textarea.style.display = 'none';
                }
            }
        }
        
        // 保存进度
        function saveAll() {
            const checkboxes = document.querySelectorAll('input[type="checkbox"]');
            checkboxes.forEach(cb => {
                localStorage.setItem(`check_${cb.dataset.id}`, cb.checked);
            });
            
            // 保存所有备注
            const textareas = document.querySelectorAll('.remark-input');
            textareas.forEach(ta => {
                const remarkId = ta.dataset.id;
                const remark = ta.value.trim();
                if (remark) {
                    localStorage.setItem(`remark_${remarkId}`, remark);
                }
            });
            
            // 显示保存提示
            const indicator = document.getElementById('saveIndicator');
            indicator.classList.add('show');
            setTimeout(() => {
                indicator.classList.remove('show');
            }, 2000);
        }
        
        // 加载进度
        function loadProgress() {
            const checkboxes = document.querySelectorAll('input[type="checkbox"]');
            checkboxes.forEach(cb => {
                const saved = localStorage.getItem(`check_${cb.dataset.id}`);
                if (saved === 'true') {
                    cb.checked = true;
                }
            });
            
            // 加载所有备注
            const textareas = document.querySelectorAll('.remark-input');
            textareas.forEach(ta => {
                loadRemark(ta.dataset.id);
            });
        }
        
        // 更新进度
        function updateProgress() {
            const allCheckboxes = document.querySelectorAll('input[type="checkbox"]');
            const checked = document.querySelectorAll('input[type="checkbox"]:checked');
            
            const total = allCheckboxes.length;
            const completed = checked.length;
            const percent = total > 0 ? Math.round((completed / total) * 100) : 0;
            
            document.getElementById('totalTests').textContent = total;
            document.getElementById('completedTests').textContent = completed;
            document.getElementById('progressPercent').textContent = percent + '%';
            document.getElementById('progressFill').style.width = percent + '%';
            
            // 自动保存
            saveAll();
        }
        
        // 定期自动保存
        setInterval(saveAll, 30000); // 每30秒自动保存
    </script>
</body>
</html>
'''
    
    return html

def main():
    # 读取 Markdown 文件
    with open('doc/tiup/e2e_test_plan_manual.md', 'r', encoding='utf-8') as f:
        content = f.read()
    
    # 解析
    prerequisites, phases = parse_markdown(content)
    
    # 生成 HTML
    html = generate_html(prerequisites, phases)
    
    # 写入文件
    with open('doc/tiup/e2e_test_plan_manual.html', 'w', encoding='utf-8') as f:
        f.write(html)
    
    print(f"✅ HTML 文件已生成: doc/tiup/e2e_test_plan_manual.html")
    print(f"   - 前置条件: {len(prerequisites)} 个")
    print(f"   - 测试阶段: {len(phases)} 个")
    print(f"   - 总测试数: {sum(len(p['tests']) for p in phases)} 个")

if __name__ == '__main__':
    main()

