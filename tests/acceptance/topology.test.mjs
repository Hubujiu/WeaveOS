import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import { test } from 'node:test';

const compose = JSON.parse(readFileSync(new URL('../../infra/acceptance/compose.json', import.meta.url)));
test('ADR-003/004: isolated PG18/Redis8.2 and ingress preserve service boundaries', () => {
  assert.equal(compose.services.postgres?.image, 'postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722');
  assert.equal(compose.services.redis?.image, 'redis:8.2.10@sha256:164c759a0c342ee69d08fc99219382b0fd682181465c0df2e0e6911f4c85d73c');
  for (const name of ['postgres', 'redis', 'bff']) assert.equal(compose.services[name]?.ports, undefined);
  assert.deepEqual(compose.services.nginx?.ports, ['127.0.0.1:19443:19443']);
  assert.equal(compose.services.bff?.environment.WEAVEOS_TRUSTED_PROXY_HOSTS, 'nginx');
  assert.equal(compose.services.bff?.environment.WEAVEOS_PUBLIC_ORIGIN, 'https://localhost:19443');
});
test('Product CI executes migration, HTTPS real API and three browsers; final release remains gated', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/acceptance.yml', import.meta.url), 'utf8');
  assert.ok(workflow.includes('infra/acceptance/run.mjs'), 'reproducible migrated HTTPS acceptance runner required');
  assert.ok(workflow.includes('pull_request:'), 'product acceptance must run on the final PR head');
  assert.ok(workflow.includes('check-release.mjs'), 'full release evidence gate must remain available');
});
test('ADR-004: ingress overwrites forwarded identity, disables shared auth cache and separates API from SPA', () => {
  const config = readFileSync(new URL('../../infra/acceptance/nginx.conf', import.meta.url), 'utf8');
  for (const line of ['proxy_set_header X-Forwarded-For $remote_addr;', 'proxy_set_header X-User-Id "";', 'proxy_set_header X-Role "";', 'proxy_cache off;', 'location /api/', 'server bff:8080 resolve;', 'resolver 127.0.0.11', 'ssl_certificate ']) assert.ok(config.includes(line), `missing ${line}`);
});
test('Browser container uses init to reap children and explicit isolated shared memory', () => {
  const runner = readFileSync(new URL('../../infra/acceptance/run.mjs', import.meta.url), 'utf8');
  assert.ok(runner.includes("'--init'"), 'Playwright Docker recommends init for browser child processes');
  assert.ok(runner.includes("'--shm-size=1g'"), 'Explicit isolated browser shared-memory allocation required');
});
test('Private container-produced fixture is assigned to the authorized host runner without world-readable permissions', () => {
  const runner = readFileSync(new URL('../../infra/acceptance/run.mjs', import.meta.url), 'utf8');
  assert.ok(runner.includes('process.getuid()'), 'Linux fixture consumer UID must be used');
  assert.ok(runner.includes("'chown'"), 'Container-created private file ownership must match the authorized consumer');
  assert.ok(!runner.includes('chmod 644'), 'Do not expose private fixtures to solve ownership');
});

test('Expanded acceptance evidence reports the actual complete HTTP and browser results', () => {
  // R3 adds personnel API and browser cases. Actual TAP/JSON reports replace the old fixed inventory.
  for (const path of ['../../infra/acceptance/run.mjs', '../../infra/runtime/run.mjs']) {
    const runner = readFileSync(new URL(path, import.meta.url), 'utf8');
    assert.match(runner, /api:\s*countAPIReport\(readFileSync/);
    assert.match(runner, /browser:\s*countBrowserReport\(JSON\.parse\(readFileSync/);
  }
});

test('Product runner executes all 19 storage cases and real observer qualifications', () => {
  const runner = readFileSync(new URL('../../infra/acceptance/run.mjs', import.meta.url), 'utf8');
  for (const file of ['integration.test.mjs','storage-observer.test.mjs','redis-gate.test.mjs']) assert.ok(runner.includes(file), `${file} must execute in product CI`);
  assert.match(runner, /storageCases:\s*19/);
  assert.ok(runner.includes('WEAVEOS_ACCEPTANCE_OBSERVER'));
});

test('Root R25 acceptance runtime injects dedicated definition key and explicit schema budgets before startup', async () => {
  const { runAcceptance } = await import('../../infra/acceptance/run.mjs');
  const { mkdtempSync, rmSync, statSync } = await import('node:fs');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  const keys = [];
  for (let run = 0; run < 2; run++) {
    const directory = mkdtempSync(join(tmpdir(), 'weaveos-root-r25-'));
    const stop = new Error('Root test stops before TLS generation and Docker startup');
    let calls = 0;
    try {
      await assert.rejects(runAcceptance({ directory, execute(command, args) {
        calls++;
        assert.ok(/openssl(?:\.exe)?$/.test(command), 'first external action must be TLS setup');
        assert.equal(args[0], 'req');
        throw stop;
      }}), error => error === stop);
      assert.equal(calls, 1, 'no actual services started');
      const file = join(directory, 'runtime.env');
      const values = Object.fromEntries(readFileSync(file, 'utf8').trim().split(/\r?\n/).map(line => {
        const index = line.indexOf('=');
        return [line.slice(0, index), line.slice(index + 1)];
      }));
      const key = values.WEAVEOS_DEFINITION_HMAC_KEY;
      assert.ok(typeof key === 'string' && Buffer.from(key, 'base64').length === 32, 'dedicated 32-byte definition key required; value withheld');
      assert.ok(Buffer.from(key, 'base64').toString('base64') === key, 'canonical base64 definition key required');
      assert.ok(key !== values.WEAVEOS_AUDIT_HMAC_KEY, 'definition key must not reuse audit key');
      assert.match(values.WEAVEOS_DEFINITION_KEY_ID ?? '', /^[A-Za-z0-9_-]{1,16}$/);
      assert.equal(values.WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS, '1000');
      assert.equal(values.WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS, '5000');
      if (process.platform !== 'win32') assert.equal(statSync(file).mode & 0o777, 0o600);
      keys.push(key);
    } finally { rmSync(directory, { recursive: true, force: true }); }
  }
  assert.ok(keys[0] !== keys[1], 'independent isolated runs must generate separate definition keys');
});

test('Root R25 immutable-runtime simulation supplies the same explicit definition configuration', () => {
  const runner = readFileSync(new URL('../../infra/runtime/run.mjs', import.meta.url), 'utf8');
  const runtime = runner.match(/file\('runtime\.env',([\s\S]*?)\);/);
  assert.ok(runtime, 'existing private runtime env writer required');
  for (const key of ['WEAVEOS_DEFINITION_HMAC_KEY', 'WEAVEOS_DEFINITION_KEY_ID', 'WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS', 'WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS']) {
    assert.ok(runtime[1].includes(key + '='), key + ' must be in the private runtime environment');
  }
  assert.match(runtime[1], /WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS=1000/);
  assert.match(runtime[1], /WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS=5000/);
});
