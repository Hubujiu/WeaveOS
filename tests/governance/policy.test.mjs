import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { verifyRepository } from '../../scripts/verify-repo.mjs';

// Expected behavior is specified in docs/repository-checks.md (GOV-01..04).
// Fixtures intentionally do not import implementation constants or generate expected results from it.
const required = [
  'AGENTS.md', 'README.md', 'CONTRIBUTING.md', 'SECURITY.md',
  'docs/notion-router.md', 'docs/notion-sources.json', 'docs/testing.md',
  'docs/project-baseline.md', 'docs/templates/task-record.md',
  '.github/pull_request_template.md', 'apps/web/AGENTS.md',
  'services/bff/AGENTS.md', 'contracts/AGENTS.md', 'db/AGENTS.md',
  'infra/AGENTS.md', 'tests/AGENTS.md',
];
const markers = ['NOTION-GATE', 'TDD-RED-FIRST', 'TEST-ORACLE', 'NO-FAKE-VERIFICATION', 'CHILD-RULES'];
const cli = fileURLToPath(new URL('../../scripts/verify-repo.mjs', import.meta.url));

function put(root, path, text) {
  mkdirSync(dirname(join(root, path)), { recursive: true });
  writeFileSync(join(root, path), text);
}
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'weaveos-policy-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const path of required) put(root, path, '# Fixture\n');
  put(root, 'AGENTS.md', markers.join('\n'));
  const keys = ['project', 'prd-home', 'architecture', 'current-prd'];
  const sources = keys.map((key, index) => {
    const pageId = index === 0 ? '3e42f5a9e64880ae9cf5ebbd2088d773' : String(index).padStart(32, '0');
    return { key, pageId, url: `https://app.notion.com/p/${pageId}`, title: key, observedStatus: 'fixture' };
  });
  const registry = {
    schemaVersion: 1, projectId: sources[0].pageId, checkedOn: '2026-09-24', sources,
    routes: {
      always: ['project', 'prd-home', 'architecture'], product: ['current-prd'],
      auth: ['current-prd'], api: ['architecture'], database: ['architecture'],
      frontend: ['current-prd'], infrastructure: ['architecture'], governance: ['project'],
    },
  };
  put(root, 'docs/notion-sources.json', JSON.stringify(registry));
  return root;
}
function mutate(root, change) {
  const path = 'docs/notion-sources.json';
  const value = JSON.parse(readFileSync(join(root, path), 'utf8'));
  change(value);
  put(root, path, JSON.stringify(value));
}
function reports(root, pattern) {
  const errors = verifyRepository(root);
  assert.ok(Array.isArray(errors), 'checker returns an array');
  assert.ok(errors.some(error => pattern.test(error)), `Expected ${pattern}; received ${JSON.stringify(errors)}`);
}

