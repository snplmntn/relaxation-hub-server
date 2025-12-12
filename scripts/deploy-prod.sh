#!/usr/bin/env bash
# Production-friendly wrapper that deploys the app then ensures Nginx is configured.
# Usage: ./scripts/deploy-prod.sh <vps-ip> <domain>

set -euo pipefail

if [[ ${#} -lt 2 ]]; then
  echo "Usage: $0 <REMOTE_HOST> <DOMAIN> [REMOTE_USER=root]"
  exit 1
fi

REMOTE_HOST="$1"
DOMAIN="$2"
REMOTE_USER=${3:-root}
REMOTE_DIR=${4:-/opt/relaxation-hub}
ENV_FILE=${5:-.env}
IMAGE_NAME=${6:-relaxation-hub-server}

# Validate env file exists locally
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file $ENV_FILE not found locally. Create one and try again." >&2
  exit 1
fi

export REMOTE_HOST
export REMOTE_USER
export REMOTE_DIR
export ENV_FILE
export IMAGE_NAME

# Run the generic deploy script
./deploy.sh

# Copy the nginx setup script to the VPS
scp scripts/nginx-setup.sh "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"
ssh "$REMOTE_USER@$REMOTE_HOST" "sudo bash $REMOTE_DIR/nginx-setup.sh ${DOMAIN} 127.0.0.1 8080"

# Optional: attempt to create TLS cert (commented out by default because DNS must be validated)
# ssh "$REMOTE_USER@$REMOTE_HOST" "sudo apt update && sudo apt install -y certbot python3-certbot-nginx && sudo certbot --nginx -d ${DOMAIN}"

# Remote post-checks
ssh "$REMOTE_USER@$REMOTE_HOST" "cd $REMOTE_DIR; docker compose ps; docker compose logs --tail=50 server"

cat <<EOF
Deployment finished. Please check logs and run the following tests:
# curl -i http://localhost:8080/health
# curl -i http://localhost:8080/api/v1/services
# curl -i https://${DOMAIN}/api/v1/services
EOF
