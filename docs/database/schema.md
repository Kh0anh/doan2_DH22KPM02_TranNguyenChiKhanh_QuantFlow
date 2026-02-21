# Thiết kế Cơ sở dữ liệu

Tài liệu này đặc tả chi tiết các thực thể và cấu trúc bảng dữ liệu phục vụ cho nền tảng giao dịch tiền mã hóa Low-code QuantFlow.

---

## 1. Bảng `Users`
Lưu trữ thông tin tài khoản đăng nhập vào nền tảng.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK), định danh duy nhất cho người dùng. |
| 2 | `username` | VARCHAR(50) | Có | Tên đăng nhập (Ràng buộc Unique - UK). |
| 3 | `password_hash` | VARCHAR(255) | Có | Mật khẩu đã được băm (hashed) để bảo mật. |
| 4 | `created_at` | TIMESTAMPTZ | Không | Thời gian khởi tạo tài khoản. |
| 5 | `updated_at` | TIMESTAMPTZ | Không | Thời gian cập nhật thông tin tài khoản gần nhất. |

## 2. Bảng `API_Keys`
Lưu trữ thông tin cấu hình khóa API kết nối với sàn giao dịch.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK). |
| 2 | `user_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Users. |
| 3 | `exchange` | VARCHAR(50) | Không | Tên sàn giao dịch (Ví dụ: Binance). |
| 4 | `api_key` | VARCHAR(255) | Có | Khóa API công khai (Access Key). |
| 5 | `secret_key_encrypted`| VARCHAR(512) | Có | Khóa bí mật (Secret Key) đã được mã hóa AES-256. |
| 6 | `status` | VARCHAR(20) | Không | Trạng thái kết nối (Ví dụ: Connected, Disconnected). |
| 7 | `created_at` | TIMESTAMPTZ | Không | Thời gian tạo cấu hình API. |
| 8 | `updated_at` | TIMESTAMPTZ | Không | Thời gian cập nhật cấu hình gần nhất. |

## 3. Bảng `Strategies`
Lưu trữ các mẫu chiến lược giao dịch được thiết kế dưới dạng khối logic từ Google Blockly.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK). |
| 2 | `user_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Users. |
| 3 | `name` | VARCHAR(100) | Có | Tên của chiến lược giao dịch. |
| 4 | `logic_json` | JSONB | Có | Cấu trúc khối logic được lưu dưới định dạng JSONB. |
| 5 | `version` | INT | Không | Phiên bản của chiến lược phục vụ Audit Log. |
| 6 | `status` | VARCHAR(20) | Không | Trạng thái của chiến lược (Ví dụ: Valid, Draft). |
| 7 | `created_at` | TIMESTAMPTZ | Không | Thời gian tạo chiến lược. |
| 8 | `updated_at` | TIMESTAMPTZ | Không | Thời gian chỉnh sửa chiến lược gần nhất. |

## 4. Bảng `Bot_Instances`
Quản lý các tiến trình Bot đang được vận hành (chạy thực tế hoặc chạy ngầm).

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK). |
| 2 | `user_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Users. |
| 3 | `strategy_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Strategies. |
| 4 | `strategy_version` | INT | Có | Phiên bản của chiến lược tại thời điểm Bot khởi chạy. |
| 5 | `bot_name` | VARCHAR(100) | Có | Tên định danh của Bot. |
| 6 | `symbol` | VARCHAR(20) | Có | Cặp tiền mã hóa Bot đang giao dịch (Ví dụ: BTCUSDT). |
| 7 | `status` | VARCHAR(20) | Có | Trạng thái hiện tại của Bot (Running, Stopped, Error). |
| 8 | `total_pnl` | DECIMAL(18,8)| Không | Tổng lợi nhuận/thua lỗ (PnL) do Bot tạo ra. |
| 9 | `created_at` | TIMESTAMPTZ | Không | Thời gian khởi tạo Bot. |
| 10 | `updated_at` | TIMESTAMPTZ | Không | Thời gian cập nhật trạng thái/PnL gần nhất. |

