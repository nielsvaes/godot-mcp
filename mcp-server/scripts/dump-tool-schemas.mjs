#!/usr/bin/env node
// One-time migration: extract tool schemas from the compiled tool modules into
// the language-neutral schemas/tools.json contract consumed by the Node server
// (via codegen) and the Go CLI (via go:embed). Run after `npm run build`.
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = resolve(here, '..', 'dist');

const { fileTools } = await import(resolve(dist, 'tools/file-tools.js'));
const { sceneTools } = await import(resolve(dist, 'tools/scene-tools.js'));
const { scriptTools } = await import(resolve(dist, 'tools/script-tools.js'));
const { projectTools } = await import(resolve(dist, 'tools/project-tools.js'));
const { assetTools } = await import(resolve(dist, 'tools/asset-tools.js'));
const { visualizerTools } = await import(resolve(dist, 'tools/visualizer-tools.js'));
const { RUNTIME_ONLY_TOOLS } = await import(resolve(dist, 'godot-bridge.js'));

const categories = [
  ['file', fileTools], ['scene', sceneTools], ['script', scriptTools],
  ['project', projectTools], ['asset', assetTools], ['visualizer', visualizerTools],
];

const tools = [];
for (const [category, arr] of categories) {
  for (const t of arr) {
    tools.push({
      name: t.name,
      category,
      target: RUNTIME_ONLY_TOOLS.has(t.name) ? 'runtime' : 'editor',
      description: t.description,
      inputSchema: t.inputSchema,
    });
  }
}

const outDir = resolve(here, '..', '..', 'schemas');
mkdirSync(outDir, { recursive: true });
const outPath = resolve(outDir, 'tools.json');
writeFileSync(outPath, JSON.stringify(tools, null, 2) + '\n', 'utf8');
console.log(`Wrote ${tools.length} tools to ${outPath}`);
