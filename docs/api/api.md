# Tài liệu API - QuantFlow Platform

---

## 1. Tổng quan (Overview)

### 1.1. Base URL

| Môi trường   | URL                            |
| :----------- | :----------------------------- |
| Development  | `http://localhost:8080/api/v1` |
| Production   | `https://quantflow.io/api/v1`  |

### 1.2. Quy chuẩn Header

Mọi request gửi đến API Server đều phải tuân thủ các Header sau:

| Header         | Giá trị              | Mô tả                                               |
| :------------- | :------------------- | :--------------------------------------------------- |
| `Content-Type` | `application/json`   | Bắt buộc cho mọi request có Body (POST, PUT, PATCH). |
| `Accept`       | `application/json`   | Quy định kiểu dữ liệu phản hồi mong muốn.          |

### 1.3. Cơ chế Xác thực (Authentication)

Hệ thống sử dụng **JWT (JSON Web Token)** kết hợp với **HttpOnly Cookie** để bảo vệ phiên làm việc.

**Luồng hoạt động:**

1.  Người dùng gửi request `POST /api/v1/auth/login` với thông tin đăng nhập.
2.  Nếu xác thực thành công, Backend tạo một JWT Token và gắn vào **HttpOnly Cookie** trong Response Header (`Set-Cookie`).
3.  Trình duyệt tự động gửi kèm Cookie này trong mọi request tiếp theo.
4.  Backend xác thực Token từ Cookie ở mỗi request đến các endpoint yêu cầu đăng nhập.
5.  Token tự động hết hạn sau **24 giờ**. Khi hết hạn, người dùng bị chuyển hướng về trang đăng nhập.

**Đặc tính bảo mật:**

-   **HttpOnly**: JavaScript phía Client không thể đọc được Cookie (chống XSS).
-   **Secure**: Cookie chỉ được gửi qua kết nối HTTPS (môi trường Production).
-   **SameSite=Lax**: Giảm thiểu nguy cơ tấn công CSRF.

---

## 2. Quy chuẩn Mã lỗi (Standard Status Codes)

Tất cả các Response trả về đều tuân theo chuẩn HTTP Status Code. Dưới đây là bảng mã lỗi áp dụng xuyên suốt hệ thống:

| Status Code | Tên                    | Mô tả & Ngữ cảnh áp dụng                                                                                                                      |
| :---------- | :--------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------- |
| `200`       | OK                     | Request thành công. Dùng cho các thao tác GET, PUT, PATCH, DELETE thành công.                                                                  |
| `201`       | Created                | Tạo tài nguyên mới thành công. Dùng cho các thao tác POST tạo Strategy, Bot, Backtest.                                                        |
| `400`       | Bad Request            | Dữ liệu đầu vào không hợp lệ. Ví dụ: thiếu trường bắt buộc, sai định dạng JSON, mật khẩu xác nhận không khớp.                               |
| `401`       | Unauthorized           | Chưa xác thực hoặc phiên làm việc đã hết hạn. Token JWT không hợp lệ hoặc không tồn tại trong Cookie.                                        |
| `403`       | Forbidden              | Đã xác thực nhưng không có quyền truy cập tài nguyên. Ví dụ: tài khoản bị khóa do đăng nhập sai quá 5 lần.                                   |
| `404`       | Not Found              | Tài nguyên không tồn tại. Ví dụ: Strategy ID, Bot ID không tìm thấy trong hệ thống.                                                          |
| `409`       | Conflict               | Xung đột khi thực hiện thao tác. Ví dụ: xóa Strategy đang được sử dụng bởi Bot đang chạy.                                                     |
| `422`       | Unprocessable Entity   | Dữ liệu hợp lệ về cú pháp nhưng vi phạm quy tắc nghiệp vụ. Ví dụ: API Key bị sàn Binance từ chối do thiếu quyền.                           |
| `429`       | Too Many Requests      | Vượt quá giới hạn tần suất gọi API. Hệ thống áp dụng Rate Limiting để bảo vệ Backend và tuân thủ Weight của Binance.                         |
| `500`       | Internal Server Error  | Lỗi nội bộ hệ thống. Ví dụ: mất kết nối Database, lỗi mã hóa/giải mã Secret Key, hoặc lỗi không lường trước từ Backend.                     |

**Cấu trúc Response lỗi chuẩn:**

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Tên đăng nhập hoặc mật khẩu không chính xác."
  }
}
```

---

## 3. Đặc tả RESTful API

---

### 3.1. Authentication

Nhóm API quản lý xác thực phiên làm việc của người dùng. Hệ thống áp dụng mô hình đơn người dùng (Single User/Admin).

---

#### 3.1.1. `POST /api/v1/auth/login`

**Mô tả nghiệp vụ:** Xác thực thông tin đăng nhập. Hệ thống so sánh mật khẩu đã băm (BCrypt/Argon2) trong cơ sở dữ liệu. Nếu nhập sai **quá 5 lần liên tiếp**, tài khoản bị **khóa 15 phút** (Brute-force Protection). Đăng nhập thành công sẽ tạo JWT Token và gắn vào HttpOnly Cookie.

**Yêu cầu Auth:** Không

**Request:**

| Tham số     | Vị trí | Kiểu   | Bắt buộc | Mô tả                |
| :---------- | :----- | :----- | :------- | :-------------------- |
| `username`  | Body   | string | Có       | Tên đăng nhập.       |
| `password`  | Body   | string | Có       | Mật khẩu người dùng. |

```json
{
  "username": "admin",
  "password": "MySecureP@ssw0rd"
}
```

**Response thành công (200 OK):**

> Header: `Set-Cookie: token=<JWT_TOKEN>; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=86400`

```json
{
  "message": "Đăng nhập thành công.",
  "data": {
    "user": {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "username": "admin"
    }
  }
}
```

**Response lỗi (401 Unauthorized) - Sai thông tin:**

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Tên đăng nhập hoặc mật khẩu không chính xác.",
    "remaining_attempts": 3
  }
}
```

**Response lỗi (403 Forbidden) - Tài khoản bị khóa:**

```json
{
  "error": {
    "code": "ACCOUNT_LOCKED",
    "message": "Bạn đã nhập sai quá nhiều lần. Vui lòng thử lại sau 15 phút.",
    "locked_until": "2026-02-28T15:30:00Z"
  }
}
```

---

#### 3.1.2. `POST /api/v1/auth/logout`

**Mô tả nghiệp vụ:** Đăng xuất khỏi hệ thống. Backend xóa Cookie phiên làm việc, vô hiệu hóa JWT Token hiện tại.

**Yêu cầu Auth:** Có

**Request:** Không có Body.

**Response thành công (200 OK):**

> Header: `Set-Cookie: token=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0`

```json
{
  "message": "Đăng xuất thành công."
}
```

---

#### 3.1.3. `GET /api/v1/auth/me`

**Mô tả nghiệp vụ:** Lấy thông tin phiên đăng nhập hiện tại. Frontend gọi endpoint này khi khởi tạo ứng dụng để kiểm tra trạng thái đăng nhập.

**Yêu cầu Auth:** Có

**Request:** Không có Body hoặc Query Parameters.

**Response thành công (200 OK):**

```json
{
  "data": {
    "user": {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "username": "admin",
      "created_at": "2026-01-15T08:00:00Z"
    }
  }
}
```

**Response lỗi (401 Unauthorized):**

```json
{
  "error": {
    "code": "SESSION_EXPIRED",
    "message": "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại."
  }
}
```

---

