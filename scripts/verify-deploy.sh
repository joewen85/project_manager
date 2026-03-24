#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[1/4] Checking docker compose config..."
if command -v docker >/dev/null 2>&1; then
  docker compose config >/dev/null
  echo "docker compose config: OK"
else
  echo "docker not installed, skip compose config"
fi

echo "[2/4] Checking helm chart lint/template..."
if command -v helm >/dev/null 2>&1; then
  helm lint ./deploy/helm/project-manager
  helm template project-manager ./deploy/helm/project-manager >/dev/null
  echo "helm lint/template: OK"
else
  echo "helm not installed, skip helm checks"
fi

echo "[3/4] Backend tests/build..."
(
  cd backend
  go test ./...
  go build ./...
)

echo "[4/4] Frontend build..."
(
  cd frontend
  npm run build
)

echo "All verification steps completed."
