#!/bin/zsh
# 启动多个 Claude 实例连接到 MiniMax API
# 用法: ./start-claude-cluster.sh [实例数量]

set -e

COUNT=${1:-3}
BASE_PORT=19514
ANTHROPIC_TOKEN="${ANTHROPIC_AUTH_TOKEN:-}"
if [ -z "$ANTHROPIC_TOKEN" ]; then
    echo "ERROR: ANTHROPIC_AUTH_TOKEN environment variable is not set."
    echo "Set it before running: export ANTHROPIC_AUTH_TOKEN=your-token-here"
    exit 1
fi

echo "Stopping any existing daemons..."
pkill -f "multicode daemon" 2>/dev/null || true
sleep 2

echo "Starting $COUNT Claude instances..."

for i in $(seq 1 $COUNT); do
    PORT=$((BASE_PORT + i - 1))
    LOG_FILE="$HOME/.multicode/daemon-claude-$i.log"
    
    echo "  Starting claude-$i on port $PORT..."
    
    (export ANTHROPIC_AUTH_TOKEN="$ANTHROPIC_TOKEN"
    export ANTHROPIC_BASE_URL="https://api.minimaxi.com/anthropic"
    export ANTHROPIC_MODEL="MiniMax-M2.7-highspeed"
    export MULTICODE_DAEMON_ID="claude-$i"
    export MULTICODE_HEALTH_PORT="$PORT"
    exec /opt/homebrew/bin/multicode daemon start --foreground) > "$LOG_FILE" 2>&1 &
    
    sleep 1
done

sleep 3

echo ""
echo "=== Running Instances ==="
ps aux | grep "multicode daemon" | grep -v grep | awk '{print "  PID:"$2, "Port:"19513+NR-1}'

echo ""
echo "=== Runtime Status ==="
TOKEN=$(cat ~/.multicode/config.json | jq -r '.token')
WS_ID=$(cat ~/.multicode/config.json | jq -r '.workspace_id')
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/runtimes?workspace_id=$WS_ID" 2>/dev/null | \
    jq -r '.[] | select(.provider=="claude" and .status=="online") | "  \(.daemon_id): \(.id | .[0:8])..."'

echo ""
echo "Log files: ~/.multicode/daemon-claude-*.log"
