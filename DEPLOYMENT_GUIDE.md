# Deployment Guide - Dockerized VPS (Supabase DB)

Concise steps to deploy Relaxation Hub Server to a VPS with Docker, using Supabase as the managed Postgres.

---

## 1) Prerequisites

- Local: Docker Desktop (or Docker Engine) with Compose plugin; SSH access to the VPS.
- VPS: Ubuntu 22.04+ recommended; open ports 80/443/8080; Docker + Compose installed.
- Supabase: project created; use the _pooler_ connection string (port 6543, `sslmode=require`).

### Install Docker on VPS (Ubuntu)

```bash
sudo apt update && sudo apt install -y ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update && sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
newgrp docker
docker version
```

---

## 2) Configure environment

Create `.env` in the repo root (used by Compose):

```env
DATABASE_URL=postgresql://<user>:<password>@<host>:6543/postgres?sslmode=require  # Supabase pooler URL
JWT_KEY=<strong-random-secret>
PORT=8080

# Optional OAuth
GOOGLE_OAUTH_CLIENT_ID=
GOOGLE_OAUTH_CLIENT_SECRET=
GOOGLE_OAUTH_CALLBACK_URL=
APPLE_OAUTH_CLIENT_ID=
APPLE_OAUTH_CLIENT_SECRET=
APPLE_OAUTH_TEAM_ID=
APPLE_OAUTH_KEY_ID=
APPLE_OAUTH_CALLBACK_URL=
APPLE_OAUTH_PRIVATE_KEY=
```

---

## 3) Build and run locally (smoke test)

```bash
docker compose up --build
# then
curl http://localhost:8080/api/v1/services
```

---

## 4) Deploy to VPS (manual)

On your workstation:

```bash
ssh root@<vps-ip> "mkdir -p /opt/relaxation-hub"
scp -r Dockerfile docker-compose.yml .env cmd internal go.mod go.sum app README.md root@<vps-ip>:/opt/relaxation-hub/
```

Then on the VPS:

```bash
ssh root@<vps-ip>
cd /opt/relaxation-hub
docker compose up --build -d
docker compose ps
docker compose logs --tail=50 server
```

---

## 4b) Deploy to VPS (automated script)

The repo includes `deploy.sh` to build, ship, and start the container on the VPS.

Prereqs: Docker + Compose on both local and VPS; SSH access; `.env` populated with Supabase pooler URL and JWT key.

On your workstation (adjust `REMOTE_HOST` as needed):

```bash
export REMOTE_HOST=<vps-ip>
export REMOTE_USER=root            # or another sudo-capable user
# optional overrides:
# export REMOTE_DIR=/opt/relaxation-hub
# export IMAGE_NAME=relaxation-hub-server
# export ENV_FILE=.env

./deploy.sh
```

What it does:

- Builds the image locally
- Saves it to a tar.gz
- SCPs image tar, `docker-compose.yml`, and `.env` to the VPS
- Loads the image, `docker compose down`, `docker compose up -d`, shows status/logs
- Cleans up the tar on both sides

---

## 5) Verify

```bash
curl http://<vps-ip>:8080/api/v1/services
docker compose ps
docker compose logs --tail=50 server
```

---

## 6) Optional: Nginx + TLS

- Point DNS `A` record to the VPS IP.
- Install Nginx: `sudo apt install -y nginx`.
- Create `/etc/nginx/sites-available/relaxation-hub`:

```nginx
server {
  listen 80;
  server_name relaxation-hub.laundrykingmnl.com;

  client_max_body_size 20M;

  location / {
    proxy_pass http://127.0.0.1:8080;
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
```

```bash
sudo ln -s /etc/nginx/sites-available/relaxation-hub /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

Add TLS with Certbot (after DNS is in place):

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d relaxation-hub.laundrykingmnl.com
sudo certbot renew --dry-run
```

---

## 7) Troubleshooting quick hits

