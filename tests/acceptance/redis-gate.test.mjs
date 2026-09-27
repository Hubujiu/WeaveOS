import assert from 'node:assert/strict';
import { test } from 'node:test';
import { execFile, execFileSync } from 'node:child_process';
import { promisify } from 'node:util';
import { resolve } from 'node:path';

test('transport barrier holds actual Redis EVAL while independent DEL proceeds', async () => {
  const project = process.env.WEAVEOS_ACCEPTANCE_PROJECT;
  assert.match(project ?? '', /^weaveos-v010-007-\d+$/);
  const name = `${project}-gate-qualification`, redisName = `${project}-redis-1`;
  const docker = args => execFileSync('docker', args, {encoding:'utf8',stdio:'pipe'}).trim();
  const native = (...args) => docker(['exec', redisName, 'redis-cli', '--raw', ...args]);
  const key = `ems:auth:session:${project}:v1:${'a'.repeat(64)}`;
  docker(['run','-d','--name',name,'--network',`${project}_default`,'-p','127.0.0.1::6380','--mount',`type=bind,src=${resolve('tests/acceptance/redis-gate.mjs')},dst=/gate.mjs,readonly`,'node:24.14.0-bookworm-slim','node','/gate.mjs','--serve']);
  let inFlight;
  try {
    const port = JSON.parse(docker(['inspect',name]))[0].NetworkSettings.Ports['6380/tcp'][0].HostPort;
    const url = `http://127.0.0.1:${port}`;
    for (let i=0;i<50;i++) { try { if ((await fetch(url+'/status')).ok) break; } catch {} await new Promise(resolve=>setTimeout(resolve,100)); }
    native('SET',key,'before','PX','60000');
    await fetch(url+'/arm',{method:'POST',body:JSON.stringify({key})});
    inFlight = promisify(execFile)('docker',['exec',redisName,'redis-cli','-h',name,'EVAL',"return redis.call('SET',KEYS[1],'after','XX','PX',60000)",'1',key],{encoding:'utf8'});
    for (let i=0;i<100;i++) { if ((await (await fetch(url+'/status')).json()).observed) break; await new Promise(resolve=>setTimeout(resolve,10)); }
    assert.equal(native('GET',key),'before','armed EVAL must not execute before release');
    assert.equal(native('DEL',key),'1','DEL must proceed while EVAL is held');
    await fetch(url+'/release',{method:'POST'}); await inFlight;
    assert.equal(native('EXISTS',key),'0','native SET XX cannot recreate deleted key');
  } finally { docker(['rm','-f',name]); await inFlight?.catch(()=>{}); native('DEL',key); }
});
