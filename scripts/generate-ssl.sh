#!/bin/bash
# ============================================================
# SSL Certificate Generator for QuantFlow (Development)
# ============================================================
# Author: Khanh
# Date: March 7, 2026
# Project: QuantFlow
# Description: Generate self-signed SSL certificate for local HTTPS testing
#
# Usage: 
# chmod +x scripts/generate-ssl.sh
# ./scripts/generate-ssl.sh
#
# Note: For production, use Let's Encrypt certificates instead
# ============================================================

set -e

CERT_DIR="./nginx/ssl"
DAYS_VALID=365

echo "========================================="
echo "SSL Certificate Generator (Development)"
echo "========================================="

# Create SSL directory if not exists
mkdir -p "$CERT_DIR"

# Generate self-signed certificate
openssl req -x509 -nodes -days $DAYS_VALID \
    -newkey rsa:2048 \
    -keyout "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"

# Set proper permissions
chmod 600 "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/server.crt"

echo ""
echo "✓ SSL Certificate generated successfully!"
echo ""
echo "Certificate: $CERT_DIR/server.crt"
echo "Private Key: $CERT_DIR/server.key"
echo "Valid for: $DAYS_VALID days"
echo ""
echo "WARNING: This is a self-signed certificate for development only."
echo "Browsers will show security warnings. For production, use Let's Encrypt."
echo ""
