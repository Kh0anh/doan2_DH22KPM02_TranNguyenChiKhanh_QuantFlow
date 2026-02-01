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

* **Backend:** Golang (Gin Framework, Gorilla WebSocket)
* **Frontend:** Next.js (TypeScript, Tailwind CSS, Shadcn/UI)
* **Công cụ cốt lõi:** Google Blockly

---
*Copyright © 2026 Khanh. Licensed under the GNU GPL v3.*