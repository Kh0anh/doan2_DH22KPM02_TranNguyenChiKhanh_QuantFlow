@echo off
REM ============================================================
REM QuantFlow - Script Khởi động Nhanh (Windows)
REM ============================================================
REM Tác giả: Khanh
REM Dự án:   QuantFlow
REM Mô tả:   Tự động cài đặt và khởi động dự án trên Windows
REM
REM Sử dụng: quick-start.bat
REM ============================================================

echo.
echo ==============================================
echo    QuantFlow - Khoi dong Nhanh
echo ==============================================
echo.

REM ===========================
REM Bước 1: Kiểm tra yêu cầu
REM ===========================
echo [Buoc 1/4] Kiem tra yeu cau he thong...
echo.

REM Kiểm tra Docker
docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [LOI] Khong tim thay Docker. Vui long cai dat Docker Desktop truoc.
    echo Tai ve: https://docs.docker.com/desktop/install/windows-install/
    pause
    exit /b 1
)
echo [OK] Da tim thay Docker

REM Kiểm tra Docker Compose
docker compose version >nul 2>&1
if %errorlevel% neq 0 (
    echo [LOI] Khong tim thay Docker Compose. Vui long cap nhat Docker Desktop.
    pause
    exit /b 1
)
echo [OK] Da tim thay Docker Compose

REM Kiểm tra Docker đang chạy
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo [LOI] Docker chua chay. Vui long khoi dong Docker Desktop.
    pause
    exit /b 1
)
echo [OK] Docker dang chay
echo.

REM ===========================
REM Bước 2: Cấu hình môi trường
REM ===========================
echo [Buoc 2/4] Cau hinh moi truong...
echo.

if exist .env (
    echo [THONG BAO] File .env da ton tai, su dung file hien co.
) else (
    copy .env.example .env >nul
    echo [OK] Da tao file .env tu .env.example
)
echo.

REM ===========================
REM Bước 3: Khởi động dịch vụ
REM ===========================
echo [Buoc 3/4] Dang khoi dong cac dich vu Docker...
echo Lan dau co the mat 5-10 phut...
echo.
echo Chung chi SSL se duoc tao tu dong neu chua co.
echo.

docker compose up --build -d

if %errorlevel% neq 0 (
    echo [LOI] Khong the khoi dong container. Kiem tra log: docker compose logs
    pause
    exit /b 1
)

echo.
echo Dang cho cac dich vu san sang...
timeout /t 20 >nul

REM ===========================
REM Bước 4: Xác nhận
REM ===========================
echo.
echo [Buoc 4/4] Kiem tra trang thai...
echo.

echo Trang thai Container:
docker compose ps
echo.

echo Kiem tra Backend API...
timeout /t 5 >nul
curl -s http://localhost/api/v1/health >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Backend API dang hoat dong
) else (
    echo [CHO] Backend API chua san sang (can them thoi gian)
)

echo.
echo Kiem tra Frontend...
curl -s http://localhost >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Frontend dang hoat dong
) else (
    echo [CHO] Frontend chua san sang (can them thoi gian)
)

REM ===========================
REM Hoàn tất
REM ===========================
echo.
echo ==============================================
echo    Hoan tat Cai dat!
echo ==============================================
echo.
echo Dia chi truy cap:
echo   Giao dien:    http://localhost
echo   Backend API:  http://localhost/api/v1
echo   WebSocket:    ws://localhost/ws
echo   Co so du lieu: localhost:5432
echo.
echo Tai khoan mac dinh:
echo   Ten dang nhap: admin
echo   Mat khau:      Admin@2026
echo   CANH BAO: Hay doi mat khau ngay sau khi dang nhap!
echo.
echo Cac lenh thuong dung:
echo   Xem log:        docker compose logs -f
echo   Dung dich vu:   docker compose stop
echo   Khoi dong lai:  docker compose restart
echo   Don dep:        docker compose down
echo.
echo Mo trinh duyet...
timeout /t 3 >nul
start http://localhost
echo.
pause
