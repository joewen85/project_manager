#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

find_free_port() {
  local port="$1"
  while lsof -iTCP:"$port" -sTCP:LISTEN -t >/dev/null 2>&1; do
    port=$((port + 1))
  done
  echo "$port"
}

images=(
  "mysql:8.4"
  "mysql:8.0"
  "registry.cn-hangzhou.aliyuncs.com/library/mysql:8.4"
  "registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0"
)

choose_image() {
  if [[ -n "${MYSQL_IMAGE:-}" ]]; then
    echo "$MYSQL_IMAGE"
    return
  fi

  for image in "${images[@]}"; do
    echo "Trying to pull $image ..." >&2
    if docker pull "$image"; then
      echo "$image"
      return
    fi
  done

  echo "Failed to pull all candidate MySQL images." >&2
  echo "Please configure Docker mirror or export MYSQL_IMAGE manually." >&2
  exit 1
}

selected_image="$(choose_image | tail -n 1)"
export MYSQL_IMAGE="$selected_image"
export MYSQL_PORT="${MYSQL_PORT:-$(find_free_port 3306)}"
export BACKEND_PORT="${BACKEND_PORT:-$(find_free_port 8080)}"
export FRONTEND_PORT="${FRONTEND_PORT:-$(find_free_port 5173)}"

echo "Using MYSQL_IMAGE=$MYSQL_IMAGE"
echo "Using MYSQL_PORT=$MYSQL_PORT BACKEND_PORT=$BACKEND_PORT FRONTEND_PORT=$FRONTEND_PORT"
docker compose up -d --build

echo "Compose started. Checking health..."
for i in {1..30}; do
  if curl -fsS "http://localhost:${BACKEND_PORT}/health" >/dev/null 2>&1; then
    echo "Backend health check passed."
    echo "Frontend URL: http://localhost:${FRONTEND_PORT}"
    echo "Backend URL: http://localhost:${BACKEND_PORT}"
    echo "MySQL host port: ${MYSQL_PORT}"
    exit 0
  fi
  sleep 2
done

echo "Backend health check timeout." >&2
exit 1
