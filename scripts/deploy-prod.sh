#!/bin/bash
# scripts/deploy-prod.sh
# Wrapper for deploy.sh with production defaults
# Usage: ./scripts/deploy-prod.sh [vps-ip]

set -e

VPS_IP="${1:-${REMOTE_HOST}}"

if [ -z "$VPS_IP" ]; then
    echo "Usage: $0 <vps-ip>"
    echo "Or set REMOTE_HOST environment variable"
    exit 1
fi

export REMOTE_HOST="$VPS_IP"
export REMOTE_USER="${REMOTE_USER:-root}"
export REMOTE_DIR="${REMOTE_DIR:-/opt/relaxation-hub}"
export ENV_FILE="${ENV_FILE:-.env}"

# Ensure deploy.sh is executable
chmod +x deploy.sh

echo "🚀 Deploying to Production ($REMOTE_HOST)..."
./deploy.sh
