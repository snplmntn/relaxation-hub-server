#!/usr/bin/env bash
# Automated deployment to a Docker-capable VPS

set -euo pipefail

# ---- Configuration (override via env vars if needed) ----
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-}"          # e.g., 66.181.46.76
REMOTE_DIR="${REMOTE_DIR:-/opt/relaxation-hub}"
IMAGE_NAME="${IMAGE_NAME:-relaxation-hub-server}"
ENV_FILE="${ENV_FILE:-.env}"            # local env file to copy to VPS as .env

# ---- Pre-flight checks ----
if [[ -z "$REMOTE_HOST" ]]; then
  echo "REMOTE_HOST is required (export REMOTE_HOST or edit this script)." >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file '$ENV_FILE' not found." >&2
  exit 1
fi

grep -q "^DATABASE_URL=" "$ENV_FILE" || { echo "DATABASE_URL missing in $ENV_FILE" >&2; exit 1; }
grep -q "^JWT_KEY=" "$ENV_FILE" || { echo "JWT_KEY missing in $ENV_FILE" >&2; exit 1; }

# ---- Build and package image locally ----
echo "[local] Building Docker image..."
docker build -t "$IMAGE_NAME:latest" .

echo "[local] Saving image to tar.gz..."
docker save "$IMAGE_NAME:latest" | gzip > "$IMAGE_NAME.tar.gz"

# ---- Copy artifacts to VPS ----
echo "[local] Ensuring remote directory..."
ssh "$REMOTE_USER@$REMOTE_HOST" "mkdir -p '$REMOTE_DIR'"

echo "[local] Uploading image + compose + env..."
scp "$IMAGE_NAME.tar.gz" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"
scp docker-compose.yml "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"
scp "$ENV_FILE" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/.env"

echo "[remote] Deploying..."
ssh "$REMOTE_USER@$REMOTE_HOST" "set -euo pipefail; cd '$REMOTE_DIR'; \
  docker load < '$IMAGE_NAME.tar.gz'; \
  docker compose down || true; \
  docker compose up -d; \
  rm '$IMAGE_NAME.tar.gz'; \
  docker compose ps; \
  docker compose logs --tail=50 server"

# ---- Cleanup local tar ----
rm "$IMAGE_NAME.tar.gz"

echo "✅ Deployment complete. Service should be available on http://$REMOTE_HOST:8080"
