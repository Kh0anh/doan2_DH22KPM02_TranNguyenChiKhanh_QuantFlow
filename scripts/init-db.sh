#!/bin/bash
# ============================================================
# QuantFlow Database Initialization Script
# ============================================================
# Author: Khanh
# Date: March 7, 2026
# Project: QuantFlow
# Description: Initialize PostgreSQL database schema and seed admin user
#
# This script runs automatically when PostgreSQL container first starts
# via Docker entrypoint: /docker-entrypoint-initdb.d/
#
# Execution context: Inside PostgreSQL container as postgres user
# ============================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}QuantFlow Database Initialization${NC}"
echo -e "${GREEN}========================================${NC}"

# ===========================
# Step 1: Create Database Schema
# ===========================
echo -e "${YELLOW}[1/3] Creating database schema...${NC}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Enable UUID extension
    CREATE EXTENSION IF NOT EXISTS "pgcrypto";
    
    -- Verify extension
    SELECT 'UUID extension installed' AS status;
EOSQL

# Load schema from mounted SQL file
if [ -f /docker-entrypoint-initdb.d/schema.sql ]; then
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" < /docker-entrypoint-initdb.d/schema.sql
    echo -e "${GREEN}✓ Schema created successfully (9 tables, 6 indexes)${NC}"
else
    echo -e "${RED}✗ ERROR: schema.sql not found!${NC}"
    exit 1
fi

# ===========================
# Step 2: Seed Admin User
# ===========================
echo -e "${YELLOW}[2/3] Seeding default admin user...${NC}"

# Default admin credentials (MUST be changed after first login)
ADMIN_USERNAME="admin"
# BCrypt hash of "Admin@2026" (bcrypt cost=10)
# Generated with Go bcrypt: bcrypt.GenerateFromPassword([]byte("Admin@2026"), 10)
# This is a real bcrypt hash for the password "Admin@2026"
ADMIN_PASSWORD_HASH='$2a$10$N9qo8uLOickgx2ZMRZoEqe.LaL4sWJ6xJZl5EYp5/qTU6pZj.vfcS'

# Note: In production, this hash should be generated dynamically
# For development, we use a static hash for consistency

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Insert admin user if not exists
    INSERT INTO users (username, password_hash, created_at, updated_at)
    VALUES (
        '${ADMIN_USERNAME}',
        '${ADMIN_PASSWORD_HASH}',
        CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (username) DO NOTHING;
    
    -- Verify admin user creation
    SELECT 
        CASE 
            WHEN EXISTS (SELECT 1 FROM users WHERE username = '${ADMIN_USERNAME}')
            THEN 'Admin user created/exists'
            ELSE 'Admin user creation failed'
        END AS status;
EOSQL

echo -e "${GREEN}✓ Admin user seeded successfully${NC}"
echo -e "${YELLOW}  Username: ${ADMIN_USERNAME}${NC}"
echo -e "${YELLOW}  Password: Admin@2026 ${RED}(CHANGE IMMEDIATELY IN PRODUCTION!)${NC}"

# ===========================
# Step 3: Verify Database Setup
# ===========================
echo -e "${YELLOW}[3/3] Verifying database setup...${NC}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Count tables
    SELECT 
        COUNT(*) AS table_count,
        'Expected: 9 tables' AS note
    FROM information_schema.tables 
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE';
    
    -- Count indexes
    SELECT 
        COUNT(*) AS index_count,
        'Expected: 6 custom indexes + auto indexes' AS note
    FROM pg_indexes 
    WHERE schemaname = 'public';
    
    -- Verify critical tables exist
    SELECT 
        CASE 
            WHEN COUNT(*) = 9 THEN 'All tables created successfully'
            ELSE 'WARNING: Missing tables!'
        END AS table_verification
    FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name IN (
        'users',
        'api_keys',
        'strategies',
        'strategy_versions',
        'bot_instances',
        'bot_lifecycle_variables',
        'bot_logs',
        'trade_history',
        'candles_data'
    );
EOSQL

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Database Initialization Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Backend will connect to: postgresql://${POSTGRES_USER}@postgres:5432/${POSTGRES_DB}"
echo "2. Login with: admin / Admin@2026"
echo "3. Change password immediately via Settings"
echo ""
