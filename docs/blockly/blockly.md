# Tài liệu Đặc tả Khối lệnh Tùy chỉnh (Custom Blocks) cho Google Blockly

**Dự án:** QuantFlow — Nền tảng Low-code hỗ trợ xây dựng và vận hành chiến lược giao dịch tiền mã hóa
**Phiên bản tài liệu:** 1.0
**Ngày cập nhật:** 02/03/2026
**Tài liệu tham chiếu:** SRS v1.0 — Phần 2.2 (FR-DESIGN-01 → FR-DESIGN-13)

---

## Mục lục

1. [Tổng quan](#1-tổng-quan)
   - 1.1. [Mục đích](#11-mục-đích)
   - 1.2. [Cơ chế Context-aware (Tự động nhận diện Current_Symbol)](#12-cơ-chế-context-aware-tự-động-nhận-diện-current_symbol)
   - 1.3. [Cơ chế giới hạn Unit Cost](#13-cơ-chế-giới-hạn-unit-cost)
   - 1.4. [Bảng tổng hợp Unit Cost](#14-bảng-tổng-hợp-unit-cost)
2. [Cấu trúc Toolbox](#2-cấu-trúc-toolbox)
3. [Đặc tả chi tiết Khối lệnh theo Toolbox Category](#3-đặc-tả-chi-tiết-khối-lệnh-theo-toolbox-category)
   - 3.1. [Nhóm Sự kiện (Event Trigger)](#31-nhóm-sự-kiện-event-trigger)
   - 3.2. [Nhóm Logic & Điều khiển](#32-nhóm-logic--điều-khiển)
   - 3.3. [Nhóm Toán học](#33-nhóm-toán-học)
   - 3.4. [Nhóm Biến (Session & Lifecycle)](#34-nhóm-biến-session--lifecycle)
   - 3.5. [Nhóm Chỉ báo Kỹ thuật](#35-nhóm-chỉ-báo-kỹ-thuật)
   - 3.6. [Nhóm Giao dịch](#36-nhóm-giao-dịch)

---

## 1. Tổng quan

### 1.1. Mục đích

Bộ thư viện khối lệnh tùy chỉnh (Custom Blocks) được thiết kế dành riêng cho nền tảng **QuantFlow**, nhằm cung cấp giao diện lập trình trực quan (kéo — thả) dựa trên thư viện **Google Blockly** (FR-DESIGN-01). Mục đích cốt lõi bao gồm:

- **Trực quan hóa logic giao dịch:** Cho phép người dùng không có nền tảng lập trình vẫn có thể thiết kế chiến lược giao dịch Futures phức tạp thông qua các khối lệnh kéo — thả.
- **Đảm bảo tính toàn vẹn logic:** Mỗi khối lệnh được thiết kế với ràng buộc đầu vào/đầu ra nghiêm ngặt (kiểu dữ liệu, kiểu kết nối), giúp ngăn chặn lỗi nối khối ngay từ giao diện.
- **Chuyển đổi sang mã thực thi:** Cấu trúc JSON của sơ đồ khối được Backend phân tích (parse) để sinh ra logic thực thi tương ứng bằng Golang, phục vụ cả Backtest (FR-RUN-02) lẫn Live Trade (FR-RUN-05).
- **Hỗ trợ Xuất/Nhập:** Người dùng có thể Lưu mẫu chiến lược vào Database (FR-DESIGN-11), Xuất ra tệp `.json` (FR-DESIGN-12), và Nhập lại từ tệp `.json` (FR-DESIGN-13).

### 1.2. Cơ chế Context-aware (Tự động nhận diện Current_Symbol)

Đây là nguyên tắc thiết kế cốt lõi của QuantFlow, được áp dụng trên toàn bộ các **Khối Chỉ báo Kỹ thuật** (FR-DESIGN-07), **Khối Dữ liệu Thị trường** (FR-DESIGN-08) và **Khối Giao dịch** (FR-DESIGN-09, FR-DESIGN-10):

> **Quy tắc:** Tất cả các khối lệnh liên quan đến dữ liệu thị trường hoặc hành động giao dịch **KHÔNG** yêu cầu người dùng nhập Symbol (cặp tiền). Thay vào đó, hệ thống tự động sử dụng biến ngữ cảnh `Current_Symbol` — được thiết lập khi khởi tạo Bot Instance (FR-RUN-05).

**Luồng hoạt động:**

```
┌─────────────────────────────────────────────────────────────┐
│ Người dùng tạo Bot Instance │
│ → Chọn Template (Mẫu chiến lược) │
│ → Chọn Symbol: BTCUSDT │
│ → Hệ thống tiêm: Current_Symbol = "BTCUSDT" │
├─────────────────────────────────────────────────────────────┤
│ Khi khối [RSI khung 15m chu kỳ 14] được thực thi: │
│ → Backend tự động lấy dữ liệu nến 15m của BTCUSDT │
│ → Tính RSI(14) và trả về kết quả │
├─────────────────────────────────────────────────────────────┤
│ Khi khối [Đặt lệnh Futures] được thực thi: │
│ → Backend tự động gửi lệnh cho cặp BTCUSDT │
└─────────────────────────────────────────────────────────────┘
```

**Lợi ích:**

- Giao diện gọn gàng, giảm tham số thừa, người dùng tập trung vào thiết kế logic.
- Cùng một mẫu chiến lược (Template) có thể được tái sử dụng cho nhiều cặp tiền khác nhau bằng cách tạo nhiều Bot Instance với các Symbol khác nhau.

### 1.3. Cơ chế giới hạn Unit Cost

Để bảo vệ hệ thống khỏi các lỗi logic do người dùng tạo ra (đặc biệt là vòng lặp vô tận), QuantFlow áp dụng hệ thống **Unit Cost** (FR-RUN-07):

> **Quy tắc:** Mỗi khối lệnh khi được thực thi sẽ tiêu tốn một lượng "Unit"nhất định. Tổng Unit sử dụng trong một **Session** (một lần kích hoạt sự kiện nến) bị giới hạn ở mức **tối đa N Unit** (mặc định: `1000 Unit`).

**Cơ chế hoạt động:**

1. Khi một Session bắt đầu (sự kiện nến mới), bộ đếm Unit được khởi tạo về `0`.
2. Mỗi khối lệnh được thực thi, bộ đếm tăng thêm giá trị Unit Cost tương ứng.
3. **Trước** khi thực thi mỗi khối, hệ thống kiểm tra: `Unit_đã_dùng + Unit_cost_khối_hiện_tại ≤ N`.
4. Nếu **vượt ngưỡng** → Cưỡng chế dừng Session ngay lập tức, ghi log cảnh báo `"UNIT_COST_EXCEEDED"`.

**Ví dụ ngăn chặn vòng lặp vô tận:**

```
Chiến lược lỗi: Lặp khi [Đúng] → Đặt lệnh (10 Unit/lần)
→ Vòng 1: Unit = 1 (while) + 10 (lệnh) = 11
→ Vòng 2: 11 + 1 + 10 = 22
→ ...
→ Vòng 83: Unit = 913 → Vẫn OK
→ Vòng 84: 913 + 1 + 10 = 924 → Vẫn OK
→ Vòng 91: 1001 → VƯỢT NGƯỠNG → Dừng Session → Ghi log lỗi
```

### 1.4. Bảng tổng hợp Unit Cost

| Nhóm khối          | Các khối lệnh                                                                       |  Unit Cost   | Giải thích                                     |
| ------------------ | ----------------------------------------------------------------------------------- | :----------: | ---------------------------------------------- |
| Sự kiện            | `event_on_candle`                                                                   |    **0**     | Điểm khởi đầu Session, không tính chi phí      |
| Logic & Điều khiển | `controls_if`, `controls_if_else`                                                   |    **1**     | Đánh giá điều kiện đơn giản trong RAM          |
| Logic & Điều khiển | `logic_compare`, `logic_operation`, `logic_negate`, `logic_boolean`                 |    **1**     | Phép toán logic cơ bản                         |
| Vòng lặp           | `controls_repeat`, `controls_while`                                                 | **1 / vòng** | Mỗi lượt lặp (iteration) tốn 1 Unit            |
| Toán học           | `math_number`, `math_arithmetic`, `math_round`, `math_random_int`                   |    **1**     | Phép tính số học đơn giản                      |
| Biến               | Tất cả khối `variables_*`                                                           |    **1**     | Đọc/ghi biến trong bộ nhớ                      |
| Chỉ báo Kỹ thuật   | `indicator_rsi`, `indicator_ema`                                                    |    **5**     | Cần truy vấn dữ liệu nến và tính toán phức tạp |
| Dữ liệu Thị trường | `data_market_price`, `data_position_info`, `data_open_orders_count`, `data_balance` |    **3**     | Cần gọi API sàn giao dịch                      |
| Đặt lệnh           | `trade_smart_order`                                                                 |    **10**    | Hành động giao dịch quan trọng, gọi nhiều API  |
| Quản lý lệnh       | `trade_close_position`, `trade_cancel_all_orders`                                   |    **10**    | Hành động giao dịch quan trọng                 |

> **Ghi chú:** Với giới hạn mặc định `1000 Unit/Session`, một chiến lược đơn giản (kiểm tra vài điều kiện rồi đặt/hủy lệnh) chỉ tiêu tốn khoảng `20–50 Unit` — hoàn toàn an toàn. Giới hạn này chủ yếu để chặn các cấu hình vòng lặp sai logic.

---

## 2. Cấu trúc Toolbox

Thanh công cụ (Toolbox) của Blockly Workspace được phân thành **6 nhóm chính** (FR-DESIGN-02), mỗi nhóm có màu sắc riêng biệt để người dùng dễ phân biệt:

|  #  | Tên nhóm (Category)            | Mã màu (Hue) | Màu hiển thị  | Mã yêu cầu           | Số lượng khối |
| :-: | ------------------------------ | :----------: | ------------- | -------------------- | :-----------: |
|  1  | **Sự kiện** (Event Trigger)    |     `30`     | Cam           | FR-DESIGN-03         |       1       |
|  2  | **Logic & Điều khiển**         |    `210`     | Xanh dương    | FR-DESIGN-05         |       8       |
|  3  | **Toán học**                   |    `230`     | Tím           | FR-DESIGN-06         |       4       |
|  4  | **Biến** (Session & Lifecycle) | `330` / `20` | Hồng / Đỏ cam | FR-DESIGN-04         |       4       |
|  5  | **Chỉ báo Kỹ thuật**           |    `160`     | Xanh lục      | FR-DESIGN-07         |       2       |
|  6  | **Giao dịch**                  | `190` / `0`  | Xanh lam / Đỏ | FR-DESIGN-08, 09, 10 |       7       |
|     |                                |              |               | **Tổng cộng**        |    **26**     |

> **Ghi chú về màu sắc nhóm Biến:** Biến Phiên (Session) sử dụng màu **Hồng (330)** và Biến Vòng đời (Lifecycle) sử dụng màu **Đỏ cam (20)** để người dùng dễ dàng phân biệt phạm vi (scope) ngay từ giao diện kéo — thả (FR-DESIGN-04).

> **Ghi chú về nhóm Giao dịch:** Nhóm này bao gồm 2 phân nhóm: Khối **Dữ liệu Thị trường & Tài khoản** (màu Xanh lam `190`) dùng để đọc thông tin, và Khối **Đặt lệnh & Quản lý lệnh** (màu Đỏ `0`) dùng để thực hiện hành động — màu đỏ cảnh báo rằng đây là khối tác động trực tiếp lên tài khoản thật.

---

## 3. Đặc tả chi tiết Khối lệnh theo Toolbox Category

---

### 3.1. Nhóm Sự kiện (Event Trigger)

**Mã yêu cầu liên quan:** FR-DESIGN-03
**Màu sắc:** Cam (Hue: `30`)
**Vai trò:** Định nghĩa thời điểm kích hoạt một phiên xử lý (Session). Đây là khối **bắt đầu bắt buộc** — mọi chiến lược phải có ít nhất một khối Sự kiện.

---

#### 3.1.1. `event_on_candle`

| Thuộc tính              | Chi tiết                                                                                                                                                                                                                                                                                                        |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `event_on_candle`                                                                                                                                                                                                                                                                                               |
| **Mục đích**            | Khối khởi đầu (Start Block) của một chiến lược. Khi một cây nến mới đóng hoặc mở trên khung thời gian đã chọn, toàn bộ logic phía dưới khối này sẽ được thực thi đúng **1 lần** (tạo thành 1 Session).                                                                                                          |
| **Đầu vào (Inputs)**    | **1.** `TRIGGER` — Dropdown: Kiểu sự kiện kích hoạt. Lựa chọn: `Khi đóng nến` / `Khi mở nến`.                                                                                                                                                                                                                   |
|                         | **2.** `TIMEFRAME` — Dropdown: Khung thời gian nến. Lựa chọn: `1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `4h`, `1d`.                                                                                                                                                                                                 |
| **Đầu ra (Output)**     | Không có Output (giá trị). Chỉ có **Next Statement** (kết nối khối phía dưới). **Không có Previous Statement** — khối này luôn nằm ở vị trí trên cùng.                                                                                                                                                          |
| **Lưu ý nghiệp vụ**     | Backend đăng ký lắng nghe WebSocket Binance cho Symbol và Timeframe tương ứng. Khi sự kiện xảy ra, engine thực thi toàn bộ chuỗi khối bên dưới. Thời điểm kích hoạt của `Khi đóng nến` là khi cây nến hoàn thành (thích hợp cho phần lớn chiến lược); `Khi mở nến` kích hoạt tại thời điểm cây nến mới bắt đầu. |

**JSON Definition:**

```json
{
  "type": "event_on_candle",

  "message0": "Khi %1 khung %2",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "TRIGGER",
      "options": [
        ["đóng nến", "ON_CANDLE_CLOSE"],
        ["mở nến", "ON_CANDLE_OPEN"]
      ]
    },
    {
      "type": "field_dropdown",
      "name": "TIMEFRAME",
      "options": [
        ["1 phút", "1m"],
        ["5 phút", "5m"],
        ["15 phút", "15m"],
        ["30 phút", "30m"],
        ["1 giờ", "1h"],
        ["4 giờ", "4h"],
        ["1 ngày", "1d"],
        ["1 tuần", "1w"]
      ]
    }
  ],

  "nextStatement": null,

  "colour": 30,
  "tooltip": "Khối sự kiện bắt đầu. Mỗi khi nến đóng/mở theo khung thời gian đã chọn, các khối bên dưới sẽ được thực thi 1 lần (1 Session).",
  "helpUrl": ""
}
```

---

### 3.2. Nhóm Logic & Điều khiển

**Mã yêu cầu liên quan:** FR-DESIGN-05
**Màu sắc:** Xanh dương (Hue: `210`)
**Vai trò:** Cung cấp các cấu trúc điều kiện, phép toán logic và vòng lặp để điều hướng luồng xử lý chiến lược.

---

#### 3.2.1. `controls_if`

| Thuộc tính              | Chi tiết                                                                             |
| ----------------------- | ------------------------------------------------------------------------------------ |
| **Tên khối (Block ID)** | `controls_if`                                                                        |
| **Mục đích**            | Kiểm tra một điều kiện. Nếu điều kiện đúng, thực thi chuỗi khối bên trong.           |
| **Đầu vào (Inputs)**    | **1.** `CONDITION` — Value Input (kiểu `Boolean`): Biểu thức điều kiện cần kiểm tra. |
|                         | **2.** `DO` — Statement Input: Chuỗi khối lệnh sẽ thực thi nếu điều kiện đúng.       |
| **Đầu ra (Output)**     | Previous Statement + Next Statement (khối lệnh hành động, nối chuỗi).                |

**JSON Definition:**

```json
{
  "type": "controls_if",

  "message0": "Nếu %1",
  "args0": [
    {
      "type": "input_value",
      "name": "CONDITION",
      "check": "Boolean"
    }
  ],

  "message1": "thì %1",
  "args1": [
    {
      "type": "input_statement",
      "name": "DO"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 210,
  "tooltip": "Kiểm tra điều kiện. Nếu đúng (True), thực thi các khối lệnh bên trong.",
  "helpUrl": ""
}
```

---

#### 3.2.2. `controls_if_else`

| Thuộc tính              | Chi tiết                                                                                           |
| ----------------------- | -------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `controls_if_else`                                                                                 |
| **Mục đích**            | Kiểm tra điều kiện với 2 nhánh: nhánh `thì` (điều kiện đúng) và nhánh `ngược lại` (điều kiện sai). |
| **Đầu vào (Inputs)**    | **1.** `CONDITION` — Value Input (kiểu `Boolean`): Biểu thức điều kiện.                            |
|                         | **2.** `DO` — Statement Input: Khối lệnh khi điều kiện đúng.                                       |
|                         | **3.** `ELSE` — Statement Input: Khối lệnh khi điều kiện sai.                                      |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                               |

**JSON Definition:**

```json
{
  "type": "controls_if_else",

  "message0": "Nếu %1",
  "args0": [
    {
      "type": "input_value",
      "name": "CONDITION",
      "check": "Boolean"
    }
  ],

  "message1": "thì %1",
  "args1": [
    {
      "type": "input_statement",
      "name": "DO"
    }
  ],

  "message2": "ngược lại %1",
  "args2": [
    {
      "type": "input_statement",
      "name": "ELSE"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 210,
  "tooltip": "Kiểm tra điều kiện. Nếu đúng → thực thi nhánh 'thì'. Nếu sai → thực thi nhánh 'ngược lại'.",
  "helpUrl": ""
}
```

---

#### 3.2.3. `logic_compare`

| Thuộc tính              | Chi tiết                                                                      |
| ----------------------- | ----------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `logic_compare`                                                               |
| **Mục đích**            | So sánh hai giá trị số học và trả về kết quả Boolean.                         |
| **Đầu vào (Inputs)**    | **1.** `A` — Value Input: Toán hạng bên trái.                                 |
|                         | **2.** `OP` — Dropdown: Phép so sánh. Lựa chọn: `=`, `≠`, `>`, `≥`, `<`, `≤`. |
|                         | **3.** `B` — Value Input: Toán hạng bên phải.                                 |
| **Đầu ra (Output)**     | `Boolean` — Kết quả so sánh (Đúng / Sai).                                     |

**JSON Definition:**

```json
{
  "type": "logic_compare",

  "message0": "%1 %2 %3",
  "args0": [
    {
      "type": "input_value",
      "name": "A"
    },
    {
      "type": "field_dropdown",
      "name": "OP",
      "options": [
        ["=", "EQ"],
        ["≠", "NEQ"],
        [">", "GT"],
        ["≥", "GTE"],
        ["<", "LT"],
        ["≤", "LTE"]
      ]
    },
    {
      "type": "input_value",
      "name": "B"
    }
  ],

  "inputsInline": true,
  "output": "Boolean",
  "colour": 210,
  "tooltip": "So sánh hai giá trị. Trả về Đúng (True) hoặc Sai (False).",
  "helpUrl": ""
}
```

---

#### 3.2.4. `logic_operation`

| Thuộc tính              | Chi tiết                                                                |
| ----------------------- | ----------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `logic_operation`                                                       |
| **Mục đích**            | Kết hợp hai biểu thức Boolean bằng phép VÀ (AND) hoặc HOẶC (OR).        |
| **Đầu vào (Inputs)**    | **1.** `A` — Value Input (kiểu `Boolean`): Biểu thức trái.              |
|                         | **2.** `OP` — Dropdown: Phép logic. Lựa chọn: `VÀ` (AND) / `HOẶC` (OR). |
|                         | **3.** `B` — Value Input (kiểu `Boolean`): Biểu thức phải.              |
| **Đầu ra (Output)**     | `Boolean`.                                                              |

**JSON Definition:**

```json
{
  "type": "logic_operation",

  "message0": "%1 %2 %3",
  "args0": [
    {
      "type": "input_value",
      "name": "A",
      "check": "Boolean"
    },
    {
      "type": "field_dropdown",
      "name": "OP",
      "options": [
        ["VÀ", "AND"],
        ["HOẶC", "OR"]
      ]
    },
    {
      "type": "input_value",
      "name": "B",
      "check": "Boolean"
    }
  ],

  "inputsInline": true,
  "output": "Boolean",
  "colour": 210,
  "tooltip": "Kết hợp hai điều kiện. VÀ: cả hai phải đúng. HOẶC: ít nhất một đúng.",
  "helpUrl": ""
}
```

---

#### 3.2.5. `logic_negate`

| Thuộc tính              | Chi tiết                                                               |
| ----------------------- | ---------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `logic_negate`                                                         |
| **Mục đích**            | Đảo ngược giá trị Boolean (Đúng → Sai, Sai → Đúng).                    |
| **Đầu vào (Inputs)**    | **1.** `BOOL` — Value Input (kiểu `Boolean`): Biểu thức cần đảo ngược. |
| **Đầu ra (Output)**     | `Boolean`.                                                             |

**JSON Definition:**

```json
{
  "type": "logic_negate",

  "message0": "KHÔNG %1",
  "args0": [
    {
      "type": "input_value",
      "name": "BOOL",
      "check": "Boolean"
    }
  ],

  "output": "Boolean",
  "colour": 210,
  "tooltip": "Đảo ngược giá trị logic. Đúng thành Sai, Sai thành Đúng.",
  "helpUrl": ""
}
```

---

#### 3.2.6. `logic_boolean`

| Thuộc tính              | Chi tiết                                              |
| ----------------------- | ----------------------------------------------------- |
| **Tên khối (Block ID)** | `logic_boolean`                                       |
| **Mục đích**            | Trả về hằng số Boolean: Đúng (True) hoặc Sai (False). |
| **Đầu vào (Inputs)**    | **1.** `BOOL` — Dropdown: `Đúng` / `Sai`.             |
| **Đầu ra (Output)**     | `Boolean`.                                            |

**JSON Definition:**

```json
{
  "type": "logic_boolean",

  "message0": "%1",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "BOOL",
      "options": [
        ["Đúng", "TRUE"],
        ["Sai", "FALSE"]
      ]
    }
  ],

  "output": "Boolean",
  "colour": 210,
  "tooltip": "Trả về giá trị logic cố định: Đúng (True) hoặc Sai (False).",
  "helpUrl": ""
}
```

---

#### 3.2.7. `controls_repeat`

| Thuộc tính              | Chi tiết                                                                                                                              |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `controls_repeat`                                                                                                                     |
| **Mục đích**            | Lặp lại chuỗi khối lệnh bên trong một số lần xác định.                                                                                |
| **Đầu vào (Inputs)**    | **1.** `TIMES` — Value Input (kiểu `Number`): Số lần lặp.                                                                             |
|                         | **2.** `DO` — Statement Input: Chuỗi khối lệnh thực thi trong mỗi vòng lặp.                                                           |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                                                                  |
| **Lưu ý nghiệp vụ**     | Mỗi vòng lặp tốn **1 Unit**. Với giới hạn 1000 Unit/Session, số lần lặp tối đa thực tế phụ thuộc vào tổng chi phí các khối bên trong. |

**JSON Definition:**

```json
{
  "type": "controls_repeat",

  "message0": "Lặp lại %1 lần",
  "args0": [
    {
      "type": "input_value",
      "name": "TIMES",
      "check": "Number"
    }
  ],

  "message1": "thực hiện %1",
  "args1": [
    {
      "type": "input_statement",
      "name": "DO"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 210,
  "tooltip": "Lặp lại các khối bên trong N lần. Mỗi vòng lặp tốn 1 Unit Cost. Chú ý: Tổng Unit bị giới hạn mỗi Session.",
  "helpUrl": ""
}
```

---

#### 3.2.8. `controls_while`

| Thuộc tính              | Chi tiết                                                                                                                                                   |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `controls_while`                                                                                                                                           |
| **Mục đích**            | Lặp lại chuỗi khối lệnh bên trong **cho đến khi** điều kiện trở thành Sai.                                                                                 |
| **Đầu vào (Inputs)**    | **1.** `CONDITION` — Value Input (kiểu `Boolean`): Điều kiện kiểm tra trước mỗi vòng lặp.                                                                  |
|                         | **2.** `DO` — Statement Input: Chuỗi khối lệnh thực thi nếu điều kiện đúng.                                                                                |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                                                                                       |
| **Lưu ý nghiệp vụ**     | **Cảnh báo:** Nếu điều kiện luôn đúng (VD: `Đúng`), vòng lặp sẽ chạy cho đến khi Unit Cost vượt ngưỡng. Đây chính là lý do cơ chế Unit Cost được thiết kế. |

**JSON Definition:**

```json
{
  "type": "controls_while",

  "message0": "Lặp khi %1",
  "args0": [
    {
      "type": "input_value",
      "name": "CONDITION",
      "check": "Boolean"
    }
  ],

  "message1": "thực hiện %1",
  "args1": [
    {
      "type": "input_statement",
      "name": "DO"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 210,
  "tooltip": "Lặp lại các khối bên trong khi điều kiện còn đúng. Mỗi vòng tốn 1 Unit. Hệ thống sẽ tự dừng nếu vượt giới hạn Unit Cost.",
  "helpUrl": ""
}
```

---

### 3.3. Nhóm Toán học

**Mã yêu cầu liên quan:** FR-DESIGN-06
**Màu sắc:** Tím (Hue: `230`)
**Vai trò:** Cung cấp các phép tính số học cơ bản và nâng cao phục vụ tính toán giá, khối lượng, tỷ lệ trong chiến lược giao dịch. Tất cả hoạt động trên kiểu dữ liệu số thực/nguyên.

---

#### 3.3.1. `math_number`

| Thuộc tính              | Chi tiết                                                                                                |
| ----------------------- | ------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `math_number`                                                                                           |
| **Mục đích**            | Cung cấp một hằng số (giá trị số cố định). Dùng làm đầu vào cho các khối tính toán, so sánh, chỉ báo... |
| **Đầu vào (Inputs)**    | **1.** `NUM` — Field Number: Giá trị số do người dùng nhập trực tiếp.                                   |
| **Đầu ra (Output)**     | `Number`.                                                                                               |

**JSON Definition:**

```json
{
  "type": "math_number",

  "message0": "%1",
  "args0": [
    {
      "type": "field_number",
      "name": "NUM",
      "value": 0
    }
  ],

  "output": "Number",
  "colour": 230,
  "tooltip": "Một giá trị số cố định. Dùng để nhập giá, khối lượng, chu kỳ chỉ báo, v.v.",
  "helpUrl": ""
}
```

---

#### 3.3.2. `math_arithmetic`

| Thuộc tính              | Chi tiết                                                                                                                                                           |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tên khối (Block ID)** | `math_arithmetic`                                                                                                                                                  |
| **Mục đích**            | Thực hiện phép tính số học cơ bản giữa hai toán hạng: cộng, trừ, nhân, chia.                                                                                       |
| **Đầu vào (Inputs)**    | **1.** `A` — Value Input (kiểu `Number`): Toán hạng bên trái.                                                                                                      |
|                         | **2.** `OP` — Dropdown: Phép toán. Lựa chọn: `+`, `−`, `×`, `÷`.                                                                                                   |
|                         | **3.** `B` — Value Input (kiểu `Number`): Toán hạng bên phải.                                                                                                      |
| **Đầu ra (Output)**     | `Number` — Kết quả phép tính.                                                                                                                                      |
| **Lưu ý nghiệp vụ**     | Backend sử dụng BigInt/Decimal để đảm bảo độ chính xác cao khi tính toán giá tiền mã hóa (tránh lỗi floating-point). Phép chia cho 0 trả về 0 và ghi log cảnh báo. |

**JSON Definition:**

```json
{
  "type": "math_arithmetic",

  "message0": "%1 %2 %3",
  "args0": [
    {
      "type": "input_value",
      "name": "A",
      "check": "Number"
    },
    {
      "type": "field_dropdown",
      "name": "OP",
      "options": [
        ["+", "ADD"],
        ["−", "MINUS"],
        ["×", "MULTIPLY"],
        ["÷", "DIVIDE"]
      ]
    },
    {
      "type": "input_value",
      "name": "B",
      "check": "Number"
    }
  ],

  "inputsInline": true,
  "output": "Number",
  "colour": 230,
  "tooltip": "Thực hiện phép tính số học (+, −, ×, ÷) giữa hai giá trị số.",
  "helpUrl": ""
}
```

---

#### 3.3.3. `math_round`

| Thuộc tính              | Chi tiết                                                                                                                                           |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `math_round`                                                                                                                                       |
| **Mục đích**            | Làm tròn một giá trị số đến số chữ số thập phân mong muốn. Rất quan trọng khi tính toán khối lượng lệnh (Quantity phải tuân thủ stepSize của sàn). |
| **Đầu vào (Inputs)**    | **1.** `NUM` — Value Input (kiểu `Number`): Giá trị cần làm tròn.                                                                                  |
|                         | **2.** `DECIMALS` — Field Number: Số chữ số thập phân (0–18, mặc định: 2).                                                                         |
| **Đầu ra (Output)**     | `Number` — Giá trị sau khi làm tròn.                                                                                                               |

**JSON Definition:**

```json
{
  "type": "math_round",

  "message0": "Làm tròn %1 đến %2 chữ số thập phân",
  "args0": [
    {
      "type": "input_value",
      "name": "NUM",
      "check": "Number"
    },
    {
      "type": "field_number",
      "name": "DECIMALS",
      "value": 2,
      "min": 0,
      "max": 18,
      "precision": 1
    }
  ],

  "output": "Number",
  "colour": 230,
  "tooltip": "Làm tròn giá trị theo số chữ số thập phân. VD: Làm tròn 3.14159 đến 2 → 3.14.",
  "helpUrl": ""
}
```

---

#### 3.3.4. `math_random_int`

| Thuộc tính              | Chi tiết                                                                                                                                         |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tên khối (Block ID)** | `math_random_int`                                                                                                                                |
| **Mục đích**            | Sinh một số nguyên ngẫu nhiên trong khoảng [từ, đến]. Có thể dùng để tạo yếu tố ngẫu nhiên trong chiến lược (VD: chỉ giao dịch nếu random > 50). |
| **Đầu vào (Inputs)**    | **1.** `FROM` — Value Input (kiểu `Number`): Giá trị nhỏ nhất.                                                                                   |
|                         | **2.** `TO` — Value Input (kiểu `Number`): Giá trị lớn nhất.                                                                                     |
| **Đầu ra (Output)**     | `Number`.                                                                                                                                        |

**JSON Definition:**

```json
{
  "type": "math_random_int",

  "message0": "Ngẫu nhiên từ %1 đến %2",
  "args0": [
    {
      "type": "input_value",
      "name": "FROM",
      "check": "Number"
    },
    {
      "type": "input_value",
      "name": "TO",
      "check": "Number"
    }
  ],

  "inputsInline": true,
  "output": "Number",
  "colour": 230,
  "tooltip": "Sinh một số nguyên ngẫu nhiên trong khoảng [từ] đến [đến] (bao gồm cả hai đầu).",
  "helpUrl": ""
}
```

---

### 3.4. Nhóm Biến (Session & Lifecycle)

**Mã yêu cầu liên quan:** FR-DESIGN-04
**Vai trò:** Cung cấp khả năng lưu trữ và truy xuất dữ liệu tạm thời hoặc vĩnh viễn trong quá trình Bot hoạt động.

QuantFlow hỗ trợ **2 loại biến** với phạm vi (scope) khác nhau, phân biệt rõ ràng bằng **màu sắc** và **biểu tượng**:

| Loại biến                              | Phạm vi (Scope)      | Vòng đời                     | Màu sắc (Hue) | Biểu tượng | Lưu trữ Backend                      |
| -------------------------------------- | -------------------- | ---------------------------- | :-----------: | ---------- | ------------------------------------ |
| **Biến Phiên** (Session Variable)      | 1 Session duy nhất   | Tự hủy khi Session kết thúc  | `330` (Hồng)  |            | RAM (biến cục bộ)                    |
| **Biến Vòng đời** (Lifecycle Variable) | Toàn bộ vòng đời Bot | Giữ giá trị giữa các Session | `20` (Đỏ cam) |            | Database (`bot_lifecycle_variables`) |

> **Ví dụ sử dụng:**
>
> - **Biến Phiên:** Lưu tạm `rsi_hiện_tại` để dùng nhiều lần trong cùng 1 Session → tránh gọi API tính RSI lặp lại.
> - **Biến Vòng đời:** Lưu `đã_vào_lệnh` để theo dõi trạng thái giao dịch xuyên suốt nhiều Session → Bot biết mình đang giữ lệnh hay chưa.

---

#### 3.4.1. `variables_session_set`

| Thuộc tính              | Chi tiết                                                                                                    |
| ----------------------- | ----------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `variables_session_set`                                                                                     |
| **Mục đích**            | Gán giá trị cho một Biến Phiên. Biến này chỉ tồn tại trong Session hiện tại và tự hủy khi Session kết thúc. |
| **Đầu vào (Inputs)**    | **1.** `VAR_NAME` — Field Input (Chuỗi): Tên biến (người dùng tự đặt).                                      |
|                         | **2.** `VALUE` — Value Input (Mọi kiểu): Giá trị cần gán.                                                   |
| **Đầu ra (Output)**     | Previous Statement + Next Statement (khối hành động).                                                       |

**JSON Definition:**

```json
{
  "type": "variables_session_set",

  "message0": "Gán biến phiên %1 = %2",
  "args0": [
    {
      "type": "field_input",
      "name": "VAR_NAME",
      "text": "tên_biến"
    },
    {
      "type": "input_value",
      "name": "VALUE"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 330,
  "tooltip": "Gán giá trị cho Biến Phiên. Biến này chỉ tồn tại trong phiên xử lý (Session) hiện tại và tự động bị xóa khi Session kết thúc.",
  "helpUrl": ""
}
```

---

#### 3.4.2. `variables_session_get`

| Thuộc tính              | Chi tiết                                                                                      |
| ----------------------- | --------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `variables_session_get`                                                                       |
| **Mục đích**            | Đọc giá trị của một Biến Phiên đã được gán trước đó trong cùng Session.                       |
| **Đầu vào (Inputs)**    | **1.** `VAR_NAME` — Field Input (Chuỗi): Tên biến cần đọc.                                    |
| **Đầu ra (Output)**     | Mọi kiểu (output không giới hạn kiểu) — giá trị của biến. Nếu biến chưa được gán, trả về `0`. |

**JSON Definition:**

```json
{
  "type": "variables_session_get",

  "message0": "Biến phiên %1",
  "args0": [
    {
      "type": "field_input",
      "name": "VAR_NAME",
      "text": "tên_biến"
    }
  ],

  "output": null,
  "colour": 330,
  "tooltip": "Đọc giá trị của Biến Phiên. Trả về 0 nếu biến chưa được gán trong Session này.",
  "helpUrl": ""
}
```

---

#### 3.4.3. `variables_lifecycle_set`

| Thuộc tính              | Chi tiết                                                                                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `variables_lifecycle_set`                                                                                                                                             |
| **Mục đích**            | Gán giá trị cho một Biến Vòng đời. Biến này được lưu vĩnh viễn trong bộ nhớ Bot và **giữ nguyên giá trị** giữa các Session (từ lần kích hoạt này sang lần tiếp theo). |
| **Đầu vào (Inputs)**    | **1.** `VAR_NAME` — Field Input (Chuỗi): Tên biến.                                                                                                                    |
|                         | **2.** `VALUE` — Value Input (Mọi kiểu): Giá trị cần gán.                                                                                                             |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                                                                                                  |
| **Lưu ý nghiệp vụ**     | Backend lưu giá trị vào bảng `bot_lifecycle_variables` (JSONB) trong PostgreSQL, đảm bảo dữ liệu không bị mất khi Server restart (NFR-REL-04).                        |

**JSON Definition:**

```json
{
  "type": "variables_lifecycle_set",

  "message0": "Gán biến vòng đời %1 = %2",
  "args0": [
    {
      "type": "field_input",
      "name": "VAR_NAME",
      "text": "tên_biến"
    },
    {
      "type": "input_value",
      "name": "VALUE"
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 20,
  "tooltip": "Gán giá trị cho Biến Vòng đời. Giá trị được lưu vĩnh viễn trong Database, giữ nguyên giữa các Session và cả khi Server restart.",
  "helpUrl": ""
}
```

---

#### 3.4.4. `variables_lifecycle_get`

| Thuộc tính              | Chi tiết                                                                       |
| ----------------------- | ------------------------------------------------------------------------------ |
| **Tên khối (Block ID)** | `variables_lifecycle_get`                                                      |
| **Mục đích**            | Đọc giá trị của một Biến Vòng đời.                                             |
| **Đầu vào (Inputs)**    | **1.** `VAR_NAME` — Field Input (Chuỗi): Tên biến cần đọc.                     |
| **Đầu ra (Output)**     | Mọi kiểu — giá trị hiện tại của biến. Nếu biến chưa từng được gán, trả về `0`. |

**JSON Definition:**

```json
{
  "type": "variables_lifecycle_get",

  "message0": "Biến vòng đời %1",
  "args0": [
    {
      "type": "field_input",
      "name": "VAR_NAME",
      "text": "tên_biến"
    }
  ],

  "output": null,
  "colour": 20,
  "tooltip": "Đọc giá trị của Biến Vòng đời. Giá trị được duy trì xuyên suốt các Session. Trả về 0 nếu chưa được gán.",
  "helpUrl": ""
}
```

---

### 3.5. Nhóm Chỉ báo Kỹ thuật

**Mã yêu cầu liên quan:** FR-DESIGN-07
**Màu sắc:** Xanh lục (Hue: `160`)
**Vai trò:** Tính toán các chỉ báo kỹ thuật (Technical Indicators) phổ biến, phục vụ phân tích xu hướng và tạo tín hiệu giao dịch.

> **Nguyên tắc Context-aware (Rất quan trọng):**
> Tất cả các khối trong nhóm này chỉ yêu cầu **2 tham số**: `Khung thời gian (Timeframe)` và `Chu kỳ (Period)`.
> **KHÔNG có tham số chọn Symbol** — hệ thống tự động sử dụng `Current_Symbol` từ ngữ cảnh Bot Instance.
> Backend tự động truy vấn dữ liệu nến của `Current_Symbol` trên `Timeframe` đã chọn để tính toán.

---

#### 3.5.1. `indicator_rsi`

| Thuộc tính              | Chi tiết                                                                                                                                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `indicator_rsi`                                                                                                                                                                                                       |
| **Mục đích**            | Tính chỉ báo **RSI** (Relative Strength Index — Chỉ số sức mạnh tương đối) cho cặp tiền hiện tại. RSI dao động từ 0 đến 100; thường dùng: RSI > 70 (quá mua), RSI < 30 (quá bán).                                     |
| **Đầu vào (Inputs)**    | **1.** `TIMEFRAME` — Dropdown: Khung thời gian nến để tính RSI. Lựa chọn: `1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `4h`, `1d`.                                                                                           |
|                         | **2.** `PERIOD` — Value Input (kiểu `Number`): Chu kỳ tính RSI (thông dụng: 14).                                                                                                                                      |
| **Đầu ra (Output)**     | `Number` — Giá trị RSI (0–100).                                                                                                                                                                                       |
| **Lưu ý nghiệp vụ**     | Khung thời gian của khối Chỉ báo **có thể khác** với khung thời gian của Khối Sự kiện. VD: Bot kích hoạt trên nến 1m nhưng tính RSI trên khung 15m. Backend cần dữ liệu ít nhất `Period + 1` cây nến lịch sử để tính. |

**JSON Definition:**

```json
{
  "type": "indicator_rsi",

  "message0": "RSI khung %1 chu kỳ %2",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "TIMEFRAME",
      "options": [
        ["1 phút", "1m"],
        ["3 phút", "3m"],
        ["5 phút", "5m"],
        ["15 phút", "15m"],
        ["30 phút", "30m"],
        ["1 giờ", "1h"],
        ["4 giờ", "4h"],
        ["1 ngày", "1d"]
      ]
    },
    {
      "type": "input_value",
      "name": "PERIOD",
      "check": "Number"
    }
  ],

  "output": "Number",
  "colour": 160,
  "tooltip": "Tính chỉ báo RSI cho cặp tiền hiện tại (Current_Symbol). RSI > 70: vùng quá mua. RSI < 30: vùng quá bán. Không cần chọn Symbol.",
  "helpUrl": ""
}
```

---

#### 3.5.2. `indicator_ema`

| Thuộc tính              | Chi tiết                                                                                                                                                                                        |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `indicator_ema`                                                                                                                                                                                 |
| **Mục đích**            | Tính chỉ báo **EMA** (Exponential Moving Average — Đường trung bình động hàm mũ) cho cặp tiền hiện tại. EMA phản ứng nhanh hơn SMA với biến động giá gần đây, thường dùng để xác định xu hướng. |
| **Đầu vào (Inputs)**    | **1.** `TIMEFRAME` — Dropdown: Khung thời gian nến. Lựa chọn: `1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `4h`, `1d`.                                                                                 |
|                         | **2.** `PERIOD` — Value Input (kiểu `Number`): Chu kỳ tính EMA (thông dụng: 9, 20, 50, 200).                                                                                                    |
| **Đầu ra (Output)**     | `Number` — Giá trị EMA (đơn vị: giá tiền).                                                                                                                                                      |
| **Lưu ý nghiệp vụ**     | Tương tự RSI — không cần chọn Symbol. EMA thường dùng để so sánh chéo: VD `EMA(9) > EMA(20)` → tín hiệu mua.                                                                                    |

**JSON Definition:**

```json
{
  "type": "indicator_ema",

  "message0": "EMA khung %1 chu kỳ %2",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "TIMEFRAME",
      "options": [
        ["1 phút", "1m"],
        ["3 phút", "3m"],
        ["5 phút", "5m"],
        ["15 phút", "15m"],
        ["30 phút", "30m"],
        ["1 giờ", "1h"],
        ["4 giờ", "4h"],
        ["1 ngày", "1d"]
      ]
    },
    {
      "type": "input_value",
      "name": "PERIOD",
      "check": "Number"
    }
  ],

  "output": "Number",
  "colour": 160,
  "tooltip": "Tính chỉ báo EMA cho cặp tiền hiện tại (Current_Symbol). Thường dùng so sánh chéo: EMA ngắn > EMA dài → xu hướng tăng. Không cần chọn Symbol.",
  "helpUrl": ""
}
```

---

### 3.6. Nhóm Giao dịch

**Mã yêu cầu liên quan:** FR-DESIGN-08, FR-DESIGN-09, FR-DESIGN-10
**Vai trò:** Bao gồm các khối truy xuất dữ liệu thị trường/tài khoản và các khối thực thi hành động giao dịch. Tất cả đều hoạt động theo nguyên tắc **Context-aware** (`Current_Symbol`).

Nhóm này được chia thành **2 phân nhóm**:

| Phân nhóm                          |  Màu sắc (Hue)   | Vai trò                                                | Kiểu khối                  |
| ---------------------------------- | :--------------: | ------------------------------------------------------ | -------------------------- |
| **Dữ liệu Thị trường & Tài khoản** | `190` (Xanh lam) | Đọc thông tin (chỉ đọc, không tác động)                | Khối giá trị (Output)      |
| **Đặt lệnh & Quản lý lệnh**        |     `0` (Đỏ)     | Thực thi hành động giao dịch (tác động tài khoản thật) | Khối hành động (Statement) |

> Các khối màu **Đỏ** tác động trực tiếp lên tài khoản sàn giao dịch thật. Màu đỏ đóng vai trò **cảnh báo trực quan** cho người dùng.

---

#### 3.6.1. `data_market_price` — Dữ liệu Thị trường

| Thuộc tính              | Chi tiết                                                                                                                                   |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tên khối (Block ID)** | `data_market_price`                                                                                                                        |
| **Mục đích**            | Lấy giá thị trường hiện tại hoặc giá đóng nến gần nhất của cặp tiền hiện tại (FR-DESIGN-08).                                               |
| **Đầu vào (Inputs)**    | **1.** `PRICE_TYPE` — Dropdown: Loại giá. Lựa chọn: `Giá hiện tại` (Last Price từ ticker) / `Giá đóng nến` (Close Price của nến gần nhất). |
| **Đầu ra (Output)**     | `Number` — Giá trị giá (đơn vị: USDT).                                                                                                     |

**JSON Definition:**

```json
{
  "type": "data_market_price",

  "message0": "Giá %1",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "PRICE_TYPE",
      "options": [
        ["hiện tại", "LAST_PRICE"],
        ["đóng nến", "CLOSE_PRICE"]
      ]
    }
  ],

  "output": "Number",
  "colour": 190,
  "tooltip": "Lấy giá của cặp tiền hiện tại (Current_Symbol). 'Giá hiện tại': ticker realtime. 'Giá đóng nến': giá close của nến cuối cùng đã hoàn thành.",
  "helpUrl": ""
}
```

---

#### 3.6.2. `data_position_info` — Dữ liệu Thị trường

| Thuộc tính              | Chi tiết                                                                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `data_position_info`                                                                                                         |
| **Mục đích**            | Lấy thông tin vị thế đang mở của cặp tiền hiện tại: kích thước, lãi/lỗ chưa chốt, hoặc giá vào lệnh (FR-DESIGN-08).          |
| **Đầu vào (Inputs)**    | **1.** `FIELD` — Dropdown: Thông tin cần lấy. Lựa chọn: `Kích thước (Size)` / `Lãi/Lỗ (PnL)` / `Giá vào lệnh (Entry Price)`. |
| **Đầu ra (Output)**     | `Number` — Giá trị của trường được chọn. Nếu không có vị thế, trả về `0`.                                                    |

**JSON Definition:**

```json
{
  "type": "data_position_info",

  "message0": "Vị thế — %1",
  "args0": [
    {
      "type": "field_dropdown",
      "name": "FIELD",
      "options": [
        ["Kích thước (Size)", "POSITION_SIZE"],
        ["Lãi/Lỗ chưa chốt (PnL)", "UNREALIZED_PNL"],
        ["Giá vào lệnh (Entry Price)", "ENTRY_PRICE"]
      ]
    }
  ],

  "output": "Number",
  "colour": 190,
  "tooltip": "Lấy thông tin vị thế đang mở cho cặp tiền hiện tại. Trả về 0 nếu không có vị thế nào.",
  "helpUrl": ""
}
```

---

#### 3.6.3. `data_open_orders_count` — Dữ liệu Thị trường

| Thuộc tính              | Chi tiết                                                                                                  |
| ----------------------- | --------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `data_open_orders_count`                                                                                  |
| **Mục đích**            | Lấy số lượng lệnh chờ (Pending Orders) đang mở của cặp tiền hiện tại (FR-DESIGN-08).                      |
| **Đầu vào (Inputs)**    | Không có (tự động lấy theo `Current_Symbol`).                                                             |
| **Đầu ra (Output)**     | `Number` — Số lượng lệnh chờ đang mở (0 nếu không có).                                                    |
| **Lưu ý nghiệp vụ**     | Hữu ích để kiểm tra trước khi đặt lệnh: VD `Nếu [Số lệnh chờ = 0] thì [Đặt lệnh]` → tránh đặt trùng lệnh. |

**JSON Definition:**

```json
{
  "type": "data_open_orders_count",

  "message0": "Số lệnh chờ đang mở",

  "output": "Number",
  "colour": 190,
  "tooltip": "Trả về số lượng lệnh chờ (Pending/Open Orders) cho cặp tiền hiện tại. Dùng để kiểm tra trước khi đặt lệnh mới.",
  "helpUrl": ""
}
```

---

#### 3.6.4. `data_balance` — Dữ liệu Thị trường

| Thuộc tính              | Chi tiết                                                                                                                                                                         |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `data_balance`                                                                                                                                                                   |
| **Mục đích**            | Lấy số dư ví khả dụng (Available Balance) của tài khoản Futures (FR-DESIGN-08).                                                                                                  |
| **Đầu vào (Inputs)**    | Không có.                                                                                                                                                                        |
| **Đầu ra (Output)**     | `Number` — Số dư khả dụng (đơn vị: USDT).                                                                                                                                        |
| **Lưu ý nghiệp vụ**     | Trả về số dư có thể dùng để mở vị thế mới, đã trừ đi phần ký quỹ (margin) đang bị chiếm. Hữu ích để tính khối lượng đặt lệnh động: VD `Khối lượng = Số dư × 10% ÷ Giá hiện tại`. |

**JSON Definition:**

```json
{
  "type": "data_balance",

  "message0": "Số dư khả dụng (USDT)",

  "output": "Number",
  "colour": 190,
  "tooltip": "Trả về số dư ví Futures khả dụng (Available Balance) tính bằng USDT. Đã trừ phần margin đang bị chiếm.",
  "helpUrl": ""
}
```

---

#### 3.6.5. `trade_smart_order` — Đặt lệnh & Quản lý

| Thuộc tính              | Chi tiết                                                                                                                                                                                                                                                                                                                            |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `trade_smart_order`                                                                                                                                                                                                                                                                                                                 |
| **Mục đích**            | Khối đặt lệnh Futures "All-in-one" — tích hợp toàn bộ tham số giao dịch trong một khối duy nhất, áp dụng cho `Current_Symbol` (FR-DESIGN-09).                                                                                                                                                                                       |
| **Đầu vào (Inputs)**    | **1.** `SIDE` — Dropdown: Vị thế. Lựa chọn: `Long (Mua)` / `Short (Bán)`.                                                                                                                                                                                                                                                           |
|                         | **2.** `ORDER_TYPE` — Dropdown: Loại lệnh. Lựa chọn: `Market (Thị trường)` / `Limit (Giới hạn)`.                                                                                                                                                                                                                                    |
|                         | **3.** `PRICE` — Value Input (kiểu `Number`): Giá đặt lệnh. Bắt buộc cho Limit, bỏ qua cho Market.                                                                                                                                                                                                                                  |
|                         | **4.** `QUANTITY` — Value Input (kiểu `Number`): Khối lượng giao dịch.                                                                                                                                                                                                                                                              |
|                         | **5.** `LEVERAGE` — Field Number: Đòn bẩy (1x – 125x, mặc định: 1).                                                                                                                                                                                                                                                                 |
|                         | **6.** `MARGIN_TYPE` — Dropdown: Kiểu ký quỹ. Lựa chọn: `Cross (Toàn phần)` / `Isolated (Cô lập)`.                                                                                                                                                                                                                                  |
| **Đầu ra (Output)**     | Previous Statement + Next Statement (khối hành động).                                                                                                                                                                                                                                                                               |
| **Lưu ý nghiệp vụ**     | **Logic Backend phức tạp:** Trước khi gửi lệnh đặt hàng, Backend phải: (1) Kiểm tra Leverage hiện tại trên sàn → Nếu khác giá trị Input → Gửi API đổi Leverage trước. (2) Kiểm tra Margin Type hiện tại → Nếu khác → Gửi API đổi Margin Type. (3) Sau cùng mới gửi API đặt lệnh. Nếu lệnh `Market`, Backend bỏ qua tham số `PRICE`. |

**JSON Definition:**

```json
{
  "type": "trade_smart_order",

  "message0": "Đặt lệnh Futures",

  "message1": "Vị thế %1 — Loại lệnh %2",
  "args1": [
    {
      "type": "field_dropdown",
      "name": "SIDE",
      "options": [
        ["Long (Mua)", "LONG"],
        ["Short (Bán)", "SHORT"]
      ]
    },
    {
      "type": "field_dropdown",
      "name": "ORDER_TYPE",
      "options": [
        ["Market (Thị trường)", "MARKET"],
        ["Limit (Giới hạn)", "LIMIT"]
      ]
    }
  ],

  "message2": "Giá %1",
  "args2": [
    {
      "type": "input_value",
      "name": "PRICE",
      "check": "Number"
    }
  ],

  "message3": "Khối lượng %1",
  "args3": [
    {
      "type": "input_value",
      "name": "QUANTITY",
      "check": "Number"
    }
  ],

  "message4": "Đòn bẩy %1 x",
  "args4": [
    {
      "type": "field_number",
      "name": "LEVERAGE",
      "value": 1,
      "min": 1,
      "max": 125,
      "precision": 1
    }
  ],

  "message5": "Ký quỹ %1",
  "args5": [
    {
      "type": "field_dropdown",
      "name": "MARGIN_TYPE",
      "options": [
        ["Cross (Toàn phần)", "CROSS"],
        ["Isolated (Cô lập)", "ISOLATED"]
      ]
    }
  ],

  "previousStatement": null,
  "nextStatement": null,
  "colour": 0,
  "tooltip": "Đặt lệnh giao dịch Futures cho cặp tiền hiện tại (Current_Symbol). Backend sẽ tự kiểm tra và điều chỉnh Đòn bẩy/Ký quỹ trên sàn trước khi gửi lệnh. Với lệnh Market, tham số Giá sẽ bị bỏ qua.",
  "helpUrl": ""
}
```

---

#### 3.6.6. `trade_close_position` — Đặt lệnh & Quản lý

| Thuộc tính              | Chi tiết                                                                                                                                      |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `trade_close_position`                                                                                                                        |
| **Mục đích**            | Đóng ngay toàn bộ vị thế đang mở của cặp tiền hiện tại (FR-DESIGN-10). Tương đương đặt lệnh Market ngược chiều với toàn bộ kích thước vị thế. |
| **Đầu vào (Inputs)**    | Không có (Context-aware — tự động dùng `Current_Symbol`).                                                                                     |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                                                                          |
| **Lưu ý nghiệp vụ**     | Nếu không có vị thế nào đang mở, khối này không thực hiện hành động nào và ghi log thông báo `"Không có vị thế để đóng"`.                     |

**JSON Definition:**

```json
{
  "type": "trade_close_position",

  "message0": "Đóng vị thế hiện tại",

  "previousStatement": null,
  "nextStatement": null,
  "colour": 0,
  "tooltip": "Đóng ngay toàn bộ vị thế đang mở cho cặp tiền hiện tại (Current_Symbol) bằng lệnh Market. Nếu không có vị thế, không thực hiện gì.",
  "helpUrl": ""
}
```

---

#### 3.6.7. `trade_cancel_all_orders` — Đặt lệnh & Quản lý

| Thuộc tính              | Chi tiết                                                                                                                                                                                                               |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tên khối (Block ID)** | `trade_cancel_all_orders`                                                                                                                                                                                              |
| **Mục đích**            | Hủy toàn bộ lệnh chờ (Pending Orders) đang mở của cặp tiền hiện tại (FR-DESIGN-10).                                                                                                                                    |
| **Đầu vào (Inputs)**    | Không có (Context-aware).                                                                                                                                                                                              |
| **Đầu ra (Output)**     | Previous Statement + Next Statement.                                                                                                                                                                                   |
| **Lưu ý nghiệp vụ**     | Hữu ích trong các tình huống: (1) Trước khi đặt lệnh mới — dọn sạch lệnh cũ để tránh trùng. (2) Khi phát hiện tín hiệu đảo chiều — hủy toàn bộ lệnh Limit chờ. Nếu không có lệnh chờ nào, khối này không thực hiện gì. |

**JSON Definition:**

```json
{
  "type": "trade_cancel_all_orders",

  "message0": "Hủy tất cả lệnh chờ",

  "previousStatement": null,
  "nextStatement": null,
  "colour": 0,
  "tooltip": "Hủy toàn bộ lệnh chờ (Pending Orders) đang mở cho cặp tiền hiện tại (Current_Symbol). Nếu không có lệnh chờ nào, không thực hiện gì.",
  "helpUrl": ""
}
```

---

## Phụ lục A: Ví dụ chiến lược mẫu

Dưới đây là ví dụ một chiến lược đơn giản sử dụng các khối lệnh đã đặc tả, giúp hình dung cách các khối nối với nhau:

**Chiến lược: "RSI Oversold Long"** — Mua Long khi RSI quá bán.

```
┌──────────────────────────────────────────────┐
│ Khi [đóng nến] khung [15 phút] │ ← event_on_candle
├──────────────────────────────────────────────┤
│ Gán biến phiên [rsi] = ┌──────────────┐ │ ← variables_session_set
│ │ RSI khung │ │ ← indicator_rsi
│ │[15m] chu kỳ │ │
│ │ [14] │ │
│ └──────────────┘ │
├──────────────────────────────────────────────┤
│ Nếu ┌────────────────────────────────────┐ │ ← controls_if
│ │ [ Biến phiên rsi] [<] [30] │ │ ← logic_compare
│ └────────────────────────────────────┘ │
│ thì │
│ ├─ Nếu ┌──────────────────────────────┐ │ ← controls_if (lồng)
│ │ │ [ Vị thế Size] [=] [0] │ │ ← logic_compare
│ │ └──────────────────────────────┘ │
│ │ thì │
│ │ ├─ Đặt lệnh Futures │ ← trade_smart_order
│ │ │ Vị thế: Long — Loại: Market │
│ │ │ Giá: (bỏ qua) │
│ │ │ Khối lượng: ┌──────────────┐ │
│ │ │ │ Số dư × 0.1│ │ ← math_arithmetic + data_balance
│ │ │ └──────────────┘ │
│ │ │ Đòn bẩy: 10x │
│ │ │ Ký quỹ: Isolated │
└───┴───┴──────────────────────────────────────┘
```

**Tính toán Unit Cost cho chiến lược trên:**

| Bước | Khối lệnh                      |  Unit Cost  |
| :--: | ------------------------------ | :---------: |
|  1   | `event_on_candle`              |      0      |
|  2   | `indicator_rsi`                |      5      |
|  3   | `variables_session_set`        |      1      |
|  4   | `variables_session_get`        |      1      |
|  5   | `logic_compare` (rsi < 30)     |      1      |
|  6   | `controls_if`                  |      1      |
|  7   | `data_position_info`           |      3      |
|  8   | `math_number` (0)              |      1      |
|  9   | `logic_compare` (size = 0)     |      1      |
|  10  | `controls_if` (lồng)           |      1      |
|  11  | `data_balance`                 |      3      |
|  12  | `math_number` (0.1)            |      1      |
|  13  | `math_arithmetic` (×)          |      1      |
|  14  | `trade_smart_order`            |     10      |
|      | **Tổng (trường hợp xấu nhất)** | **31 Unit** |

→ Chỉ chiếm **3.1%** giới hạn 1000 Unit/Session — cực kỳ an toàn.

---

## Phụ lục B: Quy ước đặt tên Block ID

| Nhóm               | Tiền tố (Prefix)       | Ví dụ                                       |
| ------------------ | ---------------------- | ------------------------------------------- |
| Sự kiện            | `event_`               | `event_on_candle`                           |
| Logic & Điều khiển | `controls_` / `logic_` | `controls_if`, `logic_compare`              |
| Toán học           | `math_`                | `math_arithmetic`, `math_round`             |
| Biến Phiên         | `variables_session_`   | `variables_session_set`                     |
| Biến Vòng đời      | `variables_lifecycle_` | `variables_lifecycle_set`                   |
| Chỉ báo Kỹ thuật   | `indicator_`           | `indicator_rsi`, `indicator_ema`            |
| Dữ liệu Thị trường | `data_`                | `data_market_price`, `data_balance`         |
| Giao dịch          | `trade_`               | `trade_smart_order`, `trade_close_position` |

---

## Phụ lục C: Bảng tổng hợp tất cả khối lệnh

|  #  | Block ID                  | Nhóm      | Kiểu khối         | Đầu ra      | Hue |
| :-: | ------------------------- | --------- | ----------------- | ----------- | :-: |
|  1  | `event_on_candle`         | Sự kiện   | Statement (Start) | Next only   | 30  |
|  2  | `controls_if`             | Logic     | Statement         | Prev + Next | 210 |
|  3  | `controls_if_else`        | Logic     | Statement         | Prev + Next | 210 |
|  4  | `logic_compare`           | Logic     | Value             | Boolean     | 210 |
|  5  | `logic_operation`         | Logic     | Value             | Boolean     | 210 |
|  6  | `logic_negate`            | Logic     | Value             | Boolean     | 210 |
|  7  | `logic_boolean`           | Logic     | Value             | Boolean     | 210 |
|  8  | `controls_repeat`         | Logic     | Statement         | Prev + Next | 210 |
|  9  | `controls_while`          | Logic     | Statement         | Prev + Next | 210 |
| 10  | `math_number`             | Toán học  | Value             | Number      | 230 |
| 11  | `math_arithmetic`         | Toán học  | Value             | Number      | 230 |
| 12  | `math_round`              | Toán học  | Value             | Number      | 230 |
| 13  | `math_random_int`         | Toán học  | Value             | Number      | 230 |
| 14  | `variables_session_set`   | Biến      | Statement         | Prev + Next | 330 |
| 15  | `variables_session_get`   | Biến      | Value             | Any         | 330 |
| 16  | `variables_lifecycle_set` | Biến      | Statement         | Prev + Next | 20  |
| 17  | `variables_lifecycle_get` | Biến      | Value             | Any         | 20  |
| 18  | `indicator_rsi`           | Chỉ báo   | Value             | Number      | 160 |
| 19  | `indicator_ema`           | Chỉ báo   | Value             | Number      | 160 |
| 20  | `data_market_price`       | Giao dịch | Value             | Number      | 190 |
| 21  | `data_position_info`      | Giao dịch | Value             | Number      | 190 |
| 22  | `data_open_orders_count`  | Giao dịch | Value             | Number      | 190 |
| 23  | `data_balance`            | Giao dịch | Value             | Number      | 190 |
| 24  | `trade_smart_order`       | Giao dịch | Statement         | Prev + Next |  0  |
| 25  | `trade_close_position`    | Giao dịch | Statement         | Prev + Next |  0  |
| 26  | `trade_cancel_all_orders` | Giao dịch | Statement         | Prev + Next |  0  |

---

_Kết thúc tài liệu. Mọi thay đổi cần được cập nhật phiên bản và ngày tương ứng._