### 3.2. Account

Nhóm API quản lý thông tin tài khoản người dùng. Mọi thay đổi thông tin nhạy cảm (Username, Password) đều bắt buộc phải truyền mật khẩu hiện tại để xác thực quyền sở hữu, chống chiếm đoạt phiên (Session Hijacking). Sau khi cập nhật thành công, hệ thống **cưỡng chế đăng xuất** để Token cũ bị vô hiệu hóa.

---

#### 3.2.1. `PUT /api/v1/account/profile`

**Mô tả nghiệp vụ:** Cập nhật thông tin tài khoản (Tên đăng nhập và/hoặc Mật khẩu). Người dùng bắt buộc phải cung cấp `current_password` để xác thực. Sau khi cập nhật thành công, phiên làm việc hiện tại bị hủy (Force Logout), người dùng phải đăng nhập lại với thông tin mới.

**Yêu cầu Auth:** Có

**Request:**

| Tham số             | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                    |
| :------------------ | :----- | :----- | :------- | :------------------------------------------------------------------------ |
| `current_password`  | Body   | string | Có       | Mật khẩu hiện tại (bắt buộc để xác thực quyền thay đổi).                |
| `new_username`      | Body   | string | Không    | Tên đăng nhập mới. Bỏ trống nếu chỉ đổi mật khẩu.                       |
| `new_password`      | Body   | string | Không    | Mật khẩu mới. Bỏ trống nếu chỉ đổi tên đăng nhập.                       |
| `confirm_password`  | Body   | string | Không    | Nhập lại mật khẩu mới (bắt buộc nếu có `new_password`).                 |

> Lưu ý: Ít nhất một trong hai trường `new_username` hoặc `new_password` phải được cung cấp.

**Payload mẫu - Đổi cả Username và Password:**

```json
{
  "current_password": "MySecureP@ssw0rd",
  "new_username": "khanh_trader",
  "new_password": "N3wStr0ngP@ss!",
  "confirm_password": "N3wStr0ngP@ss!"
}
```

**Payload mẫu - Chỉ đổi Password:**

```json
{
  "current_password": "MySecureP@ssw0rd",
  "new_password": "N3wStr0ngP@ss!",
  "confirm_password": "N3wStr0ngP@ss!"
}
```

**Response thành công (200 OK):**

> Header: `Set-Cookie: token=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0`

```json
{
  "message": "Cập nhật thành công. Vui lòng đăng nhập lại."
}
```

**Response lỗi (400 Bad Request) - Mật khẩu xác nhận không khớp:**

```json
{
  "error": {
    "code": "PASSWORD_MISMATCH",
    "message": "Mật khẩu xác nhận không trùng khớp."
  }
}
```

**Response lỗi (401 Unauthorized) - Sai mật khẩu hiện tại:**

```json
{
  "error": {
    "code": "INVALID_CURRENT_PASSWORD",
    "message": "Mật khẩu hiện tại không chính xác. Vui lòng thử lại."
  }
}
```

---

### 3.3. Exchange

Nhóm API quản lý cấu hình kết nối sàn giao dịch (Binance). Đây là nhóm API bảo mật cốt lõi với quy tắc nghiêm ngặt:

-   **Secret Key là Write-Only**: Hệ thống chỉ cho phép ghi (nhập) Secret Key, tuyệt đối **không bao giờ trả về plain-text** Secret Key trong bất kỳ Response nào.
-   Endpoint `GET` chỉ trả về **trạng thái kết nối** và **API Key đã che (Masking `******`)**.
-   Secret Key được mã hóa bằng **AES-256-GCM** trước khi lưu vào Database.

---

#### 3.3.1. `POST /api/v1/exchange/api-keys`

**Mô tả nghiệp vụ:** Lưu hoặc cập nhật cặp khóa API kết nối với sàn Binance. Trước khi lưu, Backend gửi request thử nghiệm (Ping/Account Info) đến Binance API để xác thực tính đúng đắn và quyền hạn của Key. Secret Key được mã hóa AES-256-GCM trước khi ghi vào Database.

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                      |
| :----------- | :----- | :----- | :------- | :---------------------------------------------------------- |
| `exchange`   | Body   | string | Không    | Tên sàn giao dịch. Mặc định: `"Binance"`.                  |
| `api_key`    | Body   | string | Có       | Khóa API công khai (Access Key) từ sàn Binance.            |
| `secret_key` | Body   | string | Có       | Khóa bí mật (Secret Key) từ sàn Binance.                   |

```json
{
  "exchange": "Binance",
  "api_key": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",
  "secret_key": "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j"
}
```

**Response thành công (201 Created):**

```json
{
  "message": "Kết nối sàn thành công.",
  "data": {
    "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
    "exchange": "Binance",
    "api_key_masked": "****************************Eh8A",
    "status": "Connected",
    "created_at": "2026-02-28T10:00:00Z"
  }
}
```

**Response lỗi (422 Unprocessable Entity) - Sàn từ chối:**

```json
{
  "error": {
    "code": "EXCHANGE_VALIDATION_FAILED",
    "message": "API Key không hợp lệ hoặc thiếu quyền hạn Futures Trading."
  }
}
```

**Response lỗi (400 Bad Request) - Định dạng sai:**

```json
{
  "error": {
    "code": "INVALID_KEY_FORMAT",
    "message": "Vui lòng kiểm tra lại định dạng API Key."
  }
}
```

---

#### 3.3.2. `GET /api/v1/exchange/api-keys`

**Mô tả nghiệp vụ:** Lấy thông tin cấu hình API hiện tại. **Secret Key tuyệt đối không được trả về.** Chỉ trả về trạng thái kết nối và API Key đã che (Masking hiển thị 4 ký tự cuối, phần còn lại thay bằng dấu `*`).

**Yêu cầu Auth:** Có

**Request:** Không có Body hoặc Query Parameters.

**Response thành công (200 OK) - Đã cấu hình:**

```json
{
  "data": {
    "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
    "exchange": "Binance",
    "api_key_masked": "****************************Eh8A",
    "status": "Connected",
    "updated_at": "2026-02-28T10:00:00Z"
  }
}
```

**Response thành công (200 OK) - Chưa cấu hình:**

```json
{
  "data": null
}
```

---

#### 3.3.3. `DELETE /api/v1/exchange/api-keys`

**Mô tả nghiệp vụ:** Xóa cấu hình khóa API hiện tại. Hệ thống xóa hoàn toàn bản ghi API Key (bao gồm cả Secret Key đã mã hóa) khỏi Database. Sau khi xóa, trạng thái kết nối sàn chuyển về "Chưa kết nối".

**Yêu cầu Auth:** Có

**Request:** Không có Body.

**Response thành công (200 OK):**

```json
{
  "message": "Đã xóa cấu hình kết nối sàn."
}
```

**Response lỗi (409 Conflict) - Có Bot đang chạy:**

```json
{
  "error": {
    "code": "ACTIVE_BOTS_EXIST",
    "message": "Không thể xóa cấu hình khi còn Bot đang chạy. Vui lòng dừng tất cả Bot trước."
  }
}
```

---

### 3.4. Strategies

Nhóm API quản lý mẫu chiến lược giao dịch. Chiến lược được thiết kế dưới dạng khối logic (Google Blockly) và lưu trữ ở định dạng **JSON (JSONB)**. Hệ thống hỗ trợ đầy đủ CRUD, Import/Export JSON, quản lý phiên bản (Versioning) và kiểm tra ràng buộc toàn vẹn dữ liệu trước khi xóa.

