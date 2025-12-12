#!/bin/bash
# scripts/nginx-setup.sh
# Generates Nginx config, enables it, and runs Certbot
# Usage: ./scripts/nginx-setup.sh <domain> [backend-port]

set -e

DOMAIN="$1"
PORT="${2:-8080}"

if [ -z "$DOMAIN" ]; then
    echo "Usage: $0 <domain> [backend-port]"
    exit 1
fi

CONFIG_FILE="/etc/nginx/sites-available/relaxation-hub"
SYMLINK="/etc/nginx/sites-enabled/relaxation-hub"

echo "🔧 Setting up Nginx for $DOMAIN on port $PORT..."

# Create Nginx config
cat <<EOF > "$CONFIG_FILE"
server {
    listen 80;
    server_name $DOMAIN;

    client_max_body_size 20M;

    location / {
        proxy_pass http://127.0.0.1:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 86400;
    }
}
EOF

echo "✅ Created $CONFIG_FILE"

# Enable site
if [ -L "$SYMLINK" ]; then
    rm "$SYMLINK"
fi
ln -s "$CONFIG_FILE" "$SYMLINK"
echo "✅ Enabled site (symlink created)"

# Test and reload
nginx -t
systemctl reload nginx
echo "✅ Nginx reloaded"

# Certbot
if command -v certbot &> /dev/null; then
    echo "🔒 Running Certbot..."
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m admin@$DOMAIN --redirect
else
    echo "⚠️ Certbot not found. Skipping SSL setup."
    echo "Install with: apt install certbot python3-certbot-nginx"
fi