test('GOV-01..04: valid repository passes without mutation', t => {
  const root = fixture(t);
  const before = readFileSync(join(root, 'docs/notion-sources.json'), 'utf8');
  assert.deepEqual(verifyRepository(root), []);
  assert.equal(readFileSync(join(root, 'docs/notion-sources.json'), 'utf8'), before);
});
test('GOV-01: missing file fails with its path', t => {
  const root = fixture(t);
  rmSync(join(root, 'SECURITY.md'));
  reports(root, /\[FILE\].*SECURITY\.md/);
});
test('GOV-01: whitespace-only required file is not valid', t => {
  const root = fixture(t);
  put(root, 'docs/testing.md', ' \n\t');
  reports(root, /\[FILE\].*docs\/testing\.md/);
});
test('GOV-02: deleting each hard-rule marker fails', t => {
  const root = fixture(t);
  for (const marker of markers) {
    put(root, 'AGENTS.md', markers.filter(value => value !== marker).join('\n'));
    reports(root, new RegExp(`\\[AGENTS\\].*${marker}`));
  }
});
test('GOV-03: malformed JSON is reported, not thrown', t => {
  const root = fixture(t);
  put(root, 'docs/notion-sources.json', '{broken');
  reports(root, /\[SOURCES\].*JSON/);
});
test('GOV-03: incorrect version and project fail', t => {
  const root = fixture(t);
  mutate(root, value => { value.schemaVersion = 2; value.projectId = 'another-project'; });
  reports(root, /\[SOURCES\].*schemaVersion/);
  reports(root, /\[SOURCES\].*projectId/);
});
test('GOV-03: impossible calendar date fails', t => {
  const root = fixture(t);
  mutate(root, value => { value.checkedOn = '2026-02-30'; });
  reports(root, /\[SOURCES\].*checkedOn/);
});
test('GOV-03: empty sources and non-object registry fail', t => {
  const root = fixture(t);
  mutate(root, value => { value.sources = []; });
  reports(root, /\[SOURCES\].*sources/);
  put(root, 'docs/notion-sources.json', 'null');
  reports(root, /\[SOURCES\]/);
});
test('GOV-03: duplicate keys and page IDs fail', t => {
  const root = fixture(t);
  mutate(root, value => { value.sources.push({ ...value.sources[0] }); });
  reports(root, /\[SOURCES\].*duplicate.*key/);
  reports(root, /\[SOURCES\].*duplicate.*pageId/);
});
test('GOV-03: external or mismatched source URLs fail', t => {
  const root = fixture(t);
  mutate(root, value => { value.sources[0].url = 'https://example.com/fake'; });
  reports(root, /\[SOURCES\].*url/);
});
test('GOV-03: missing metadata and malformed page ID fail', t => {
  const root = fixture(t);
  mutate(root, value => { value.sources[0].observedStatus = ''; value.sources[0].pageId = 'bad'; });
  reports(root, /\[SOURCES\].*observedStatus/);
  reports(root, /\[SOURCES\].*pageId/);
});
test('GOV-03: unknown, duplicate and empty route references fail', t => {
  const root = fixture(t);
  mutate(root, value => {
    value.routes.auth = ['missing']; value.routes.api = ['architecture', 'architecture'];
    value.routes.database = [];
  });
  reports(root, /\[SOURCES\].*auth/);
  reports(root, /\[SOURCES\].*api/);
  reports(root, /\[SOURCES\].*database/);
});
test('GOV-03: mandatory routes and always/product anchors cannot disappear', t => {
  const root = fixture(t);
  mutate(root, value => { delete value.routes.governance; value.routes.always = ['project']; value.routes.product = ['project']; });
  reports(root, /\[SOURCES\].*governance/);
  reports(root, /\[SOURCES\].*always/);
  reports(root, /\[SOURCES\].*product/);
});
test('GOV-04: nested instruction override fails', t => {
  const root = fixture(t);
  put(root, 'apps/web/nested/AGENTS.override.md', 'override');
  reports(root, /\[OVERRIDE\].*apps\/web\/nested\/AGENTS\.override\.md/);
});
test('GOV-04: generated/dependency directories are ignored', t => {
  const root = fixture(t);
  put(root, 'node_modules/dependency/AGENTS.override.md', 'vendor');
  put(root, '.work/AGENTS.override.md', 'temporary');
  assert.deepEqual(verifyRepository(root), []);
});
test('GOV-01: nonexistent root reports errors', () => {
  reports(resolve(tmpdir(), 'weaveos-nonexistent', 'missing-root'), /\[FILE\]/);
});
test('CLI: valid repository exits zero', t => {
  const result = spawnSync(process.execPath, [cli, fixture(t)], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
});
test('CLI: invalid repository exits one and explains failure', t => {
  const root = fixture(t);
  rmSync(join(root, 'AGENTS.md'));
  const result = spawnSync(process.execPath, [cli, root], { encoding: 'utf8' });
  assert.equal(result.status, 1);
  assert.match(result.stdout + result.stderr, /AGENTS\.md/);
});
