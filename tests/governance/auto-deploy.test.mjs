import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {validateCompatibility,promote} from '../../infra/server/deploy/policy.mjs';
import {candidateCompose,publicNginx} from '../../infra/server/deploy/configuration.mjs';
import {validateEnvelope,receiveBundle,approvalMap} from '../../infra/server/deploy/bundle.mjs';
import {mkdtempSync,rmSync,existsSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {Readable} from 'node:stream';
import {createHash} from 'node:crypto';
const sha='a'.repeat(64),other='b'.repeat(64),commit='c'.repeat(40);
const current={runId:100,commit:'d'.repeat(40),migrations:{'migrations/00001_auth.sql':sha}};
const release={runId:101,commit,migrations:{'migrations/00001_auth.sql':sha,'migrations/00002_expand.sql':other},approved:{'migrations/00001_auth.sql':sha,'migrations/00002_expand.sql':other},backwardCompatible:true};

test('Q18 accepts declared compatible expansion but rejects history rewrite, missing approval and stale releases',()=>{
 assert.equal(validateCompatibility(current,release),true);
 for(const changed of [{...release,runId:99},{...release,backwardCompatible:false},{...release,approved:{}},{...release,migrations:{'migrations/00001_auth.sql':other}}, {...release,migrations:{'migrations/00002_expand.sql':other}},{...release,commit:'../invalid'}])assert.throws(()=>validateCompatibility(current,changed));
});

test('Q18 deployment only changes approved runtime config and exact images, retaining private volumes and origin',()=>{
 const base=JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url)));
 const original=structuredClone(base);
 const candidate=candidateCompose(base,{bff:'sha256:'+sha,web:'sha256:'+other});
 assert.equal(candidate.services.bff.image,'sha256:'+sha);
 assert.equal(candidate.services['audit-maintenance'].image,'sha256:'+sha);
 assert.equal(candidate.services.nginx.image,'sha256:'+other);
 assert.equal(candidate.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN,'https://weave.hubujiu.site');
 assert.deepEqual(candidate.services.nginx.ports,['0.0.0.0:443:19443','0.0.0.0:80:80','127.0.0.1:19443:19443']);
 for(const name of ['postgres','redis','bff','audit-maintenance'])assert.equal(candidate.services[name].ports,undefined);
 assert.deepEqual(candidate.volumes,base.volumes);
 assert.equal(candidate.networks.default.internal,true);
 assert.deepEqual(base,original);
 for(const mutate of [b=>b.services.redis.ports=['6379:6379'],b=>b.services.bff.privileged=true,b=>b.services.bff.volumes=['/:/host'],b=>b.services.bff.env_file=['/tmp/foreign.env']]){
  const unsafe=structuredClone(base);mutate(unsafe);assert.throws(()=>candidateCompose(unsafe,{bff:'sha256:'+sha,web:'sha256:'+other}));
 }
});

test('approved config cannot include other host compose files, weaken service confinement or join external networks',()=>{
 const base=JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url)));
 for(const mutate of [b=>b.include=['/tmp/foreign-compose.json'],b=>b.networks.edge={external:true},b=>b.services.nginx.networks=['host'],b=>b.services.bff.read_only=false,b=>b.services.bff.user='0:0',b=>b.services.bff.cap_drop=[],b=>b.services.bff.security_opt=[],b=>b.services.bff.environment.WEAVEOS_TRUSTED_PROXY_HOSTS='*']){
  const unsafe=structuredClone(base);mutate(unsafe);assert.throws(()=>candidateCompose(unsafe,{bff:'sha256:'+sha,web:'sha256:'+other}));
 }
});

test('public Nginx rendering preserves API boundary and creates fixed HTTPS redirect',()=>{
 const conf=publicNginx(readFileSync(new URL('../../infra/acceptance/nginx.conf',import.meta.url),'utf8'));
 assert.match(conf,/server_name weave\.hubujiu\.site;/);
 assert.match(conf,/return 308 https:\/\/weave\.hubujiu\.site\$request_uri;/);
 assert.match(conf,/proxy_set_header X-Forwarded-For \$remote_addr;/);
 assert.match(conf,/add_header Cache-Control "no-store" always;/);
});

