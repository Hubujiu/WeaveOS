import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, rmSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { serverCompose, requireEmptyDirectory, serverSchedules } from '../../infra/server/plan.mjs';
import { serverContext } from '../../infra/server/context.mjs';
const runtime=JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url)));
test('Q15 server exposes only loopback HTTPS and preserves internal data network',()=>{
 const c=serverCompose(runtime);
 assert.deepEqual(c.services?.nginx?.ports,['127.0.0.1:19443:19443']);
 assert.equal(c.networks?.default?.internal,true);
 for(const s of Object.values(c.services)) assert.equal(s.build,undefined);
 for(const name of ['postgres','redis','bff','audit-maintenance']) assert.equal(c.services[name].ports,undefined);
});
test('operational schedules retain TLS verification, prevent overlap, and invoke only runtime tools',()=>{
 const cron=serverSchedules();
 assert.match(cron,/^\*\/5 \* \* \* \* root /m);
 assert.match(cron,/^15 3 \* \* \* root /m);
 assert.match(cron,/NODE_EXTRA_CA_CERTS=\/opt\/weaveos-v010\/tls\/cert.pem/);
 assert.equal((cron.match(/flock -n/g)??[]).length,2);
 assert.match(cron,/operations\.mjs monitor/);
 assert.match(cron,/operations\.mjs backup/);
 assert.doesNotMatch(cron,/playwright|pnpm|go test|build/);
});
test('server maintenance addresses only the authorized deployment and does not run test/build commands',()=>{
 assert.throws(()=>serverContext('/opt/unrelated'),/authorized/i);
 const calls=[];
 const command=(bin,args,options)=>{calls.push({bin,args,options});return Buffer.from(args.includes('inspect')?'/weaveos-v010-011-postgres-1\n':args.includes('-At')?'1\n':'container-id\n');};
 const c=serverContext('/opt/weaveos-v010',command);
 c.sql('postgres','SELECT 1;');
 assert.deepEqual(calls[0].args,['compose','--env-file','/opt/weaveos-v010/.env','-p','weaveos-v010-011','-f','/opt/weaveos-v010/compose.json','ps','-aq','postgres']);
 assert.equal(calls.at(-1).options.input,'SELECT 1;');
 assert.equal(calls.at(-1).args.at(-1),'postgres');
 assert.ok(calls.every(call=>!call.args.includes('build')&&!call.args.includes('test')));
});
test('finished services restart automatically and have finite resource and log budgets',()=>{
 const c=serverCompose(runtime);
 assert.deepEqual(Object.keys(c.services??{}).sort(),['audit-maintenance','bff','nginx','postgres','redis']);
 for(const s of Object.values(c.services)){assert.equal(s.restart,'unless-stopped');assert.ok(s.mem_limit);assert.ok(s.logging.options['max-size']);}
 assert.equal(c.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN,'https://localhost:19443');
});
test('deployment refuses every existing directory, including apparently empty ones',()=>{
 const parent=mkdtempSync(join(tmpdir(),'weaveos-server-'));
 try {const existing=join(parent,'private');mkdirSync(existing);assert.throws(()=>requireEmptyDirectory(existing),/existing/i);assert.doesNotThrow(()=>requireEmptyDirectory(join(parent,'new')));}
 finally {rmSync(parent,{recursive:true});}
});
