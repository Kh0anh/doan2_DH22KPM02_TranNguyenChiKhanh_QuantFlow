/**
 * [3.2.2] QuantFlow Custom Block Definitions — 26 blocks across 6 color groups.
 *
 * Groups:
 *   1. Event Trigger   — Hue  30 (Cam)           — 1 block
 *   2. Logic & Control — Hue 210 (Xanh dương)    — 8 blocks
 *   3. Math            — Hue 230 (Tím)            — 4 blocks
 *   4. Variables       — Hue 330/20 (Hồng/Đỏ cam) — 4 blocks
 *   5. Indicators      — Hue 160 (Xanh lục)       — 2 blocks
 *   6. Trading         — Hue 190/0 (Xanh lam/Đỏ)  — 7 blocks
 *
 * Register via: registerCustomBlocks() — called once at module init from
 * lib/blockly/blocks.ts (side-effect import in blockly-workspace.tsx).
 *
 * Block type names must match the backend Block Registry (WBS 2.5.1).
 * Spec source: docs/blockly/blockly.md v1.0
 */

import * as Blockly from "blockly";

// ---------------------------------------------------------------------------
// Type alias — matches Blockly.defineBlocksWithJsonArray parameter element type
// ---------------------------------------------------------------------------
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type JsonBlockDef = Record<string, any>;

// ---------------------------------------------------------------------------
// 26 Block Definitions
// ---------------------------------------------------------------------------

