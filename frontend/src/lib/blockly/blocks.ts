/**
 * [3.2.2] Block Registration Entry Point
 *
 * This module is imported as a side-effect in blockly-workspace.tsx:
 *   import '@/lib/blockly/blocks';
 *
 * Importing this module triggers registerCustomBlocks() exactly once,
 * ensuring all 26 QuantFlow custom blocks are available in the Blockly
 * registry before any workspace calls Blockly.inject().
 *
 * The file is intentionally thin — routing concern only.
 * All block definitions live in components/editor/custom-blocks.ts.
 */

import { registerCustomBlocks } from "@/components/editor/custom-blocks";

registerCustomBlocks();

export {};
