#!/bin/sh
set -e

echo "==> Starting multicode container..."

# Start Go backend
echo "==> Starting Go backend on port ${PORT:-8080}..."
./server &
SERVER_PID=$!

# Start Next.js frontend (standalone output)
echo "==> Starting Next.js frontend on port ${FRONTEND_PORT:-3000}..."
cd /app
PORT=${FRONTEND_PORT:-3000} node server.js &
FRONTEND_PID=$!

# Graceful shutdown
shutdown() {
  echo "==> Shutting down..."
  kill -TERM "$SERVER_PID" 2>/dev/null
  kill -TERM "$FRONTEND_PID" 2>/dev/null
  wait "$SERVER_PID" 2>/dev/null
  wait "$FRONTEND_PID" 2>/dev/null
  echo "==> Shutdown complete."
  exit 0
}

trap shutdown SIGTERM SIGINT

# Wait for both processes
while kill -0 "$SERVER_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 1
done

# If one process exits, shut down the other
echo "==> One of the processes exited unexpectedly."
shutdown
