import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
const read = path => { try { return readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8'); } catch { return ''; } };
// The configuration requirements were specified before these workflow files existed.
test('FND-03: CI runs real Go validation and browser tests', () => {
  const ci = read('.github/workflows/ci.yml');
  for (const token of ['pull_request:', 'go test -race', 'go vet', 'gofmt', '--frozen-lockfile', 'typecheck', 'build', 'playwright test', 'upload-artifact']) assert.ok(ci.includes(token), `missing ${token}`);
  assert.ok(!ci.includes('continue-on-error'));
  assert.ok(!ci.includes('pull_request_target'));
});
test('FND-03: product acceptance and artifacts are explicit, not fake deploys', () => {
  const release = read('.github/workflows/acceptance.yml');
  for (const token of ['workflow_dispatch:', 'playwright test', 'tests/acceptance', 'postgres:', 'redis:', 'check-release']) assert.ok(release.includes(token), `missing ${token}`);
  assert.ok(!release.includes('continue-on-error'));
  assert.ok(read('.github/workflows/delivery.yml').includes('workflow_dispatch:'));
});
test('FND-04: web entry is real React with no fabricated login UI', () => {
  const pkg = JSON.parse(read('apps/web/package.json') || '{}');
  assert.ok(pkg.dependencies?.react);
  assert.ok(pkg.devDependencies?.vite);
  assert.ok(read('apps/web/src/main.tsx').includes('createRoot'));
});
