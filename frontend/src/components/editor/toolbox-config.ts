/**
 * [3.2.3] QuantFlow Toolbox Configuration — 6 categorized block groups.
 *
 * Structure (per blockly.md §2 — FR-DESIGN-02):
 *   1. Sự kiện         (Event Trigger)      — Hue  30 (Cam)            — 1 block
 *   2. Logic & Điều khiển                   — Hue 210 (Xanh dương)     — 8 blocks
 *   3. Toán học        (Math)               — Hue 230 (Tím)            — 4 blocks
 *   4. Biến            (Session+Lifecycle)  — Hue 330 (Hồng)           — 4 blocks
 *   5. Chỉ báo Kỹ thuật (Technical Indicators) — Hue 160 (Xanh lục)   — 2 blocks
 *   6. Giao dịch       (Trading)            — Hue 190 (Xanh lam)       — 7 blocks
 *
 * Total: 26 blocks — matches custom-blocks.ts and backend block registry (WBS 2.5.1).
 *
 * Shadow blocks are added for numeric inputs so users see sensible defaults
 * the moment they drag a block out of the toolbox.
 *
 * Spec: docs/blockly/blockly.md v1.0
 */

import type * as Blockly from "blockly";

// ---------------------------------------------------------------------------
// Helper — shorthand to build a shadow math_number block
// ---------------------------------------------------------------------------
function shadowNumber(value: number): Blockly.utils.toolbox.BlockInfo {
  return {
    kind: "block",
    type: "math_number",
    fields: { NUM: value },
  } as Blockly.utils.toolbox.BlockInfo;
}

