import test from 'node:test';import assert from 'node:assert/strict';import {resolve} from 'node:path';import {readFileSync} from 'node:fs';
import {runtimeContext} from './context.mjs';import {sampleRuntime} from './probe.mjs';import {receiveAlarms} from './monitor.mjs';
const c=runtimeContext(),receipt=resolve(c.dir,'public/alerts.jsonl');
test('live operational probe observes healthy service, capacity failure and local certificate expiry alarm',async()=>{
 const normal=await sampleRuntime(c);
 assert.equal(normal.ready,true);assert.equal(normal.live,true);assert.equal(normal.database,true);assert.equal(normal.redis,true);
 assert.ok(normal.redisLimitBytes>0&&normal.diskFreeRatio>0&&normal.containerMemoryRatios.length===5);
 assert.ok(receiveAlarms(receipt,normal).includes('CERTIFICATE'),'two-day simulation certificate must warn before expiry');
 c.compose('exec','-T','redis','redis-cli','CONFIG','SET','maxmemory','1');
 try{assert.ok(receiveAlarms(receipt,await sampleRuntime(c)).includes('REDIS_CAPACITY'));}
 finally{c.compose('exec','-T','redis','redis-cli','CONFIG','SET','maxmemory','128mb');}
 c.compose('stop','redis');
 try{const unavailable=await sampleRuntime(c);assert.equal(unavailable.live,true);assert.equal(unavailable.ready,false);assert.equal(unavailable.redis,false);assert.ok(receiveAlarms(receipt,unavailable).includes('REDIS'));}
 finally{c.compose('start','redis');}
 const codes=readFileSync(receipt,'utf8').trim().split('\n').flatMap(s=>JSON.parse(s).codes);assert.ok(codes.includes('CERTIFICATE')&&codes.includes('REDIS_CAPACITY')&&codes.includes('REDIS'));
});
