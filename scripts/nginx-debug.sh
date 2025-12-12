#!/usr/bin/env bash
# Debug Nginx config and proxy forwarding issues that result in 405 Method Not Allowed
# Usage: sudo ./nginx-debug.sh <domain> <path> [local-port]

set -euo pipefail

DOMAIN=${1:-127.0.0.1}
PATH_TO_CHECK=${2:-/api/v1/register}
LOCAL_PORT=${3:-8080}

URL_PUBLIC="https://${DOMAIN}${PATH_TO_CHECK}"
URL_LOCAL="http://127.0.0.1:${LOCAL_PORT}${PATH_TO_CHECK}"

METHOD=POST
BODY='{"full_name":"Test User","provider":"email","provider_key":"test+debug@test.com","password":"TestPass123!","role":"client"}'

if [[ $EUID -ne 0 ]]; then
  echo "This script needs sudo to show nginx config and logs. Please run with sudo."
  exit 1
fi

# Test the app directly (bypass nginx)
echo "=> Direct local request to backend"
curl -sS -i -X ${METHOD} "${URL_LOCAL}" -H "Content-Type: application/json" -d "$BODY" || true

# Test via public domain (through Nginx)
echo -e "\n=> Request through public domain (via Nginx)"
curl -sS -i -X ${METHOD} "${URL_PUBLIC}" -H "Content-Type: application/json" -d "$BODY" || true

# Show nginx config and check for suspicious rules
echo -e "\n=> Nginx effective config (first 300 lines)"
nginx -T | sed -n '1,300p'

# Look for common gotchas: limit_except and proxy_pass trailing slash
echo -e "\n=> Nginx limit_except occurrences (should be none for /)"
grep -R --line-number "limit_except" /etc/nginx || echo "none found"

echo -e "\n=> Nginx proxy_pass statements for our domain (should point to 127.0.0.1:8080)"
grep -R --line-number "proxy_pass" /etc/nginx | sed -n '1,200p' || true

# Show logs for the last hits
echo -e "\n=> Nginx access log (last 100 lines)"
tail -n 100 /var/log/nginx/access.log || true

echo -e "\n=> Nginx error log (last 100 lines)"
tail -n 100 /var/log/nginx/error.log || true


cat <<EOF

Debug checklist:
 - If local request returns 200/400/201, but public request returns 405 -> Nginx causing 405 (look for limit_except or other config)
 - If local request returns 405, the backend route is not registered or is blocking method (check server logs for handler mapping)
 - Check `Server` response header to see if Nginx is returning the 405 or the backend
EOF
