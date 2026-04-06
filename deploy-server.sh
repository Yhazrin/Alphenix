#!/bin/bash
# alphenix Server 部署脚本
# 用法: ./deploy-server.sh [端口] [数据库URL]

set -e

PORT=${1:-8080}
DB_URL=${2:-"postgres://alphenix:alphenix@localhost:5432/alphenix?sslmode=disable"}

echo "=== Alphenix Server 部署 ==="
echo "端口: $PORT"
echo "数据库: ${DB_URL%%@*}@..."

# 检查端口是否被占用
if lsof -i :$PORT > /dev/null 2>&1; then
    echo "错误: 端口 $PORT 已被占用"
    exit 1
fi

# 检查二进制文件
if [ ! -f "./server/alphenix-server" ] && [ ! -f "./alphenix-server" ]; then
    echo "编译 server..."
    cd server && go build -o ../alphenix-server ./cmd/server && cd ..
fi

echo "启动 server..."
export PORT=$PORT
export DATABASE_URL=$DB_URL

./alphenix-server

echo "Server 已停止"
