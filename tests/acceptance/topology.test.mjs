import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import { test } from 'node:test';

const compose = JSON.parse(readFileSync(new URL('../../infra/acceptance/compose.json', import.meta.url)));
test('ADR-003/004: isolated PG18/Redis8.2 and ingress preserve service boundaries', () => {
  assert.equal(compose.services.postgres?.image, 'postgres:18.0');
  assert.equal(compose.services.redis?.image, 'redis:8.2.1');
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
  for (const line of ['proxy_set_header X-Forwarded-For $remote_addr;', 'proxy_set_header X-User-Id "";', 'proxy_set_header X-Role "";', 'proxy_cache off;', 'location /api/', 'proxy_pass http://bff:8080;', 'ssl_certificate ']) assert.ok(config.includes(line), `missing ${line}`);
});
