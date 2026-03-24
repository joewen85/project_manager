#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

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
    echo "Trying to pull $image ..."
    if docker pull "$image"; then
      echo "$image"
      return
    fi
  done

  echo "Failed to pull all candidate MySQL images." >&2
  echo "Please configure Docker mirror or export MYSQL_IMAGE manually." >&2
  exit 1
}

selected_image="$(choose_image)"
export MYSQL_IMAGE="$selected_image"

echo "Using MYSQL_IMAGE=$MYSQL_IMAGE"
docker compose up -d --build

echo "Compose started. Checking health..."
for i in {1..30}; do
  if curl -fsS "http://localhost:8080/health" >/dev/null 2>&1; then
    echo "Backend health check passed."
    exit 0
  fi
  sleep 2
done

echo "Backend health check timeout." >&2
exit 1