---

#### 3.4.1. `GET /api/v1/strategies`

**Mô tả nghiệp vụ:** Lấy danh sách tất cả chiến lược giao dịch của người dùng. Hỗ trợ phân trang và tìm kiếm theo tên để đảm bảo tốc độ tải trang nhanh khi có nhiều chiến lược.

**Yêu cầu Auth:** Có

**Request:**

| Tham số   | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                    |
| :-------- | :----- | :----- | :------- | :-------------------------------------------------------- |
| `page`    | Query  | int    | Không    | Số trang hiện tại. Mặc định: `1`.                         |
| `limit`   | Query  | int    | Không    | Số bản ghi mỗi trang. Mặc định: `20`. Tối đa: `100`.     |
| `search`  | Query  | string | Không    | Lọc chiến lược theo tên (tìm kiếm gần đúng).             |

**Response thành công (200 OK):**

```json
{
  "data": [
    {
      "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "name": "EMA Crossover Strategy",
      "version": 3,
      "status": "Valid",
      "created_at": "2026-02-01T08:00:00Z",
      "updated_at": "2026-02-25T14:30:00Z"
    },
    {
      "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "name": "RSI Reversal",
      "version": 1,
      "status": "Draft",
      "created_at": "2026-02-20T10:00:00Z",
      "updated_at": "2026-02-20T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "total_pages": 1
  }
}
```

---

#### 3.4.2. `GET /api/v1/strategies/:id`

**Mô tả nghiệp vụ:** Lấy chi tiết một chiến lược, bao gồm toàn bộ cấu trúc khối logic JSON (Blockly). Nếu chiến lược đang được sử dụng bởi Bot đang chạy, Response sẽ kèm cảnh báo để Frontend hiển thị Banner thông báo cho người dùng.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu   | Bắt buộc | Mô tả                      |
| :------ | :----- | :----- | :------- | :-------------------------- |
| `id`    | Path   | UUID   | Có       | ID của chiến lược cần xem. |

**Response thành công (200 OK):**

```json
{
  "data": {
    "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
    "name": "EMA Crossover Strategy",
    "version": 3,
    "status": "Valid",
    "logic_json": {
      "blocks": {
        "languageVersion": 0,
        "blocks": [
          {
            "type": "event_on_candle_close",
            "fields": { "TIMEFRAME": "15m" },
            "next": {
              "block": {
                "type": "indicator_ema",
                "fields": { "PERIOD": 9, "SOURCE": "close" },
                "next": {
                  "block": {
                    "type": "logic_compare",
                    "fields": { "OP": "GT" },
                    "next": {
                      "block": {
                        "type": "trade_futures_order",
                        "fields": {
                          "SIDE": "Long",
                          "ORDER_TYPE": "Market",
                          "QUANTITY": 0.01,
                          "LEVERAGE": 10
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        ]
      }
    },
    "warning": "Chiến lược này đang được sử dụng bởi Bot đang chạy. Các thay đổi của bạn sẽ chỉ áp dụng cho các phiên chạy mới.",
    "active_bot_ids": ["e5f6a7b8-c9d0-1234-efab-345678901234"],
    "created_at": "2026-02-01T08:00:00Z",
    "updated_at": "2026-02-25T14:30:00Z"
  }
}
```

> Trường `warning` và `active_bot_ids` chỉ xuất hiện khi chiến lược đang được sử dụng bởi ít nhất một Bot có trạng thái `Running`.

**Response lỗi (404 Not Found):**

```json
{
  "error": {
    "code": "STRATEGY_NOT_FOUND",
    "message": "Chiến lược không tồn tại."
  }
}
```

---

#### 3.4.3. `POST /api/v1/strategies`

**Mô tả nghiệp vụ:** Tạo mới một chiến lược giao dịch. Backend thực hiện Validation: kiểm tra sự tồn tại của khối Sự kiện (Event Trigger) trong `logic_json` và kiểm tra tính liên kết của các khối lệnh. Phiên bản khởi tạo mặc định là `1`.

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                         |
| :----------- | :----- | :----- | :------- | :------------------------------------------------------------- |
| `name`       | Body   | string | Có       | Tên chiến lược. Tối đa 100 ký tự.                              |
| `logic_json` | Body   | object | Có       | Cấu trúc khối logic Blockly dưới định dạng JSON.               |
| `status`     | Body   | string | Không    | Trạng thái chiến lược: `Valid` hoặc `Draft`. Mặc định: `Valid`. |

```json
{
  "name": "EMA Crossover Strategy",
  "logic_json": {
    "blocks": {
      "languageVersion": 0,
      "blocks": [
        {
          "type": "event_on_candle_close",
          "fields": { "TIMEFRAME": "15m" },
          "next": {
            "block": {
              "type": "indicator_ema",
              "fields": { "PERIOD": 9, "SOURCE": "close" }
            }
          }
        }
      ]
    }
  },
  "status": "Valid"
}
```

**Response thành công (201 Created):**

```json
{
  "message": "Lưu chiến lược thành công.",
  "data": {
    "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
    "name": "EMA Crossover Strategy",
    "version": 1,
    "status": "Valid",
    "created_at": "2026-02-28T10:00:00Z"
  }
}
```

**Response lỗi (400 Bad Request) - Thiếu khối Sự kiện:**

```json
{
  "error": {
    "code": "MISSING_EVENT_TRIGGER",
    "message": "Chiến lược phải bắt đầu bằng khối Sự kiện (Event Trigger)."
  }
}
```

---

#### 3.4.4. `PUT /api/v1/strategies/:id`

**Mô tả nghiệp vụ:** Cập nhật chiến lược hiện có. Mỗi lần cập nhật thành công, `version` tự động tăng lên 1 đơn vị phục vụ Audit Log. Nếu chiến lược đang được sử dụng bởi Bot đang chạy, hệ thống vẫn cho phép lưu nhưng kèm cảnh báo: thay đổi chỉ áp dụng cho các phiên chạy mới (Bot đang chạy sử dụng Snapshot version cũ).

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                          |
| :----------- | :----- | :----- | :------- | :---------------------------------------------- |
| `id`         | Path   | UUID   | Có       | ID của chiến lược cần cập nhật.                 |
| `name`       | Body   | string | Không    | Tên chiến lược mới.                              |
| `logic_json` | Body   | object | Không    | Cấu trúc khối logic Blockly mới.                |
| `status`     | Body   | string | Không    | Trạng thái mới (`Valid` / `Draft`).              |

```json
{
  "name": "EMA Crossover v2",
  "logic_json": {
    "blocks": {
      "languageVersion": 0,
      "blocks": [
        {
          "type": "event_on_candle_close",
          "fields": { "TIMEFRAME": "1h" },
          "next": {
            "block": {
              "type": "indicator_ema",
              "fields": { "PERIOD": 21, "SOURCE": "close" }
            }
          }
        }
      ]
    }
  }
}
```

**Response thành công (200 OK):**

```json
{
  "message": "Cập nhật chiến lược thành công.",
  "data": {
    "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
    "name": "EMA Crossover v2",
    "version": 4,
    "status": "Valid",
    "warning": "Chiến lược này đang được sử dụng bởi Bot đang chạy. Các thay đổi của bạn sẽ chỉ áp dụng cho các phiên chạy mới.",
    "updated_at": "2026-02-28T11:00:00Z"
  }
}
```

---

#### 3.4.5. `DELETE /api/v1/strategies/:id`