## 5. Bảng `Bot_Lifecycle_Variables`
Lưu trữ các biến vòng đời phục vụ quá trình tính toán logic liên tục của Bot.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK). |
| 2 | `bot_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Bot_Instances. |
| 3 | `variable_name` | VARCHAR(100) | Có | Tên của biến vòng đời. |
| 4 | `variable_value` | JSONB | Có | Giá trị của biến (có thể là số, chuỗi, mảng...). |
| 5 | `updated_at` | TIMESTAMPTZ | Có | Thời gian biến được cập nhật giá trị gần nhất. |

## 6. Bảng `Bot_Logs`
Ghi nhận nhật ký hoạt động (log) chi tiết của các Bot, tối ưu cho tần suất ghi cao.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | BIGSERIAL | Có | Khóa chính tự tăng (PK), tối ưu cho tốc độ insert. |
| 2 | `bot_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Bot_Instances. |
| 3 | `action_decision`| VARCHAR(50) | Không | Quyết định hành động của Bot trong phiên (VD: Đặt lệnh). |
| 4 | `unit_used` | INT | Không | Số lượng Unit (tài nguyên) đã tiêu thụ cho phiên xử lý. |
| 5 | `message` | TEXT | Không | Thông báo chi tiết hoặc lỗi trong quá trình thực thi. |
| 6 | `created_at` | TIMESTAMPTZ | Có | Thời gian log được sinh ra. |

## 7. Bảng `Trade_History`
Lưu trữ lịch sử tất cả các lệnh giao dịch đã được thực thi trên sàn phục vụ đối soát.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | UUID | Có | Khóa chính (PK). |
| 2 | `user_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Users. |
| 3 | `bot_id` | UUID | Có | Khóa ngoại (FK) liên kết tới bảng Bot_Instances. |
| 4 | `symbol` | VARCHAR(20) | Có | Cặp tiền mã hóa giao dịch. |
| 5 | `side` | VARCHAR(10) | Có | Chiều giao dịch (Long/Short). |
| 6 | `quantity` | DECIMAL(18,8)| Có | Khối lượng/Kích thước của lệnh. |
| 7 | `fill_price` | DECIMAL(18,8)| Có | Mức giá khớp lệnh thực tế. |
| 8 | `fee` | DECIMAL(18,8)| Có | Chi phí giao dịch phát sinh. |
| 9 | `realized_pnl` | DECIMAL(18,8)| Có | Lợi nhuận/thua lỗ thực tế đã chốt của lệnh. |
| 10 | `status` | VARCHAR(20) | Có | Trạng thái cuối cùng của lệnh (Filled, Canceled). |
| 11 | `executed_at` | TIMESTAMPTZ | Có | Thời gian chính xác lệnh được thực thi. |

## 8. Bảng `Candles_Data`
Lưu trữ dữ liệu nến thị trường (OHLCV) phục vụ Backtest và vẽ biểu đồ.

| STT | Tên cột | Kiểu dữ liệu | Not Null | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `id` | BIGSERIAL | Có | Khóa chính tự tăng (PK). |
| 2 | `symbol` | VARCHAR(20) | Có | Cặp tiền mã hóa (Ràng buộc Unique - UK). |
| 3 | `open_time` | TIMESTAMPTZ | Có | Thời gian bắt đầu mở nến (Ràng buộc Unique - UK). |
| 4 | `open_price` | DECIMAL(18,8)| Có | Mức giá mở cửa của nến. |
| 5 | `high_price` | DECIMAL(18,8)| Có | Mức giá cao nhất trong phiên nến. |
| 6 | `low_price` | DECIMAL(18,8)| Có | Mức giá thấp nhất trong phiên nến. |
| 7 | `close_price` | DECIMAL(18,8)| Có | Mức giá đóng cửa của nến. |
| 8 | `volume` | DECIMAL(18,8)| Có | Khối lượng giao dịch trong phiên nến. |
| 9 | `is_closed` | BOOLEAN | Không | Trạng thái cờ đánh dấu nến đã đóng hoàn toàn hay chưa. |

## Danh mục Index Tối ưu Hiệu năng (Performance Optimization)
Để đáp ứng yêu cầu xử lý khối lượng dữ liệu lớn dưới 10 giây (NFR-PERF-02) và truy vấn thời gian thực < 0.5s, hệ thống áp dụng các chỉ mục sau:

1. **`idx_candles_symbol_time`**: Composite Index trên bảng `Candles_Data` `(symbol ASC, open_time DESC)`. Tối ưu hóa truy vấn nến theo cặp tiền và sắp xếp thời gian đảo ngược phục vụ thuật toán Backtest và lấy nến mới nhất.
2. **`idx_trade_history_lookup`**: Composite Index trên bảng `Trade_History` `(user_id, bot_id, symbol, executed_at DESC)`. Phục vụ chức năng lọc lịch sử giao dịch nhiều điều kiện kết hợp cuộn trang vô hạn (Infinite Scroll).
3. **`idx_bot_logs_created_at`**: Index trên bảng `Bot_Logs` `(bot_id, created_at DESC)`. Đẩy nhanh tốc độ tải Live Logs ra giao diện Console.
4. **`idx_bot_variables_lookup`**: Index trên `Bot_Lifecycle_Variables` `(bot_id, variable_name)`. Tối ưu tốc độ lấy giá trị biến vòng đời của mỗi Bot trong từng chu kỳ tính toán logic.