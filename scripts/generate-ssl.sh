#!/bin/bash
# ============================================================
# QuantFlow - Tạo Chứng chỉ SSL (Phát triển)
# ============================================================
# Tác giả: Khanh
# Mô tả:   Tạo chứng chỉ SSL tự ký cho HTTPS cục bộ
#
# Sử dụng:
#   chmod +x scripts/generate-ssl.sh
#   ./scripts/generate-ssl.sh
#
# Lưu ý: Khi lên production, sử dụng Let's Encrypt thay thế
# ============================================================

set -e

CERT_DIR="./nginx/ssl"
DAYS_VALID=365

echo "========================================="
echo "Tạo Chứng chỉ SSL (Phát triển)"
echo "========================================="

# Tạo thư mục SSL nếu chưa có
mkdir -p "$CERT_DIR"

# Tạo chứng chỉ tự ký
openssl req -x509 -nodes -days $DAYS_VALID \
    -newkey rsa:2048 \
    -keyout "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"

# Đặt quyền truy cập
chmod 600 "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/server.crt"

echo ""
echo "✓ Đã tạo chứng chỉ SSL thành công!"
echo ""
echo "Chứng chỉ: $CERT_DIR/server.crt"
echo "Khóa:      $CERT_DIR/server.key"
echo "Thời hạn:  $DAYS_VALID ngày"
echo ""
echo "CẢNH BÁO: Đây là chứng chỉ tự ký, chỉ dùng cho phát triển."
echo "Trình duyệt sẽ hiện cảnh báo bảo mật. Khi lên production, dùng Let's Encrypt."
echo ""
