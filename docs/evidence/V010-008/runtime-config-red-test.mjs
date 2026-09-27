import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const root=new URL('../../',import.meta.url);
const read=path=>readFileSync(new URL(path,root),'utf8');
test('CI and full product pipeline provide separate real archive storage before audit tests',()=>{
 const ci=read('.github/workflows/ci.yml'),runner=read('infra/acceptance/run.mjs');
 assert.ok(ci.includes('WEAVEOS_TEST_ARCHIVE_DATABASE_URL')&&ci.includes('weaveos_ci_archive_test'),'CI needs independent archive test database');
 assert.ok(runner.includes('WEAVEOS_TEST_ARCHIVE_DATABASE_URL')&&runner.includes('weaveos_ci_archive_test'),'full product pipeline must run audit against independent database');
});
test('local runtime Redis cannot resurrect historical Session files or silently evict sessions',()=>{
 const redis=JSON.parse(read('infra/runtime/compose.json')).services.redis;
 assert.ok(redis,'dedicated runtime Redis required');
 const cmd=redis.command;
 assert.ok(cmd.includes('--save')&&cmd[cmd.indexOf('--save')+1]==='','RDB must be disabled');
 assert.ok(cmd.includes('--appendonly')&&cmd[cmd.indexOf('--appendonly')+1]==='no','AOF must be disabled');
 assert.ok(cmd.includes('--maxmemory-policy')&&cmd[cmd.indexOf('--maxmemory-policy')+1]==='noeviction');
 assert.ok(cmd.includes('--maxmemory'),'explicit Session capacity required');
 assert.equal(redis.ports,undefined,'Redis must be internal');
});
test('runtime uses restricted immutable applications and automatic controlled audit maintenance',()=>{
 const s=JSON.parse(read('infra/runtime/compose.json')).services;
 assert.ok(s['audit-maintenance'],'automatic maintenance process required');
 for(const name of ['bff','audit-maintenance']){
  const service=s[name];assert.equal(service.user,'65532:65532');assert.equal(service.read_only,true);assert.deepEqual(service.cap_drop,['ALL']);
  assert.ok(service.image.includes('${WEAVEOS_'),'immutable verified artifact selected by run record');assert.equal(service.ports,undefined);
  assert.ok(!JSON.stringify(service.volumes??[]).includes('docker.sock'));
 }
 assert.ok(s['audit-maintenance'].env_file[0].includes('maintenance.env'));
 assert.ok(s.bff.env_file[0].includes('runtime.env'));
 assert.equal(s.postgres.ports,undefined);
 assert.ok(s.nginx.ports.every(p=>p.startsWith('127.0.0.1:')),'Q5/Q6 authorize local exposure only');
});
