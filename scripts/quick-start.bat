@echo off
REM ============================================================
REM QuantFlow Quick Start Script (Windows)
REM ============================================================
REM Author: Khanh
REM Date: March 7, 2026
REM Description: Automated setup script for Windows users
REM
REM Usage: quick-start.bat
REM ============================================================

echo.
echo ==============================================
echo    QuantFlow Quick Start Setup Script
echo ==============================================
echo.

REM ===========================
REM Step 1: Check Prerequisites
REM ===========================
echo [Step 1/5] Checking prerequisites...
echo.

REM Check Docker
docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker not found. Please install Docker Desktop first.
    echo Visit: https://docs.docker.com/desktop/install/windows-install/
    pause
    exit /b 1
)
echo [OK] Docker found

REM Check Docker Compose
docker compose version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker Compose not found. Please update Docker Desktop.
    pause
    exit /b 1
)
echo [OK] Docker Compose found

REM Check if Docker is running
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker daemon is not running. Please start Docker Desktop.
    pause
    exit /b 1
)
echo [OK] Docker daemon is running
echo.

REM ===========================
REM Step 2: Setup Environment
REM ===========================
echo [Step 2/5] Setting up environment variables...
echo.

if exist .env (
    echo [WARNING] .env file already exists
    set /p OVERWRITE="Do you want to overwrite it? (y/N): "
    if /i "%OVERWRITE%"=="y" (
        copy /y .env.example .env >nul
        echo [OK] .env file created from template
    ) else (
        echo [SKIP] Using existing .env file
    )
) else (
    copy .env.example .env >nul
    echo [OK] .env file created from template
)

echo.
echo IMPORTANT: Please edit .env file and change:
echo   1. POSTGRES_PASSWORD
echo   2. JWT_SECRET (min 32 chars)
echo   3. AES_KEY (exactly 32 bytes)
echo.
echo Opening .env file in notepad...
timeout /t 2 >nul
notepad .env
echo.
pause

REM ===========================
REM Step 3: Generate SSL Certificate
REM ===========================
echo.
echo [Step 3/5] Generating SSL certificate...
echo.

if exist nginx\ssl\server.crt (
    echo [WARNING] SSL certificate already exists. Skipping...
) else (
    REM Check if OpenSSL is available
    where openssl >nul 2>&1
    if %errorlevel% equ 0 (
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout nginx\ssl\server.key -out nginx\ssl\server.crt -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"
        echo [OK] SSL certificate generated
    ) else (
        echo [WARNING] OpenSSL not found. Attempting to use Git Bash...
        if exist "C:\Program Files\Git\usr\bin\openssl.exe" (
            "C:\Program Files\Git\usr\bin\openssl.exe" req -x509 -nodes -days 365 -newkey rsa:2048 -keyout nginx\ssl\server.key -out nginx\ssl\server.crt -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"
            echo [OK] SSL certificate generated
        ) else (
            echo [ERROR] OpenSSL not available. Please install Git for Windows or OpenSSL.
            echo You can also run scripts/generate-ssl.sh in Git Bash
            pause
        )
    )
)

REM ===========================
REM Step 4: Build and Start Services
REM ===========================
echo.
echo [Step 4/5] Building and starting Docker containers...
echo This may take 5-10 minutes on first run...
echo.

docker compose up --build -d

if %errorlevel% neq 0 (
    echo [ERROR] Failed to start containers. Check logs with: docker compose logs
    pause
    exit /b 1
)

echo.
echo Waiting for services to be healthy...
timeout /t 20 >nul

REM ===========================
REM Step 5: Verify Setup
REM ===========================
echo.
echo [Step 5/5] Verifying setup...
echo.

echo Container Status:
docker compose ps
echo.

echo Testing backend health endpoint...
timeout /t 5 >nul
curl -s http://localhost/api/v1/health >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Backend API is responding
) else (
    echo [WARNING] Backend API not responding yet (may need more time)
)

echo.
echo Testing frontend...
curl -s http://localhost >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Frontend is responding
) else (
    echo [WARNING] Frontend not responding yet (may need more time)
)

REM ===========================
REM Summary
REM ===========================
echo.
echo ==============================================
echo    Setup Complete!
echo ==============================================
echo.
echo Access Points:
echo   Frontend:     http://localhost
echo   Backend API:  http://localhost/api/v1
echo   WebSocket:    ws://localhost/ws
echo   Database:     localhost:5432 (quantflow_db)
echo.
echo Default Login:
echo   Username: admin
echo   Password: Admin@2026
echo   WARNING: Change password immediately after first login!
echo.
echo Useful Commands:
echo   View logs:      docker compose logs -f
echo   Stop services:  docker compose stop
echo   Start services: docker compose start
echo   Restart all:    docker compose restart
echo   Clean up:       docker compose down
echo.
echo For more details, see: README.md
echo For validation: DOCKER_VALIDATION_CHECKLIST.md
echo.
echo Opening browser to http://localhost...
timeout /t 3 >nul
start http://localhost
echo.
pause
