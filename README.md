# QuantFlow - Low-code Crypto Trading Platform

![Project Status](https://img.shields.io/badge/Status-Developing-orange)
![Field](https://img.shields.io/badge/Field-Software_Engineering-blue)
![University](https://img.shields.io/badge/University-Nam_Can_Tho_University-green)

## 📌 Giới thiệu đề tài
**QuantFlow** là đồ án chuyên ngành Kỹ thuật phần mềm, tập trung vào việc giải quyết rào cản kỹ thuật cho người dùng cá nhân trong thị trường tiền mã hóa. Hệ thống cung cấp một nền tảng lập trình trực quan giúp xây dựng và vận hành bot giao dịch tự động trên các sàn giao dịch tập trung (CEX) mà không yêu cầu kỹ năng lập trình chuyên sâu.

* **Tên đề tài:** Phát triển nền tảng Low-code hỗ trợ xây dựng và vận hành chiến lược giao dịch tiền mã hóa trên sàn giao dịch tập trung (CEX)
* **Sản phẩm:** QuantFlow
* **Sinh viên thực hiện:** Trần Nguyễn Chí Khanh
* **MSSV:** 220979
* **Đơn vị:** Trường Đại học Nam Cần Thơ
* **Thời gian thực hiện:** 12/01/2026 – 23/03/2026

---

## 🚀 Phạm vi và Chức năng hệ thống

### 1. Tổng quan sản phẩm
**QuantFlow** là một ứng dụng web cung cấp môi trường Low-code/No-code để người dùng tự cấu hình bot giao dịch tự động.

### 2. Các chức năng chính (In-scope)
* **Quản trị & Cấu hình:**
    * Hệ thống hỗ trợ mô hình đơn người dùng (Single User/Admin)
    * Quản lý thông tin cá nhân: Đăng nhập, đổi tên và mật khẩu
    * Quản lý an toàn API Key/Secret Key kết nối với sàn giao dịch
* **Xây dựng chiến lược (Strategy Builder):**
    * Giao diện kéo thả trực quan sử dụng thư viện **Google Blockly**
    * Cung cấp bộ khối logic: Điều kiện (If/Else), Vòng lặp, Toán học, Hàm chỉ báo và các khối giao dịch (Mua/Bán/Hủy lệnh)
    * Khả năng chuyển đổi tự động từ sơ đồ khối sang mã thực thi (Code Generation)
* **Vận hành & Giám sát:**
    * Khởi chạy và dừng bot linh hoạt theo yêu cầu
    * Hiển thị nhật ký (Logs) và trạng thái lệnh Real-time
* **Kết nối thị trường:**
    * Lấy dữ liệu giá thị trường (Market Data) từ sàn CEX
    * Gửi lệnh giao dịch (Order Placement) qua API của sàn

---

## 🛠 Công nghệ sử dụng
Hệ thống được phát triển dựa trên các công nghệ hiện đại nhằm đảm bảo tính ổn định và thời gian thực:

* **Backend:** Golang 1.24 (GORM + pgx, gorilla/websocket)
* **Frontend:** Next.js 16 (React 19, TypeScript, Tailwind CSS, shadcn/ui)
* **Database:** PostgreSQL 16
* **Reverse Proxy:** Nginx 1.26
* **Containerization:** Docker & Docker Compose
* **Block-based Editor:** Google Blockly

---

## 🐳 Cài đặt và Chạy Hệ thống (Docker)

### Yêu cầu hệ thống (Prerequisites)

Đảm bảo máy tính của bạn đã cài đặt các công cụ sau:

| Công cụ | Phiên bản tối thiểu | Kiểm tra phiên bản |
|---------|---------------------|-------------------|
| **Docker Engine** | 24.0+ | `docker --version` |
| **Docker Compose** | 2.20+ | `docker compose version` |
| **Git** | 2.30+ | `git --version` |

> **Chú ý:** Đối với Windows, khuyến nghị cài đặt **Docker Desktop**  
> Đối với Linux, cài đặt Docker Engine + Docker Compose plugin

### Bước 1: Clone Repository

```bash
git clone https://github.com/Kh0anh/doan2_DH22KPM02_TranNguyenChiKhanh_QuantFlow.git
cd doan2_DH22KPM02_TranNguyenChiKhanh_QuantFlow
```

### Bước 2: Cấu hình Environment Variables

```bash
# Sao chép file template
cp .env.example .env

# Chỉnh sửa file .env với editor yêu thích
nano .env   # hoặc: vi .env, code .env, notepad .env
```

**⚠️ QUAN TRỌNG:** Thay đổi các giá trị sau trong file `.env`:

```env
# Đổi mật khẩu database
POSTGRES_PASSWORD=your_strong_password_here

# Tạo JWT Secret (tối thiểu 32 ký tự)
JWT_SECRET=$(openssl rand -base64 32)

# Tạo AES Key (chính xác 32 bytes)
AES_KEY=$(openssl rand -hex 32)
```

### Bước 3: Tạo SSL Certificate (Development)

```bash
# Cấp quyền thực thi cho script
chmod +x scripts/generate-ssl.sh

# Chạy script tạo self-signed certificate
./scripts/generate-ssl.sh
```

> **Lưu ý:** Chỉ cần làm một lần. Certificate có hiệu lực 365 ngày.  
> Production: Sử dụng Let's Encrypt certificate thay vì self-signed.

### Bước 4: Khởi động Stack

#### Development Mode (với hot reload)

```bash
# Khởi động tất cả services
docker compose up

# Hoặc chạy ngầm (detached mode)
docker compose up -d

# Xem logs
docker compose logs -f

# Xem logs của service cụ thể
docker compose logs -f backend
docker compose logs -f frontend
```

#### Production Mode

```bash
# Build và chạy production stack
docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d

# Kiểm tra trạng thái
docker compose ps
```

### Bước 5: Truy cập Hệ thống

Sau khi khởi động thành công, truy cập các endpoint sau:

| Service | URL | Mô tả |
|---------|-----|-------|
| **Frontend** | http://localhost | Giao diện người dùng |
| **Backend API** | http://localhost/api/v1 | RESTful API |
| **WebSocket** | ws://localhost/ws | Real-time connection |
| **PostgreSQL** | localhost:5432 | Database (chỉ development) |

**Thông tin đăng nhập mặc định:**
- Username: `admin`
- Password: `Admin@2026`

> **⚠️ BẢO MẬT:** Đổi mật khẩu ngay sau lần đăng nhập đầu tiên!

### Các lệnh Docker Compose hữu ích

```bash
# Dừng tất cả services (giữ nguyên data)
docker compose stop

# Khởi động lại services đã dừng
docker compose start

# Dừng và xóa containers (giữ nguyên volumes)
docker compose down

# Dừng và xóa containers + volumes (XÓA TOÀN BỘ DỮ LIỆU!)
docker compose down -v

# Xem trạng thái services
docker compose ps

# Rebuild images (sau khi sửa code)
docker compose up --build

# Chạy lệnh trong container đang chạy
docker compose exec backend sh
docker compose exec postgres psql -U quantflow_user -d quantflow_db

# Xem resource usage
docker stats
```

### Troubleshooting (Xử lý lỗi thường gặp)

#### 1. Port already in use (Cổng đã bị chiếm)

```bash
# Kiểm tra port nào đang sử dụng
# Windows
netstat -ano | findstr :80
netstat -ano | findstr :5432

# Linux/Mac
lsof -i :80
lsof -i :5432

# Giải pháp: Thay đổi port trong docker-compose.yml
# Hoặc dừng service đang chiếm port
```

#### 2. Backend không kết nối được Database

```bash
# Kiểm tra PostgreSQL đã ready chưa
docker compose logs postgres

# Khởi động lại backend sau khi postgres ready
docker compose restart backend
```

#### 3. Permission denied khi mount volumes (Linux)

```bash
# Thêm user ID vào docker-compose.yml
user: "${UID}:${GID}"

# Hoặc chạy với sudo (không khuyến nghị)
sudo docker compose up
```

#### 4. Frontend không load được (404 Not Found)

```bash
# Clear Next.js cache và rebuild
docker compose exec frontend rm -rf .next
docker compose restart frontend
```

#### 5. SSL Certificate error

```bash
# Tạo lại certificate
rm -rf nginx/ssl/*
./scripts/generate-ssl.sh
docker compose restart nginx
```

### Database Management

#### Backup Database

```bash
# Tạo backup file
docker compose exec postgres pg_dump -U quantflow_user quantflow_db > backup.sql

# Hoặc với format custom (nén)
docker compose exec postgres pg_dump -U quantflow_user -Fc quantflow_db > backup.dump
```

#### Restore Database

```bash
# Restore từ SQL file
docker compose exec -T postgres psql -U quantflow_user quantflow_db < backup.sql

# Restore từ custom format
docker compose exec postgres pg_restore -U quantflow_user -d quantflow_db backup.dump
```

#### Kết nối trực tiếp database bằng client (DBeaver, pgAdmin)

```
Host: localhost
Port: 5432
Database: quantflow_db
Username: quantflow_user
Password: <giá trị trong .env>
```

---
*Copyright © 2026 Khanh. Licensed under the GNU GPL v3.*