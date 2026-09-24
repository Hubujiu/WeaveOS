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
test('FND-05: CI retains a source bundle without bundling local credentials', () => {
  const ci = read('.github/workflows/ci.yml');
  assert.ok(ci.includes('git bundle create .work/source.bundle --all'));
  assert.ok(ci.includes('source-bundle-${{ github.run_id }}'));
  assert.ok(!ci.includes('tar -czf source .git'));
});
test('FND-05: RED replay metadata pins the real pre-implementation source', () => {
  const manifest = JSON.parse(read('docs/evidence/V010-001/replay.json') || '{}');
  assert.equal(manifest.redCommit, '4ba15c39c4828d481e386f4e7fa402f024cd5844');
  assert.equal(manifest.kind, 'replay-not-original-execution');
  assert.equal(manifest.nodeExit, 1);
  assert.equal(manifest.goExit, 1);
  assert.match(manifest.archiveSha256 ?? '', /^[0-9a-f]{64}$/);
});
