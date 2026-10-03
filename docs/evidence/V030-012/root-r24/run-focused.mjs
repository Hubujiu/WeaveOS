// Root-authored focused real record journey. Not the full product acceptance gate.
// Uses the existing isolated Compose topology, real BFF/PG/Redis and no API mocks.
import {spawnSync} from 'node:child_process';
import {existsSync,mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {randomBytes} from 'node:crypto';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../../../../',import.meta.url));
const relative='.work/root-record-acceptance',dir=resolve(root,relative);
if(existsSync(dir))throw new Error('Existing focused acceptance directory: inspect it; never overwrite');
mkdirSync(dir,{recursive:true,mode:0o700});mkdirSync(resolve(dir,'tls'),{mode:0o700});mkdirSync(resolve(dir,'public'),{mode:0o700});
const project='weaveos-root-record-'+Date.now(),env={...process.env,WEAVEOS_ACCEPTANCE_DIR:dir};
const secrets=[randomBytes(32).toString('hex'),randomBytes(32).toString('base64')];
const redact=text=>secrets.reduce((value,secret)=>secret?value.split(secret).join('[REDACTED]'):value,String(text));
const pub=(name,value)=>writeFileSync(resolve(dir,'public',name),value);
const privateFile=(name,value)=>writeFileSync(resolve(dir,name),value,{mode:0o600,flag:'wx'});
let sequence=0;
function run(command,args,options={}){
 const label=String(++sequence).padStart(3,'0');
 pub(label+'-command.json',JSON.stringify({command,args,cwd:options.cwd??root,environment:{WEAVEOS_ACCEPTANCE_DIR:dir}},null,2));
 const result=spawnSync(command,args,{cwd:root,env,encoding:'utf8',stdio:'pipe',maxBuffer:64*1024*1024,timeout:900_000,...options});
 const code=result.status??1;
 pub(label+'.log',redact(String(result.stdout??'')+'\n'+String(result.stderr??'')+(result.error?'\n'+String(result.error.message):'')));
 pub(label+'.exit',String(code)+'\n');
 if(code!==0)throw new Error('Focused acceptance stage '+label+' failed; see sanitized public log');
 return String(result.stdout??'');
}
const composeArgs=['compose','-p',project,'-f',resolve(root,'infra/acceptance/compose.json')];
const compose=(...args)=>run('docker',[...composeArgs,...args]);
const container=name=>compose('ps','-q',name).trim();
const mount=['--mount','type=bind,src='+root+',dst=/repo'];
const pgURL='postgres://weaveos_test:'+secrets[0]+'@127.0.0.1:5432/weaveos_acceptance?sslmode=disable&connect_timeout=2';
privateFile('postgres.env','POSTGRES_USER=weaveos_test\nPOSTGRES_PASSWORD='+secrets[0]+'\nPOSTGRES_DB=weaveos_ci_test\n');
privateFile('runtime.env','WEAVEOS_DATABASE_URL='+pgURL.replace('@127.0.0.1:','@postgres:')+'\nWEAVEOS_REDIS_URL=redis://redis:6379/0\nWEAVEOS_SESSION_GENERATION='+project+'\nWEAVEOS_AUDIT_KEY_ID=test\nWEAVEOS_AUDIT_HMAC_KEY='+secrets[1]+'\n');
privateFile('seed.env','WEAVEOS_TEST_DATABASE_URL='+pgURL+'\nWEAVEOS_ACCEPTANCE_FIXTURES=/repo/'+relative+'/fixtures.json\n');
const goImage='golang:1.27.1';
const browserImage='mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27';
const go=script=>run('docker',['run','--rm','--network','container:'+container('postgres'),...mount,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','--env-file',resolve(dir,'seed.env'),'-e','GOTOOLCHAIN=local','-e','GOFLAGS=-buildvcs=false','-e','GOBIN=/repo/'+relative+'/tools','-w','/repo/services/bff',goImage,'sh','-ec',script]);
const node=(script,network)=>run('docker',['run','--rm','--init','--shm-size=1g',...(network?['--network','container:'+network]:[]),...mount,'--mount','type=volume,src=weaveos-v010-linux-node,dst=/repo/node_modules','--mount','type=volume,src=weaveos-v010-linux-web-node,dst=/repo/apps/web/node_modules','-e','CI=true','-e','WEAVEOS_WEB_URL=https://localhost:19443','-e','WEAVEOS_ACCEPTANCE_FIXTURES=/repo/'+relative+'/fixtures.json','-e','NODE_EXTRA_CA_CERTS=/repo/'+relative+'/tls/cert.pem','-w','/repo',browserImage,'bash','-euc','npm install --global pnpm@10.28.2 --ignore-scripts; '+script]);
const started=Date.now();
pub('result.json',JSON.stringify({result:'running',project,scope:'one Root real record journey; not full product acceptance'}));
let startedCompose=false;
try{
 run('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-days','2','-keyout',resolve(dir,'tls/key.pem'),'-out',resolve(dir,'tls/cert.pem'),'-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1']);
 startedCompose=true;compose('up','-d','--wait','postgres','redis');
 compose('exec','-T','postgres','createdb','-U','weaveos_test','weaveos_acceptance');
 go('test "$(go env GOVERSION)" = "go$(cat /repo/.go-version)"; go install github.com/pressly/goose/v3/cmd/goose@v3.28.0; /repo/'+relative+'/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up; CGO_ENABLED=0 go build -o /repo/'+relative+'/bff ./cmd/bff; go run ./cmd/acceptance-seed');
 if(typeof process.getuid==='function')run('docker',['run','--rm',...mount,goImage,'chown',process.getuid()+':'+process.getgid(),'/repo/'+relative+'/fixtures.json']);
 const fixture=JSON.parse(readFileSync(resolve(dir,'fixtures.json'),'utf8'));
 const capturePasswords=value=>{if(value&&typeof value==='object')for(const [key,child] of Object.entries(value)){if(/password|secret|token/i.test(key)&&typeof child==='string')secrets.push(child);else capturePasswords(child);}};
 capturePasswords(fixture);
 node('pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store; pnpm typecheck; pnpm build');
 compose('up','-d','bff','nginx');
 const readiness="(async()=>{let ready=false;for(let i=0;i<60;i++){try{const response=await fetch('https://localhost:19443/api/v1/sessions/current');if(response.status===401){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,500));}if(!ready)throw new Error('Isolated HTTPS session endpoint did not become ready');})().catch(error=>{console.error(error.message);process.exitCode=1;});";
 const testTitle='^Root real HTTPS full journey creates an application, saves a form, and creates and edits a persisted record$';
 node('node -e '+JSON.stringify(readiness)+'; PLAYWRIGHT_JSON_OUTPUT_NAME=/repo/'+relative+'/public/journey.json pnpm exec playwright test --config apps/web/playwright.root-record.integration.config.ts --grep '+JSON.stringify(testTitle)+' --project=chromium --reporter=line,json --output /repo/'+relative+'/public/playwright',container('nginx'));
 const browserReport=JSON.parse(readFileSync(resolve(dir,'public/journey.json'),'utf8'));
 if(browserReport.stats?.expected!==1||browserReport.stats?.unexpected!==0||browserReport.stats?.skipped!==0)throw new Error('Expected exactly one passing journey with no skipped cases');
 pub('result.json',JSON.stringify({result:'passed',project,elapsedSeconds:(Date.now()-started)/1000,scope:'one real Chromium Root journey; full product/Firefox/WebKit gates not run'},null,2));
}catch(error){pub('result.json',JSON.stringify({result:'failed',project,elapsedSeconds:(Date.now()-started)/1000,reason:String(error.message),scope:'focused Root journey'},null,2));throw error;}
finally{const report=resolve(dir,'public/journey.json');if(existsSync(report))writeFileSync(report,redact(readFileSync(report,'utf8')));if(startedCompose)compose('stop');}
