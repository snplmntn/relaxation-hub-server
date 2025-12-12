#!/bin/bash
# scripts/nginx-debug.sh
# Diagnoses common Nginx and 405 errors
# Usage: ./scripts/nginx-debug.sh

echo "🔍 Starting Nginx Debugger..."

# 1. Check Nginx Status
echo "--- Nginx Status ---"
systemctl status nginx --no-pager | grep "Active:" || echo "Nginx not running"

# 2. Check Backend Connectivity
echo -e "\n--- Backend Connectivity (Local) ---"
if curl -s -I http://127.0.0.1:8080/health > /dev/null; then
    echo "✅ Backend is reachable at 127.0.0.1:8080"
else
    echo "❌ Backend NOT reachable at 127.0.0.1:8080"
    echo "Check docker container: docker compose ps"
fi

# 3. Check Nginx Config for Common 405 Causes
echo -e "\n--- Checking Nginx Config for 405 Triggers ---"
CONFIG_FILE="/etc/nginx/sites-enabled/relaxation-hub"
if [ -f "$CONFIG_FILE" ]; then
    echo "Config file: $CONFIG_FILE"
    
    if grep -q "try_files" "$CONFIG_FILE"; then
        echo "⚠️ WARNING: 'try_files' directive found. This often causes 405 on POST requests if not handled correctly."
        grep "try_files" "$CONFIG_FILE"
    fi

    if grep -q "limit_except" "$CONFIG_FILE"; then
        echo "⚠️ WARNING: 'limit_except' directive found. Ensure POST is allowed."
        grep "limit_except" "$CONFIG_FILE"
    fi

    if grep -q "error_page 405" "$CONFIG_FILE"; then
        echo "ℹ️ Note: Custom 405 error page defined."
    fi
else
    echo "❌ Config file $CONFIG_FILE not found."
fi

# 4. Test POST Request Locally (Bypass Nginx)
echo -e "\n--- Testing POST /api/v1/register (Direct to Backend) ---"
# We expect 400 Bad Request (invalid body) or 200/201, but NOT 405
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:8080/api/v1/register)
echo "Backend returned: $HTTP_CODE"
if [ "$HTTP_CODE" == "405" ]; then
    echo "❌ Backend itself is returning 405! Check route definition in Go code."
else
    echo "✅ Backend accepts POST (Code $HTTP_CODE is not 405)."
fi

# 5. Test POST Request via Nginx
echo -e "\n--- Testing POST /api/v1/register (Via Nginx) ---"
HTTP_CODE_NGINX=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: relaxation-hub.laundrykingmnl.com" -X POST http://127.0.0.1/api/v1/register)
echo "Nginx returned: $HTTP_CODE_NGINX"

if [ "$HTTP_CODE_NGINX" == "405" ]; then
    echo "❌ Nginx is returning 405. Issue is likely in Nginx config."
    echo "Possible fixes:"
    echo "1. Remove 'try_files' if serving an API."
    echo "2. Ensure 'proxy_pass' does NOT have a trailing slash (e.g. use 'http://127.0.0.1:8080' not '...:8080/')."
    echo "3. Check if you are redirecting HTTP->HTTPS and client converts POST to GET."
elif [ "$HTTP_CODE_NGINX" == "301" ] || [ "$HTTP_CODE_NGINX" == "308" ]; then
    echo "ℹ️ Nginx is redirecting (Code $HTTP_CODE_NGINX). Ensure client follows redirects correctly (Postman: enable 'Follow Authorization Header', check 'Follow Redirects')."
    echo "If client changes POST to GET on 301, backend will return 405."
else
    echo "✅ Nginx passed POST successfully (Code $HTTP_CODE_NGINX)."
fi

echo -e "\n--- Debug Complete ---"
