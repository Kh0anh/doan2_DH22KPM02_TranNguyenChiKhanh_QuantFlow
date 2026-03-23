# QuantFlow - Nền tảng Low-code Giao dịch Tự động

![Trạng thái](https://img.shields.io/badge/Trạng_thái-Đang_phát_triển-orange)
![Lĩnh vực](https://img.shields.io/badge/Lĩnh_vực-Kỹ_thuật_Phần_mềm-blue)
![Trường](https://img.shields.io/badge/Trường-ĐH_Nam_Cần_Thơ-green)
![License](https://img.shields.io/badge/License-GPL_v3-red)

## Giới thiệu

**QuantFlow** là đồ án chuyên ngành Kỹ thuật Phần mềm, tập trung vào việc giải quyết rào cản kỹ thuật cho người dùng cá nhân trong thị trường tiền mã hóa. Hệ thống cung cấp một nền tảng lập trình trực quan giúp xây dựng và vận hành bot giao dịch tự động trên các sàn giao dịch tập trung (CEX) mà không yêu cầu kỹ năng lập trình chuyên sâu.

| Thông tin | Chi tiết |
|-----------|----------|
| **Tên đề tài** | Phát triển nền tảng Low-code hỗ trợ xây dựng và vận hành chiến lược giao dịch tiền mã hóa trên sàn giao dịch tập trung (CEX) |
| **Sản phẩm** | QuantFlow |
| **Sinh viên** | Trần Nguyễn Chí Khanh |
| **MSSV** | 220979 |
| **Đơn vị** | Trường Đại học Nam Cần Thơ |
| **Thời gian** | 12/01/2026 – 23/03/2026 |

---

## Chức năng chính

### Quản trị và Cấu hình
- Hệ thống hỗ trợ mô hình đơn người dùng (Single User/Admin)
- Quản lý thông tin cá nhân: Đăng nhập, đổi tên và mật khẩu
- Quản lý an toàn API Key/Secret Key kết nối với sàn giao dịch

### Xây dựng Chiến lược (Strategy Builder)
- Giao diện kéo thả trực quan sử dụng thư viện **Google Blockly**
- Bộ khối logic: Điều kiện (If/Else), Vòng lặp, Toán học, Hàm chỉ báo và các khối giao dịch (Mua/Bán/Hủy lệnh)
- Chuyển đổi tự động từ sơ đồ khối sang mã thực thi (Code Generation)

### Vận hành và Giám sát
- Khởi chạy và dừng bot linh hoạt theo yêu cầu
- Hiển thị nhật ký (Logs) và trạng thái lệnh Real-time

### Kết nối Thị trường
- Lấy dữ liệu giá thị trường (Market Data) từ sàn CEX
- Gửi lệnh giao dịch (Order Placement) qua API của sàn

---

## Công nghệ sử dụng

| Thành phần | Công nghệ |
|------------|-----------|
| **Backend** | Golang 1.25 (GORM + pgx, gorilla/websocket) |
| **Frontend** | Next.js 16 (React 19, TypeScript, Tailwind CSS v4, shadcn/ui) |
| **Cơ sở dữ liệu** | PostgreSQL 16 |
| **Reverse Proxy** | Nginx 1.26 |
| **Container** | Docker & Docker Compose |
| **Trình soạn khối** | Google Blockly v12 |

---

## Cài đặt và Chạy Hệ thống

### Yêu cầu hệ thống

| Công cụ | Phiên bản tối thiểu | Kiểm tra |
|---------|---------------------|----------|
| **Docker Engine** | 24.0+ | `docker --version` |
| **Docker Compose** | 2.20+ | `docker compose version` |
| **Git** | 2.30+ | `git --version` |

> **Lưu ý:** Đối với Windows, khuyến nghị cài đặt **Docker Desktop**.
> Đối với Linux, cài đặt Docker Engine + Docker Compose plugin.

### Bước 1: Clone Repository

```bash
git clone https://github.com/Kh0anh/doan2_DH22KPM02_TranNguyenChiKhanh_QuantFlow.git
cd doan2_DH22KPM02_TranNguyenChiKhanh_QuantFlow
```

### Bước 2: Cấu hình Môi trường

```bash
# Sao chép file mẫu (không cần chỉnh sửa gì cho development)
cp .env.example .env
```

> File `.env.example` đã có sẵn giá trị mặc định, sẵn sàng chạy ngay.
> Nếu muốn tùy chỉnh, mở file `.env` và thay đổi giá trị tương ứng.

### Bước 3: Khởi động

```bash
# Khởi động tất cả dịch vụ
docker compose up -d

# Xem logs
docker compose logs -f
```

> Chứng chỉ SSL sẽ được **tự động tạo** khi khởi động lần đầu.
> Lần đầu chạy có thể mất 5-10 phút để tải image và cài đặt.

### Bước 4: Truy cập Hệ thống

| Dịch vụ | Địa chỉ | Mô tả |
|---------|---------|-------|
| **Giao diện** | http://localhost | Trang web chính |
| **Backend API** | http://localhost/api/v1 | RESTful API |
| **WebSocket** | ws://localhost/ws | Kết nối thời gian thực |
| **Cơ sở dữ liệu** | localhost:5432 | PostgreSQL |

**Tài khoản mặc định:**
- Tên đăng nhập: `admin`
- Mật khẩu: `123456`

> **BẢO MẬT:** Hãy đổi mật khẩu ngay sau lần đăng nhập đầu tiên!

---

## Các lệnh thường dùng

```bash
# Dừng tất cả dịch vụ (giữ nguyên dữ liệu)
docker compose stop

# Khởi động lại
docker compose restart

# Dừng và xóa container (giữ nguyên dữ liệu)
docker compose down

# Dừng và xóa container + dữ liệu (XÓA TOÀN BỘ!)
docker compose down -v

# Xem trạng thái
docker compose ps

# Rebuild (sau khi sửa code)
docker compose up --build -d
```

---

## Xử lý Lỗi Thường Gặp

### 1. Cổng đã bị chiếm (Port already in use)

```bash
# Windows
netstat -ano | findstr :80
netstat -ano | findstr :5432

# Linux/Mac
lsof -i :80
lsof -i :5432
```

### 2. Backend không kết nối được Database

```bash
# Kiểm tra PostgreSQL đã sẵn sàng chưa
docker compose logs postgres

# Khởi động lại backend
docker compose restart backend
```

### 3. Frontend không load được (404)

```bash
# Xóa cache và rebuild
docker compose exec frontend rm -rf .next
docker compose restart frontend
```

### 4. Lỗi Chứng chỉ SSL

```bash
# Xóa và tạo lại (tự động tạo khi restart nginx)
rm -rf nginx/ssl/*
docker compose restart nginx
```

---

## Quản lý Cơ sở Dữ liệu

### Sao lưu (Backup)

```bash
# Xuất file SQL
docker compose exec postgres pg_dump -U postgres postgres > backup.sql
```

### Phục hồi (Restore)

```bash
# Nhập từ file SQL
docker compose exec -T postgres psql -U postgres postgres < backup.sql
```

### Kết nối bằng Client (DBeaver, pgAdmin)

```
Host:     localhost
Port:     5432
Database: postgres
Username: postgres
Password: 123 (hoặc giá trị trong file .env)
```

---

## Cấu trúc Dự án

```
QuantFlow/
├── backend/          # Go API Server
│   ├── cmd/          # Điểm khởi động (main.go)
│   ├── config/       # Cấu hình ứng dụng
│   ├── internal/     # Logic nghiệp vụ
│   │   ├── engine/   # Bot engine & Blockly interpreter
│   │   ├── handler/  # HTTP & WebSocket handlers
│   │   ├── logic/    # Business logic
│   │   ├── model/    # Database models
│   │   └── repository/ # Data access layer
│   └── go.mod
├── frontend/         # Next.js Frontend
│   ├── src/
│   │   ├── app/      # App Router pages
│   │   ├── components/  # React components
│   │   ├── lib/      # Hooks, API client, utilities
│   │   └── store/    # Zustand state management
│   └── package.json
├── nginx/            # Reverse Proxy config
├── scripts/          # Scripts tiện ích
├── docs/             # Tài liệu dự án
├── docker-compose.yml
├── .env.example      # Mẫu cấu hình môi trường
└── README.md
```

*Copyright (c) 2026 Trần Nguyễn Chí Khanh. Licensed under GNU GPL v3.*