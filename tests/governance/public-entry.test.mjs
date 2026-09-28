import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { serverCompose, serverSchedules } from '../../infra/server/plan.mjs';
import { domainHealth } from '../../infra/server/probe.mjs';

// Oracle: accepted ADR-004 sections 5.2/15 and user deployment request, 2026-09-28.
const runtime = JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json', import.meta.url)));
const nginx = readFileSync(new URL('../../infra/server/public-nginx.conf', import.meta.url), 'utf8');

test('public HTTPS exposes only edge ports and preserves private data, images and input', () => {
  const before = structuredClone(runtime);
  const config = serverCompose(runtime, { publicTLS: true, publicAccess: true });
  assert.deepEqual(config.services.nginx.ports, ['0.0.0.0:443:19443', '0.0.0.0:80:80', '127.0.0.1:19443:19443']);
  assert.equal(config.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN, 'https://weave.hubujiu.site');
  assert.ok(config.services.nginx.volumes.includes('${WEAVEOS_RUNTIME_DIR}/public-nginx.conf:/etc/nginx/nginx.conf:ro'));
  assert.equal(config.networks.default.internal, true);
  for (const name of ['postgres', 'redis', 'bff', 'audit-maintenance']) assert.equal(config.services[name].ports, undefined);
  for (const [name, service] of Object.entries(config.services)) {
    assert.equal(service.image, before.services[name].image);
    assert.equal(service.build, undefined);
  }
  assert.deepEqual(runtime, before);
});

test('public access rejects untrusted TLS while default deployment stays private', () => {
  assert.throws(() => serverCompose(runtime, { publicAccess: true }), /public.*TLS/i);
  assert.deepEqual(serverCompose(runtime).services.nginx.ports, ['127.0.0.1:19443:19443']);
});

test('HTTP redirects all paths to the fixed HTTPS origin without trusting Host', () => {
  assert.match(nginx, /listen 80;/);
  assert.match(nginx, /return 308 https:\/\/weave\.hubujiu\.site\$request_uri;/);
  assert.doesNotMatch(nginx, /return 30[1278] https?:\/\/\$(host|http_host)/);
});

test('public TLS preserves same-origin API, non-cacheable auth and trusted proxy boundary', () => {
  assert.match(nginx, /listen 19443 ssl;/);
  assert.match(nginx, /server_name weave\.hubujiu\.site;/);
  assert.match(nginx, /ssl_certificate \/etc\/nginx\/tls\/cert.pem;/);
  assert.match(nginx, /location \/api\/ \{[^}]*proxy_pass http:\/\/authentication;/s);
  assert.match(nginx, /proxy_set_header Host \$http_host;/);
  assert.match(nginx, /proxy_set_header X-Forwarded-For \$remote_addr;/);
  assert.match(nginx, /proxy_set_header X-Forwarded-Proto https;/);
  for (const header of ['X-User-Id', 'X-Role', 'X-Tenant-Id']) assert.ok(nginx.includes(`proxy_set_header ${header} "";`));
  assert.match(nginx, /proxy_cache off;/);
  assert.match(nginx, /add_header Cache-Control "no-store" always;/);
  assert.match(nginx, /location \/ \{ try_files \$uri \$uri\/ \/index.html; \}/);
});

test('existing loopback health and DNS-01 renewal still work independently of public DNS', async () => {
  let options;
  const request = (o, callback) => { options = o; return { once() {}, setTimeout() {}, end() { callback({ statusCode: 200, resume() {} }); } }; };
  assert.equal(await domainHealth('ready', request), true);
  assert.equal(options.port, 19443);
  assert.equal(options.servername, 'weave.hubujiu.site');
  assert.equal(options.rejectUnauthorized, true);
  options.lookup(options.hostname, {}, (error, address) => { assert.equal(error, null); assert.equal(address, '127.0.0.1'); });
  const cron = serverSchedules({ publicTLS: true });
  assert.match(cron, /33 2,14.*acme-run\.mjs renew/);
  assert.doesNotMatch(cron, /NODE_EXTRA_CA_CERTS/);
});
