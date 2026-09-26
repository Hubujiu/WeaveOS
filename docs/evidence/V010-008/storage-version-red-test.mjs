import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';import {execFileSync} from 'node:child_process';
// Current approved release lines, independently verified against PostgreSQL
// release 18.6 and Redis 8.2.10 official security notes on 2026-09-27.
for(const profile of ['acceptance','runtime']){
 const services=JSON.parse(readFileSync(`infra/${profile}/compose.json`,'utf8')).services;
 for(const [name,binary,expected] of [['postgres','postgres',/PostgreSQL\) 18\.6\b/],['redis','redis-server',/v=8\.2\.10\b/]]){
  test(`${profile} actually executes the patched ${name} release`,()=>{
   const version=execFileSync('docker',['run','--rm','--network','none','--entrypoint',binary,services[name].image,'--version'],{encoding:'utf8',timeout:180000});
   assert.match(version,expected,'configured image must contain the approved security patch');
  });
 }
}