**Mô tả nghiệp vụ:** Xóa một chiến lược khỏi hệ thống. **Hệ thống bắt buộc phải kiểm tra**: nếu chiến lược đang được sử dụng bởi Bot có trạng thái `Running`, thao tác xóa bị **chặn hoàn toàn** và trả về lỗi `409 Conflict`.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả                      |
| :------ | :----- | :--- | :------- | :-------------------------- |
| `id`    | Path   | UUID | Có       | ID của chiến lược cần xóa. |

**Response thành công (200 OK):**

```json
{
  "message": "Đã xóa chiến lược thành công."
}
```

**Response lỗi (409 Conflict) - Chiến lược đang được sử dụng:**

```json
{
  "error": {
    "code": "STRATEGY_IN_USE",
    "message": "Chiến lược đang được sử dụng bởi Bot [bot_name]. Vui lòng dừng Bot trước khi xóa.",
    "active_bot_ids": ["e5f6a7b8-c9d0-1234-efab-345678901234"]
  }
}
```

---

#### 3.4.6. `POST /api/v1/strategies/import`

**Mô tả nghiệp vụ:** Nhập chiến lược từ file JSON. Hệ thống kiểm tra cấu trúc file (Validate JSON Schema), nếu hợp lệ thì tạo một bản ghi chiến lược mới trong Database với nội dung từ file.

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                           |
| :----------- | :----- | :----- | :------- | :----------------------------------------------- |
| `name`       | Body   | string | Có       | Tên chiến lược mới khi nhập.                      |
| `logic_json` | Body   | object | Có       | Nội dung JSON logic đọc từ file tải lên.          |

```json
{
  "name": "Imported - EMA Strategy",
  "logic_json": {
    "blocks": {
      "languageVersion": 0,
      "blocks": [
        {
          "type": "event_on_candle_close",
          "fields": { "TIMEFRAME": "15m" }
        }
      ]
    }
  }
}
```

**Response thành công (201 Created):**

```json
{
  "message": "Nhập chiến lược thành công.",
  "data": {
    "id": "f6a7b8c9-d0e1-2345-fgab-456789012345",
    "name": "Imported - EMA Strategy",
    "version": 1,
    "status": "Valid",
    "created_at": "2026-02-28T12:00:00Z"
  }
}
```

**Response lỗi (400 Bad Request) - JSON không hợp lệ:**

```json
{
  "error": {
    "code": "INVALID_JSON_STRUCTURE",
    "message": "Cấu trúc file JSON không hợp lệ. Vui lòng kiểm tra lại định dạng."
  }
}
```

---

#### 3.4.7. `GET /api/v1/strategies/:id/export`

**Mô tả nghiệp vụ:** Xuất chiến lược ra file JSON. Backend trả về nội dung JSON đầy đủ của chiến lược để Frontend tạo file tải xuống cho người dùng (trigger download `.json`).

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả                       |
| :------ | :----- | :--- | :------- | :--------------------------- |
| `id`    | Path   | UUID | Có       | ID của chiến lược cần xuất. |

**Response thành công (200 OK):**

> Header: `Content-Disposition: attachment; filename="ema-crossover-strategy.json"`

```json
{
  "name": "EMA Crossover Strategy",
  "logic_json": {
    "blocks": {
      "languageVersion": 0,
      "blocks": [
        {
          "type": "event_on_candle_close",
          "fields": { "TIMEFRAME": "15m" },
          "next": {
            "block": {
              "type": "indicator_ema",
              "fields": { "PERIOD": 9, "SOURCE": "close" }
            }
          }
        }
      ]
    }
  },
  "version": 3,
  "exported_at": "2026-02-28T12:30:00Z"
}
```

---

### 3.5. Backtests

Nhóm API thực thi mô phỏng chiến lược giao dịch trên dữ liệu lịch sử. Body request bắt buộc phải bao gồm đầy đủ các tham số cấu hình giả lập: **Strategy ID, Symbol, Timeframe, Time Range, Vốn ban đầu (Initial Capital), Phí giao dịch (Fee)** và **Unit**.

---

#### 3.5.1. `POST /api/v1/backtests`

**Mô tả nghiệp vụ:** Khởi tạo phiên Backtest mới. Backend tải dữ liệu nến lịch sử từ Database (hoặc tự động lấp đầy từ Binance API nếu thiếu), chạy logic chiến lược, giả lập khớp lệnh và tổng hợp kết quả. Lệnh Market giả định khớp tại giá Open nến tiếp theo; lệnh Limit khớp dựa trên giá High/Low trong khung thời gian đã chọn.

**Yêu cầu Auth:** Có

**Request:**

| Tham số           | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                         |
| :---------------- | :----- | :----- | :------- | :----------------------------------------------------------------------------- |
| `strategy_id`     | Body   | UUID   | Có       | ID chiến lược dùng để chạy Backtest.                                           |
| `symbol`          | Body   | string | Có       | Cặp tiền mã hóa (VD: `BTCUSDT`).                                              |
| `timeframe`       | Body   | string | Có       | Khung thời gian nến. Giá trị: `1m`, `5m`, `15m`, `1h`, `4h`, `1D`.            |
| `start_time`      | Body   | string | Có       | Thời gian bắt đầu (ISO 8601). VD: `2025-01-01T00:00:00Z`.                     |
| `end_time`        | Body   | string | Có       | Thời gian kết thúc (ISO 8601). VD: `2025-12-31T23:59:59Z`.                    |
| `initial_capital` | Body   | number | Có       | Vốn giả lập ban đầu (đơn vị: USDT). VD: `1000`.                               |
| `fee_rate`        | Body   | number | Có       | Phí giao dịch giả lập mỗi lệnh (%). VD: `0.04` tương ứng 0.04%.              |
| `max_unit`        | Body   | number | Không    | Giới hạn Unit cho mỗi phiên chạy. Mặc định: `1000`.                            |

```json
{
  "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "symbol": "BTCUSDT",
  "timeframe": "15m",
  "start_time": "2025-01-01T00:00:00Z",
  "end_time": "2025-12-31T23:59:59Z",
  "initial_capital": 1000,
  "fee_rate": 0.04,
  "max_unit": 1000
}
```

**Response thành công (201 Created):**

```json
{
  "message": "Phiên Backtest đã được khởi tạo.",
  "data": {
    "backtest_id": "a7b8c9d0-e1f2-3456-abcd-567890123456",
    "status": "processing",
    "created_at": "2026-02-28T10:00:00Z"
  }
}
```

**Response lỗi (400 Bad Request) - Thiếu tham số:**

```json
{
  "error": {
    "code": "MISSING_REQUIRED_FIELDS",
    "message": "Vui lòng cung cấp đầy đủ các tham số: strategy_id, symbol, timeframe, start_time, end_time, initial_capital, fee_rate."
  }
}
```

**Response lỗi (404 Not Found) - Chiến lược không tồn tại:**

```json
{
  "error": {
    "code": "STRATEGY_NOT_FOUND",
    "message": "Chiến lược không tồn tại."
  }
}
```

---

#### 3.5.2. `GET /api/v1/backtests/:id`

**Mô tả nghiệp vụ:** Lấy kết quả chi tiết của phiên Backtest. Bao gồm thống kê tổng quan (Total PnL, Win Rate, Max Drawdown, Profit Factor) và dữ liệu Equity Curve phục vụ vẽ biểu đồ Mini-Chart trên Frontend.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả                             |
| :------ | :----- | :--- | :------- | :--------------------------------- |
| `id`    | Path   | UUID | Có       | ID của phiên Backtest cần xem.    |