const BLOCK_DEFINITIONS: JsonBlockDef[] = [
  // =========================================================================
  // GROUP 1 — SỰ KIỆN (Event Trigger) | Hue: 30 (Cam) | FR-DESIGN-03
  // =========================================================================

  /**
   * event_on_candle — Start Block.
   * Triggers a new Session when a candle opens or closes on the chosen timeframe.
   * Has nextStatement only (always top-most block, no previousStatement).
   */
  {
    type: "event_on_candle",
    message0: "Khi %1 khung %2",
    args0: [
      {
        type: "field_dropdown",
        name: "TRIGGER",
        options: [
          ["đóng nến", "ON_CANDLE_CLOSE"],
          ["mở nến", "ON_CANDLE_OPEN"],
        ],
      },
      {
        type: "field_dropdown",
        name: "TIMEFRAME",
        options: [
          ["1 phút", "1m"],
          ["5 phút", "5m"],
          ["15 phút", "15m"],
          ["30 phút", "30m"],
          ["1 giờ", "1h"],
          ["4 giờ", "4h"],
          ["1 ngày", "1d"],
          ["1 tuần", "1w"],
        ],
      },
    ],
    nextStatement: null,
    colour: 30,
    tooltip:
      "Khối sự kiện bắt đầu. Mỗi khi nến đóng/mở theo khung thời gian đã chọn, các khối bên dưới sẽ được thực thi 1 lần (1 Session).",
    helpUrl: "",
  },

  // =========================================================================
  // GROUP 2 — LOGIC & ĐIỀU KHIỂN | Hue: 210 (Xanh dương) | FR-DESIGN-05
  // =========================================================================

  /**
   * controls_if — If block (1 branch).
   */
  {
    type: "controls_if",
    message0: "Nếu %1",
    args0: [
      {
        type: "input_value",
        name: "CONDITION",
        check: "Boolean",
      },
    ],
    message1: "thì %1",
    args1: [
      {
        type: "input_statement",
        name: "DO",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 210,
    tooltip:
      "Kiểm tra điều kiện. Nếu đúng (True), thực thi các khối lệnh bên trong.",
    helpUrl: "",
  },

  /**
   * controls_if_else — If/Else block (2 branches).
   */
  {
    type: "controls_if_else",
    message0: "Nếu %1",
    args0: [
      {
        type: "input_value",
        name: "CONDITION",
        check: "Boolean",
      },
    ],
    message1: "thì %1",
    args1: [
      {
        type: "input_statement",
        name: "DO",
      },
    ],
    message2: "ngược lại %1",
    args2: [
      {
        type: "input_statement",
        name: "ELSE",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 210,
    tooltip:
      "Kiểm tra điều kiện. Nếu đúng → thực thi nhánh 'thì'. Nếu sai → thực thi nhánh 'ngược lại'.",
    helpUrl: "",
  },

  /**
   * logic_compare — Compare two values, returns Boolean.
   */
  {
    type: "logic_compare",
    message0: "%1 %2 %3",
    args0: [
      {
        type: "input_value",
        name: "A",
      },
      {
        type: "field_dropdown",
        name: "OP",
        options: [
          ["=", "EQ"],
          ["≠", "NEQ"],
          [">", "GT"],
          ["≥", "GTE"],
          ["<", "LT"],
          ["≤", "LTE"],
        ],
      },
      {
        type: "input_value",
        name: "B",
      },
    ],
    inputsInline: true,
    output: "Boolean",
    colour: 210,
    tooltip: "So sánh hai giá trị. Trả về Đúng (True) hoặc Sai (False).",
    helpUrl: "",
  },

  /**
   * logic_operation — AND / OR combinator, returns Boolean.
   */
  {
    type: "logic_operation",
    message0: "%1 %2 %3",
    args0: [
      {
        type: "input_value",
        name: "A",
        check: "Boolean",
      },
      {
        type: "field_dropdown",
        name: "OP",
        options: [
          ["VÀ", "AND"],
          ["HOẶC", "OR"],
        ],
      },
      {
        type: "input_value",
        name: "B",
        check: "Boolean",
      },
    ],
    inputsInline: true,
    output: "Boolean",
    colour: 210,
    tooltip:
      "Kết hợp hai điều kiện. VÀ: cả hai phải đúng. HOẶC: ít nhất một đúng.",
    helpUrl: "",
  },

  /**
   * logic_negate — NOT operator, returns Boolean.
   */
  {
    type: "logic_negate",
    message0: "KHÔNG %1",
    args0: [
      {
        type: "input_value",
        name: "BOOL",
        check: "Boolean",
      },
    ],
    output: "Boolean",
    colour: 210,
    tooltip: "Đảo ngược giá trị logic. Đúng thành Sai, Sai thành Đúng.",
    helpUrl: "",
  },

  /**
   * logic_boolean — Boolean literal (True / False).
   */
  {
    type: "logic_boolean",
    message0: "%1",
    args0: [
      {
        type: "field_dropdown",
        name: "BOOL",
        options: [
          ["Đúng", "TRUE"],
          ["Sai", "FALSE"],
        ],
      },
    ],
    output: "Boolean",
    colour: 210,
    tooltip:
      "Trả về giá trị logic cố định: Đúng (True) hoặc Sai (False).",
    helpUrl: "",
  },

  /**
   * controls_repeat — Repeat N times loop.
   * Unit Cost: 1/iteration (enforced by backend engine).
   */
  {
    type: "controls_repeat",
    message0: "Lặp lại %1 lần",
    args0: [
      {
        type: "input_value",
        name: "TIMES",
        check: "Number",
      },
    ],
    message1: "thực hiện %1",
    args1: [
      {
        type: "input_statement",
        name: "DO",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 210,
    tooltip:
      "Lặp lại các khối bên trong N lần. Mỗi vòng lặp tốn 1 Unit Cost. Chú ý: Tổng Unit bị giới hạn mỗi Session.",
    helpUrl: "",
  },

  /**
   * controls_while — While-condition loop.
   * Unit Cost: 1/iteration. Infinite loops are terminated by the Unit Cost guard.
   */
  {
    type: "controls_while",
    message0: "Lặp khi %1",
    args0: [
      {
        type: "input_value",
        name: "CONDITION",
        check: "Boolean",
      },
    ],
    message1: "thực hiện %1",
    args1: [
      {
        type: "input_statement",
        name: "DO",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 210,
    tooltip:
      "Lặp lại các khối bên trong khi điều kiện còn đúng. Mỗi vòng tốn 1 Unit. Hệ thống sẽ tự dừng nếu vượt giới hạn Unit Cost.",
    helpUrl: "",
  },

  // =========================================================================
  // GROUP 3 — TOÁN HỌC (Math) | Hue: 230 (Tím) | FR-DESIGN-06
  // =========================================================================

  /**
   * math_number — Numeric literal constant.
   */
  {
    type: "math_number",
    message0: "%1",
    args0: [
      {
        type: "field_number",
        name: "NUM",
        value: 0,
      },
    ],
    output: "Number",
    colour: 230,
    tooltip:
      "Một giá trị số cố định. Dùng để nhập giá, khối lượng, chu kỳ chỉ báo, v.v.",
    helpUrl: "",
  },

  /**
   * math_arithmetic — Basic arithmetic (+, −, ×, ÷).
   * Backend uses BigDecimal precision. Division by zero returns 0 + logs warning.
   */
  {
    type: "math_arithmetic",
    message0: "%1 %2 %3",
    args0: [
      {
        type: "input_value",
        name: "A",
        check: "Number",
      },
      {
        type: "field_dropdown",
        name: "OP",
        options: [
          ["+", "ADD"],
          ["−", "MINUS"],
          ["×", "MULTIPLY"],
          ["÷", "DIVIDE"],
        ],
      },
      {
        type: "input_value",
        name: "B",
        check: "Number",
      },
    ],
    inputsInline: true,
    output: "Number",
    colour: 230,
    tooltip:
      "Thực hiện phép tính số học (+, −, ×, ÷) giữa hai giá trị số.",
    helpUrl: "",
  },

  /**
   * math_round — Round to N decimal places.
   * Essential for quantity/price precision compliance with exchange stepSize rules.
   */
  {
    type: "math_round",
    message0: "Làm tròn %1 đến %2 chữ số thập phân",
    args0: [
      {
        type: "input_value",
        name: "NUM",
        check: "Number",
      },
      {
        type: "field_number",
        name: "DECIMALS",
        value: 2,
        min: 0,
        max: 18,
        precision: 1,
      },
    ],
    output: "Number",
    colour: 230,
    tooltip:
      "Làm tròn giá trị theo số chữ số thập phân. VD: Làm tròn 3.14159 đến 2 → 3.14.",
    helpUrl: "",
  },

  /**
   * math_random_int — Random integer in [FROM, TO].
   */
  {
    type: "math_random_int",
    message0: "Ngẫu nhiên từ %1 đến %2",
    args0: [
      {
        type: "input_value",
        name: "FROM",
        check: "Number",
      },
      {
        type: "input_value",
        name: "TO",
        check: "Number",
      },
    ],
    inputsInline: true,
    output: "Number",
    colour: 230,
    tooltip:
      "Sinh một số nguyên ngẫu nhiên trong khoảng [từ] đến [đến] (bao gồm cả hai đầu).",
    helpUrl: "",
  },

  // =========================================================================
  // GROUP 4 — BIẾN (Variables) | FR-DESIGN-04
  //   Session Variables  — Hue: 330 (Hồng)  — stored in RAM, cleared per Session
  //   Lifecycle Variables — Hue: 20 (Đỏ cam) — stored in DB JSONB, persist across Sessions
  // =========================================================================

  /**
   * variables_session_set — Assign a Session variable (RAM, current Session only).
   */
  {
    type: "variables_session_set",
    message0: "Gán biến phiên %1 = %2",
    args0: [
      {
        type: "field_input",
        name: "VAR_NAME",
        text: "tên_biến",
      },
      {
        type: "input_value",
        name: "VALUE",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 330,
    tooltip:
      "Gán giá trị cho Biến Phiên. Biến này chỉ tồn tại trong phiên xử lý (Session) hiện tại và tự động bị xóa khi Session kết thúc.",
    helpUrl: "",
  },

  /**
   * variables_session_get — Read a Session variable. Returns 0 if not set.
   */
  {
    type: "variables_session_get",
    message0: "Biến phiên %1",
    args0: [
      {
        type: "field_input",
        name: "VAR_NAME",
        text: "tên_biến",
      },
    ],
    output: null,
    colour: 330,
    tooltip:
      "Đọc giá trị của Biến Phiên. Trả về 0 nếu biến chưa được gán trong Session này.",
    helpUrl: "",
  },

  /**
   * variables_lifecycle_set — Assign a Lifecycle variable (persisted in DB JSONB).
   */
  {
    type: "variables_lifecycle_set",
    message0: "Gán biến vòng đời %1 = %2",
    args0: [
      {
        type: "field_input",
        name: "VAR_NAME",
        text: "tên_biến",
      },
      {
        type: "input_value",
        name: "VALUE",
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 20,
    tooltip:
      "Gán giá trị cho Biến Vòng đời. Giá trị được lưu vĩnh viễn trong Database, giữ nguyên giữa các Session và cả khi Server restart.",
    helpUrl: "",
  },

  /**
   * variables_lifecycle_get — Read a Lifecycle variable. Returns 0 if never set.
   */
  {
    type: "variables_lifecycle_get",
    message0: "Biến vòng đời %1",
    args0: [
      {
        type: "field_input",
        name: "VAR_NAME",
        text: "tên_biến",
      },
    ],
    output: null,
    colour: 20,
    tooltip:
      "Đọc giá trị của Biến Vòng đời. Giá trị được duy trì xuyên suốt các Session. Trả về 0 nếu chưa được gán.",
    helpUrl: "",
  },

  // =========================================================================
  // GROUP 5 — CHỈ BÁO KỸ THUẬT (Technical Indicators) | Hue: 160 (Xanh lục)
  //           FR-DESIGN-07 | Context-aware: uses Current_Symbol automatically
  // =========================================================================

  /**
   * indicator_rsi — RSI (Relative Strength Index).
   * Unit Cost: 5 (backend queries candle data + computes).
   * No Symbol param — backend uses Current_Symbol from Bot context.
   */
  {
    type: "indicator_rsi",
    message0: "RSI khung %1 chu kỳ %2",
    args0: [
      {
        type: "field_dropdown",
        name: "TIMEFRAME",
        options: [
          ["1 phút", "1m"],
          ["3 phút", "3m"],
          ["5 phút", "5m"],
          ["15 phút", "15m"],
          ["30 phút", "30m"],
          ["1 giờ", "1h"],
          ["4 giờ", "4h"],
          ["1 ngày", "1d"],
        ],
      },
      {
        type: "input_value",
        name: "PERIOD",
        check: "Number",
      },
    ],
    output: "Number",
    colour: 160,
    tooltip:
      "Tính chỉ báo RSI cho cặp tiền hiện tại (Current_Symbol). RSI > 70: vùng quá mua. RSI < 30: vùng quá bán. Không cần chọn Symbol.",
    helpUrl: "",
  },

  /**
   * indicator_ema — EMA (Exponential Moving Average).
   * Unit Cost: 5. No Symbol param — uses Current_Symbol.
   */
  {
    type: "indicator_ema",
    message0: "EMA khung %1 chu kỳ %2",
    args0: [
      {
        type: "field_dropdown",
        name: "TIMEFRAME",
        options: [
          ["1 phút", "1m"],
          ["3 phút", "3m"],
          ["5 phút", "5m"],
          ["15 phút", "15m"],
          ["30 phút", "30m"],
          ["1 giờ", "1h"],
          ["4 giờ", "4h"],
          ["1 ngày", "1d"],
        ],
      },
      {
        type: "input_value",
        name: "PERIOD",
        check: "Number",
      },
    ],
    output: "Number",
    colour: 160,
    tooltip:
      "Tính chỉ báo EMA cho cặp tiền hiện tại (Current_Symbol). Thường dùng so sánh chéo: EMA ngắn > EMA dài → xu hướng tăng. Không cần chọn Symbol.",
    helpUrl: "",
  },

  // =========================================================================
  // GROUP 6 — GIAO DỊCH (Trading) | FR-DESIGN-08/09/10
  //   Sub-group A: Market Data & Account  — Hue: 190 (Xanh lam) — read-only
  //   Sub-group B: Order & Position Mgmt  — Hue: 0   (Đỏ)       — write actions
  // =========================================================================

  // --- Sub-group A: Data (Hue 190) ----------------------------------------

  /**
   * data_market_price — Current market price or latest candle close price.
   * Unit Cost: 3 (calls exchange API).
   */
  {
    type: "data_market_price",
    message0: "Giá %1",
    args0: [
      {
        type: "field_dropdown",
        name: "PRICE_TYPE",
        options: [
          ["hiện tại", "LAST_PRICE"],
          ["đóng nến", "CLOSE_PRICE"],
        ],
      },
    ],
    output: "Number",
    colour: 190,
    tooltip:
      "Lấy giá của cặp tiền hiện tại (Current_Symbol). 'Giá hiện tại': ticker realtime. 'Giá đóng nến': giá close của nến cuối cùng đã hoàn thành.",
    helpUrl: "",
  },

  /**
   * data_position_info — Position size, unrealized PnL, or entry price.
   * Unit Cost: 3. Returns 0 when no open position.
   */
  {
    type: "data_position_info",
    message0: "Vị thế — %1",
    args0: [
      {
        type: "field_dropdown",
        name: "FIELD",
        options: [
          ["Kích thước (Size)", "POSITION_SIZE"],
          ["Lãi/Lỗ chưa chốt (PnL)", "UNREALIZED_PNL"],
          ["Giá vào lệnh (Entry Price)", "ENTRY_PRICE"],
        ],
      },
    ],
    output: "Number",
    colour: 190,
    tooltip:
      "Lấy thông tin vị thế đang mở cho cặp tiền hiện tại. Trả về 0 nếu không có vị thế nào.",
    helpUrl: "",
  },

  /**
   * data_open_orders_count — Number of pending/open orders for Current_Symbol.
   * Unit Cost: 3. Useful to guard against duplicate order placement.
   */
  {
    type: "data_open_orders_count",
    message0: "Số lệnh chờ đang mở",
    output: "Number",
    colour: 190,
    tooltip:
      "Trả về số lượng lệnh chờ (Pending/Open Orders) cho cặp tiền hiện tại. Dùng để kiểm tra trước khi đặt lệnh mới.",
    helpUrl: "",
  },

  /**
   * data_balance — Available Futures wallet balance in USDT.
   * Unit Cost: 3. Excludes margin already locked in open positions.
   */
  {
    type: "data_balance",
    message0: "Số dư khả dụng (USDT)",
    output: "Number",
    colour: 190,
    tooltip:
      "Trả về số dư ví Futures khả dụng (Available Balance) tính bằng USDT. Đã trừ phần margin đang bị chiếm.",
    helpUrl: "",
  },

  // --- Sub-group B: Orders (Hue 0 — Red, visual danger indicator) ----------

  /**
   * trade_smart_order — All-in-one Futures order block.
   * Unit Cost: 10.
   * Backend pre-flight: adjust Leverage + Margin Type on exchange before placing order.
   * Context-aware: always applies to Current_Symbol.
   */
  {
    type: "trade_smart_order",
    message0: "Đặt lệnh Futures",
    message1: "Vị thế %1 — Loại lệnh %2",
    args1: [
      {
        type: "field_dropdown",
        name: "SIDE",
        options: [
          ["Long (Mua)", "LONG"],
          ["Short (Bán)", "SHORT"],
        ],
      },
      {
        type: "field_dropdown",
        name: "ORDER_TYPE",
        options: [
          ["Market (Thị trường)", "MARKET"],
          ["Limit (Giới hạn)", "LIMIT"],
        ],
      },
    ],
    message2: "Giá %1",
    args2: [
      {
        type: "input_value",
        name: "PRICE",
        check: "Number",
      },
    ],
    message3: "Khối lượng %1",
    args3: [
      {
        type: "input_value",
        name: "QUANTITY",
        check: "Number",
      },
    ],
    message4: "Đòn bẩy %1 x",
    args4: [
      {
        type: "field_number",
        name: "LEVERAGE",
        value: 1,
        min: 1,
        max: 125,
        precision: 1,
      },
    ],
    message5: "Ký quỹ %1",
    args5: [
      {
        type: "field_dropdown",
        name: "MARGIN_TYPE",
        options: [
          ["Cross (Toàn phần)", "CROSS"],
          ["Isolated (Cô lập)", "ISOLATED"],
        ],
      },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 0,
    tooltip:
      "Đặt lệnh giao dịch Futures cho cặp tiền hiện tại (Current_Symbol). Backend sẽ tự kiểm tra và điều chỉnh Đòn bẩy/Ký quỹ trên sàn trước khi gửi lệnh. Với lệnh Market, tham số Giá sẽ bị bỏ qua.",
    helpUrl: "",
  },

  /**
   * trade_close_position — Market-close entire open position for Current_Symbol.
   * Unit Cost: 10. No-op (+ logs info) when no position is open.
   */
  {
    type: "trade_close_position",
    message0: "Đóng vị thế hiện tại",
    previousStatement: null,
    nextStatement: null,
    colour: 0,
    tooltip:
      "Đóng ngay toàn bộ vị thế đang mở cho cặp tiền hiện tại (Current_Symbol) bằng lệnh Market. Nếu không có vị thế, không thực hiện gì.",
    helpUrl: "",
  },

  /**
   * trade_cancel_all_orders — Cancel all pending/open orders for Current_Symbol.
   * Unit Cost: 10. No-op when no open orders exist.
   */
  {
    type: "trade_cancel_all_orders",
    message0: "Hủy tất cả lệnh chờ",
    previousStatement: null,
    nextStatement: null,
    colour: 0,
    tooltip:
      "Hủy toàn bộ lệnh chờ (Pending Orders) đang mở cho cặp tiền hiện tại (Current_Symbol). Nếu không có lệnh chờ nào, không thực hiện gì.",
    helpUrl: "",
  },
];

// ---------------------------------------------------------------------------
// Registration — idempotency guard prevents double-registration in React
// Strict Mode (which calls effects twice in development).
// ---------------------------------------------------------------------------

let _registered = false;

/**
 * Register all 26 QuantFlow custom blocks with the global Blockly registry.
 * Safe to call multiple times — subsequent calls are no-ops.
 * Must be called before any Blockly.inject() that uses these block types.
 */
export function registerCustomBlocks(): void {
  if (_registered) return;
  _registered = true;
  Blockly.defineBlocksWithJsonArray(BLOCK_DEFINITIONS);
}
