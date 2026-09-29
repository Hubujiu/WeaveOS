import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, rmSync, readFileSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { serverCompose, requireEmptyDirectory, serverSchedules, bootstrapCredentials } from '../../infra/server/plan.mjs';
import { serverContext } from '../../infra/server/context.mjs';
import { acmePlan } from '../../infra/server/acme.mjs';
import { validateCertificate, replaceCertificateFiles } from '../../infra/server/tls.mjs';
import { credentialsFromCSV, credentialsFromEnv } from '../../infra/server/credentials.mjs';
import { runAcme } from '../../infra/server/acme-run.mjs';
import { deployCertificate } from '../../infra/server/tls.mjs';
import { domainHealth } from '../../infra/server/probe.mjs';
test('ACME runner fixes operations, passes credentials only in environment and reports sanitized failure',()=>{
 const calls=[],credentials={Tencent_SecretId:'sample-id',Tencent_SecretKey:'sample-value'};
 const command=(bin,args,options)=>{calls.push({bin,args,options});return Buffer.from('private client output');};
 assert.deepEqual(runAcme({operation:'renew',credentials,command}),{operation:'renew',status:'passed',exitCode:0});
 assert.equal(calls[0].bin,'/bin/sh');assert.deepEqual(calls[0].args,['/opt/weaveos-v010/acme-client/acme.sh',...acmePlan().renew]);
 assert.equal(calls[0].options.env.Tencent_SecretKey,'sample-value');assert.ok(!calls[0].args.includes('sample-value'));
 assert.throws(()=>runAcme({operation:'arbitrary',credentials,command}),/operation/i);
 assert.deepEqual(runAcme({operation:'issue',credentials,command:()=>{throw Object.assign(Error('sample-value'),{status:9,stderr:Buffer.from('sample-value')});}}),{operation:'issue',status:'failed',exitCode:9});
});
test('certificate activation verifies system trust before file replacement and validates Nginx before reload',()=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-activate-')),calls=[];
 const openssl=process.platform==='win32'?'C:/Program Files/Git/usr/bin/openssl.exe':'openssl';
 try{
  mkdirSync(join(dir,'acme-stage'));mkdirSync(join(dir,'tls'));
  execFileSync(openssl,['req','-x509','-newkey','rsa:2048','-nodes','-days','10','-keyout',join(dir,'acme-stage/key.pem'),'-out',join(dir,'acme-stage/cert.pem'),'-subj','/CN=weave.hubujiu.site','-addext','basicConstraints=critical,CA:FALSE','-addext','subjectAltName=DNS:weave.hubujiu.site'],{stdio:'pipe'});
  writeFileSync(join(dir,'tls/cert.pem'),'old-cert');writeFileSync(join(dir,'tls/key.pem'),'old-key');
  const c={dir,command:(bin,args)=>{calls.push([bin,...args]);throw Error('untrusted');},compose:(...args)=>calls.push(['compose',...args])};
  assert.throws(()=>deployCertificate(c),/trust/i);assert.equal(readFileSync(join(dir,'tls/cert.pem'),'utf8'),'old-cert');assert.equal(calls[0][0],'openssl');assert.ok(calls[0].includes('-verify_hostname'));
  calls.length=0;c.command=(bin,args)=>{calls.push([bin,...args]);return Buffer.from('OK');};
  assert.equal(deployCertificate(c).domain,'weave.hubujiu.site');
  assert.deepEqual(calls.slice(-2),[['compose','exec','-T','nginx','nginx','-t'],['compose','exec','-T','nginx','nginx','-s','reload']]);
  assert.equal(readFileSync(join(dir,'tls/cert.pem'),'utf8'),readFileSync(join(dir,'acme-stage/cert.pem'),'utf8'));
 }finally{rmSync(dir,{recursive:true});}
});
test('domain health verifies hostname with default trust while reaching only the SSH loopback endpoint',async()=>{
 let options;
 const request=(o,callback)=>{options=o;return {once(){return this;},setTimeout(){return this;},end(){callback({statusCode:200,resume(){}});},destroy(){}};};
 assert.equal(await domainHealth('ready',request),true);
 assert.equal(options.hostname,'weave.hubujiu.site');assert.equal(options.port,19443);assert.equal(options.servername,'weave.hubujiu.site');assert.equal(options.rejectUnauthorized,true);assert.equal(options.ca,undefined);
 options.lookup('weave.hubujiu.site',{},(error,address,family)=>{assert.equal(error,null);assert.equal(address,'127.0.0.1');assert.equal(family,4);});
});
test('domain health fixes IPv4 so Node never requests an all-address lookup result',async()=>{
 let family;
 const request=(options,callback)=>{family=options.family;return {once(){return this;},setTimeout(){return this;},end(){callback({statusCode:200,resume(){}});}};};
 await domainHealth('live',request);assert.equal(family,4);
});
const runtime=JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url)));
test('provided CSV is data only and maps exactly one credential pair without executing fields',async t=>{
 await t.test('BOM, quoted headers and CRLF are supported',()=>assert.deepEqual(credentialsFromCSV('\uFEFF"SecretId","SecretKey"\r\n"sample-id","sample-value"\r\n'),{Tencent_SecretId:'sample-id',Tencent_SecretKey:'sample-value'}));
 await t.test('missing, multiple and non-data values fail closed',()=>{
  for(const csv of ['SecretId,SecretKey\na,\n','SecretId,SecretKey\na,b\nc,d\n','SecretId,SecretKey\na,$(execute)\n'])assert.throws(()=>credentialsFromCSV(csv),/credential/i);
 });
 await t.test('dotenv admits only the required names and never evaluates shell',()=>{
  assert.deepEqual(credentialsFromEnv('Tencent_SecretId=sample-id\nTencent_SecretKey=sample-value\n'),{Tencent_SecretId:'sample-id',Tencent_SecretKey:'sample-value'});
  assert.throws(()=>credentialsFromEnv('Tencent_SecretId=sample-id\nTencent_SecretKey=$(execute)\n'),/credential/i);
 });
});
test('Q16 public-trust domain config keeps the SSH listener and schedules automatic renewal',async t=>{
 const c=serverCompose(runtime,{publicTLS:true});
 await t.test('domain origin with loopback only',()=>{
  assert.equal(c.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN,'https://weave.hubujiu.site:19443');
  assert.deepEqual(c.services.nginx.ports,['127.0.0.1:19443:19443']);
 });
 const cron=serverSchedules({publicTLS:true});
 await t.test('renew twice daily without self-signed trust',()=>{
  assert.match(cron,/^33 2,14 \* \* \* root .*flock -n .*acme\.lock .*acme-run\.mjs renew/m);
  assert.doesNotMatch(cron,/NODE_EXTRA_CA_CERTS/);
 });
});
test('Q16 validates the actual certificate host, private key and remaining lifetime before replacement',async t=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-certificate-'));
 const openssl=process.platform==='win32'?'C:/Program Files/Git/usr/bin/openssl.exe':'openssl';
 try{
  for(const [name,host] of [['valid','weave.hubujiu.site'],['other','unrelated.example']]) execFileSync(openssl,['req','-x509','-newkey','rsa:2048','-nodes','-days','10','-keyout',join(dir,name+'.key'),'-out',join(dir,name+'.pem'),'-subj','/CN='+host,'-addext','basicConstraints=critical,CA:FALSE','-addext','extendedKeyUsage=serverAuth','-addext','subjectAltName=DNS:'+host],{stdio:'pipe'});
  const chain=readFileSync(join(dir,'valid.pem')),key=readFileSync(join(dir,'valid.key'));
  await t.test('matching actual identity and key are accepted',()=>assert.equal(validateCertificate(chain,key).domain,'weave.hubujiu.site'));
  await t.test('unrelated certificate is rejected',()=>assert.throws(()=>validateCertificate(readFileSync(join(dir,'other.pem')),readFileSync(join(dir,'other.key'))),/identity/i));
  await t.test('mismatched private key is rejected',()=>assert.throws(()=>validateCertificate(chain,readFileSync(join(dir,'other.key'))),/key/i));
  await t.test('expired certificate is rejected',()=>assert.throws(()=>validateCertificate(chain,key,Date.now()+11*86400000),/lifetime/i));
 }finally{rmSync(dir,{recursive:true});}
});
test('Q16 a failed Nginx validation restores both previous files and reloads the previous certificate',()=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-replace-'));let attempts=0;
 try{
  writeFileSync(join(dir,'cert.pem'),'old-cert');writeFileSync(join(dir,'key.pem'),'old-key');
  assert.throws(()=>replaceCertificateFiles(dir,Buffer.from('new-cert'),Buffer.from('new-key'),()=>{attempts++;if(attempts===1){assert.equal(readFileSync(join(dir,'cert.pem'),'utf8'),'new-cert');throw Error('nginx config rejected');}}),/replacement failed/i);
  assert.equal(readFileSync(join(dir,'cert.pem'),'utf8'),'old-cert');assert.equal(readFileSync(join(dir,'key.pem'),'utf8'),'old-key');assert.equal(attempts,2);
 }finally{rmSync(dir,{recursive:true});}
});
test('Q16 explicitly uses Let\'s Encrypt DNSPod DNS-01 for the confirmed domain and no public listener',()=>{
 const p=acmePlan();
 assert.equal(p.domain,'weave.hubujiu.site');
 assert.equal(p.origin,'https://weave.hubujiu.site:19443');
 assert.equal(p.source.version,'3.1.6');
 assert.deepEqual(p.issue.slice(0,7),['--issue','--server','https://acme-v02.api.letsencrypt.org/directory','--dns','dns_tencent','-d','weave.hubujiu.site']);
 assert.ok(p.issue.includes('--keylength')&&p.issue.includes('ec-256'));
 assert.ok(p.renew.includes('--cron')&&p.renew.includes('--server'));
 assert.ok(!JSON.stringify(p).includes('--standalone')&&!JSON.stringify(p).includes('--debug'));
 assert.equal(p.credentials,'/opt/weaveos-v010/secrets/acme.env');
 assert.equal(p.stagedCertificate,'/opt/weaveos-v010/acme-stage/cert.pem');
 assert.equal(p.stagedKey,'/opt/weaveos-v010/acme-stage/key.pem');
 assert.equal(p.install[p.install.indexOf('--reloadcmd')+1],'/usr/local/bin/node /opt/weaveos-v010/infra/server/tls.mjs deploy');
});
test('Q15 server exposes only loopback HTTPS and preserves internal data network',()=>{
 const c=serverCompose(runtime);
 assert.deepEqual(c.services?.nginx?.ports,['127.0.0.1:19443:19443']);
 assert.equal(c.networks?.default?.internal,true);
 for(const s of Object.values(c.services)) assert.equal(s.build,undefined);
 for(const name of ['postgres','redis','bff','audit-maintenance']) assert.equal(c.services[name].ports,undefined);
});
test('private Bootstrap credentials preserve the accepted seed JSON password field and exclude test fixtures',()=>{
 const seeded={admin:{account:'acceptance-admin-example',password:'Aa1!Sample'},user:{account:'test-user',password:'unrelated'},invitations:{valid:'synthetic'}};
 assert.deepEqual(bootstrapCredentials(seeded),{account:'bootstrap-admin',password:'Aa1!Sample'});
 assert.throws(()=>bootstrapCredentials({admin:{account:'example'}}),/password/i);
});
test('operational schedules retain TLS verification, prevent overlap, and invoke only runtime tools',()=>{
 const cron=serverSchedules();
 assert.match(cron,/^\*\/5 \* \* \* \* root /m);
 assert.match(cron,/^15 3 \* \* \* root /m);
 assert.match(cron,/NODE_EXTRA_CA_CERTS=\/opt\/weaveos-v010\/tls\/cert.pem/);
 assert.equal((cron.match(/flock -n/g)??[]).length,3); // monitor, backup and the approved minute log collector
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
 for(const [name,s] of Object.entries(c.services)){assert.equal(s.restart,name==='audit-maintenance'?'no':'unless-stopped');assert.ok(s.mem_limit);assert.ok(s.logging.options['max-size']);}
 assert.equal(c.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN,'https://localhost:19443');
});
test('deployment refuses every existing directory, including apparently empty ones',()=>{
 const parent=mkdtempSync(join(tmpdir(),'weaveos-server-'));
 try {const existing=join(parent,'private');mkdirSync(existing);assert.throws(()=>requireEmptyDirectory(existing),/existing/i);assert.doesNotThrow(()=>requireEmptyDirectory(join(parent,'new')));}
 finally {rmSync(parent,{recursive:true});}
});