**Response thành công (200 OK) - Đã hoàn tất:**

```json
{
  "data": {
    "backtest_id": "a7b8c9d0-e1f2-3456-abcd-567890123456",
    "status": "completed",
    "config": {
      "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "strategy_name": "EMA Crossover Strategy",
      "symbol": "BTCUSDT",
      "timeframe": "15m",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-12-31T23:59:59Z",
      "initial_capital": 1000,
      "fee_rate": 0.04
    },
    "summary": {
      "total_pnl": 245.67,
      "total_pnl_percent": 24.57,
      "win_rate": 62.5,
      "total_trades": 48,
      "winning_trades": 30,
      "losing_trades": 18,
      "max_drawdown": -8.45,
      "max_drawdown_percent": -6.92,
      "profit_factor": 1.85
    },
    "equity_curve": [
      { "timestamp": "2025-01-02T00:00:00Z", "equity": 1000.00 },
      { "timestamp": "2025-01-15T12:00:00Z", "equity": 1025.30 },
      { "timestamp": "2025-03-01T00:00:00Z", "equity": 980.50 },
      { "timestamp": "2025-06-15T00:00:00Z", "equity": 1120.80 },
      { "timestamp": "2025-12-31T23:45:00Z", "equity": 1245.67 }
    ],
    "trades": [
      {
        "open_time": "2025-01-05T14:15:00Z",
        "close_time": "2025-01-05T16:30:00Z",
        "side": "Long",
        "entry_price": 42150.00,
        "exit_price": 42380.50,
        "quantity": 0.01,
        "fee": 0.34,
        "pnl": 1.97
      }
    ],
    "created_at": "2026-02-28T10:00:00Z",
    "completed_at": "2026-02-28T10:00:08Z"
  }
}
```

**Response thành công (200 OK) - Đang xử lý:**

```json
{
  "data": {
    "backtest_id": "a7b8c9d0-e1f2-3456-abcd-567890123456",
    "status": "processing",
    "progress": 65
  }
}
```

---

#### 3.5.3. `POST /api/v1/backtests/:id/cancel`

**Mô tả nghiệp vụ:** Hủy bỏ phiên Backtest đang chạy giữa chừng. Hệ thống dừng tiến trình xử lý và giải phóng tài nguyên.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả                              |
| :------ | :----- | :--- | :------- | :---------------------------------- |
| `id`    | Path   | UUID | Có       | ID của phiên Backtest cần hủy.     |

**Response thành công (200 OK):**

```json
{
  "message": "Đã hủy phiên Backtest."
}
```

---

### 3.6. Bots

Nhóm API quản lý phiên bản thực thi Bot (Bot Instance). Bot được khởi tạo từ một mẫu chiến lược có sẵn, khi tạo sẽ thực hiện **Snapshot (Deep Copy)** bản logic tại thời điểm đó. Sửa chiến lược gốc không ảnh hưởng Bot đang chạy. Backend tự động đọc tham số Leverage và Margin Type từ khối Smart Order trong JSON chiến lược và gửi API lên Binance để cấu hình trước khi đặt lệnh.

---

#### 3.6.1. `GET /api/v1/bots`

**Mô tả nghiệp vụ:** Lấy danh sách tất cả Bot của người dùng. Mỗi Bot trả về thông tin tóm tắt gồm: tên Bot, cặp tiền, trạng thái hoạt động, tổng PnL. Hỗ trợ lọc theo trạng thái.

**Yêu cầu Auth:** Có

**Request:**

| Tham số  | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                 |
| :------- | :----- | :----- | :------- | :--------------------------------------------------------------------- |
| `status` | Query  | string | Không    | Lọc theo trạng thái Bot: `Running`, `Stopped`, `Error`. Bỏ trống = tất cả. |

**Response thành công (200 OK):**

```json
{
  "data": [
    {
      "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
      "bot_name": "BTC Scalper",
      "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "strategy_name": "EMA Crossover Strategy",
      "strategy_version": 3,
      "symbol": "BTCUSDT",
      "status": "Running",
      "total_pnl": 125.45,
      "created_at": "2026-02-20T08:00:00Z",
      "updated_at": "2026-02-28T10:30:00Z"
    },
    {
      "id": "f6a7b8c9-d0e1-2345-fgab-456789012345",
      "bot_name": "ETH Swing",
      "strategy_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "strategy_name": "RSI Reversal",
      "strategy_version": 1,
      "symbol": "ETHUSDT",
      "status": "Stopped",
      "total_pnl": -12.30,
      "created_at": "2026-02-22T14:00:00Z",
      "updated_at": "2026-02-25T18:00:00Z"
    }
  ]
}
```

---

#### 3.6.2. `GET /api/v1/bots/:id`

**Mô tả nghiệp vụ:** Lấy chi tiết một Bot, bao gồm thông tin vị thế hiện tại (Position), danh sách lệnh chờ (Open Orders), và trạng thái hoạt động. Dữ liệu PnL chưa chốt (Unrealized PnL) và trạng thái lệnh được đồng bộ thời gian thực từ sàn.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả              |
| :------ | :----- | :--- | :------- | :------------------ |
| `id`    | Path   | UUID | Có       | ID của Bot cần xem. |

**Response thành công (200 OK):**

```json
{
  "data": {
    "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
    "strategy_name": "EMA Crossover Strategy",
    "strategy_version": 3,
    "symbol": "BTCUSDT",
    "status": "Running",
    "total_pnl": 125.45,
    "position": {
      "side": "Long",
      "entry_price": 64500.00,
      "quantity": 0.05,
      "leverage": 10,
      "unrealized_pnl": 32.50,
      "margin_type": "Isolated"
    },
    "open_orders": [
      {
        "order_id": "ORD-001",
        "side": "Sell",
        "type": "Limit",
        "price": 65200.00,
        "quantity": 0.05,
        "status": "Pending"
      }
    ],
    "created_at": "2026-02-20T08:00:00Z",
    "updated_at": "2026-02-28T10:30:00Z"
  }
}
```

---

#### 3.6.3. `POST /api/v1/bots`

**Mô tả nghiệp vụ:** Khởi tạo một Bot mới từ mẫu chiến lược có sẵn. Hệ thống thực hiện Snapshot (Deep Copy) logic chiến lược và ghi lại `strategy_version` tại thời điểm tạo. Bot được khởi tạo với trạng thái `Running` và bắt đầu chạy ngầm ngay lập tức.

**Yêu cầu Auth:** Có

**Điều kiện tiên quyết:** Đã cấu hình API Key (trạng thái `Connected`), có ít nhất một chiến lược hợp lệ.

**Request:**

| Tham số       | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                        |
| :------------ | :----- | :----- | :------- | :------------------------------------------------------------ |
| `bot_name`    | Body   | string | Có       | Tên định danh cho Bot. Tối đa 100 ký tự.                      |
| `strategy_id` | Body   | UUID   | Có       | ID chiến lược dùng làm nền tảng logic cho Bot.                |
| `symbol`      | Body   | string | Có       | Cặp tiền mã hóa giao dịch (VD: `BTCUSDT`).                   |

```json
{
  "bot_name": "BTC Scalper",
  "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "symbol": "BTCUSDT"
}
```

**Response thành công (201 Created):**

```json
{
  "message": "Bot đã được khởi tạo và bắt đầu chạy.",
  "data": {
    "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "strategy_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
    "strategy_version": 3,
    "symbol": "BTCUSDT",
    "status": "Running",
    "total_pnl": 0,
    "created_at": "2026-02-28T10:00:00Z"
  }
}
```

