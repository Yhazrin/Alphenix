#!/usr/bin/env bash
set -euo pipefail

BACKEND_PORT="${1:-8080}"
FRONTEND_PORT="${2:-3000}"

check_port() {
  local port=$1
  local name=$2
  if command -v lsof >/dev/null 2>&1; then
    if lsof -ti:"$port" >/dev/null 2>&1; then
      echo "ERROR: Port $port ($name) is already in use."
      echo "  Run 'make stop' to kill existing processes, or:"
      echo "  lsof -ti:$port | xargs kill -9"
      exit 1
    fi
  elif command -v ss >/dev/null 2>&1; then
    if ss -tlnp 2>/dev/null | grep -q ":${port} "; then
      echo "ERROR: Port $port ($name) is already in use."
      echo "  Run 'make stop' to kill existing processes."
      exit 1
    fi
  fi
}

check_port "$BACKEND_PORT" "backend"
check_port "$FRONTEND_PORT" "frontend"
echo "✓ Ports $BACKEND_PORT and $FRONTEND_PORT are available."
