#!/bin/zsh
# 启动多个 Claude runtime 实例
# 用法: ./start-multi-claude.sh [数量]

COUNT=${1:-3}  # 默认3个实例
BASE_PORT=19514

for i in $(seq 1 $COUNT); do
    INSTANCE_NAME="claude-$i"
    PORT=$((BASE_PORT + i))
    
    echo "Starting instance $INSTANCE_NAME on port $PORT..."
    
    # 每个实例有不同的 ALPHENIX_DAEMON_ID 和 instance_id
    INSTANCE_ID="$INSTANCE_NAME" zsh -l -c "
        export ALPHENIX_DAEMON_ID='daemon-$i'
        export ALPHENIX_INSTANCE_ID='$INSTANCE_NAME'
        alphenix daemon start --foreground --health-port $PORT &
    " &> ~/.alphenix/daemon-$i.log &
done

echo "Started $COUNT Claude instances"
echo "Check logs at: ~/.alphenix/daemon-*.log"