**Response lỗi (422 Unprocessable Entity) - Chưa kết nối sàn:**

```json
{
  "error": {
    "code": "EXCHANGE_NOT_CONFIGURED",
    "message": "Chưa cấu hình kết nối sàn. Vui lòng thiết lập API Key trước khi khởi tạo Bot."
  }
}
```

---

#### 3.6.4. `POST /api/v1/bots/:id/stop`

**Mô tả nghiệp vụ:** Dừng một Bot đang chạy. Hệ thống hỗ trợ hai phương án dừng: (1) Dừng Bot và đồng thời đóng tất cả vị thế đang mở trên sàn, hoặc (2) Chỉ dừng Bot mà giữ nguyên vị thế hiện tại. Trạng thái Bot chuyển sang `Stopped`.

**Yêu cầu Auth:** Có

**Request:**

| Tham số          | Vị trí | Kiểu    | Bắt buộc | Mô tả                                                                                 |
| :--------------- | :----- | :------ | :------- | :------------------------------------------------------------------------------------- |
| `id`             | Path   | UUID    | Có       | ID của Bot cần dừng.                                                                   |
| `close_position` | Body   | boolean | Không    | `true`: Dừng Bot + Đóng vị thế trên sàn. `false` (mặc định): Chỉ dừng Bot.           |

```json
{
  "close_position": true
}
```

**Response thành công (200 OK):**

```json
{
  "message": "Bot đã được dừng và vị thế đã được đóng.",
  "data": {
    "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "status": "Stopped",
    "total_pnl": 125.45,
    "updated_at": "2026-02-28T12:00:00Z"
  }
}
```

---

#### 3.6.5. `POST /api/v1/bots/:id/start`

**Mô tả nghiệp vụ:** Khởi động lại một Bot đã dừng (`Stopped`). Hệ thống đọc trạng thái đã lưu trong Database để biết Bot đang giữ vị thế nào (nếu có) và tiếp tục vận hành từ trạng thái đó, thay vì bắt đầu lại từ con số 0 (Data Integrity).

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả                        |
| :------ | :----- | :--- | :------- | :---------------------------- |
| `id`    | Path   | UUID | Có       | ID của Bot cần khởi động lại. |

**Response thành công (200 OK):**

```json
{
  "message": "Bot đã được khởi động lại.",
  "data": {
    "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "status": "Running",
    "updated_at": "2026-02-28T13:00:00Z"
  }
}
```

**Response lỗi (409 Conflict) - Bot đang chạy:**

```json
{
  "error": {
    "code": "BOT_ALREADY_RUNNING",
    "message": "Bot đang trong trạng thái Running."
  }
}
```

---

#### 3.6.6. `DELETE /api/v1/bots/:id`

**Mô tả nghiệp vụ:** Xóa một Bot khỏi hệ thống. Chỉ cho phép xóa Bot có trạng thái `Stopped`. Nếu Bot đang `Running`, phải dừng trước khi xóa.

**Yêu cầu Auth:** Có

**Request:**

| Tham số | Vị trí | Kiểu | Bắt buộc | Mô tả              |
| :------ | :----- | :--- | :------- | :------------------ |
| `id`    | Path   | UUID | Có       | ID của Bot cần xóa. |

**Response thành công (200 OK):**

```json
{
  "message": "Đã xóa Bot thành công."
}
```

**Response lỗi (409 Conflict) - Bot đang chạy:**

```json
{
  "error": {
    "code": "BOT_STILL_RUNNING",
    "message": "Không thể xóa Bot đang chạy. Vui lòng dừng Bot trước."
  }
}
```

---

#### 3.6.7. `GET /api/v1/bots/:id/logs`

**Mô tả nghiệp vụ:** Lấy lịch sử nhật ký hoạt động (Logs) của một Bot. Hỗ trợ phân trang dạng cuộn (cursor-based) để tải thêm log cũ hơn. Khi mở giao diện Console Logs lần đầu, Frontend gọi endpoint này để tải lịch sử, sau đó nhận log mới qua WebSocket.

**Yêu cầu Auth:** Có

**Request:**

| Tham số  | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                |
| :------- | :----- | :----- | :------- | :-------------------------------------------------------------------- |
| `id`     | Path   | UUID   | Có       | ID của Bot.                                                           |
| `cursor` | Query  | string | Không    | Con trỏ phân trang (ID log cuối cùng). Bỏ trống = lấy log mới nhất.  |
| `limit`  | Query  | int    | Không    | Số dòng log mỗi lần tải. Mặc định: `50`. Tối đa: `200`.              |

**Response thành công (200 OK):**

```json
{
  "data": [
    {
      "id": 10542,
      "action_decision": "Đặt lệnh Long",
      "unit_used": 15,
      "message": "RSI = 28.5 < 30 → Mở vị thế Long 0.01 BTC tại giá 64500.00",
      "created_at": "2026-02-28T10:15:00Z"
    },
    {
      "id": 10541,
      "action_decision": "Bỏ qua",
      "unit_used": 3,
      "message": "EMA9 = 64350 < EMA21 = 64400 → Chưa đủ điều kiện vào lệnh.",
      "created_at": "2026-02-28T10:00:00Z"
    }
  ],
  "pagination": {
    "next_cursor": "10540",
    "has_more": true
  }
}
```

---

### 3.7. Trades

Nhóm API truy xuất lịch sử giao dịch. Dữ liệu là dữ liệu đã chốt (Realized), trích xuất từ Database, phục vụ đối soát khoản phí và lợi nhuận thực tế. **Bắt buộc hỗ trợ Query Parameters để phân trang và lọc theo Bot ID, Symbol.**

---

#### 3.7.1. `GET /api/v1/trades`

**Mô tả nghiệp vụ:** Lấy danh sách lịch sử giao dịch đã hoàn tất. Mặc định trả về các lệnh trong **7 ngày gần nhất**. Hỗ trợ phân trang dạng cuộn (Infinite Scroll) và lọc theo nhiều tiêu chí.

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                                |
| :----------- | :----- | :----- | :------- | :------------------------------------------------------------------------------------ |
| `bot_id`     | Query  | UUID   | Không    | Lọc theo ID Bot. Bỏ trống = tất cả Bot.                                              |
| `symbol`     | Query  | string | Không    | Lọc theo cặp tiền (VD: `BTCUSDT`). Bỏ trống = tất cả symbol.                         |
| `side`       | Query  | string | Không    | Lọc theo chiều giao dịch: `Long`, `Short`. Bỏ trống = tất cả.                         |
| `status`     | Query  | string | Không    | Lọc theo trạng thái lệnh: `Filled`, `Canceled`. Bỏ trống = tất cả.                    |
| `start_date` | Query  | string | Không    | Lọc từ ngày (ISO 8601). Mặc định: 7 ngày trước.                                       |
| `end_date`   | Query  | string | Không    | Lọc đến ngày (ISO 8601). Mặc định: hiện tại.                                           |
| `cursor`     | Query  | string | Không    | Con trỏ phân trang (ID lệnh cuối cùng) phục vụ Infinite Scroll. Bỏ trống = trang đầu. |
| `limit`      | Query  | int    | Không    | Số bản ghi mỗi lần tải. Mặc định: `50`. Tối đa: `200`.                                |

**Ví dụ request:**

```
GET /api/v1/trades?bot_id=e5f6a7b8-c9d0-1234-efab-345678901234&symbol=BTCUSDT&limit=50
```

