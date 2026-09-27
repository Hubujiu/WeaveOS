import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync, copyFileSync, cpSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';
import { privateFile } from '../infra/runtime/backup.mjs';
import { serverCompose, requireEmptyDirectory } from '../infra/server/plan.mjs';
const root=resolve('.'),dir=resolve('.work/deploy');requireEmptyDirectory(dir);
for(const p of ['tls','secrets','backups','tools','migrations','archive-migrations'])mkdirSync(resolve(dir,p),{recursive:true});
const run=(cmd,args,opts={})=>execFileSync(cmd,args,{stdio:'pipe',...opts});
const secret=()=>randomBytes(32).toString('hex');
const owner=secret(),app=secret(),reader=secret(),maintenance=secret(),redis=secret();
const pg=(u,p,d,host='postgres')=>`postgres://${u}:${p}@${host}:5432/${d}?sslmode=disable&connect_timeout=2&timezone=UTC${u==='weaveos_owner'?'':'&pool_max_conns=8'}`;
const file=(name,bytes)=>privateFile(resolve(dir,name),bytes);
file('postgres.env',`POSTGRES_USER=weaveos_owner\nPOSTGRES_PASSWORD=${owner}\nPOSTGRES_DB=weaveos_runtime\n`);
file('redis.env',`REDISCLI_AUTH=${redis}\n`);file('redis.conf',`requirepass ${redis}\n`);
file('runtime.env',`WEAVEOS_DATABASE_URL=${pg('weaveos_runtime_app',app,'weaveos_runtime')}\nWEAVEOS_REDIS_URL=redis://:${redis}@redis:6379/0\nWEAVEOS_SESSION_GENERATION=${secret()}\nWEAVEOS_AUDIT_KEY_ID=server\nWEAVEOS_AUDIT_HMAC_KEY=${randomBytes(32).toString('base64')}\n`);
file('reader.env',`WEAVEOS_AUDIT_READ_DATABASE_URL=${pg('weaveos_runtime_reader',reader,'weaveos_runtime')}\nWEAVEOS_REDIS_URL=redis://:${redis}@redis:6379/0\n`+readFileSync(resolve(dir,'runtime.env'),'utf8').split('\n').filter(s=>s.startsWith('WEAVEOS_SESSION_GENERATION=')).join('\n')+'\n');
file('maintenance.env',`WEAVEOS_AUDIT_LIVE_DATABASE_URL=${pg('weaveos_runtime_maintenance',maintenance,'weaveos_runtime')}\nWEAVEOS_AUDIT_COLD_DATABASE_URL=${pg('weaveos_runtime_maintenance',maintenance,'weaveos_cold_archive')}\n`);
file('migration.env',`GOOSE_DRIVER=postgres\nGOOSE_DBSTRING=${pg('weaveos_owner',owner,'weaveos_runtime','127.0.0.1')}\n`);
file('cold-migration.env',`GOOSE_DRIVER=postgres\nGOOSE_DBSTRING=${pg('weaveos_owner',owner,'weaveos_cold_archive','127.0.0.1')}\n`);
file('secrets/backup.key',randomBytes(32));
file('tls/key.pem',Buffer.alloc(0));
run('C:/Program Files/Git/usr/bin/openssl.exe',['req','-x509','-newkey','rsa:2048','-nodes','-days','365','-keyout',resolve(dir,'tls/key.pem'),'-out',resolve(dir,'tls/cert.pem'),'-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1']);
const src='D:/Workspace/WeaveOS-worktrees/V010-008/.work/delivery-36284110511';
const record=JSON.parse(readFileSync(resolve(src,'BUILD.json')));
const receipts={source:record.commit,originalOCI:{},dockerArchives:{}};
for(const name of ['bff','web']){
 const original=readFileSync(resolve(src,record[name].archive));
 if(createHash('sha256').update(original).digest('hex')!==record[name].archiveSHA256)throw Error('OCI checksum changed');
 receipts.originalOCI[name]={digest:record[name].manifestDigest,sha256:record[name].archiveSHA256};
 run('docker',['save','--output',resolve(dir,`${name}.docker.tar`),record[name].tag]);
 const inspect=JSON.parse(run('docker',['image','inspect',record[name].tag],{encoding:'utf8'}))[0];
 receipts.dockerArchives[name]={tag:record[name].tag,sha256:createHash('sha256').update(readFileSync(resolve(dir,`${name}.docker.tar`))).digest('hex'),layers:inspect.RootFS.Layers,revision:inspect.Config.Labels['org.opencontainers.image.revision']};
}
file('.env',`WEAVEOS_RUNTIME_DIR=/opt/weaveos-v010\nWEAVEOS_BFF_IMAGE=${record.bff.tag}\nWEAVEOS_WEB_IMAGE=${record.web.tag}\n`);
writeFileSync(resolve(dir,'compose.json'),JSON.stringify(serverCompose(JSON.parse(readFileSync('infra/runtime/compose.json'))),null,2));
writeFileSync(resolve(dir,'BUILD.json'),JSON.stringify(record,null,2));writeFileSync(resolve(dir,'TRANSFER.json'),JSON.stringify(receipts,null,2));
cpSync('db/migrations',resolve(dir,'migrations'),{recursive:true});cpSync('db/archive-migrations',resolve(dir,'archive-migrations'),{recursive:true});
copyFileSync('.work/tools/goose',resolve(dir,'tools/goose'));copyFileSync('infra/runtime/roles.sql',resolve(dir,'roles.sql'));copyFileSync('infra/runtime/cold-roles.sql',resolve(dir,'cold-roles.sql'));
file('login-roles.sql',`CREATE ROLE weaveos_runtime_app LOGIN PASSWORD '${app}' IN ROLE auth_app;\nCREATE ROLE weaveos_runtime_reader LOGIN PASSWORD '${reader}' IN ROLE auth_reader;\nCREATE ROLE weaveos_runtime_maintenance LOGIN PASSWORD '${maintenance}' IN ROLE auth_maintenance;\nCREATE ROLE weaveos_backup LOGIN IN ROLE auth_backup;\nREVOKE CONNECT ON DATABASE weaveos_cold_archive FROM PUBLIC;\nGRANT CONNECT ON DATABASE weaveos_cold_archive TO weaveos_owner,auth_maintenance,auth_backup;\n`);
// Run the previously accepted seed only on a local, isolated database. Export
// exactly its Bootstrap rows; never send test accounts or invitation fixtures.
const image=JSON.parse(readFileSync('infra/runtime/compose.json')).services.postgres.image;
const name='weaveos-v011-bootstrap-preparation';
run('docker',['run','-d','--name',name,'-e','POSTGRES_HOST_AUTH_METHOD=trust','-e','POSTGRES_USER=weaveos_owner','-e','POSTGRES_DB=weaveos_prepare',image]);
try{
 for(let i=0;i<60;i++){try{run('docker',['exec',name,'pg_isready','-U','weaveos_owner']);break;}catch{await new Promise(r=>setTimeout(r,500));}}
 const exec=(args,options={})=>run('docker',['exec',...args,name,...options.tail??[]],options);
 const sql=(text)=>run('docker',['exec','-i',name,'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d','weaveos_prepare'],{input:text});
 const migration=readFileSync('db/migrations/00001_auth.sql','utf8').split('-- +goose Down')[0].replace(/-- \+goose.*\r?\n/g,'');sql(migration);
 file('seed.env','WEAVEOS_TEST_DATABASE_URL=postgres://weaveos_owner@127.0.0.1:5432/weaveos_prepare?sslmode=disable\nWEAVEOS_ACCEPTANCE_FIXTURES=/private/fixtures.json\n');
 run('docker',['run','--rm','--network',`container:${name}`,'--mount',`type=bind,src=${dir},dst=/private`,'--mount',`type=bind,src=${root}/.work/tools,dst=/tools,readonly`,'--env-file',resolve(dir,'seed.env'),record.bff.tag,'/tools/seed']);
 const f=JSON.parse(readFileSync(resolve(dir,'fixtures.json')));
 const query=`SELECT json_build_object('user',row_to_json(u),'credential',row_to_json(c),'event',row_to_json(e)) FROM auth.users u JOIN auth.password_credentials c ON c.user_id=u.id JOIN auth.authentication_events e ON e.subject_user_id=u.id AND e.event_type='bootstrap_created' WHERE u.is_bootstrap_admin`;
 const result=JSON.parse(run('docker',['exec',name,'psql','-X','-t','-A','-U','weaveos_owner','-d','weaveos_prepare','-c',query],{encoding:'utf8'}));
 const q=s=>"'"+s.replaceAll("'","''")+"'";
 file('bootstrap.sql',`BEGIN;\nSELECT pg_advisory_xact_lock(7765301003);\nDO $$ BEGIN IF EXISTS (SELECT 1 FROM auth.users) THEN RAISE EXCEPTION 'Refuse nonempty deployment bootstrap'; END IF; END $$;\nINSERT INTO auth.users(id,account,is_bootstrap_admin) VALUES(${q(result.user.id)},'bootstrap-admin',true);\nINSERT INTO auth.password_credentials(user_id,password_hash) VALUES(${q(result.user.id)},${q(result.credential.password_hash)});\nINSERT INTO auth.authentication_events(id,event_type,outcome,subject_user_id,request_id) VALUES(${q(result.event.id)},'bootstrap_created','success',${q(result.user.id)},'server-bootstrap-v010011');\nCOMMIT;\n`);
 file('admin.json',JSON.stringify({account:'bootstrap-admin',password:f.admin.Password},null,2));
 // Fixtures are deliberately omitted from the transfer allowlist below.
}finally{run('docker',['stop',name]);}
console.log(JSON.stringify({prepared:true,source:record.commit,bootstrap:'one admin only',directory:dir}));
