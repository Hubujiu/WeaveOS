// Q5/Q6: isolated local Linux simulation only. Never a production deploy command.
import { execFileSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import { packageImages, verifyArtifacts, exportArtifacts } from './artifacts.mjs';
import { privateFile } from './backup.mjs';
import {runAuditTask} from './audit-task.mjs';
import {countAPIReport,countBrowserReport} from '../acceptance/result-counts.mjs';
const root=resolve('.'),dir=resolve('.work/runtime');
if(existsSync(resolve(dir,'CURRENT.json')))throw new Error('Existing runtime: preserve and inspect before rerun');
for(const sub of ['tls','secrets','backups','public'])mkdirSync(resolve(dir,sub),{recursive:true});
const commit=execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
const artifacts=process.env.WEAVEOS_ARTIFACT_RECORD?JSON.parse(readFileSync(process.env.WEAVEOS_ARTIFACT_RECORD,'utf8')):packageImages({root,commit,outputDir:resolve(dir,'artifacts')});
// Security-patched derivative of the previously verified 85c2ee snapshot.
// This fixture must verify the new candidate; it is not a production rollback.
const previous=packageImages({root,commit:'c37226731a6bdbf5c6187aad6cfe1ff9be5daadd',outputDir:resolve(dir,'previous-artifacts')});
verifyArtifacts(artifacts);verifyArtifacts(previous);
const project=`weaveos-v010-008-${Date.now()}`,generation=project;
const env={...process.env,WEAVEOS_RUNTIME_DIR:dir,WEAVEOS_BFF_IMAGE:artifacts.bff.imageID,WEAVEOS_WEB_IMAGE:artifacts.web.imageID};
const composeArgs=['compose','-p',project,'-f',resolve('infra/runtime/compose.json')];
const call=(cmd,args,options={})=>execFileSync(cmd,args,{cwd:root,env,stdio:'inherit',...options});
const compose=(...args)=>call('docker',[...composeArgs,...args]);
const id=name=>call('docker',[...composeArgs,'ps','-aq',name],{encoding:'utf8',stdio:'pipe'}).trim();
const secret=()=>randomBytes(32).toString('hex'),ownerPassword=secret(),appPassword=secret(),readerPassword=secret(),maintenancePassword=secret(),redisPassword=secret();
const pg=(user,password,database,host='postgres')=>`postgres://${user}:${password}@${host}:5432/${database}?sslmode=disable&connect_timeout=2&timezone=UTC${user==='weaveos_owner'?'':'&pool_max_conns=8'}`;
const redis=`redis://:${redisPassword}@redis:6379/0`;
const file=(name,bytes)=>privateFile(resolve(dir,name),bytes);
file('postgres.env',`POSTGRES_USER=weaveos_owner\nPOSTGRES_PASSWORD=${ownerPassword}\nPOSTGRES_DB=weaveos_runtime\n`);
file('redis.env',`REDISCLI_AUTH=${redisPassword}\n`);file('redis.conf',`requirepass ${redisPassword}\n`);
file('runtime.env',`WEAVEOS_DATABASE_URL=${pg('weaveos_runtime_app',appPassword,'weaveos_runtime')}\nWEAVEOS_REDIS_URL=${redis}\nWEAVEOS_SESSION_GENERATION=${generation}\nWEAVEOS_AUDIT_KEY_ID=local\nWEAVEOS_AUDIT_HMAC_KEY=${randomBytes(32).toString('base64')}\nWEAVEOS_DEFINITION_HMAC_KEY=${randomBytes(32).toString('base64')}\nWEAVEOS_DEFINITION_KEY_ID=test\nWEAVEOS_SCHEMA_LOCK_TIMEOUT_MS=1000\nWEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS=5000\n`);
file('reader.env',`WEAVEOS_AUDIT_READ_DATABASE_URL=${pg('weaveos_runtime_reader',readerPassword,'weaveos_runtime')}\nWEAVEOS_REDIS_URL=${redis}\nWEAVEOS_SESSION_GENERATION=${generation}\n`);
file('maintenance.env',`WEAVEOS_AUDIT_LIVE_DATABASE_URL=${pg('weaveos_runtime_maintenance',maintenancePassword,'weaveos_runtime')}\nWEAVEOS_AUDIT_COLD_DATABASE_URL=${pg('weaveos_runtime_maintenance',maintenancePassword,'weaveos_cold_archive')}\n`);
file('secrets/backup.key',randomBytes(32));
file('migration.env',`WEAVEOS_TEST_DATABASE_URL=${pg('weaveos_owner',ownerPassword,'weaveos_runtime','127.0.0.1')}\nWEAVEOS_TEST_ARCHIVE_DATABASE_URL=${pg('weaveos_owner',ownerPassword,'weaveos_cold_archive','127.0.0.1')}\nWEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/runtime/fixtures.json\n`);
const openssl=process.platform==='win32'?'C:/Program Files/Git/usr/bin/openssl.exe':'openssl';
// Restrict ownership before OpenSSL writes private key bytes, including Windows ACLs.
file('tls/key.pem',Buffer.alloc(0));
call(openssl,['req','-x509','-newkey','rsa:2048','-nodes','-days','2','-keyout',resolve(dir,'tls/key.pem'),'-out',resolve(dir,'tls/cert.pem'),'-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1'],{stdio:'pipe'});
writeFileSync(resolve(dir,'CURRENT.json'),JSON.stringify({project,generation,dir,artifacts,previous,classification:'local-development-only'}));
const go=script=>call('docker',['run','--rm','--network',`container:${id('postgres')}`,'--mount',`type=bind,src=${root},dst=/repo`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','--env-file',resolve(dir,'migration.env'),'-e','GOFLAGS=-buildvcs=false','-e','GOBIN=/repo/.work/runtime/tools','-w','/repo/services/bff','golang:1.27.1','sh','-ec',script]);
const sql=(database,text)=>call('docker',['exec','-i',id('postgres'),'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',database],{input:text,stdio:'pipe'});
const start=Date.now();
try{
 // Build tools before entering the closed runtime network. No runtime secrets
 // are supplied to this internet-capable build container.
 call('docker',['run','--rm','--mount',`type=bind,src=${root},dst=/repo`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-e','GOBIN=/repo/.work/runtime/tools','-w','/repo/services/bff','golang:1.27.1','sh','-ec','go install github.com/pressly/goose/v3/cmd/goose@v3.28.0; CGO_ENABLED=0 go build -o /repo/.work/runtime/tools/seed ./cmd/acceptance-seed']);
 // Linux private config belongs to the image's dedicated Redis reader.
 if(typeof process.getuid==='function')call('docker',['run','--rm','--mount',`type=bind,src=${dir},dst=/private`,'debian:bookworm-slim','chown','999:999','/private/redis.conf']);
 compose('up','-d','--wait','postgres','redis');sql('postgres','CREATE DATABASE weaveos_cold_archive;');
 go('/repo/.work/runtime/tools/goose -dir /repo/db/archive-migrations postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up; /repo/.work/runtime/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up; /repo/.work/runtime/tools/seed');
 if(typeof process.getuid==='function')call('docker',['run','--rm','--mount',`type=bind,src=${root},dst=/repo`,'debian:bookworm-slim','chown',`${process.getuid()}:${process.getgid()}`,'/repo/.work/runtime/fixtures.json']);
 sql('weaveos_runtime',readFileSync('infra/runtime/roles.sql','utf8'));sql('weaveos_cold_archive',readFileSync('infra/runtime/cold-roles.sql','utf8'));
 sql('postgres',`CREATE ROLE weaveos_runtime_app LOGIN PASSWORD '${appPassword}' IN ROLE auth_app; CREATE ROLE weaveos_runtime_reader LOGIN PASSWORD '${readerPassword}' IN ROLE auth_reader; CREATE ROLE weaveos_runtime_maintenance LOGIN PASSWORD '${maintenancePassword}' IN ROLE auth_maintenance; CREATE ROLE weaveos_backup LOGIN IN ROLE auth_backup; REVOKE CONNECT ON DATABASE weaveos_cold_archive FROM PUBLIC; GRANT CONNECT ON DATABASE weaveos_cold_archive TO weaveos_owner,auth_maintenance,auth_backup;`);
 sql('weaveos_runtime',"INSERT INTO auth.authentication_events(event_type,outcome,request_id,occurred_at) VALUES('login','failure','runtime-monthly-fixture',date_trunc('month',clock_timestamp())-interval '1 day');");
 compose('up','-d','bff','nginx');
 if(runAuditTask({dir,args:composeArgs,command:(bin,args,options)=>call(bin,args,{...options,stdio:'pipe'})}).status!=='complete')throw Error('Initial one-shot maintenance failed');
 const testEnv={...env,WEAVEOS_RUNTIME_CONTEXT:resolve(dir,'CURRENT.json'),NODE_EXTRA_CA_CERTS:resolve(dir,'tls/cert.pem')};
 call(process.execPath,['--test','infra/runtime/audit-task.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/scheduler-transition.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/logs.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/retention.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/ingress-recreation.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/operations.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/dns.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/monitor.integration.test.mjs'],{env:testEnv});
 compose('up','-d','--wait','redis');
 call(process.execPath,['--test','infra/runtime/tls.integration.test.mjs'],{env:testEnv});
 call(process.execPath,['--test','infra/runtime/backup.test.mjs'],{env:{...testEnv,WEAVEOS_BACKUP_TEST_CONTAINER:call('docker',['inspect',id('postgres'),'--format','{{.Name}}'],{encoding:'utf8',stdio:'pipe'}).trim().slice(1),WEAVEOS_BACKUP_TEST_USER:'weaveos_owner'}});
 call(process.execPath,['--test','infra/runtime/artifact-integrity.test.mjs','infra/runtime/artifact-transfer.test.mjs','infra/runtime/image-scan.test.mjs'],{env:{...testEnv,WEAVEOS_ARTIFACT_RECORD:artifacts.recordFile,WEAVEOS_IMAGE_SCAN_DIR:resolve(dir,'public/image-scan')}});
 // Validate the exact promoted artifacts, using the already-frozen test runner.
 call('docker',['run','--rm','--init','--shm-size=1g','--network',`container:${id('nginx')}`,'--mount',`type=bind,src=${root},dst=/repo`,'--mount','type=volume,src=weaveos-v010-linux-node,dst=/repo/node_modules','--mount','type=volume,src=weaveos-v010-linux-web-node,dst=/repo/apps/web/node_modules','-e','CI=true','-e','WEAVEOS_API_URL=https://localhost:19443','-e','WEAVEOS_WEB_URL=https://localhost:19443','-e','WEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/runtime/fixtures.json','-e','NODE_EXTRA_CA_CERTS=/repo/.work/runtime/tls/cert.pem','-w','/repo','mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27','bash','-euc','npm install --global pnpm@10.28.2 --ignore-scripts; pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store; node --test --test-reporter=spec --test-reporter-destination=stdout --test-reporter=tap --test-reporter-destination=/repo/.work/runtime/api.tap tests/acceptance/api.test.mjs; PLAYWRIGHT_JSON_OUTPUT_NAME=/repo/.work/runtime/browser.json pnpm exec playwright test --config apps/web/playwright.integration.config.ts --reporter=line,json']);
 exportArtifacts(artifacts,resolve(dir,'export'));
 writeFileSync(resolve(dir,'public/result.json'),JSON.stringify({result:'passed',source:artifacts.commit,artifacts:{bff:artifacts.bff.manifestDigest,web:artifacts.web.manifestDigest},imageIDs:{bff:artifacts.bff.imageID,web:artifacts.web.imageID},previousSource:previous.commit,operations:6,dns:1,monitor:1,tls:2,backup:4,artifactIntegrity:3,api:countAPIReport(readFileSync(resolve(dir,'api.tap'),'utf8')),browser:countBrowserReport(JSON.parse(readFileSync(resolve(dir,'browser.json'),'utf8'))),elapsedSeconds:(Date.now()-start)/1000,target:'local WSL Linux only; same-machine restore simulation; not production'}));
}catch{writeFileSync(resolve(dir,'public/result.json'),JSON.stringify({result:'failed',source:artifacts.commit,elapsedSeconds:(Date.now()-start)/1000}));throw new Error('Isolated runtime verification failed; preserve private environment and inspect safe results');}
finally{compose('stop');}
