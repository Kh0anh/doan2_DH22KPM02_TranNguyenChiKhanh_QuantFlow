@echo off
REM ============================================================
REM QuantFlow - Tao Chung chi SSL (Phat trien)
REM ============================================================
REM Tac gia: Khanh
REM Mo ta:   Tao chung chi SSL tu ky cho HTTPS cuc bo
REM
REM Su dung: scripts\generate-ssl.bat
REM
REM Luu y: Khi len production, su dung Let's Encrypt thay the
REM ============================================================

echo.
echo =========================================
echo Tao Chung chi SSL (Phat trien)
echo =========================================
echo.

REM Tao thu muc SSL neu chua co
if not exist nginx\ssl mkdir nginx\ssl

REM Kiem tra OpenSSL
where openssl >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Tim thay OpenSSL, dang tao chung chi...
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout nginx\ssl\server.key -out nginx\ssl\server.crt -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"
    goto :done
)

REM Thu dung OpenSSL tu Git for Windows
if exist "C:\Program Files\Git\usr\bin\openssl.exe" (
    echo [OK] Tim thay OpenSSL tu Git, dang tao chung chi...
    "C:\Program Files\Git\usr\bin\openssl.exe" req -x509 -nodes -days 365 -newkey rsa:2048 -keyout nginx\ssl\server.key -out nginx\ssl\server.crt -subj "/C=VN/ST=CanTho/L=CanTho/O=QuantFlow/OU=Development/CN=localhost"
    goto :done
)

echo [LOI] Khong tim thay OpenSSL.
echo Vui long cai dat Git for Windows hoac OpenSSL.
echo Tai Git: https://git-scm.com/download/win
pause
exit /b 1

:done
echo.
echo [OK] Da tao chung chi SSL thanh cong!
echo.
echo Chung chi: nginx\ssl\server.crt
echo Khoa:      nginx\ssl\server.key
echo Thoi han:  365 ngay
echo.
echo CANH BAO: Day la chung chi tu ky, chi dung cho phat trien.
echo Trinh duyet se hien canh bao bao mat.
echo Khi len production, dung Let's Encrypt.
echo.
pause