**Response thành công (200 OK):**

```json
{
  "data": [
    {
      "id": "t1a2b3c4-d5e6-7890-abcd-ef1234567890",
      "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
      "bot_name": "BTC Scalper",
      "symbol": "BTCUSDT",
      "side": "Long",
      "quantity": 0.01,
      "fill_price": 64500.00,
      "fee": 0.26,
      "realized_pnl": 5.74,
      "status": "Filled",
      "executed_at": "2026-02-28T10:15:30Z"
    },
    {
      "id": "t2b3c4d5-e6f7-8901-bcde-f12345678901",
      "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
      "bot_name": "BTC Scalper",
      "symbol": "BTCUSDT",
      "side": "Short",
      "quantity": 0.01,
      "fill_price": 64200.00,
      "fee": 0.26,
      "realized_pnl": -2.86,
      "status": "Filled",
      "executed_at": "2026-02-28T08:30:00Z"
    }
  ],
  "pagination": {
    "next_cursor": "t3c4d5e6-f7a8-9012-cdef-234567890123",
    "has_more": true
  }
}
```

**Response thành công (200 OK) - Không có dữ liệu:**

```json
{
  "data": [],
  "pagination": {
    "next_cursor": null,
    "has_more": false
  },
  "message": "Chưa có lịch sử giao dịch nào."
}
```

---

#### 3.7.2. `GET /api/v1/trades/export`

**Mô tả nghiệp vụ:** Xuất toàn bộ lịch sử giao dịch ra file CSV (theo bộ lọc hiện hành). Frontend nhận file từ Response để trigger download.

**Yêu cầu Auth:** Có

**Request:**

| Tham số      | Vị trí | Kiểu   | Bắt buộc | Mô tả                                          |
| :----------- | :----- | :----- | :------- | :---------------------------------------------- |
| `bot_id`     | Query  | UUID   | Không    | Lọc theo Bot ID.                                |
| `symbol`     | Query  | string | Không    | Lọc theo cặp tiền.                              |
| `start_date` | Query  | string | Không    | Lọc từ ngày (ISO 8601).                         |
| `end_date`   | Query  | string | Không    | Lọc đến ngày (ISO 8601).                        |

**Response thành công (200 OK):**

> Header: `Content-Type: text/csv`
> Header: `Content-Disposition: attachment; filename="trade-history-20260228.csv"`

```csv
ID,Bot,Symbol,Side,Quantity,Fill Price,Fee,Realized PnL,Status,Executed At
t1a2b3c4-...,BTC Scalper,BTCUSDT,Long,0.01,64500.00,0.26,5.74,Filled,2026-02-28T10:15:30Z
t2b3c4d5-...,BTC Scalper,BTCUSDT,Short,0.01,64200.00,0.26,-2.86,Filled,2026-02-28T08:30:00Z
```

---

### 3.8. Market

Nhóm API cung cấp dữ liệu thị trường. Frontend **tuyệt đối không kết nối trực tiếp** đến WebSocket của Binance. Dữ liệu giá biến động được Backend nhận từ sàn, xử lý, sau đó đẩy về Frontend qua kênh WebSocket nội bộ (xem mục 4).

---

#### 3.8.1. `GET /api/v1/market/symbols`

**Mô tả nghiệp vụ:** Lấy danh sách các cặp tiền mã hóa hỗ trợ giao dịch trên hệ thống (Market Watch). Mỗi symbol kèm theo giá hiện tại và phần trăm thay đổi trong 24h.

**Yêu cầu Auth:** Có

**Request:**

| Tham số  | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                            |
| :------- | :----- | :----- | :------- | :---------------------------------------------------------------- |
| `search` | Query  | string | Không    | Tìm kiếm cặp tiền theo từ khóa (VD: `BTC`). Bỏ trống = tất cả. |

**Response thành công (200 OK):**

```json
{
  "data": [
    {
      "symbol": "BTCUSDT",
      "last_price": 64500.00,
      "price_change_percent": 2.35,
      "volume_24h": 28543219000.50
    },
    {
      "symbol": "ETHUSDT",
      "last_price": 3420.50,
      "price_change_percent": -1.20,
      "volume_24h": 12874562000.80
    },
    {
      "symbol": "SOLUSDT",
      "last_price": 142.30,
      "price_change_percent": 5.67,
      "volume_24h": 4521890000.25
    }
  ]
}
```

---

#### 3.8.2. `GET /api/v1/market/candles`

**Mô tả nghiệp vụ:** Lấy dữ liệu nến lịch sử (OHLCV) của một cặp tiền. Backend tải từ Database, nếu khoảng thời gian yêu cầu chưa có trong Database, hệ thống tự động gọi REST API Binance để tải về (On-demand Sync). Dữ liệu trả về kèm theo danh sách Trade Markers (điểm khớp lệnh) của các Bot đang chạy trên cặp tiền này để trực quan hóa trên biểu đồ.

**Yêu cầu Auth:** Có

**Request:**

| Tham số     | Vị trí | Kiểu   | Bắt buộc | Mô tả                                                                    |
| :---------- | :----- | :----- | :------- | :------------------------------------------------------------------------ |
| `symbol`    | Query  | string | Có       | Cặp tiền mã hóa (VD: `BTCUSDT`).                                         |
| `timeframe` | Query  | string | Có       | Khung thời gian nến: `1m`, `5m`, `15m`, `1h`, `4h`, `1D`.                 |
| `start`     | Query  | string | Không    | Thời gian bắt đầu (ISO 8601). Mặc định: 500 nến gần nhất.                |
| `end`       | Query  | string | Không    | Thời gian kết thúc (ISO 8601). Mặc định: hiện tại.                        |
| `limit`     | Query  | int    | Không    | Số lượng nến tối đa. Mặc định: `500`. Tối đa: `1500`.                     |

**Ví dụ request:**

```
GET /api/v1/market/candles?symbol=BTCUSDT&timeframe=15m&limit=500
```

**Response thành công (200 OK):**

```json
{
  "data": {
    "symbol": "BTCUSDT",
    "timeframe": "15m",
    "candles": [
      {
        "open_time": "2026-02-28T09:00:00Z",
        "open": 64350.00,
        "high": 64520.00,
        "low": 64300.00,
        "close": 64480.00,
        "volume": 125.43,
        "is_closed": true
      },
      {
        "open_time": "2026-02-28T09:15:00Z",
        "open": 64480.00,
        "high": 64550.00,
        "low": 64420.00,
        "close": 64500.00,
        "volume": 98.72,
        "is_closed": true
      }
    ],
    "markers": [
      {
        "time": "2026-02-28T09:15:00Z",
        "price": 64500.00,
        "side": "Long",
        "bot_name": "BTC Scalper",
        "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234"
      }
    ]
  }
}
```

---

## 4. Đặc tả WebSocket (Real-time Channels)

Nền tảng giao dịch yêu cầu kết nối thời gian thực để đồng bộ dữ liệu thị trường, nhật ký Bot và trạng thái vị thế liên tục giữa Backend và Frontend.

### 4.1. Endpoint kết nối

| Môi trường  | URL                              |
| :---------- | :------------------------------- |
| Development | `ws://localhost:8080/ws`         |
| Production  | `wss://quantflow.io/ws`          |

**Xác thực kết nối:** Client phải gửi JWT Token thông qua query parameter khi thiết lập kết nối:

```
wss://quantflow.io/ws?token=<JWT_TOKEN>
```

