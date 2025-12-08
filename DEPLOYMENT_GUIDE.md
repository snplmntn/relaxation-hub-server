# Deployment Guide - Dockerized VPS Deployment

Complete guide for deploying Relaxation Hub Server to a VPS using Docker.

---

## Prerequisites

### Local Machine

- Docker installed
- Docker Compose installed
- SSH access configured to your VPS

### Supabase Setup

- Supabase account (free tier available)
- Project created at https://supabase.com
- Database connection string obtained

### VPS Requirements

- Ubuntu 20.04+ or Debian 11+ (recommended)
- Minimum: 1GB RAM, 1 CPU core, 10GB storage (database hosted on Supabase)
- Recommended: 2GB RAM, 1 CPU core, 20GB storage
- Docker and Docker Compose installed
- Open ports: 80 (HTTP), 443 (HTTPS), 8080 (API)

---

## Step 1: VPS Setup

### 1.1 Connect to VPS

```bash
ssh root@your-vps-ip
```

### 1.2 Update System

```bash
apt update && apt upgrade -y
```

### 1.3 Install Docker

```bash
# Install dependencies
apt install -y apt-transport-https ca-certificates curl software-properties-common

# Add Docker GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Add Docker repository
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
apt update
apt install -y docker-ce docker-ce-cli containerd.io

# Verify installation
docker --version
```

### 1.4 Install Docker Compose

```bash
# Download Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose

# Make executable
chmod +x /usr/local/bin/docker-compose

# Verify installation
docker-compose --version
```

### 1.5 Configure Firewall

```bash
# Enable UFW
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 8080/tcp
ufw enable

# Check status
ufw status
```

---

## Step 2: Setup Supabase Database

### 2.1 Create Supabase Project

1. Go to https://supabase.com and sign in
2. Click "New Project"
3. Choose project name: `relaxation-hub`
4. Set database password (save this securely)
5. Select region closest to your VPS
6. Wait for project to be provisioned (~2 minutes)
   Edit `.env.production` with production values:

```bash
# Generate strong JWT key
openssl rand -base64 32

# Update .env.production with:
DATABASE_URL=your-supabase-connection-string-here
JWT_KEY=generated-jwt-key-here
```

Example `.env.production`:

```bash
DATABASE_URL=postgresql://postgres.abcdefghijklmnopqrst:MySecurePass123!@aws-0-us-east-1.pooler.supabase.com:5432/postgres
PORT=8080
JWT_KEY=xK7mP9nQ2rS5tU8vW0yZ3aB6cD9eF2gH5jK8mN1pQ4rS7tU0vW3yZ6aB9cD2eF5g=
```

### 3.2 Configure OAuth (Optional)shboard

2. Click "New Query"
3. Copy contents of `internal/db/migrations/001.sql`
4. Paste and run
5. Verify tables created in Table Editor

**Option B: Using psql CLI**

```bash
# Install psql (if not installed)
apt install postgresql-client

# Connect to Supabase
psql "postgresql://postgres.xxxxxxxxxxxxxxxxxxxx:your-password@aws-0-us-east-1.pooler.supabase.com:5432/postgres"

# Run migrations
\i internal/db/migrations/001.sql
\q
```

---

## Step 3: Configure Environment

### 3.1 Create Production Environment File

On your local machine:

```bash
cp .env.production.example .env.production
```

Edit `.env.production` with production values:

```bash
# Generate strong JWT key
openssl rand -base64 32

# Update .env.production
DB_PASSWORD=your-strong-password-here
JWT_KEY=generated-jwt-key-here
DATABASE_URL=postgresql://postgres:your-strong-password-here@postgres:5432/relaxation_hub?sslmode=require
```

### 2.2 Configure OAuth (Optional)

---

## Step 4: Deploy Using Automated Script

### 4.1 Update Deployment Script//yourdomain.com/api/v1/oauth/callback

```

---

## Step 3: Deploy Using Automated Script

REMOTE_DIR="/opt/relaxation-hub"
```

### 4.2 Make Script Executable

```bash
chmod +x deploy.sh
```

### 4.3 Run Deployment

### 3.2 Make Script Executable

```bash
chmod +x deploy.sh
```

### 3.3 Run Deployment

```bash
./deploy.sh
```

---

## Step 5: Manual Deployment (Alternative)

If automated script fails, deploy manually:

### 5.1 Build Docker Image

---

## Step 4: Manual Deployment (Alternative)

If automated script fails, deploy manually:

### 4.1 Build Docker Image

```bash
docker build -t relaxation-hub-server:latest .
```

### 5.2 Save and Transfer Image

```bash
# Save image
docker save relaxation-hub-server:latest | gzip > relaxation-hub-server.tar.gz

# Transfer to VPS
scp relaxation-hub-server.tar.gz root@your-vps-ip:/opt/relaxation-hub/
scp docker-compose.yml root@your-vps-ip:/opt/relaxation-hub/
scp .env.production root@your-vps-ip:/opt/relaxation-hub/.env
```

### 5.3 Load and Run on VPS

SSH into VPS:

```bash
ssh root@your-vps-ip
cd /opt/relaxation-hub

# Load image
docker load < relaxation-hub-server.tar.gz

# Start containers
docker-compose up -d

# Check logs
docker-compose logs -f
```

---

## Step 6: Verify Supabase Connection

### 6.1 Test Database Connection

```bash
# From VPS
docker-compose exec server wget -O- http://localhost:8080/api/v1/services

# Should return JSON array (empty if no services created yet)
```

### 6.2 Check Logs for Database Errors

```bash
docker-compose logs server | grep -i "database\|postgres\|connection"
```

---

## Step 7: Setup Nginx Reverse Proxy (Recommended)

## Step 7: Setup Nginx Reverse Proxy (Recommended)

### 7.1 Install Nginx

```bash
apt install -y nginx
apt install -y nginx
```

### 7.2 Configure Nginx

Create `/etc/nginx/sites-available/relaxation-hub`:

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    client_max_body_size 20M;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }

    # WebSocket support
    location /api/v1/ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }
}
}
```

### 7.3 Enable Site

```bash
ln -s /etc/nginx/sites-available/relaxation-hub /etc/nginx/sites-enabled/
nginx -t
systemctl restart nginx
```

---

---

## Step 8: Setup SSL with Let's Encrypt

### 8.1 Install Certbot

```bash
apt install -y certbot python3-certbot-nginx
apt install -y certbot python3-certbot-nginx
```

### 8.2 Obtain Certificate

```bash
certbot --nginx -d yourdomain.com

# Follow prompts
# Select option 2 to redirect HTTP to HTTPS
# Select option 2 to redirect HTTP to HTTPS
```

### 8.3 Auto-Renewal

```bash
# Test renewal
certbot renew --dry-run

# Renewal happens automatically via cron
```

---

---

## Step 9: Verify Deployment

### 9.1 Check Container Status

```bash
docker-compose ps
```

Expected output:
Expected output:

```
NAME                       COMMAND                  STATUS              PORTS
relaxation-hub-server      "./server"               Up (healthy)        0.0.0.0:8080->8080/tcp
```

### 9.2 Check Logs

```bash
# All logs
# All logs
docker-compose logs -f

# Server logs
docker-compose logs -f server
```

### 9.3 Test API Endpoints

```bash
# Health check (public endpoint)
curl http://your-vps-ip:8080/api/v1/services

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
```

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
```

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

```bash
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
```
