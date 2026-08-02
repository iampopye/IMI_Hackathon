#!/bin/bash
# Setup script for AWS EC2 - Run this on your server
# Usage: bash setup-ec2.sh

set -e

# Root of the checkout. Override if you cloned somewhere else.
PROJECT_DIR="${PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"

echo "=== Installing Docker ==="
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    echo "Docker installed successfully"
else
    echo "Docker already installed"
fi

echo "=== Installing Docker Compose plugin ==="
apt-get update
apt-get install -y docker-compose-plugin

echo "=== Installing Go ==="
if ! command -v go &> /dev/null; then
    snap install go --classic
    echo "Go installed successfully"
else
    echo "Go already installed"
fi

echo "=== Installing Git ==="
apt-get install -y git

echo "=== Setting up the project ==="
cd $PROJECT_DIR/search-system

# Create config from example
cp config/config.yaml.example config/config.yaml

# Update config with correct credentials
sed -i 's/user: CHANGE_ME/user: searchuser/' config/config.yaml
sed -i 's/password: CHANGE_ME/password: searchpass/' config/config.yaml

echo "=== Starting PostgreSQL and Redis ==="
docker compose up -d postgres redis

echo "=== Waiting for PostgreSQL to be ready ==="
sleep 20

# Check if containers are running
docker ps

echo "=== Building and starting the API server ==="
# Create data directory for Bleve
mkdir -p $PROJECT_DIR/search-system/data/bleve

# Run the server in the background
nohup go run ./cmd/server > /root/server.log 2>&1 &

echo "=== Waiting for API to start ==="
sleep 30

echo "=== Checking API health ==="
curl -s http://localhost:8080/health || echo "API may still be starting..."

echo ""
echo "=== Setup Complete ==="
echo "API running at: http://$(curl -s --max-time 3 ifconfig.me || echo '<server-ip>'):8080"
echo "Check logs: tail -f /root/server.log"
echo ""
echo "Next: Open port 8080 in your AWS Security Group"
