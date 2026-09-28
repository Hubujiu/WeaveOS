// One-time authorized installation, executed with the existing administrator SSH.
// Public key only. Private key and environment secrets never enter this file.
import {readFileSync,writeFileSync,mkdirSync,existsSync,copyFileSync,chmodSync,appendFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const root='/opt/weaveos-v010',source=root+'/releases/V010-015',target=root+'/infra/server/deploy',state=root+'/deploy-state';
process.umask(0o077);
if(existsSync(state+'/current.json'))throw Error('Preserve initialized deployment state');
const approved=JSON.parse(readFileSync(source+'/compatibility.json'));
const migrations={};
for(const [name,expected] of Object.entries(approved.migrations)){
 const actual=createHash('sha256').update(readFileSync(root+'/'+name,'utf8').replaceAll('\r\n','\n')).digest('hex');
 if(actual!==expected)throw Error('Installed migration source differs from accepted baseline');migrations[name]=actual;
}
const run=(bin,args)=>execFileSync(bin,args,{stdio:'pipe',encoding:'utf8',timeout:30000});
const args=['compose','--env-file',root+'/.env','-p','weaveos-v010-011','-f',root+'/compose.json'];
const db=run('docker',[...args,'ps','-aq','postgres']).trim();
for(const database of ['weaveos_runtime','weaveos_cold_archive'])if(run('docker',['exec',db,'psql','-X','-At','-U','weaveos_owner','-d',database,'-c','SELECT max(version_id) FROM goose_db_version WHERE is_applied']).trim()!=='1')throw Error('Unexpected installed schema version');
mkdirSync(target,{recursive:true,mode:0o700});mkdirSync(state,{mode:0o700});
for(const name of ['policy.mjs','bundle.mjs','migrate.mjs','receive.mjs']){copyFileSync(source+'/'+name,target+'/'+name);chmodSync(target+'/'+name,0o600);}
const installed=JSON.parse(readFileSync(root+'/INSTALLED.json'));
const current={runId:0,commit:installed.source,migrations,initializedAt:new Date().toISOString()};
for(const name of ['current','ledger'])writeFileSync(state+'/'+name+'.json',JSON.stringify(current,null,2),{flag:'wx',mode:0o600});
const publicKey=readFileSync(source+'/deploy_ed25519.pub','utf8').trim();
if(!/^ssh-ed25519 [A-Za-z0-9+/=]+ weaveos-main-deploy-v010016$/.test(publicKey))throw Error('Unexpected public key');
const command='/usr/bin/flock -n /opt/weaveos-v010/deploy.lock /usr/bin/timeout 1200 /usr/local/bin/node /opt/weaveos-v010/infra/server/deploy/receive.mjs';
const entry='restrict,command="'+command+'" '+publicKey;
const authorized='/root/.ssh/authorized_keys';
if(readFileSync(authorized,'utf8').includes(publicKey.split(' ')[1]))throw Error('Key already installed; inspect rather than duplicate');
appendFileSync(authorized,'\n'+entry+'\n');chmodSync(authorized,0o600);
console.log(JSON.stringify({receiverInstalled:true,baseline:current.commit,migrations:current.migrations,restrictedKey:true,applicationChanged:false}));