function operations(fail){const calls=[];return {calls,ops:Object.fromEntries(['validate','backup','migrate','activate','health','record','restore','healthPrevious'].map(name=>[name,async()=>{calls.push(name);if(name===fail)throw Error('private credentials must not escape');return true;}]))};}
test('promotion validates and backs up before migration and records only a healthy deployment',async()=>{
 const {calls,ops}=operations();assert.equal((await promote(ops)).status,'deployed');
 assert.deepEqual(calls,['validate','backup','migrate','activate','health','record']);
});
test('backup failure never migrates; activation or health failure restores prior app/config without Down',async()=>{
 for(const phase of ['validate','backup','migrate','activate','health']){
  const {calls,ops}=operations(phase);const result=await promote(ops);
  assert.equal(result.status,'failed');assert.equal(result.phase,phase);assert.ok(!JSON.stringify(result).includes('private credentials'));
  if(['validate','backup','migrate'].includes(phase))assert.ok(!calls.includes('activate'));
  if(['activate','health'].includes(phase))assert.deepEqual(calls.slice(-2),['restore','healthPrevious']);
  assert.ok(!calls.includes('record'));
 }
});

test('main delivery runs verification and acceptance before a serialized secret-scoped deployment',()=>{
 const yml=readFileSync(new URL('../../.github/workflows/delivery.yml',import.meta.url),'utf8');
 assert.match(yml,/push:\s*\n\s*branches: \[main\]/);
 assert.match(yml,/deploy:\s*\n\s*needs: package/);
 assert.match(yml,/environment:\s*weaveos-production/);
 assert.match(yml,/cancel-in-progress: false/);
 assert.match(yml,/StrictHostKeyChecking=yes/);
 assert.match(yml,/needs: acceptance/);
 assert.match(yml,/secrets\.WEAVEOS_DEPLOY_KEY/);
 assert.doesNotMatch(yml,/pull_request_target|StrictHostKeyChecking=no/);
});

const hash=x=>createHash('sha256').update(x).digest('hex');
test('compatibility registry uses explicit digest records, rejects ambiguous duplicate approvals',()=>{
 const records=[{path:'migrations/00001_auth.sql',sha256:sha},{path:'archive-migrations/00001_archive.sql',sha256:other}];
 assert.deepEqual(approvalMap(records),{'migrations/00001_auth.sql':sha,'archive-migrations/00001_archive.sql':other});
 for(const invalid of [[...records,records[0]],[{path:'../../runtime.env',sha256:sha}],[{path:'migrations/00001_auth.sql',sha256:'invalid'}],{}])assert.throws(()=>approvalMap(invalid));
});
function envelope(){
 const compose=readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url),'utf8');
 const nginx=readFileSync(new URL('../../infra/acceptance/nginx.conf',import.meta.url),'utf8');
 return {...release,protocol:1,files:{'compose.json':compose,'nginx.conf':nginx,'migrations/00001_auth.sql':'-- baseline','migrations/00002_expand.sql':'-- expansion'},approved:{'migrations/00001_auth.sql':hash('-- baseline'),'migrations/00002_expand.sql':hash('-- expansion')},migrations:{'migrations/00001_auth.sql':hash('-- baseline'),'migrations/00002_expand.sql':hash('-- expansion')},config:{'compose.json':hash(compose),'nginx.conf':hash(nginx)},images:Object.fromEntries(['bff','web'].map(n=>[n,{size:3,sha256:hash('abc'),imageID:'sha256:'+sha,layers:['sha256:'+other]}]))};
}
test('transport rejects undeclared content, checksum tampering, secret paths, invalid image identities and oversized payloads',()=>{
 assert.equal(validateEnvelope(envelope()),true);
 for(const mutate of [e=>e.files['../../runtime.env']='secret',e=>e.files['compose.json']+=' ',e=>e.files['migrations/00001_auth.sql']='modified',e=>e.images.bff.size=2**40,e=>e.images.bff.imageID='latest',e=>e.protocol=99,e=>delete e.images.web,e=>e.backwardCompatible=false]){
  const e=envelope();mutate(e);assert.throws(()=>validateEnvelope(e));
 }
});
test('framed receiver verifies every byte and rejects truncated, appended and corrupt images',async()=>{
 const e=envelope(),header=Buffer.from(JSON.stringify(e)),length=Buffer.alloc(4);length.writeUInt32BE(header.length);
 const good=Buffer.concat([length,header,Buffer.from('abcabc')]);
 for(const [i,bytes] of [good,good.subarray(0,-1),Buffer.concat([good,Buffer.from('extra')]),Buffer.concat([length,header,Buffer.from('badabc')])].entries()){
  const dir=mkdtempSync(join(tmpdir(),'weaveos-016-'));
  try {
   const read=()=>receiveBundle(Readable.from([bytes]),dir);
   if(i===0){assert.equal((await read()).commit,commit);assert.ok(existsSync(join(dir,'bff.docker.tar')));}
   else await assert.rejects(read);
  }finally{rmSync(dir,{recursive:true});}
 }
});
