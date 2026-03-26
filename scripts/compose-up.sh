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

mysql_images=(
  "registry.cn-guangzhou.aliyuncs.com/joe/mysql:lts"
  "mysql:8.4"
)

go_builder_images=(
  "docker.m.daocloud.io/library/golang:1.25-alpine"
  "golang:1.25-alpine"
)

app_runtime_images=(
  "docker.m.daocloud.io/library/alpine:3.20"
  "alpine:3.20"
)

node_builder_images=(
  "docker.m.daocloud.io/library/node:22-alpine"
  "node:22-alpine"
)

nginx_images=(
  "registry.cn-guangzhou.aliyuncs.com/joe/nginx:alpine"
  "nginx:alpine"
)

choose_image() {
  local var_name="$1"
  shift
  local configured
  configured="$(eval "printf '%s' \"\${$var_name:-}\"")"
  if [[ -n "$configured" ]]; then
    echo "$configured"
    return
  fi

  local image
  for image in "$@"; do
    echo "Trying to pull $image ..." >&2
    if docker pull "$image" >/dev/null 2>&1; then
      echo "Pulled $image" >&2
      echo "$image"
      return
    fi
  done

  echo "Failed to pull all candidate images for $var_name." >&2
  echo "Please configure Docker mirror or export $var_name manually." >&2
  exit 1
}

export MYSQL_IMAGE="$(choose_image "MYSQL_IMAGE" "${mysql_images[@]}")"
export GO_BUILDER_IMAGE="$(choose_image "GO_BUILDER_IMAGE" "${go_builder_images[@]}")"
export APP_RUNTIME_IMAGE="$(choose_image "APP_RUNTIME_IMAGE" "${app_runtime_images[@]}")"
export NODE_BUILDER_IMAGE="$(choose_image "NODE_BUILDER_IMAGE" "${node_builder_images[@]}")"
export NGINX_IMAGE="$(choose_image "NGINX_IMAGE" "${nginx_images[@]}")"
export GO_PROXY="${GO_PROXY:-https://goproxy.cn,direct}"
export NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}"
export MYSQL_PORT="${MYSQL_PORT:-$(find_free_port 3306)}"
export BACKEND_PORT="${BACKEND_PORT:-$(find_free_port 8080)}"
export FRONTEND_PORT="${FRONTEND_PORT:-$(find_free_port 5173)}"

echo "Using MYSQL_IMAGE=$MYSQL_IMAGE"
echo "Using GO_BUILDER_IMAGE=$GO_BUILDER_IMAGE"
echo "Using APP_RUNTIME_IMAGE=$APP_RUNTIME_IMAGE"
echo "Using NODE_BUILDER_IMAGE=$NODE_BUILDER_IMAGE"
echo "Using NGINX_IMAGE=$NGINX_IMAGE"
echo "Using GO_PROXY=$GO_PROXY"
echo "Using NPM_REGISTRY=$NPM_REGISTRY"
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
