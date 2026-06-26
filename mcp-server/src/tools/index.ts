/**
 * Tool registry. The single source of truth for tool schemas is
 * /schemas/tools.json; `generated-tools.ts` is produced from it at build time
 * by scripts/sync-schemas.mjs.
 */

import { allTools } from './generated-tools.js';
import type { ToolDefinition } from '../types.js';

export { allTools };
export type { ToolDefinition };

export function toolExists(toolName: string): boolean {
  return allTools.some((t) => t.name === toolName);
}
