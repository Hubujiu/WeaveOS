import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { open } from './redis-observer.mjs';
// Tests the test observer against a real isolated Redis, never a business mock.
test('Redis observer uses native expiration on only the supplied session', async () => {
 const container=process.env.WEAVEOS_OBSERVER_TEST_CONTAINER;
 assert.match(container ?? '', /^weaveos-v010009-redis$/);
 const cli=(...args)=>execFileSync('docker',['exec',container,'redis-cli',...args],{encoding:'utf8'}).trim();
 const raw=Buffer.alloc(32,9),sid=raw.toString('base64url'), generation='weaveos-v010-009-observer-test';
 const key='ems:auth:session:'+generation+':v1:'+createHash('sha256').update(raw).digest('hex');
 const peer=key+':peer'; cli('SET',key,'{}','PX','3600000');cli('SET',peer,'{}','PX','3600000');
 const o=await open({redisURL:'redis://127.0.0.1:26379/0',generation});
 try { await o.expireSession('__Host-session='+sid); assert.equal(cli('PTTL',key),'-2');assert.equal(cli('EXISTS',peer),'1'); }
 finally { cli('DEL',key,peer);await o.close(); }
});
