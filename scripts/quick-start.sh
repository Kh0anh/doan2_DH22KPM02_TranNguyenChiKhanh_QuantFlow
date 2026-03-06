#!/bin/bash
# ============================================================
# QuantFlow Quick Start Script
# ============================================================
# Author: Khanh
# Date: March 7, 2026
# Description: Automated setup script for first-time users
#
# Usage: ./scripts/quick-start.sh
# ============================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "=============================================="
echo "   QuantFlow Quick Start Setup Script"
echo "=============================================="
echo -e "${NC}"

# ===========================
# Step 1: Check Prerequisites
# ===========================
echo -e "${YELLOW}[Step 1/5] Checking prerequisites...${NC}"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker not found. Please install Docker first.${NC}"
    echo "  Visit: https://docs.docker.com/get-docker/"
    exit 1
fi
echo -e "${GREEN}✓ Docker found: $(docker --version)${NC}"

# Check Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${RED}✗ Docker Compose not found. Please install Docker Compose.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker Compose found: $(docker compose version)${NC}"

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo -e "${RED}✗ Docker daemon is not running. Please start Docker Desktop/Daemon.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker daemon is running${NC}"

# ===========================
# Step 2: Setup Environment
# ===========================
echo ""
echo -e "${YELLOW}[Step 2/5] Setting up environment variables...${NC}"

if [ -f .env ]; then
    echo -e "${YELLOW}⚠ .env file already exists. Skipping...${NC}"
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cp .env.example .env
        echo -e "${GREEN}✓ .env file created from template${NC}"
    fi
else
    cp .env.example .env
    echo -e "${GREEN}✓ .env file created from template${NC}"
fi

echo -e "${YELLOW}"
echo "IMPORTANT: Please edit .env file and change the following:"
echo "  1. POSTGRES_PASSWORD"
echo "  2. JWT_SECRET (min 32 chars)"
echo "  3. AES_KEY (exactly 32 bytes)"
echo -e "${NC}"

read -p "Press Enter to continue after editing .env, or Ctrl+C to exit..."

# ===========================
# Step 3: Generate SSL Certificate
# ===========================
echo ""
echo -e "${YELLOW}[Step 3/5] Generating SSL certificate...${NC}"

if [ -f nginx/ssl/server.crt ]; then
    echo -e "${YELLOW}⚠ SSL certificate already exists. Skipping...${NC}"
else
    chmod +x scripts/generate-ssl.sh
    ./scripts/generate-ssl.sh
    echo -e "${GREEN}✓ SSL certificate generated${NC}"
fi

# ===========================
# Step 4: Build and Start Services
# ===========================
echo ""
echo -e "${YELLOW}[Step 4/5] Building and starting Docker containers...${NC}"
echo "This may take 5-10 minutes on first run..."

docker compose up --build -d

# Wait for services to be healthy
echo ""
echo -e "${YELLOW}Waiting for services to be healthy...${NC}"
sleep 10

MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker compose ps | grep -q "healthy"; then
        echo -e "${GREEN}✓ Services are starting up...${NC}"
        break
    fi
    echo -n "."
    sleep 2
    RETRY_COUNT=$((RETRY_COUNT + 1))
done
echo ""

# ===========================
# Step 5: Verify Setup
# ===========================
echo ""
echo -e "${YELLOW}[Step 5/5] Verifying setup...${NC}"

# Check container status
echo -e "${BLUE}Container Status:${NC}"
docker compose ps

# Test backend health
echo ""
echo -e "${YELLOW}Testing backend health endpoint...${NC}"
sleep 5
if curl -s http://localhost/api/v1/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Backend API is responding${NC}"
else
    echo -e "${RED}✗ Backend API not responding yet (may need more time)${NC}"
fi

# Test frontend
echo ""
echo -e "${YELLOW}Testing frontend...${NC}"
if curl -s http://localhost > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Frontend is responding${NC}"
else
    echo -e "${RED}✗ Frontend not responding yet (may need more time)${NC}"
fi

# ===========================
# Summary
# ===========================
echo ""
echo -e "${GREEN}"
echo "=============================================="
echo "   Setup Complete!"
echo "=============================================="
echo -e "${NC}"

echo -e "${BLUE}Access Points:${NC}"
echo "  🌐 Frontend:  http://localhost"
echo "  🔌 Backend API: http://localhost/api/v1"
echo "  💬 WebSocket:   ws://localhost/ws"
echo "  🗄️  Database:   localhost:5432 (quantflow_db)"
echo ""
echo -e "${BLUE}Default Login:${NC}"
echo "  Username: admin"
echo "  Password: Admin@2026"
echo "  ${RED}⚠️  Change password immediately after first login!${NC}"
echo ""
echo -e "${BLUE}Useful Commands:${NC}"
echo "  View logs:       docker compose logs -f"
echo "  Stop services:   docker compose stop"
echo "  Start services:  docker compose start"
echo "  Restart all:     docker compose restart"
echo "  Clean up:        docker compose down"
echo ""
echo -e "${YELLOW}📖 For more details, see: README.md${NC}"
echo -e "${YELLOW}✅ For validation checklist: DOCKER_VALIDATION_CHECKLIST.md${NC}"
echo ""
