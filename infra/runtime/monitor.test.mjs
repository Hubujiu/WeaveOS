import test from 'node:test';import assert from 'node:assert/strict';
import {readFileSync,mkdirSync} from 'node:fs';import {resolve} from 'node:path';
import {alarms,receiveAlarms} from './monitor.mjs';
const good={ready:true,live:true,database:true,redis:true,redisUsedBytes:10,redisLimitBytes:100,containerMemoryRatios:[0.1],diskFreeRatio:0.9,certificateRemainingMs:10*86400000,backupFailed:false};
test('operational faults produce fixed safe alarms from independently defined samples',()=>{
 assert.deepEqual(alarms(good),[]);
 for(const [key,value,code] of [['ready',false,'READINESS'],['live',false,'LIVENESS'],['database',false,'DATABASE'],['redis',false,'REDIS'],['redisUsedBytes',90,'REDIS_CAPACITY'],['containerMemoryRatios',[0.91],'MEMORY'],['diskFreeRatio',0.05,'DISK'],['certificateRemainingMs',86400000,'CERTIFICATE'],['backupFailed',true,'BACKUP']])assert.ok(alarms({...good,[key]:value}).includes(code),code);
});
test('local receiver actually persists redacted alerts, including backup failure',()=>{
 const dir=resolve('.work/monitor-test',String(Date.now()));mkdirSync(dir,{recursive:true});const file=resolve(dir,'alerts.jsonl');
 receiveAlarms(file,{...good,backupFailed:true,password:'must-not-be-copied',cookie:'must-not-be-copied'});
 const receipt=JSON.parse(readFileSync(file,'utf8'));assert.deepEqual(receipt.codes,['BACKUP']);assert.equal(receipt.receiver,'local operator review');assert.equal(Object.keys(receipt).length,3);
});
