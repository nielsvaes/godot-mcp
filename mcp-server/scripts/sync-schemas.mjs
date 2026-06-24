#!/usr/bin/env node
// Build step: generate src/tools/generated-tools.ts from the shared
// schemas/tools.json contract so the published dist/ is self-contained
// (the npm package ships dist/, not the repo-root schemas/ dir).
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const jsonPath = resolve(here, '..', '..', 'schemas', 'tools.json');
const outPath = resolve(here, '..', 'src', 'tools', 'generated-tools.ts');

const tools = JSON.parse(readFileSync(jsonPath, 'utf8'));
const ToolDef = tools.map((t) => ({
  name: t.name,
  description: t.description,
  inputSchema: t.inputSchema,
}));

const banner = `// AUTO-GENERATED from /schemas/tools.json by scripts/sync-schemas.mjs.\n// Do not edit by hand; edit schemas/tools.json and rebuild.\n`;
const body = `import type { ToolDefinition } from '../types.js';\n\nexport const allTools: ToolDefinition[] = ${JSON.stringify(ToolDef, null, 2)} as unknown as ToolDefinition[];\n`;
writeFileSync(outPath, banner + body, 'utf8');
console.log(`Generated ${outPath} (${ToolDef.length} tools)`);
