#!/bin/bash
# Deployment script for VPS with Supabase

set -e

echo "🚀 Starting deployment..."

# Configuration
REMOTE_USER="root"
REMOTE_HOST="your-vps-ip"  # CHANGE THIS to your actual VPS IP
REMOTE_DIR="/opt/relaxation-hub"
DOCKER_IMAGE="relaxation-hub-server"

# Check if .env exists (use .env for local, .env.production for prod)
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found"
    echo "Copy .env.production.example to .env and configure it with your Supabase credentials"
    exit 1
fi

# Verify DATABASE_URL is set
if ! grep -q "DATABASE_URL=" .env; then
    echo "❌ Error: DATABASE_URL not set in .env"
    exit 1
fi

echo "✅ Configuration validated"

# Build Docker image locally
echo "📦 Building Docker image..."
docker build -t $DOCKER_IMAGE:latest .

# Save Docker image to tar
echo "💾 Saving Docker image..."
docker save $DOCKER_IMAGE:latest | gzip > relaxation-hub-server.tar.gz

# Create remote directory
echo "📁 Creating remote directory..."
ssh $REMOTE_USER@$REMOTE_HOST "mkdir -p $REMOTE_DIR"

# Copy files to VPS
echo "📤 Uploading files to VPS..."
scp relaxation-hub-server.tar.gz $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/
scp docker-compose.yml $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/
scp .env $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/
echo "✅ Files uploaded"

# Deploy on VPS
echo "🔧 Deploying on VPS..."
ssh $REMOTE_USER@$REMOTE_HOST << 'ENDSSH'
cd /opt/relaxation-hub

# Load Docker image
echo "Loading Docker image..."
docker load < relaxation-hub-server.tar.gz

# Stop and remove old containers
echo "Stopping old containers..."
docker-compose down || true

# Start new containers
echo "Starting new containers..."
docker-compose up -d

# Clean up
echo "Cleaning up..."
rm relaxation-hub-server.tar.gz

# Show status
echo "Checking container status..."
docker-compose ps

# Check logs
echo "Recent logs:"
docker-compose logs --tail=20 server

echo "✅ Deployment complete!"
ENDSSH

# Clean up local tar file
rm relaxation-hub-server.tar.gz

echo "✅ Deployment completed successfully!"
echo "🌐 Your server is running at http://$REMOTE_HOST:8080"
echo "📝 Check logs: ssh $REMOTE_USER@$REMOTE_HOST 'cd /opt/relaxation-hub && docker-compose logs -f server'"
docker-compose down || true

# Start new containers
echo "Starting new containers..."
docker-compose up -d

# Clean up
echo "Cleaning up..."
rm relaxation-hub-server.tar.gz

# Show status
echo "Checking container status..."
docker-compose ps

echo "✅ Deployment complete!"
ENDSSH

# Clean up local tar file
rm relaxation-hub-server.tar.gz

echo "✅ Deployment completed successfully!"
echo "🌐 Your server should be running at http://$REMOTE_HOST:8080"