Hoặc Backend tự động đọc Token từ Cookie HttpOnly (nếu trình duyệt hỗ trợ gửi Cookie cùng WebSocket handshake).

Nếu Token không hợp lệ hoặc hết hạn, Server gửi message lỗi và **đóng kết nối**:

```json
{
  "event": "error",
  "data": {
    "code": "AUTH_FAILED",
    "message": "Phiên làm việc không hợp lệ hoặc đã hết hạn."
  }
}
```

### 4.2. Cơ chế Subscribe / Unsubscribe

Client gửi message JSON để đăng ký (subscribe) hoặc hủy đăng ký (unsubscribe) các kênh dữ liệu.

**Subscribe (Đăng ký kênh):**

```json
{
  "action": "subscribe",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

**Server xác nhận:**

```json
{
  "event": "subscribed",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

**Unsubscribe (Hủy đăng ký):**

```json
{
  "action": "unsubscribe",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

**Server xác nhận:**

```json
{
  "event": "unsubscribed",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

### 4.3. Danh sách kênh sự kiện (Event Channels)

---

#### 4.3.1. `market_ticker` — Dữ liệu nến/giá thời gian thực

**Mô tả:** Đẩy liên tục dữ liệu giá biến động và nến mới từ Backend về Frontend. Backend nhận luồng dữ liệu từ Binance WebSocket, xử lý, sau đó chuyển tiếp nội bộ sang Client. Frontend **tuyệt đối không kết nối trực tiếp** đến Binance.

**Tham số Subscribe:**

| Tham số  | Kiểu   | Bắt buộc | Mô tả                                          |
| :------- | :----- | :------- | :---------------------------------------------- |
| `symbol` | string | Có       | Cặp tiền cần nhận dữ liệu (VD: `BTCUSDT`).    |

**Payload sự kiện — Cập nhật giá ticker:**

```json
{
  "event": "market_ticker",
  "channel": "market_ticker",
  "data": {
    "symbol": "BTCUSDT",
    "last_price": 64520.50,
    "price_change_percent": 2.41,
    "high_24h": 65100.00,
    "low_24h": 63200.00,
    "volume_24h": 28650432000.75,
    "timestamp": "2026-02-28T10:30:05Z"
  }
}
```

**Payload sự kiện — Cập nhật nến (Candle Update):**

```json
{
  "event": "market_candle",
  "channel": "market_ticker",
  "data": {
    "symbol": "BTCUSDT",
    "timeframe": "15m",
    "candle": {
      "open_time": "2026-02-28T10:30:00Z",
      "open": 64500.00,
      "high": 64550.00,
      "low": 64480.00,
      "close": 64520.50,
      "volume": 45.32,
      "is_closed": false
    }
  }
}
```

> Khi `is_closed = false`, nến đang hình thành (cập nhật liên tục). Khi `is_closed = true`, nến đã đóng hoàn toàn và được Backend INSERT vào Database.

---

#### 4.3.2. `bot_logs` — Nhật ký hệ thống thời gian thực

**Mô tả:** Bắn log hoạt động thời gian thực từ Backend về Frontend. Mỗi khi Bot thực thi một phiên logic (Session), Backend ghi log vào Database đồng thời đẩy dòng log mới qua WebSocket để giao diện Console hiển thị ngay lập tức.

**Tham số Subscribe:**

| Tham số  | Kiểu | Bắt buộc | Mô tả                                |
| :------- | :--- | :------- | :------------------------------------ |
| `bot_id` | UUID | Có       | ID Bot cần nhận luồng log.           |

**Subscribe:**

```json
{
  "action": "subscribe",
  "channel": "bot_logs",
  "params": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234"
  }
}
```

**Payload sự kiện:**

```json
{
  "event": "bot_log",
  "channel": "bot_logs",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "log": {
      "id": 10543,
      "action_decision": "Đặt lệnh Long",
      "unit_used": 15,
      "message": "RSI = 28.5 < 30 → Mở vị thế Long 0.01 BTC tại giá 64500.00",
      "created_at": "2026-02-28T10:30:00Z"
    }
  }
}
```

---

#### 4.3.3. `position_update` — Đồng bộ PnL và vị thế Bot

**Mô tả:** Đồng bộ liên tục trạng thái vị thế và lợi nhuận chưa chốt (Unrealized PnL) của tất cả Bot đang chạy về Frontend. Dữ liệu được cập nhật theo nhịp đập của thị trường, tương ứng với biến động giá trên Chart.

**Tham số Subscribe:**

Không yêu cầu tham số. Khi subscribe, Client sẽ nhận cập nhật của **tất cả Bot** thuộc quyền sở hữu đang ở trạng thái `Running`.

**Subscribe:**

```json
{
  "action": "subscribe",
  "channel": "position_update"
}
```

**Payload sự kiện — Cập nhật vị thế:**

```json
{
  "event": "position_update",
  "channel": "position_update",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "symbol": "BTCUSDT",
    "status": "Running",
    "total_pnl": 128.95,
    "position": {
      "side": "Long",
      "entry_price": 64500.00,
      "quantity": 0.05,
      "leverage": 10,
      "unrealized_pnl": 35.50,
      "margin_type": "Isolated"
    },
    "open_orders": [
      {
        "order_id": "ORD-001",
        "side": "Sell",
        "type": "Limit",
        "price": 65200.00,
        "quantity": 0.05,
        "status": "Pending"
      }
    ],
    "timestamp": "2026-02-28T10:30:05Z"
  }
}
```

**Payload sự kiện — Bot thay đổi trạng thái:**

```json
{
  "event": "bot_status_change",
  "channel": "position_update",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "previous_status": "Running",
    "new_status": "Stopped",
    "reason": "Người dùng dừng Bot thủ công.",
    "timestamp": "2026-02-28T12:00:00Z"
  }
}
```

**Payload sự kiện — Lỗi đặt lệnh (Sàn từ chối):**

```json
{
  "event": "bot_error",
  "channel": "position_update",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "error_type": "ORDER_REJECTED",
    "message": "Lỗi đặt lệnh: Không đủ số dư khả dụng.",
    "timestamp": "2026-02-28T10:35:00Z"
  }
}
```

---

### 4.4. Cơ chế Heartbeat & Reconnect

**Heartbeat (Kiểm tra kết nối):**

Server gửi message `ping` định kỳ mỗi **30 giây**. Client phải phản hồi `pong` trong vòng **10 giây**. Nếu không nhận được `pong`, Server đóng kết nối.

```json
// Server → Client
{ "event": "ping", "timestamp": "2026-02-28T10:30:00Z" }

// Client → Server
{ "event": "pong", "timestamp": "2026-02-28T10:30:00Z" }
```

**Auto-reconnect (Tự động kết nối lại):**

Khi Client phát hiện mất kết nối WebSocket, áp dụng thuật toán **Exponential Backoff** để thử kết nối lại:

-   Lần 1: Thử lại sau **1 giây**.
-   Lần 2: Thử lại sau **2 giây**.
-   Lần 3: Thử lại sau **4 giây**.
-   Lần 4: Thử lại sau **8 giây**.
-   Tối đa: **30 giây** giữa mỗi lần thử.

Khi kết nối lại thành công, Client tự động re-subscribe tất cả các kênh đã đăng ký trước đó.

**Thông báo trạng thái kết nối:**

```json
{
  "event": "connection_status",
  "data": {
    "status": "reconnecting",
    "message": "Mất kết nối. Đang thử kết nối lại...",
    "attempt": 3
  }
}
```
