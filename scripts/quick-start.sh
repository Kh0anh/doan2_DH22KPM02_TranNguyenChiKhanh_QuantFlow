#!/bin/bash
# ============================================================
# QuantFlow - Script Khởi động Nhanh (Linux/macOS)
# ============================================================
# Tác giả: Khanh
# Dự án:   QuantFlow
# Mô tả:   Tự động cài đặt và khởi động dự án
#
# Sử dụng:
#   chmod +x scripts/quick-start.sh
#   ./scripts/quick-start.sh
# ============================================================

set -e

echo ""
echo "=============================================="
echo "   QuantFlow - Khởi động Nhanh"
echo "=============================================="
echo ""

# ===========================
# Bước 1: Kiểm tra yêu cầu
# ===========================
echo "[Bước 1/4] Kiểm tra yêu cầu hệ thống..."
echo ""

# Kiểm tra Docker
if ! command -v docker &> /dev/null; then
    echo "[LỖI] Không tìm thấy Docker. Vui lòng cài đặt Docker trước."
    echo "Tải về: https://docs.docker.com/get-docker/"
    exit 1
fi
echo "[OK] Đã tìm thấy Docker"

# Kiểm tra Docker Compose
if ! docker compose version &> /dev/null; then
    echo "[LỖI] Không tìm thấy Docker Compose. Vui lòng cập nhật Docker."
    exit 1
fi
echo "[OK] Đã tìm thấy Docker Compose"

# Kiểm tra Docker đang chạy
if ! docker info &> /dev/null; then
    echo "[LỖI] Docker chưa chạy. Vui lòng khởi động Docker."
    exit 1
fi
echo "[OK] Docker đang chạy"
echo ""

# ===========================
# Bước 2: Cấu hình môi trường
# ===========================
echo "[Bước 2/4] Cấu hình môi trường..."
echo ""

if [ -f .env ]; then
    echo "[THÔNG BÁO] File .env đã tồn tại, sử dụng file hiện có."
else
    cp .env.example .env
    echo "[OK] Đã tạo file .env từ .env.example"
fi
echo ""

# ===========================
# Bước 3: Khởi động dịch vụ
# ===========================
echo "[Bước 3/4] Đang khởi động các dịch vụ Docker..."
echo "Lần đầu có thể mất 5-10 phút..."
echo ""
echo "Chứng chỉ SSL sẽ được tạo tự động nếu chưa có."
echo ""

docker compose up --build -d

echo ""
echo "Đang chờ các dịch vụ sẵn sàng..."
sleep 20

# ===========================
# Bước 4: Xác nhận
# ===========================
echo ""
echo "[Bước 4/4] Kiểm tra trạng thái..."
echo ""

echo "Trạng thái Container:"
docker compose ps
echo ""

echo "Kiểm tra Backend API..."
sleep 5
if curl -s http://localhost/api/v1/health > /dev/null 2>&1; then
    echo "[OK] Backend API đang hoạt động"
else
    echo "[CHỜ] Backend API chưa sẵn sàng (cần thêm thời gian)"
fi

echo ""
echo "Kiểm tra Frontend..."
if curl -s http://localhost > /dev/null 2>&1; then
    echo "[OK] Frontend đang hoạt động"
else
    echo "[CHỜ] Frontend chưa sẵn sàng (cần thêm thời gian)"
fi

# ===========================
# Hoàn tất
# ===========================
echo ""
echo "=============================================="
echo "   Hoàn tất Cài đặt!"
echo "=============================================="
echo ""
echo "Địa chỉ truy cập:"
echo "  Giao diện:      http://localhost"
echo "  Backend API:    http://localhost/api/v1"
echo "  WebSocket:      ws://localhost/ws"
echo "  Cơ sở dữ liệu: localhost:5432"
echo ""
echo "Tài khoản mặc định:"
echo "  Tên đăng nhập: admin"
echo "  Mật khẩu:      Admin@2026"
echo "  CẢNH BÁO: Hãy đổi mật khẩu ngay sau khi đăng nhập!"
echo ""
echo "Các lệnh thường dùng:"
echo "  Xem log:        docker compose logs -f"
echo "  Dừng dịch vụ:   docker compose stop"
echo "  Khởi động lại:  docker compose restart"
echo "  Dọn dẹp:        docker compose down"
echo ""
