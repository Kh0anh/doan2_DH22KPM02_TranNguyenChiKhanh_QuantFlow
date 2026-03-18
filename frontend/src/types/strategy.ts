// ============================================================
// QuantFlow — Strategy Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/api.yaml (Strategy schemas), docs/database/schema.md (§3, §4)
// ============================================================

// ─── Enums ─────────────────────────────────────────────────

export type StrategyStatus = 'Draft' | 'Valid' | 'Archived';

// ─── Blockly JSON ──────────────────────────────────────────

/** Blockly workspace serialized JSON (recursive block structure) */
export interface BlocklyLogicJson {
  blocks: {
    languageVersion: number;
    blocks: BlocklyBlock[];
  };
}

/** Single Blockly block (recursive — may contain nested blocks) */
export interface BlocklyBlock {
  type: string;
  id?: string;
  fields?: Record<string, unknown>;
  inputs?: Record<string, { block?: BlocklyBlock }>;
  next?: { block?: BlocklyBlock };
  [key: string]: unknown;
}

// ─── Strategy Domain Types ─────────────────────────────────

/** Strategy list item (GET /strategies response) */
export interface StrategySummary {
  id: string;
  name: string;
  version: number;
  status: StrategyStatus;
  created_at: string;
  updated_at: string;
}

/** Full strategy detail (GET /strategies/:id response) */
export interface StrategyDetail {
  id: string;
  name: string;
  version: number;
  status: StrategyStatus;
  logic_json: BlocklyLogicJson;
  /** Warning when strategy is used by running bot(s) */
  warning?: string | null;
  /** IDs of running bots using this strategy */
  active_bot_ids?: string[] | null;
  created_at: string;
  updated_at: string;
}

/** Response after creating a strategy */
export interface StrategyCreated {
  id: string;
  name: string;
  version: number;
  status: string;
  created_at: string;
}

/** Response after updating a strategy */
export interface StrategyUpdated {
  id: string;
  name: string;
  version: number;
  status: string;
  warning?: string | null;
  updated_at: string;
}

/** Strategy export payload (GET /strategies/:id/export) */
export interface StrategyExport {
  name: string;
  logic_json: BlocklyLogicJson;
  version: number;
  exported_at: string;
}

// ─── Request Types ─────────────────────────────────────────

export interface CreateStrategyRequest {
  name: string;
  logic_json: BlocklyLogicJson;
  status?: 'Valid' | 'Draft';
}

export interface UpdateStrategyRequest {
  name?: string;
  logic_json?: BlocklyLogicJson;
  status?: 'Valid' | 'Draft';
}

export interface ImportStrategyRequest {
  name: string;
  logic_json: BlocklyLogicJson;
}
