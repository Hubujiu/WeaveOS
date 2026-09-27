import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { assertResponseSchema } from '../../tests/acceptance/response-schema.mjs';
const root = fileURLToPath(new URL('../../', import.meta.url));
const project = process.env.WEAVEOS_ACCEPTANCE_PROJECT;
assert.ok(/^weaveos-v010-007-\d+$/.test(project ?? ''), 'Only this runner-owned project may be fault-injected');
const compose = (...args) => execFileSync('docker', ['compose','-p',project,'-f',fileURLToPath(new URL('./compose.json',import.meta.url)),...args], { cwd:root, stdio:'pipe' });
const base = 'https://localhost:19443';
const f = JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES));
const request = (path, options = {}) => fetch(base+path, { signal:AbortSignal.timeout(10000), ...options });
async function ready() {
  for(let i=0;i<30;i++) { try { if((await request('/health/ready')).status===200)return; } catch {} await new Promise(resolve=>setTimeout(resolve,500)); }
  throw new Error('Owned isolated stack did not become ready');
}
test('ADR-004: unknown API stays JSON404; untrusted identity/forwarding cannot grant access',async()=>{
  await ready();
  const unknown=await request('/api/unknown');
  assert.equal(unknown.status,404);assert.ok(unknown.headers.get('content-type').includes('application/json'));
  await assertResponseSchema(unknown,'/api/unknown','GET');
  const forged=await request('/api/v1/sessions/current',{headers:{'X-User-Id':'bootstrap','X-Role':'ALL','X-Forwarded-For':'127.0.0.1'}});
  assert.equal(forged.status,401);
});
for(const dependency of ['redis','postgres']) test(`Real ${dependency} outage denies authenticated access and login; readiness recovers`,async()=>{
  await ready();
  const login=await request('/api/v1/sessions',{method:'POST',headers:{Origin:base,'Content-Type':'application/json'},body:JSON.stringify(f.user)});
  assert.equal(login.status,201);
  const cookie=login.headers.getSetCookie().map(value=>value.split(';')[0]).join('; ');
  compose('stop',dependency);
  try {
    assert.equal((await request('/health/ready')).status,503);
    const current=await request('/api/v1/sessions/current',{headers:{Cookie:cookie}});
    assert.equal(current.status,503);
    await assertResponseSchema(current,'/api/v1/sessions/current','GET');
    const failed=await request('/api/v1/sessions',{method:'POST',headers:{Origin:base,'Content-Type':'application/json'},body:JSON.stringify(f.user)});
    assert.equal(failed.status,503);assert.equal(failed.headers.get('set-cookie'),null);
    await assertResponseSchema(failed,'/api/v1/sessions','POST');
  } finally {compose('start',dependency);await ready();}
});
