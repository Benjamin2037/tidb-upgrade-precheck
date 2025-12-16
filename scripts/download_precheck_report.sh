#!/bin/bash
# 从 VM 下载最新的 HTML precheck 报告到本地 macOS

# 配置（请修改为你的实际值）
VM_IP="${VM_IP:-your-vm-ip-address}"  # 可以通过环境变量设置，或直接修改这里
VM_USER="${VM_USER:-bearc}"            # 可以通过环境变量设置，或直接修改这里
REPORTS_DIR="${REPORTS_DIR:-~/.tiup/storage/cluster/upgrade_precheck/reports}"
DOWNLOAD_DIR="${DOWNLOAD_DIR:-~/Downloads/precheck-reports}"

# 检查参数
if [ "$VM_IP" = "your-vm-ip-address" ]; then
    echo "错误: 请设置 VM_IP 环境变量或修改脚本中的 VM_IP 值"
    echo "用法: VM_IP=192.168.1.100 $0"
    echo "或者: VM_USER=bearc VM_IP=192.168.1.100 $0"
    exit 1
fi

echo "正在从 ${VM_USER}@${VM_IP} 下载 HTML 报告..."
echo "报告目录: ${REPORTS_DIR}"
echo "下载到: ${DOWNLOAD_DIR}"

# 创建下载目录
mkdir -p "${DOWNLOAD_DIR}"

# 方法 1: 下载最新的 HTML 报告
echo ""
echo "方法 1: 下载最新的 HTML 报告..."
LATEST_REPORT=$(ssh ${VM_USER}@${VM_IP} "ls -t ${REPORTS_DIR}/*.html 2>/dev/null | head -1")
if [ -n "$LATEST_REPORT" ]; then
    REPORT_NAME=$(basename "$LATEST_REPORT")
    scp ${VM_USER}@${VM_IP}:"${LATEST_REPORT}" "${DOWNLOAD_DIR}/${REPORT_NAME}"
    echo "✓ 已下载: ${DOWNLOAD_DIR}/${REPORT_NAME}"
    
    # 在 macOS 上打开
    if [[ "$OSTYPE" == "darwin"* ]]; then
        open "${DOWNLOAD_DIR}/${REPORT_NAME}"
        echo "✓ 已在浏览器中打开报告"
    fi
else
    echo "✗ 未找到 HTML 报告"
    echo "请确认:"
    echo "  1. 已在 VM 上运行: ./bin/tiup-cluster upgrade ... --precheck --precheck-output html"
    echo "  2. 报告目录存在: ssh ${VM_USER}@${VM_IP} 'ls -la ${REPORTS_DIR}'"
fi

# 方法 2: 列出所有报告文件
echo ""
echo "VM 上的所有报告文件:"
ssh ${VM_USER}@${VM_IP} "ls -lh ${REPORTS_DIR}/*.{html,md,txt,json} 2>/dev/null | tail -10"

echo ""
echo "完成！报告已下载到: ${DOWNLOAD_DIR}"

