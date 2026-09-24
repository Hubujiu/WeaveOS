import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const requiredFiles = [
  'AGENTS.md', 'README.md', 'CONTRIBUTING.md', 'SECURITY.md',
  'docs/notion-router.md', 'docs/notion-sources.json', 'docs/testing.md',
  'docs/project-baseline.md', 'docs/templates/task-record.md',
  '.github/pull_request_template.md', 'apps/web/AGENTS.md',
  'services/bff/AGENTS.md', 'contracts/AGENTS.md', 'db/AGENTS.md',
  'infra/AGENTS.md', 'tests/AGENTS.md',
];
const hardRules = ['NOTION-GATE', 'TDD-RED-FIRST', 'TEST-ORACLE', 'NO-FAKE-VERIFICATION', 'CHILD-RULES'];
const requiredRoutes = ['always', 'product', 'auth', 'api', 'database', 'frontend', 'infrastructure', 'governance'];
const ignoredDirs = new Set(['.git', 'node_modules', '.cache', '.work', 'dist', 'build', 'coverage']);
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const text = value => typeof value === 'string' && value.trim().length > 0;

function validateSources(value, errors) {
  const fail = message => errors.push(`[SOURCES] ${message}`);
  if (!object(value)) { fail('registry must be an object'); return; }
  if (value.schemaVersion !== 1) fail('schemaVersion must be 1');
  if (value.projectId !== '3e42f5a9e64880ae9cf5ebbd2088d773') fail('projectId does not match WeaveOS');
  const date = typeof value.checkedOn === 'string' ? new Date(`${value.checkedOn}T00:00:00Z`) : new Date(NaN);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value.checkedOn) || Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== value.checkedOn) {
    fail('checkedOn must be a valid YYYY-MM-DD date');
  }
  const keys = new Set();
  const ids = new Set();
  if (!Array.isArray(value.sources) || value.sources.length === 0) fail('sources must be a nonempty array');
  for (const [index, source] of (Array.isArray(value.sources) ? value.sources : []).entries()) {
    if (!object(source)) { fail(`sources[${index}] must be an object`); continue; }
    for (const field of ['key', 'title', 'observedStatus']) {
      if (!text(source[field])) fail(`sources[${index}].${field} must be nonempty`);
    }
    if (keys.has(source.key)) fail(`duplicate key: ${source.key}`);
    if (text(source.key)) keys.add(source.key);
    if (typeof source.pageId !== 'string' || !/^[0-9a-f]{32}$/.test(source.pageId)) fail(`sources[${index}].pageId is invalid`);
    if (ids.has(source.pageId)) fail(`duplicate pageId: ${source.pageId}`);
    ids.add(source.pageId);
    if (source.url !== `https://app.notion.com/p/${source.pageId}`) fail(`sources[${index}].url does not match pageId`);
  }
  if (!object(value.routes)) { fail('routes must be an object'); return; }
  for (const name of requiredRoutes) {
    if (!Array.isArray(value.routes[name]) || value.routes[name].length === 0) fail(`route ${name} must be a nonempty array`);
  }
  for (const [name, route] of Object.entries(value.routes)) {
    if (!Array.isArray(route)) { fail(`route ${name} must be an array`); continue; }
    const seen = new Set();
    for (const key of route) {
      if (typeof key !== 'string' || !keys.has(key)) fail(`route ${name} references unknown source: ${String(key)}`);
      if (seen.has(key)) fail(`route ${name} contains duplicate source: ${String(key)}`);
      seen.add(key);
    }
  }
  for (const key of ['project', 'prd-home', 'architecture']) {
    if (!Array.isArray(value.routes.always) || !value.routes.always.includes(key)) fail(`route always must include ${key}`);
  }
  if (!Array.isArray(value.routes.product) || !value.routes.product.includes('current-prd')) fail('route product must include current-prd');
}

/** Read-only, offline structural checks; this is not a TDD or Notion access verifier. */
export function verifyRepository(root) {
  const errors = [];
  const contents = new Map();
  for (const path of requiredFiles) {
    try {
      const content = readFileSync(join(root, path), 'utf8');
      contents.set(path, content);
      if (!content.trim()) errors.push(`[FILE] ${path} must not be empty`);
    } catch {
      errors.push(`[FILE] ${path} is missing or unreadable`);
    }
  }
  for (const rule of hardRules) {
    if (!(contents.get('AGENTS.md') ?? '').includes(rule)) errors.push(`[AGENTS] Missing rule: ${rule}`);
  }
  const raw = contents.get('docs/notion-sources.json');
  if (raw !== undefined) {
    try { validateSources(JSON.parse(raw), errors); }
    catch { errors.push('[SOURCES] Invalid JSON registry'); }
  }
  function scan(relative = '') {
    let entries;
    try { entries = readdirSync(join(root, relative), { withFileTypes: true }); }
    catch { errors.push(`[FILE] Cannot scan ${relative || '.'}`); return; }
    for (const entry of entries) {
      const path = relative ? `${relative}/${entry.name}` : entry.name;
      if (entry.name === 'AGENTS.override.md') errors.push(`[OVERRIDE] ${path} is not allowed`);
      if (entry.isDirectory() && !ignoredDirs.has(entry.name)) scan(path);
    }
  }
  scan();
  return errors.sort();
}

const thisFile = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === thisFile) {
  const root = resolve(process.argv[2] ?? join(dirname(thisFile), '..'));
  const errors = verifyRepository(root);
  if (errors.length) {
    console.error(errors.join('\n'));
    process.exitCode = 1;
  } else {
    console.log('Repository structure checks passed (not a business/TDD/Notion verification).');
  }
}