// ---------------------------------------------------------------------------
// QUANTFLOW_TOOLBOX — exported and consumed by BlocklyWorkspace inject options
// ---------------------------------------------------------------------------
export const QUANTFLOW_TOOLBOX: Blockly.utils.toolbox.ToolboxDefinition = {
  kind: "categoryToolbox",
  contents: [
    // =========================================================================
    // 1. SỰ KIỆN (Event Trigger) | Hue: 30 | FR-DESIGN-03 | 1 block
    // =========================================================================
    {
      kind: "category",
      name: "Sự kiện",
      colour: "30",
      contents: [
        {
          // event_on_candle — Start Block (no previousStatement)
          kind: "block",
          type: "event_on_candle",
        },
      ],
    },

    // =========================================================================
    // 2. LOGIC & ĐIỀU KHIỂN | Hue: 210 | FR-DESIGN-05 | 8 blocks
    // =========================================================================
    {
      kind: "category",
      name: "Logic & Điều khiển",
      colour: "210",
      contents: [
        // controls_if — If statement
        {
          kind: "block",
          type: "controls_if",
        },
        // controls_if_else — If / Else statement
        {
          kind: "block",
          type: "controls_if_else",
        },
        // logic_compare — A <OP> B → Boolean
        {
          kind: "block",
          type: "logic_compare",
        },
        // logic_operation — A AND/OR B → Boolean
        {
          kind: "block",
          type: "logic_operation",
        },
        // logic_negate — NOT bool → Boolean
        {
          kind: "block",
          type: "logic_negate",
        },
        // logic_boolean — True / False constant
        {
          kind: "block",
          type: "logic_boolean",
        },
        // controls_repeat — Repeat N times (shadow: 10)
        {
          kind: "block",
          type: "controls_repeat",
          inputs: {
            TIMES: {
              shadow: shadowNumber(10),
            },
          },
        } as Blockly.utils.toolbox.BlockInfo,
        // controls_while — While condition do
        {
          kind: "block",
          type: "controls_while",
        },
      ],
    },

    // =========================================================================
    // 3. TOÁN HỌC (Math) | Hue: 230 | FR-DESIGN-06 | 4 blocks
    // =========================================================================
    {
      kind: "category",
      name: "Toán học",
      colour: "230",
      contents: [
        // math_number — Numeric constant (shadow: 0)
        {
          kind: "block",
          type: "math_number",
          fields: { NUM: 0 },
        },
        // math_arithmetic — A <OP> B (shadows: 0 + 0)
        {
          kind: "block",
          type: "math_arithmetic",
          inputs: {
            A: { shadow: shadowNumber(0) },
            B: { shadow: shadowNumber(0) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
        // math_round — Round NUM to DECIMALS places (shadow: 0, decimals: 2)
        {
          kind: "block",
          type: "math_round",
          inputs: {
            NUM: { shadow: shadowNumber(0) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
        // math_random_int — Random int from FROM to TO (shadows: 1, 100)
        {
          kind: "block",
          type: "math_random_int",
          inputs: {
            FROM: { shadow: shadowNumber(1) },
            TO: { shadow: shadowNumber(100) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
      ],
    },

    // =========================================================================
    // 4. BIẾN (Session & Lifecycle) | Hue: 330 (category) | FR-DESIGN-04 | 4 blocks
    // Note: Session blocks render at Hue 330 (Hồng), Lifecycle at Hue 20 (Đỏ cam)
    //       as defined in their individual block JSON definitions — both are shown
    //       under a single "Biến" category for toolbox organisation.
    // =========================================================================
    {
      kind: "category",
      name: "Biến",
      colour: "330",
      contents: [
        // variables_session_set — Assign Session variable
        {
          kind: "block",
          type: "variables_session_set",
        },
        // variables_session_get — Read Session variable
        {
          kind: "block",
          type: "variables_session_get",
        },
        // variables_lifecycle_set — Assign Lifecycle variable (persisted in DB)
        {
          kind: "block",
          type: "variables_lifecycle_set",
        },
        // variables_lifecycle_get — Read Lifecycle variable
        {
          kind: "block",
          type: "variables_lifecycle_get",
        },
      ],
    },

    // =========================================================================
    // 5. CHỈ BÁO KỸ THUẬT (Technical Indicators) | Hue: 160 | FR-DESIGN-07 | 2 blocks
    // =========================================================================
    {
      kind: "category",
      name: "Chỉ báo Kỹ thuật",
      colour: "160",
      contents: [
        // indicator_rsi — RSI value (shadow period: 14)
        {
          kind: "block",
          type: "indicator_rsi",
          inputs: {
            PERIOD: { shadow: shadowNumber(14) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
        // indicator_ema — EMA value (shadow period: 9)
        {
          kind: "block",
          type: "indicator_ema",
          inputs: {
            PERIOD: { shadow: shadowNumber(9) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
      ],
    },

    // =========================================================================
    // 6. GIAO DỊCH (Trading) | Hue: 190 (category) | FR-DESIGN-08/09/10 | 7 blocks
    // Note: Data blocks render at Hue 190 (Xanh lam), Action blocks at Hue 0 (Đỏ)
    //       as defined in block JSON definitions. Category colour uses 190.
    // =========================================================================
    {
      kind: "category",
      name: "Giao dịch",
      colour: "190",
      contents: [
        // ── Dữ liệu Thị trường & Tài khoản (Hue 190) ────────────────────────
        // data_market_price — Current/close price
        {
          kind: "block",
          type: "data_market_price",
        },
        // data_position_info — Position size / PnL / entry price
        {
          kind: "block",
          type: "data_position_info",
        },
        // data_open_orders_count — Count of pending orders
        {
          kind: "block",
          type: "data_open_orders_count",
        },
        // data_balance — Available USDT balance
        {
          kind: "block",
          type: "data_balance",
        },
        // ── Đặt lệnh & Quản lý lệnh (Hue 0 — Red) ──────────────────────────
        // trade_smart_order — Place Futures order (with shadows)
        {
          kind: "block",
          type: "trade_smart_order",
          inputs: {
            PRICE: { shadow: shadowNumber(0) },
            QUANTITY: { shadow: shadowNumber(0) },
          },
        } as Blockly.utils.toolbox.BlockInfo,
        // trade_close_position — Close entire open position (Market)
        {
          kind: "block",
          type: "trade_close_position",
        },
        // trade_cancel_all_orders — Cancel all pending orders
        {
          kind: "block",
          type: "trade_cancel_all_orders",
        },
      ],
    },
  ],
};