- Docker not running on Windows: start Docker Desktop; confirm with `docker version`.
- Build fails `go >= 1.25.4`: ensure Dockerfile uses `golang:1.25.4-alpine` (already set).
- DB errors: confirm `DATABASE_URL` uses Supabase pooler (port 6543, `sslmode=require`).
- Permission errors on VPS: if using non-root, add that user to the `docker` group and re-login.

---

## About shell scripts

There are currently no `.sh` scripts in the repo. If you want an automated deploy script (e.g., build, scp, and `docker compose up` on the VPS), we can add one—say `deploy.sh` targeting `/opt/relaxation-hub`.

# Expected: JSON array of services

```

### 8.4 Test WebSocket Connection

# Expected: JSON array of services

```

### 9.4 Test WebSocket Connection

# Connect (replace with your JWT token)

wscat -c "ws://your-vps-ip:8080/api/v1/ws?token=YOUR_JWT_TOKEN"

```

---

## Step 9: Monitoring and Maintenance

---

## Step 10: Monitoring and Maintenance

### 10.1 View Resource Usage

# Disk usage

docker system df

```

### 9.2 Database Backup

docker system df

````

### 10.2 Database Backup (Supabase)

**Automatic Backups:**

- Supabase automatically backs up your database daily
- Access backups in: Supabase Dashboard → Database → Backups

**Manual Backup:**

```bash
# Install psql client
apt install postgresql-client

# Create backup
pg_dump "postgresql://postgres.xxxx:password@aws-0-us-east-1.pooler.supabase.com:5432/postgres" > backup_$(date +%Y%m%d).sql

# Restore backup
psql "postgresql://postgres.xxxx:password@aws-0-us-east-1.pooler.supabase.com:5432/postgres" < backup_20241209.sql
````

docker-compose up -d

```

### 10.4 View Logs with Date Filter
docker-compose pull
docker-compose up -d
```

### 9.4 View Logs with Date Filter

---

## Step 11: Troubleshooting

### Container won't start

---

## Step 10: Troubleshooting

### Container won't start

````bash
# Check logs
docker-compose logs server

# Check environment variables
docker-compose config
### Supabase connection failed

```bash
# Check DATABASE_URL is correct
cat .env | grep DATABASE_URL

# Test connection from VPS
apt install postgresql-client
psql "$DATABASE_URL" -c "SELECT 1;"

# Check Supabase project is active
# Go to Supabase Dashboard → check project status

# Verify IP not blocked
# Supabase Dashboard → Settings → Database → Connection pooling
# Check if your VPS IP needs to be whitelisted
```est connection
docker-compose exec postgres psql -U postgres -d relaxation_hub -c "SELECT 1;"

# Verify DATABASE_URL in .env
cat .env | grep DATABASE_URL
````

### Port already in use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or change port in .env and docker-compose.yml
```

### Out of disk space

```bash
# Clean up Docker
docker system prune -a --volumes

# Check disk usage
df -h
```

### High memory usage

```bash
# Limit container memory in docker-compose.yml
services:
  server:
    deploy:
      resources:
        limits:
          memory: 512M
```

---

## Security Best Practices

1. **Change default passwords** in `.env.production`
2. **Enable firewall** with UFW
3. **Use HTTPS** with Let's Encrypt SSL
4. **Restrict database access** to localhost only
5. **Regular backups** of database
6. **Update Docker images** regularly
7. **Monitor logs** for suspicious activity
8. **Use non-root user** in Docker containers (already configured)
9. **Disable password authentication** for SSH (use keys only)
10. **Keep system updated**: `apt update && apt upgrade`

---

## Performance Optimization

### Enable Connection Pooling

Already configured in `internal/db/database.go`:

```go
MaxConns: 25
MinConns: 5
```

### Nginx Caching

Add to Nginx config:

```nginx
proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=api_cache:10m max_size=1g inactive=60m;

location /api/v1/services {
    proxy_cache api_cache;
    proxy_cache_valid 200 5m;
    proxy_pass http://localhost:8080;
}
```

### Database Indexes

Run on production database:

```sql
-- Already created in migrations
CREATE INDEX idx_bookings_client ON bookings(client_id);
CREATE INDEX idx_bookings_therapist ON bookings(therapist_id);
CREATE INDEX idx_bookings_status ON bookings(status);
```

