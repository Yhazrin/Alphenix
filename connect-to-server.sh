#!/bin/bash
# 连接到远程 multica server
# 用法: ./connect-to-server.sh <服务器URL>

set -e

SERVER_URL=${1:-""}

if [ -z "$SERVER_URL" ]; then
    echo "用法: ./connect-to-server.sh <服务器URL>"
    echo "例如: ./connect-to-server.sh http://你的服务器IP:8080"
    exit 1
fi

echo "=== 连接到 Multica Server ==="
echo "服务器: $SERVER_URL"

# 配置 server URL
export MULTICA_SERVER_URL=$SERVER_URL

echo ""
echo "请确保:"
echo "1. 服务器已启动并可访问"
echo "2. 你有访问权限（已登录）"
echo "3. 防火墙已开放端口"
echo ""

# 检查连接
echo "测试连接..."
curl -s --connect-timeout 5 "${SERVER_URL}/health" | jq '.' || echo "无法连接到服务器"

echo ""
echo "要启动 daemon 并连接到服务器，运行:"
echo "  export MULTICA_SERVER_URL=$SERVER_URL"
echo "  multica daemon start"
