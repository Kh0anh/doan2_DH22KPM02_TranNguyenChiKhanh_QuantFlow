# Đặc tả WebSocket — Real-time Channels

> **Dự án:** QuantFlow — Nền tảng Low-code hỗ trợ xây dựng và vận hành chiến lược giao dịch tiền mã hóa trên sàn giao dịch tập trung (CEX).
>
> **Phiên bản:** 1.0 &nbsp;|&nbsp; **Cập nhật lần cuối:** 2026-03-05

---

## Mục lục

1. [Tổng quan Kết nối (Connection Overview)](#1-tổng-quan-kết-nối-connection-overview)
   - 1.1 [Endpoint kết nối](#11-endpoint-kết-nối)
   - 1.2 [Xác thực (Authentication)](#12-xác-thực-authentication)
   - 1.3 [Cơ chế Heartbeat & Reconnection](#13-cơ-chế-heartbeat--reconnection)
2. [Quy chuẩn Định dạng Message (Message Format)](#2-quy-chuẩn-định-dạng-message-message-format)
   - 2.1 [Client Request — Subscribe / Unsubscribe](#21-client-request--subscribe--unsubscribe)
   - 2.2 [Server Response — Xác nhận & Lỗi](#22-server-response--xác-nhận--lỗi)
   - 2.3 [Server Event — Dữ liệu đẩy về (Push Data)](#23-server-event--dữ-liệu-đẩy-về-push-data)
3. [Đặc tả các Kênh (Channels & Events)](#3-đặc-tả-các-kênh-channels--events)
   - 3.1 [Channel: `market_ticker`](#31-channel-market_ticker)
   - 3.2 [Channel: `bot_logs`](#32-channel-bot_logs)
   - 3.3 [Channel: `position_update`](#33-channel-position_update)

---

## 1. Tổng quan Kết nối (Connection Overview)

### 1.1 Endpoint kết nối

| Môi trường    | Base URI                                |
| :------------ | :-------------------------------------- |
| Development   | `ws://localhost:8080/v1/ws`             |
| Production    | `wss://api.quantflow.com/v1/ws`         |

---

### 1.2 Xác thực (Authentication)

Kết nối WebSocket yêu cầu xác thực danh tính người dùng thông qua **JWT Token** đã được cấp phát bởi API `POST /auth/login`.

#### Phương thức 1 — Query Parameter

Client gửi JWT Token dưới dạng query parameter `token` khi thiết lập kết nối:

```
wss://api.quantflow.com/v1/ws?token=<JWT_TOKEN>
```

#### Phương thức 2 — HttpOnly Cookie

Nếu trình duyệt hỗ trợ gửi Cookie cùng quá trình WebSocket Handshake, Backend sẽ tự động đọc JWT Token từ HttpOnly Cookie `token` mà không cần Client gửi tường minh.

#### Xử lý lỗi xác thực

Khi Token không hợp lệ, đã hết hạn, hoặc không được cung cấp, Server gửi message lỗi và **đóng kết nối ngay lập tức** (Close Code `4001`):

```json
{
  "event": "error",
  "data": {
    "code": "AUTH_FAILED",
    "message": "Phiên làm việc không hợp lệ hoặc đã hết hạn. Vui lòng đăng nhập lại."
  }
}
```

| Tình huống                        | Hành vi Server                                                   |
| :-------------------------------- | :--------------------------------------------------------------- |
| Không có Token                    | Gửi lỗi `AUTH_FAILED` → Đóng kết nối (Close Code `4001`)        |
| Token hết hạn (> 24h)            | Gửi lỗi `AUTH_FAILED` → Đóng kết nối (Close Code `4001`)        |
| Token không đúng chữ ký (Invalid) | Gửi lỗi `AUTH_FAILED` → Đóng kết nối (Close Code `4001`)        |
| Token hợp lệ                     | Chấp nhận kết nối → Cho phép Subscribe kênh dữ liệu             |

---

### 1.3 Cơ chế Heartbeat & Reconnection

#### Heartbeat (Ping / Pong)

Để duy trì kết nối ổn định và phát hiện sớm mất kết nối, Server và Client thực hiện cơ chế Heartbeat:

| Thông số                | Giá trị                                                        |
| :---------------------- | :-------------------------------------------------------------- |
| Server gửi `ping`      | Mỗi **30 giây**                                                |
| Client phản hồi `pong` | Trong vòng **10 giây** kể từ khi nhận `ping`                   |
| Timeout                 | Nếu Client không phản hồi `pong` sau 10 giây → Server đóng kết nối |

**Server → Client (Ping):**

```json
{
  "event": "ping",
  "timestamp": "2026-03-05T10:30:00Z"
}
```

**Client → Server (Pong):**

```json
{
  "event": "pong",
  "timestamp": "2026-03-05T10:30:01Z"
}
```

#### Chiến lược Reconnection (Exponential Backoff)

Khi Client phát hiện mất kết nối (sự kiện `onclose` hoặc `onerror`), Client **bắt buộc** áp dụng thuật toán **Exponential Backoff** để tự động kết nối lại:

| Lần thử lại | Thời gian chờ | Ghi chú                                   |
| :---------- | :------------ | :----------------------------------------- |
| 1           | 1 giây        |                                            |
| 2           | 2 giây        |                                            |
| 3           | 4 giây        |                                            |
| 4           | 8 giây        |                                            |
| 5           | 16 giây       |                                            |
| 6+          | 30 giây       | Giới hạn tối đa (cap), tiếp tục thử liên tục |

**Quy tắc sau khi kết nối lại thành công:**

1. Client phải tự động **re-subscribe** tất cả các kênh đã đăng ký trước đó.
2. Giao diện Frontend chuyển trạng thái từ `Reconnecting` (vàng) về `Connected` (xanh).
3. Nếu Token hết hạn trong quá trình reconnect → Client chuyển hướng người dùng về màn hình Đăng nhập.

---

## 2. Quy chuẩn Định dạng Message (Message Format)

Mọi message trao đổi giữa Client và Server đều sử dụng định dạng **JSON** (UTF-8). Dưới đây là ba loại message chính trong hệ thống.

### 2.1 Client Request — Subscribe / Unsubscribe

Client gửi message JSON để **đăng ký** hoặc **hủy đăng ký** nhận dữ liệu từ một kênh cụ thể.

#### Subscribe (Đăng ký kênh)

```json
{
  "action": "subscribe",
  "channel": "<channel_name>",
  "params": {
    "<key>": "<value>"
  }
}
```

#### Unsubscribe (Hủy đăng ký kênh)

```json
{
  "action": "unsubscribe",
  "channel": "<channel_name>",
  "params": {
    "<key>": "<value>"
  }
}
```

| Trường     | Kiểu dữ liệu | Bắt buộc | Mô tả                                                                 |
| :--------- | :------------ | :------- | :--------------------------------------------------------------------- |
| `action`   | string        | Có       | Hành động: `subscribe` hoặc `unsubscribe`.                            |
| `channel`  | string        | Có       | Tên kênh cần đăng ký: `market_ticker`, `bot_logs`, `position_update`. |
| `params`   | object        | Không    | Tham số bổ sung tùy theo kênh (VD: `symbol`, `bot_id`).               |

---

### 2.2 Server Response — Xác nhận & Lỗi

Sau khi nhận Client Request, Server phản hồi kết quả xử lý.

#### Xác nhận thành công

```json
{
  "event": "subscribed",
  "channel": "<channel_name>",
  "params": {
    "<key>": "<value>"
  }
}
```

#### Xác nhận hủy thành công

```json
{
  "event": "unsubscribed",
  "channel": "<channel_name>",
  "params": {
    "<key>": "<value>"
  }
}
```

#### Phản hồi lỗi

Khi Client gửi request không hợp lệ (ví dụ: kênh không tồn tại, thiếu tham số bắt buộc, hoặc không có quyền):

```json
{
  "event": "error",
  "data": {
    "code": "<ERROR_CODE>",
    "message": "<Mô tả lỗi chi tiết>"
  }
}
```

**Danh sách mã lỗi WebSocket:**

| Mã lỗi (`code`)         | Mô tả                                                             |
| :----------------------- | :----------------------------------------------------------------- |
| `AUTH_FAILED`            | Token không hợp lệ hoặc đã hết hạn.                               |
| `INVALID_ACTION`         | Trường `action` không phải `subscribe` hoặc `unsubscribe`.        |
| `INVALID_CHANNEL`        | Tên kênh (`channel`) không tồn tại trong hệ thống.                |
| `MISSING_PARAMS`         | Thiếu tham số bắt buộc cho kênh (VD: thiếu `symbol`, `bot_id`).  |
| `INVALID_PARAMS`         | Tham số không hợp lệ (VD: `bot_id` không đúng định dạng UUID).   |
| `BOT_NOT_FOUND`          | Bot ID được cung cấp không tồn tại hoặc không thuộc quyền sở hữu.|
| `INTERNAL_ERROR`         | Lỗi nội bộ Server.                                                |

---

### 2.3 Server Event — Dữ liệu đẩy về (Push Data)

Sau khi Client subscribe thành công, Server **chủ động** đẩy dữ liệu về Client theo cấu trúc thống nhất:

```json
{
  "event": "<event_name>",
  "channel": "<channel_name>",
  "data": {
    ...
  }
}
```

| Trường     | Kiểu dữ liệu | Mô tả                                                               |
| :--------- | :------------ | :------------------------------------------------------------------- |
| `event`    | string        | Tên sự kiện cụ thể (VD: `market_ticker`, `bot_log`, `bot_error`).   |
| `channel`  | string        | Kênh phát sinh sự kiện (VD: `market_ticker`, `bot_logs`).           |
| `data`     | object        | Payload dữ liệu chi tiết, cấu trúc tùy theo từng event.            |

---

## 3. Đặc tả các Kênh (Channels & Events)

### 3.1 Channel: `market_ticker`

#### Mục đích

Stream liên tục dữ liệu **giá biến động** (Ticker) và **nến OHLCV** (Candle) từ Backend về Frontend theo từng cặp giao dịch (Symbol). Backend nhận luồng dữ liệu từ Binance WebSocket nội bộ, xử lý và chuyển tiếp xuống Client. Frontend tuyệt đối **không** kết nối trực tiếp đến Binance.

#### Subscribe

Client đăng ký nhận dữ liệu giá theo cặp tiền cụ thể:

```json
{
  "action": "subscribe",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

| Tham số   | Kiểu    | Bắt buộc | Mô tả                                                    |
| :-------- | :------ | :------- | :-------------------------------------------------------- |
| `symbol`  | string  | Có       | Cặp tiền mã hóa cần nhận dữ liệu (VD: `BTCUSDT`, `ETHUSDT`). |

#### Unsubscribe

```json
{
  "action": "unsubscribe",
  "channel": "market_ticker",
  "params": {
    "symbol": "BTCUSDT"
  }
}
```

#### Event: `market_ticker` — Cập nhật giá Ticker

Dữ liệu giá ticker thời gian thực, phục vụ hiển thị bảng Market Watch và cập nhật giá trên giao diện.

**Payload mẫu:**

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
    "timestamp": "2026-03-05T10:30:05Z"
  }
}
```

| Trường                  | Kiểu      | Mô tả                                               |
| :---------------------- | :-------- | :--------------------------------------------------- |
| `symbol`                | string    | Cặp tiền mã hóa.                                    |
| `last_price`            | number    | Giá khớp gần nhất.                                  |
| `price_change_percent`  | number    | Phần trăm thay đổi giá trong 24 giờ.                |
| `high_24h`              | number    | Giá cao nhất trong 24 giờ.                           |
| `low_24h`               | number    | Giá thấp nhất trong 24 giờ.                          |
| `volume_24h`            | number    | Tổng khối lượng giao dịch 24 giờ (USDT).            |
| `timestamp`             | string    | Thời điểm dữ liệu (ISO 8601, UTC).                 |

#### Event: `market_candle` — Cập nhật nến OHLCV

Dữ liệu nến (Candlestick) cập nhật liên tục theo Symbol. Khi `is_closed = false`, nến đang hình thành (cập nhật liên tục giá Close, High, Low, Volume). Khi `is_closed = true`, nến đã đóng hoàn toàn và được Backend `INSERT` vào bảng `candles_data` trong Database.

**Payload mẫu:**

```json
{
  "event": "market_candle",
  "channel": "market_ticker",
  "data": {
    "symbol": "BTCUSDT",
    "timeframe": "15m",
    "candle": {
      "open_time": "2026-03-05T10:30:00Z",
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

| Trường               | Kiểu    | Mô tả                                                            |
| :------------------- | :------ | :---------------------------------------------------------------- |
| `symbol`             | string  | Cặp tiền mã hóa.                                                 |
| `timeframe`          | string  | Khung thời gian nến (`1m`, `5m`, `15m`, `1h`, `4h`, `1d`).       |
| `candle.open_time`   | string  | Thời gian mở nến (ISO 8601, UTC).                                |
| `candle.open`        | number  | Giá mở cửa (Open).                                               |
| `candle.high`        | number  | Giá cao nhất (High).                                              |
| `candle.low`         | number  | Giá thấp nhất (Low).                                             |
| `candle.close`       | number  | Giá đóng cửa (Close).                                            |
| `candle.volume`      | number  | Khối lượng giao dịch trong nến.                                   |
| `candle.is_closed`   | boolean | `false` = nến đang hình thành; `true` = nến đã đóng hoàn toàn.   |

> **Nghiệp vụ:** Khi `is_closed = true`, Frontend cần thêm cây nến mới vào biểu đồ (Candle Chart). Khi `is_closed = false`, Frontend cập nhật cây nến cuối cùng đang hiện trên biểu đồ. Dữ liệu nến tương ứng với cấu trúc bảng `candles_data` trong Database (`symbol`, `interval`, `open_time`, `open_price`, `high_price`, `low_price`, `close_price`, `volume`, `is_closed`).

---

### 3.2 Channel: `bot_logs`

#### Mục đích

Đẩy **nhật ký hoạt động** (Live Logs) thời gian thực từ Backend về Frontend cho từng Bot cụ thể. Mỗi khi Bot thực thi một phiên logic (Session), Backend ghi log vào bảng `bot_logs` trong Database đồng thời đẩy dòng log mới qua WebSocket để giao diện Console hiển thị ngay lập tức mà không cần reload hay polling.

> **Luồng khuyến nghị:** Khi mở giao diện Console Logs lần đầu, Frontend gọi REST API `GET /bots/{botId}/logs` để tải lịch sử log (Cursor-based Pagination). Sau đó, Frontend subscribe kênh `bot_logs` để nhận log mới theo thời gian thực.

#### Subscribe

Client đăng ký nhận log theo Bot ID cụ thể:

```json
{
  "action": "subscribe",
  "channel": "bot_logs",
  "params": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234"
  }
}
```

| Tham số   | Kiểu          | Bắt buộc | Mô tả                                     |
| :-------- | :------------ | :------- | :----------------------------------------- |
| `bot_id`  | string (UUID) | Có       | ID của Bot cần nhận luồng log thời gian thực. |

#### Unsubscribe

```json
{
  "action": "unsubscribe",
  "channel": "bot_logs",
  "params": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234"
  }
}
```

#### Event: `bot_log` — Dòng log mới từ Bot

Mỗi khi một phiên logic (Session) hoàn tất quyết định, Backend đẩy dòng log chi tiết xuống Client. Mỗi dòng log hiển thị: Thời gian kích hoạt Session, Quyết định hành động (Đặt lệnh / Bỏ qua), Số lượng Unit đã sử dụng, và Message mô tả chi tiết.

**Payload mẫu:**

```json
{
  "event": "bot_log",
  "channel": "bot_logs",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "log": {
      "id": 10543,
      "action_decision": "Đặt lệnh Long",
      "message": "Mở vị thế Long 0.01 BTC tại giá 64500.00",
      "created_at": "2026-03-05T10:30:00Z"
    }
  }
}
```

| Trường                    | Kiểu    | Mô tả                                                                |
| :------------------------ | :------ | :-------------------------------------------------------------------- |
| `bot_id`                  | string  | UUID của Bot phát sinh log.                                           |
| `log.id`                  | integer | ID tự tăng của dòng log (BIGSERIAL), tương ứng PK bảng `bot_logs`.   |
| `log.action_decision`     | string  | Quyết định hành động của Bot (VD: `Đặt lệnh Long`, `Bỏ qua`).       |
| `log.unit_used`           | integer | Số lượng Unit đã tiêu thụ trong Session (cơ chế chống treo Unit Cost).|
| `log.message`             | string  | Thông báo chi tiết mô tả logic quyết định hoặc lỗi phát sinh.        |
| `log.created_at`          | string  | Thời gian log được sinh ra (ISO 8601, UTC).                           |

> **Nghiệp vụ hiển thị:** Giao diện Console mô phỏng Terminal (nền đen, chữ sáng). Khi có dòng log mới, Frontend tự động cuộn (Auto-scroll) xuống cuối danh sách. Áp dụng Virtual Scroll hoặc giới hạn hiển thị tối đa **1000 dòng log gần nhất** trên DOM để tránh giật lag trình duyệt khi Bot hoạt động tần suất cao.

---

### 3.3 Channel: `position_update`

#### Mục đích

Đồng bộ liên tục **trạng thái vị thế**, **lợi nhuận chưa chốt (Unrealized PnL)**, và **các sự kiện vận hành** của tất cả Bot đang chạy (trạng thái `Running`) về Frontend. Dữ liệu được cập nhật theo nhịp đập thị trường, phản ánh ngay lập tức khi có khớp lệnh mới trên sàn Binance.

> **Đặc điểm:** Kênh này **không yêu cầu tham số** khi subscribe — Client tự động nhận cập nhật của tất cả Bot thuộc quyền sở hữu đang ở trạng thái `Running`.

#### Subscribe

```json
{
  "action": "subscribe",
  "channel": "position_update",
  "params": {}
}
```

#### Unsubscribe

```json
{
  "action": "unsubscribe",
  "channel": "position_update",
  "params": {}
}
```

#### Event: `position_update` — Cập nhật vị thế và PnL

Dữ liệu đồng bộ vị thế hiện tại, PnL tổng, và danh sách lệnh chờ (Open Orders) của từng Bot. Được đẩy liên tục theo nhịp biến động giá thị trường.

**Payload mẫu:**

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
    "timestamp": "2026-03-05T10:30:05Z"
  }
}
```

| Trường                       | Kiểu    | Mô tả                                                                    |
| :--------------------------- | :------ | :------------------------------------------------------------------------ |
| `bot_id`                     | string  | UUID của Bot (liên kết bảng `bot_instances`).                             |
| `bot_name`                   | string  | Tên định danh của Bot.                                                    |
| `symbol`                     | string  | Cặp tiền đang giao dịch.                                                 |
| `status`                     | string  | Trạng thái Bot: `Running`, `Stopped`, `Error`.                           |
| `total_pnl`                  | number  | Tổng PnL tích lũy (Realized) của Bot, tương ứng cột `total_pnl` trong DB.|
| `position.side`              | string  | Chiều vị thế hiện tại: `Long` hoặc `Short`. `null` nếu không có vị thế. |
| `position.entry_price`       | number  | Giá vào lệnh trung bình.                                                 |
| `position.quantity`          | number  | Khối lượng đang nắm giữ.                                                 |
| `position.leverage`          | integer | Đòn bẩy đang áp dụng (1x – 125x).                                       |
| `position.unrealized_pnl`    | number  | Lợi nhuận chưa chốt tính theo giá hiện tại.                              |
| `position.margin_type`       | string  | Kiểu ký quỹ: `Isolated` hoặc `Cross`.                                    |
| `open_orders`                | array   | Danh sách lệnh chờ đang treo trên sàn.                                   |
| `open_orders[].order_id`     | string  | Mã lệnh trên sàn Binance.                                                |
| `open_orders[].side`         | string  | Chiều lệnh: `Buy` hoặc `Sell`.                                           |
| `open_orders[].type`         | string  | Loại lệnh: `Limit`, `Market`, `Stop`.                                    |
| `open_orders[].price`        | number  | Giá đặt lệnh (áp dụng cho Limit/Stop).                                   |
| `open_orders[].quantity`     | number  | Khối lượng lệnh.                                                         |
| `open_orders[].status`       | string  | Trạng thái lệnh: `Pending`, `PartialFilled`.                             |
| `timestamp`                  | string  | Thời điểm cập nhật (ISO 8601, UTC).                                      |

#### Event: `bot_status_change` — Bot thay đổi trạng thái

Được đẩy khi Bot chuyển trạng thái vận hành (VD: `Running → Stopped`, `Running → Error`).

**Payload mẫu:**

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
    "timestamp": "2026-03-05T12:00:00Z"
  }
}
```

| Trường             | Kiểu   | Mô tả                                                         |
| :----------------- | :----- | :------------------------------------------------------------- |
| `bot_id`           | string | UUID của Bot.                                                  |
| `bot_name`         | string | Tên Bot.                                                       |
| `previous_status`  | string | Trạng thái trước khi thay đổi.                                |
| `new_status`       | string | Trạng thái mới: `Running`, `Stopped`, `Error`.                |
| `reason`           | string | Lý do thay đổi trạng thái (mô tả ngắn gọn).                  |
| `timestamp`        | string | Thời điểm thay đổi (ISO 8601, UTC).                           |

#### Event: `bot_error` — Lỗi vận hành Bot

Được đẩy khi có lỗi xảy ra trong quá trình Bot hoạt động (VD: sàn từ chối lệnh do không đủ số dư, vượt quá giới hạn Unit Cost, mất kết nối API sàn). Bot **không nhất thiết dừng** khi gặp lỗi — hệ thống ghi log lỗi và tiếp tục chạy.

**Payload mẫu:**

```json
{
  "event": "bot_error",
  "channel": "position_update",
  "data": {
    "bot_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "bot_name": "BTC Scalper",
    "error_type": "ORDER_REJECTED",
    "message": "Lỗi đặt lệnh: Không đủ số dư khả dụng.",
    "timestamp": "2026-03-05T10:35:00Z"
  }
}
```

| Trường        | Kiểu   | Mô tả                                                                          |
| :------------ | :----- | :------------------------------------------------------------------------------ |
| `bot_id`      | string | UUID của Bot gặp lỗi.                                                          |
| `bot_name`    | string | Tên Bot.                                                                        |
| `error_type`  | string | Mã loại lỗi (xem bảng phân loại bên dưới).                                    |
| `message`     | string | Mô tả chi tiết nguyên nhân lỗi.                                                |
| `timestamp`   | string | Thời điểm lỗi xảy ra (ISO 8601, UTC).                                          |

**Phân loại mã lỗi (`error_type`):**

| Mã lỗi                | Mô tả                                                                  |
| :--------------------- | :---------------------------------------------------------------------- |
| `ORDER_REJECTED`       | Sàn từ chối lệnh (không đủ số dư, sai leverage, sai margin type).      |
| `UNIT_COST_EXCEEDED`   | Phiên logic vượt quá giới hạn Unit Cost cho phép (cơ chế chống treo).  |
| `API_CONNECTION_LOST`  | Mất kết nối đến API sàn Binance.                                       |
| `EXECUTION_ERROR`      | Lỗi nội bộ khi thực thi logic chiến lược.                              |
| `LIQUIDATION_ALERT`    | Vị thế bị sàn thanh lý (Liquidation) do thua lỗ vượt ngưỡng ký quỹ.   |

> **Nghiệp vụ hiển thị:** Frontend hiển thị thông báo dạng Toast/Notification màu đỏ khi nhận event `bot_error`. Đồng thời, dòng lỗi cũng được ghi vào Console Logs (kênh `bot_logs`) để người dùng truy vết.

---

## Phụ lục — Tổng hợp Kênh & Sự kiện

| Kênh (`channel`)       | Sự kiện (`event`)     | Hướng          | Mô tả ngắn gọn                                 |
| :---------------------- | :-------------------- | :------------- | :---------------------------------------------- |
| `market_ticker`         | `market_ticker`       | Server → Client | Giá ticker thời gian thực theo Symbol.           |
| `market_ticker`         | `market_candle`       | Server → Client | Nến OHLCV cập nhật liên tục / đóng nến.         |
| `bot_logs`              | `bot_log`             | Server → Client | Dòng log mới từ phiên logic Bot.                 |
| `position_update`       | `position_update`     | Server → Client | Vị thế, PnL, lệnh chờ của Bot đang chạy.        |
| `position_update`       | `bot_status_change`   | Server → Client | Bot thay đổi trạng thái (Running/Stopped/Error). |
| `position_update`       | `bot_error`           | Server → Client | Lỗi vận hành phát sinh khi Bot hoạt động.        |

---

## Phụ lục — Mapping với Database

| Kênh / Event          | Bảng Database liên quan     | Ghi chú                                                                 |
| :-------------------- | :-------------------------- | :----------------------------------------------------------------------- |
| `market_ticker`       | `candles_data`              | Nến đóng (`is_closed = true`) được INSERT vào DB ngay lập tức.          |
| `bot_log`             | `bot_logs`                  | Mỗi dòng log INSERT đồng thời vào DB và đẩy qua WebSocket.             |
| `position_update`     | `bot_instances`             | Cột `total_pnl`, `status` được cập nhật tương ứng.                      |
| `bot_status_change`   | `bot_instances`             | Cột `status` thay đổi (`Running`, `Stopped`, `Error`).                  |
| `bot_error`           | `bot_logs`                  | Lỗi cũng được ghi thành dòng log với `action_decision` phù hợp.        |
| Khớp lệnh (via sàn)  | `trade_history`             | Lệnh khớp thành công INSERT vào `trade_history` và đồng bộ qua `position_update`. |
