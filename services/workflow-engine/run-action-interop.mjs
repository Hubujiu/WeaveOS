// Root-owned isolated Java + application database worker proof. Linux/Docker only.
import {execFileSync,spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync,readFileSync,realpathSync} from 'node:fs';
import {resolve,dirname} from 'node:path';
import {pathToFileURL,fileURLToPath} from 'node:url';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
process.chdir(root);
if(typeof process.getuid!=='function'||typeof process.getgid!=='function')throw Error('Linux fixture required');
const engine=resolve('services/workflow-engine');
// The prepared cache may live on a separate fixture filesystem. Mount its
// canonical path too so .work symlinks remain valid inside both containers.
const engineCache=realpathSync(resolve(engine,'.work'));
const dir=resolve('.work/v037-actions-java-worker-'+Date.now());mkdirSync(dir,{recursive:true,mode:0o700});
const go=process.env.WEAVEOS_ACTIONS_GO||'go',goose=process.env.WEAVEOS_ACTIONS_GOOSE||'goose';
const env={...process.env,GOTOOLCHAIN:'local',GOFLAGS:'-buildvcs=false'};
const call=(cmd,args,opts={})=>execFileSync(cmd,args,{cwd:root,env,stdio:'pipe',maxBuffer:32*1024*1024,...opts});
const step=(name,cmd,args,opts={})=>{
 const r=spawnSync(cmd,args,{cwd:root,env,encoding:'utf8',maxBuffer:32*1024*1024,...opts});
 writeFileSync(resolve(dir,name+'.stdout'),r.stdout??'',{mode:0o600});writeFileSync(resolve(dir,name+'.stderr'),r.stderr??'',{mode:0o600});writeFileSync(resolve(dir,name+'.exit'),String(r.status??1),{mode:0o600});
 if(r.status!==0)throw Error(name+' failed; exact private stdout/stderr retained');return r;
};
const delay=ms=>new Promise(r=>setTimeout(r,ms));
const {startB2TestDatabase}=await import(pathToFileURL(resolve('infra/acceptance/run.mjs')).href);
const prefix='weaveos-v037-actions-real-'+Date.now(),network=prefix+'-net',enginePG=prefix+'-pg',java=prefix+'-java',redis=prefix+'-redis';
const pgImage='docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722';
const mvnImage='mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b';
let pg,javaStarted=false;const owned=[];
try{
 if(!call(go,['version'],{encoding:'utf8'}).includes('go1.27.2 '))throw Error('wrong Go compiler');
 step('compile',go,['test','-tags','workflowrpc_integration','-c','-o',resolve(engine,'.work/action-interop.test'),'./cmd/bff'],{cwd:resolve('services/bff'),env:{...env,CGO_ENABLED:'0',GOOS:'linux',GOARCH:'amd64'}});
 if(process.env.WEAVEOS_ACTIONS_REUSE_STATE){
  const statePath=resolve(process.env.WEAVEOS_ACTIONS_REUSE_STATE);
  if(!statePath.startsWith(root+'/.work/'))throw Error('Explicit current-worktree state required');
  const state=JSON.parse(readFileSync(statePath,'utf8'));
  const expected='postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722';
  const info=JSON.parse(call('docker',['inspect',state.container],{encoding:'utf8'}))[0];
  if(state.version!==1||!/^weaveos-b2-ci-[0-9a-f]{32}$/.test(state.container)||state.container!=='weaveos-b2-ci-'+state.owner||!/^\/tmp\/weaveos-b2-ci-[A-Za-z0-9]+$/.test(state.root)||state.socket!==state.root+'/socket'||info.State.Running||info.Config.Image!==expected||info.Config.Labels?.['com.weaveos.test-fixture']!=='b2-ci'||info.Config.Labels?.['com.weaveos.b2-fixture.owner']!==state.owner||info.HostConfig.NetworkMode!=='none'||Object.keys(info.HostConfig.PortBindings??{}).length||!info.Config.Env?.includes('POSTGRES_USER=weaveos_b2_test')||!info.Config.Env?.includes('POSTGRES_DB=weaveos_b2_isolated_test')||!info.Mounts?.some(m=>m.Type==='bind'&&m.Source===state.socket&&m.Destination==='/var/run/postgresql'))throw Error('Refuse unowned/running/nonisolated PG fixture');
  call('docker',['start',state.container]);pg={...state,url:'postgres:///weaveos_actions_'+Date.now()+'?host='+encodeURIComponent(state.socket)+'&user=weaveos_b2_test&sslmode=disable'};
 }else{pg=await startB2TestDatabase({stateFile:resolve(dir,'pg-state.json')});}
 owned.push(pg.container);
 let ready=false;for(let i=0;i<120;i++){
  try{ready=call('docker',['exec',pg.container,'sh','-ec','test "$(cat /proc/1/comm)" = postgres; psql -X -U weaveos_b2_test -d weaveos_b2_isolated_test -tAc "SELECT 1"'],{encoding:'utf8'}).trim()==='1'}catch{}
  if(ready)break;await delay(250);
 }if(!ready)throw Error('application PG final server not ready');
 const hotName=new URL(pg.url).pathname.slice(1);
 if(process.env.WEAVEOS_ACTIONS_REUSE_STATE)call('docker',['exec',pg.container,'createdb','-U','weaveos_b2_test',hotName]);
 const archive=new URL(pg.url);archive.pathname='/weaveos_actions_archive_'+Date.now();call('docker',['exec',pg.container,'createdb','-U','weaveos_b2_test',archive.pathname.slice(1)]);
 for(const [folder,url,name]of [['db/archive-migrations',archive.href,'archive'],['db/migrations',pg.url,'hot']])step('migration-'+name,goose,['-dir',resolve(folder),'postgres',url,'up']);
 step('roles','docker',['exec','-i',pg.container,'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_b2_test','-d',hotName],{input:readFileSync('infra/runtime/roles.sql','utf8')});
 const redisSocket=resolve(dir,'redis');mkdirSync(redisSocket,{mode:0o700});
 call('docker',['run','-d','--name',redis,'--network','none','--user',process.getuid()+':'+process.getgid(),'-v',redisSocket+':/socket','--workdir','/socket','--entrypoint','redis-server','redis:8.2.10@sha256:164c759a0c342ee69d08fc99219382b0fd682181465c0df2e0e6911f4c85d73c','--port','0','--unixsocket','/socket/redis.sock','--unixsocketperm','700','--save','','--appendonly','no']);owned.push(redis);
 ready=false;for(let i=0;i<80;i++){try{ready=call('docker',['exec',redis,'redis-cli','-s','/socket/redis.sock','ping'],{encoding:'utf8'}).trim()==='PONG'}catch{}if(ready)break;await delay(250)}if(!ready)throw Error('Redis not ready');
 call('docker',['network','create','--internal','--label','weaveos.package=V030-037',network]);
 call('docker',['run','-d','--name',enginePG,'--network',network,'--network-alias','b3-postgres','--label','weaveos.package=V030-037','-e','POSTGRES_DB=b3_flowable_fixture','-e','POSTGRES_USER=b3_fixture','-e','POSTGRES_PASSWORD=b3_fixture_only',pgImage]);owned.push(enginePG);
 ready=false;for(let i=0;i<120;i++){try{ready=call('docker',['exec',enginePG,'sh','-ec','test "$(cat /proc/1/comm)" = postgres; psql -X -U b3_fixture -d b3_flowable_fixture -tAc "SELECT 1"'],{encoding:'utf8'}).trim()==='1'}catch{}if(ready)break;await delay(500)}if(!ready)throw Error('engine PG final server not ready');
 step('maven-compile','docker',['run','--rm','--user',process.getuid()+':'+process.getgid(),'--network',network,'-v',engine+':/proof','-v',engineCache+':'+engineCache,'-v',engine+'/.work/m2:/m2','-w','/proof','--entrypoint','mvn',mvnImage,'-B','-ntp','-o','-s','maven-settings.xml','-Duser.home=/tmp','-Dmaven.repo.local=/m2','test-compile','org.apache.maven.plugins:maven-dependency-plugin:3.9.0:build-classpath','-Dmdep.outputFile=.work/rpc-classpath','-Dmdep.includeScope=test']);
 call('docker',['run','-d','--name',java,'--user',process.getuid()+':'+process.getgid(),'--network',network,'--network-alias','b3-workflow','--label','weaveos.package=V030-037','-v',engine+':/proof','-v',engineCache+':'+engineCache+':ro','-v',engine+'/.work/m2:/m2:ro','-v',root+':'+root,'-v',pg.socket+':'+pg.socket,'-w','/proof','-e','B3_TEST_JDBC_URL=jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture','--entrypoint','sh',mvnImage,'-ec','exec java -cp "target/classes:target/test-classes:$(cat .work/rpc-classpath)" org.weaveos.workflow.RootExecutionRpcFixtureMain']);javaStarted=true;owned.push(java);
 ready=false;for(let i=0;i<120;i++){try{call('docker',['exec',java,'test','-f','/tmp/weaveos-rpc-ready']);ready=true}catch{}if(ready)break;await delay(500)}if(!ready)throw Error('Java fixture not ready');
 const port=call('docker',['exec',java,'cat','/tmp/weaveos-rpc-ready'],{encoding:'utf8'}).trim();if(!/^\d+$/.test(port))throw Error('invalid fixture port');
 const isolation={networkInternal:call('docker',['network','inspect','--format','{{.Internal}}',network],{encoding:'utf8'}).trim(),containers:[]};
 if(isolation.networkInternal!=='true')throw Error('network is not internal');
 for(const name of owned){const info=JSON.parse(call('docker',['inspect',name],{encoding:'utf8'}))[0];if(info.HostConfig.PortBindings&&Object.keys(info.HostConfig.PortBindings).length)throw Error('unexpected published ports');isolation.containers.push({name,ports:info.HostConfig.PortBindings,network:info.HostConfig.NetworkMode});}
 writeFileSync(resolve(dir,'isolation.json'),JSON.stringify(isolation,null,2),{mode:0o600});
 const r=spawnSync('docker',['exec','-w',resolve('services/bff'),'-e','NO_PROXY=b3-workflow,b3-postgres,127.0.0.1,localhost','-e','no_proxy=b3-workflow,b3-postgres,127.0.0.1,localhost','-e','WEAVEOS_RPC_TEST_TARGET=b3-workflow:'+port,'-e','WEAVEOS_TEST_DATABASE_URL='+pg.url,'-e','WEAVEOS_TEST_ARCHIVE_DATABASE_URL='+archive.href,'-e','WEAVEOS_TEST_REDIS_URL=unix://'+redisSocket+'/redis.sock?db=15',java,'/proof/.work/action-interop.test','-test.v=test2json','-test.run','^TestRootWorkflowActionJava.*Interop$','-test.timeout=150s'],{cwd:root,env,encoding:'utf8',maxBuffer:32*1024*1024});
 writeFileSync(resolve(dir,'test.stdout'),r.stdout??'',{mode:0o600});writeFileSync(resolve(dir,'test.stderr'),r.stderr??'',{mode:0o600});writeFileSync(resolve(dir,'test.exit'),String(r.status??1),{mode:0o600});
 const convert=step('convert',go,['tool','test2json','-t','-p','github.com/Hubujiu/WeaveOS/services/bff/cmd/bff'],{input:r.stdout??''});
 if(process.env.WEAVEOS_ACTIONS_JSON_REPORT){
  const report=resolve(process.env.WEAVEOS_ACTIONS_JSON_REPORT);mkdirSync(dirname(report),{recursive:true});writeFileSync(report,convert.stdout,{mode:0o600,flag:'wx'});
 }
 const events=convert.stdout.split('\n').filter(Boolean).map(s=>JSON.parse(s));
 const tests=events.filter(x=>x.Test&&!x.Test.includes('/')&&['pass','fail','skip'].includes(x.Action)).map(x=>({name:x.Test,status:x.Action,elapsed:x.Elapsed}));
 const summary={head:call('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim(),exit:r.status??1,tests,passed:tests.filter(x=>x.status==='pass').length,failed:tests.filter(x=>x.status==='fail').length,skipped:tests.filter(x=>x.status==='skip').length,scope:'real Java Flowable/gRPC + separate engine/app PostgreSQL; real HTTPS existing-task approval chain; not new-instance triggers or complete backend acceptance'};
 writeFileSync(resolve(dir,'summary.json'),JSON.stringify(summary,null,2),{mode:0o600});console.log(JSON.stringify({directory:dir,...summary}));
 process.exitCode=r.status??1;if(tests.length!==3||summary.passed!==3||summary.failed||summary.skipped)process.exitCode=1;
}finally{
 if(javaStarted){
  try{writeFileSync(resolve(dir,'java.log'),call('docker',['logs',java]),{mode:0o600})}catch{}
  // Snapshot before the test fixture's normal shutdown hook drops its schema.
  try{writeFileSync(resolve(dir,'engine-proof.dump'),call('docker',['exec',enginePG,'pg_dump','-Fc','-U','b3_fixture','-d','b3_flowable_fixture']),{mode:0o600})}catch{}
  // Only the dedicated synthetic Java fixture is killed to retain DB evidence.
  try{call('docker',['kill','--signal','KILL',java])}catch{}
 }
 for(const name of owned.filter(n=>n!==java).reverse()){try{call('docker',['stop',name])}catch{}}
 console.log('Private evidence directory: '+dir);
 // No container/volume/network removal or global prune; all evidence retained.
}