---

## Rollback Procedure

If deployment fails:

````bash
# On VPS
cd /opt/relaxation-hub

---

## Cost Estimation

### Total Monthly Costs

| Service       | Plan          | Features                      | Price/Month |
|---------------|---------------|-------------------------------|-------------|
| Supabase      | Free Tier     | 500MB DB, 2GB bandwidth       | $0          |
| Supabase      | Pro Plan      | 8GB DB, 50GB bandwidth        | $25         |
| DigitalOcean  | Basic Droplet | 1GB RAM, 1 CPU, 25GB storage  | $6          |
| DigitalOcean  | Basic Droplet | 2GB RAM, 1 CPU, 50GB storage  | $12         |
| Linode        | Nanode        | 1GB RAM, 1 CPU, 25GB storage  | $5          |
| Vultr         | Regular       | 1GB RAM, 1 CPU, 25GB storage  | $6          |

**Recommended Setup:**
- **Development:** Supabase Free + $6 VPS = **$6/month**
- **Production:** Supabase Pro + $12 VPS = **$37/month**

**Note:** Using Supabase eliminates need for database hosting on VPS, reducing RAM requirements.
### VPS Providers

| Provider      | Plan          | RAM   | CPU | Storage | Price/Month |
|---------------|---------------|-------|-----|---------|-------------|
| DigitalOcean  | Basic Droplet | 2GB   | 1   | 50GB    | $12         |
| Linode        | Nanode        | 1GB   | 1   | 25GB    | $5          |
| Vultr         | Regular       | 2GB   | 1   | 55GB    | $12         |
| AWS Lightsail | 2GB Plan      | 2GB   | 1   | 60GB    | $12         |

**Recommended:** DigitalOcean or Vultr for production workloads.

---

## Support

For issues:
1. Check logs: `docker-compose logs -f`
2. Verify environment: `docker-compose config`
3. Review documentation in README.md
4. Check API flows in API_FLOWS_DOCUMENTATION.md

---

**Last Updated:** December 9, 2024
**Tested On:** Ubuntu 22.04 LTS, Docker 24.0.7, Docker Compose 2.23.0

---

## Updating the deployment from GitHub (pull on VPS)

If you'd prefer to update the running deployment by pulling changes on the VPS (instead of SCP-ing artifacts), follow these steps. This assumes the repo is cloned to the VPS at the same `REMOTE_DIR` used for deploy (example: `/opt/relaxation-hub`) and the remote is configured to point to your GitHub repo.

1. SSH into the VPS and switch to the deployment directory:

```bash
ssh root@<vps-ip>
cd /opt/relaxation-hub
````

2. Pull the latest changes from the desired branch (e.g. `main`):

```bash
git fetch origin
git checkout main
git pull origin main
```

3. (Optional) If your Docker image is built inside the repo (via `Dockerfile`), rebuild and restart the service with Compose:

```bash
docker compose down
docker compose pull        # if images are published to a registry
docker compose build       # rebuild local image if needed
docker compose up -d
```

4. Verify the service:

```bash
docker compose ps
docker compose logs --tail=100 server
curl http://127.0.0.1:8080/api/v1/services
```

Notes and recommendations:

- Ensure the `.env` on the VPS is up-to-date and not overwritten by `git pull` (keep `.env` in `.gitignore` and out of the repo), or store env in a secure location (e.g. Docker secrets).
- If you prefer zero-downtime deploys, use `docker compose up -d --no-deps --build server` targeting just the service and/or consider rolling updates with a container orchestrator.
- Consider adding a lightweight `update_from_git.sh` script on the VPS and an SSH-protected GitHub webhook to trigger pulls / rebuilds (or use CI/CD to build/push images to a registry and pull on the VPS).

If you'd like, I can:

- Add the `update_from_git.sh` script to the repo (with safe checks), or
- Add a `deploy-from-git` section to `deploy.sh` to optionally support remote `git pull` flows.

```

```
