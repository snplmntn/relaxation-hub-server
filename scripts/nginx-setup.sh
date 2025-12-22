#!/usr/bin/env bash
# Create and enable an Nginx server block and optionally install TLS with Certbot
# Usage: sudo ./nginx-setup.sh <domain> <proxy_host> <proxy_port>

set -euo pipefail

DOMAIN=${1:-relaxation-hub.laundrykingmnl.com}
PROXY_HOST=${2:-127.0.0.1}
PROXY_PORT=${3:-8080}

SITE_CONF="/etc/nginx/sites-available/relaxation-hub"
SITE_ENABLED="/etc/nginx/sites-enabled/relaxation-hub"

if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root (sudo)"
  exit 1
fi

cat > "$SITE_CONF" <<EOF
server {
  listen 80;
  server_name ${DOMAIN};

  client_max_body_size 20M;

  location / {
    proxy_pass http://${PROXY_HOST}:${PROXY_PORT};
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 86400;
  }
}
EOF

# ensure enabled (symlink)
ln -sf "$SITE_CONF" "$SITE_ENABLED"

# test and reload
nginx -t
systemctl reload nginx

cat <<EOF
Nginx site created and enabled: ${DOMAIN}
To enable TLS, run: sudo certbot --nginx -d ${DOMAIN}
EOF
