// Authored by the coordinating assistant. Scope: migration-registration metadata only.
// Oracle: approved V030-013 cold4/hot7-12 expansion; published registrations must not
// disappear or change. Passing this file does not establish deploy/restore safety.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const manifest = JSON.parse(readFileSync(resolve(root, 'infra/server/deploy/compatibility.json'), 'utf8'));
const prior = [
  {
    "path": "migrations/00001_auth.sql",
    "sha256": "ab5c96db2fb775501df3fc2c55a16f9ffd6c8630c6e9f875aa238230b6b522b4"
  },
  {
    "path": "archive-migrations/00001_archive.sql",
    "sha256": "8896aa6c3a43d661425fefb5dff001222bef827cd8e4b5093ae409ac4179db2e"
  },
  {
    "path": "migrations/00002_personnel.sql",
    "sha256": "544512f1654ae0a96f51e0fa24f61b58e3f15c8a2e3fa2928b0dd969e5a1b91e"
  },
  {
    "path": "archive-migrations/00002_personnel_audit.sql",
    "sha256": "7276893525b0a06176f63a1845287182dc7ec2589952849789ab9490b975c995"
  },
  {
    "path": "migrations/00003_query_drafts.sql",
    "sha256": "9b1f546d158931d09245070a4d5b6b46dd64b128b008363ae7a44f74bae8860a"
  },
  {
    "path": "migrations/00004_query_revision_writers.sql",
    "sha256": "85faf7406d0e82ce7ca14aaf10276f80ef5a88a5c957170f111a261e910c925d"
  },
  {
    "path": "migrations/00005_table_presets.sql",
    "sha256": "9f8ec8b86e7d7ab208940b88f43283ca11895a34e18b3af328e25b1e8ba40509"
  },
  {
    "path": "archive-migrations/00003_apps_audit.sql",
    "sha256": "70c8493acefb442c354f412ddf25b140f8361f930353fcf4cd342d257db0f4ab"
  },
  {
    "path": "migrations/00006_apps_policy.sql",
    "sha256": "1cca96dc6e5b7acf11c0c9f40364d1fdb5d9d57b4f8ddd8b0036dd5b5bf15fb0"
  }
];
const additions = [
  'archive-migrations/00004_app_structure_audit.sql',
  'migrations/00007_app_structure.sql',
  'migrations/00008_app_records.sql',
  'migrations/00009_record_save_history.sql',
  'migrations/00010_member_source_label_width.sql',
  'migrations/00011_record_reference_defaults.sql',
  'migrations/00012_actual_reference_defaults.sql',
];

test('Root R03: every approved migration is registered once with its actual source digest', () => {
  assert.ok(Array.isArray(manifest.migrations), 'migrations must be an array');
  const paths = manifest.migrations.map(entry => entry.path);
  assert.equal(new Set(paths).size, paths.length, 'duplicate migration registration');
  for (const path of [...prior.map(entry => entry.path), ...additions]) {
    const entries = manifest.migrations.filter(entry => entry.path === path);
    assert.equal(entries.length, 1, `missing or duplicate reviewed migration: ${path}`);
    assert.match(entries[0].sha256, /^[0-9a-f]{64}$/, `invalid SHA-256 shape: ${path}`);
    const actual = createHash('sha256').update(readFileSync(resolve(root, 'db', path))).digest('hex');
    assert.equal(entries[0].sha256, actual, `registered digest differs from immutable migration bytes: ${path}`);
  }
});

test('Root R03: prior reviewed migration registrations retain their exact paths and hashes', () => {
  for (const entry of prior) {
    const actual = manifest.migrations.find(item => item.path === entry.path);
    assert.deepEqual(actual, entry, `previously reviewed registration changed: ${entry.path}`);
  }
});

test('Root R03: the new cold audit registration precedes all dependent hot registrations', () => {
  const paths = manifest.migrations.map(entry => entry.path);
  const cold = paths.indexOf(additions[0]);
  assert.ok(cold >= 0, 'cold4 registration is required');
  let previous = cold;
  for (const path of additions.slice(1)) {
    const index = paths.indexOf(path);
    assert.ok(index > previous, `migration order must preserve cold4 then ascending hot7-12: ${path}`);
    previous = index;
  }
});

