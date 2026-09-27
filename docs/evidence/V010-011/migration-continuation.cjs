const {execFileSync}=require('node:child_process');const {readFileSync,writeFileSync,existsSync}=require('node:fs');
const dir='/opt/weaveos-v010';let phase='migration';
const run=(bin,args,options={})=>execFileSync(bin,args,{cwd:dir,stdio:'pipe',timeout:120000,...options});
const compose=(...a)=>run('docker',['compose','--env-file',dir+'/.env','-p','weaveos-v010-011','-f',dir+'/compose.json',...a]);
try{
 if(existsSync(dir+'/INSTALLED.json'))throw Error('Already installed');
 const container=compose('ps','-aq','postgres').toString().trim();
 // dotenv entries are data, not shell code: URL query ampersands must never
 // be interpreted by `source`. Pass parsed values directly to the process.
 for(const [file,migrations] of [['migration.env','migrations'],['cold-migration.env','archive-migrations']]){
  const env=readFileSync(dir+'/'+file,'utf8').trim().split('\n').flatMap(line=>['-e',line]);
  run('docker',['exec',...env,container,'/tmp/weaveos-goose','-dir','/tmp/'+migrations,'up']);
 }
 console.log('Hot and cold published migrations applied');
 const sql=(db,input)=>run('docker',['exec','-i',container,'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',db],{input});
 phase='bootstrap';sql('weaveos_runtime',readFileSync(dir+'/bootstrap.sql'));
 phase='roles';sql('weaveos_runtime',readFileSync(dir+'/roles.sql'));sql('weaveos_cold_archive',readFileSync(dir+'/cold-roles.sql'));sql('postgres',readFileSync(dir+'/login-roles.sql'));
 run('docker',['exec',container,'rm','-f','/tmp/migration.env','/tmp/cold-migration.env','/tmp/weaveos-goose']);
 phase='start';compose('up','-d','bff','audit-maintenance','nginx');
 phase='readiness';let ready=false;
 for(let i=0;i<60;i++){try{run('curl',['--fail','--silent','--cacert',dir+'/tls/cert.pem','https://localhost:19443/health/ready']);ready=true;break;}catch{run('sleep',['1']);}}
 if(!ready)throw Error('Not ready');
 const record=JSON.parse(readFileSync(dir+'/BUILD.json')),images={};
 for(const name of ['bff','web']){const image=JSON.parse(run('docker',['inspect',record[name].tag],{encoding:'utf8'}))[0];images[name]={configID:image.Id,originalManifest:record[name].manifestDigest,source:record.commit};}
 const result={source:record.commit,images,project:'weaveos-v010-011',installedAt:new Date().toISOString(),access:'SSH tunnel only; 127.0.0.1:19443',productTestsOnServer:false,buildsOnServer:false};
 writeFileSync(dir+'/INSTALLED.json',JSON.stringify(result,null,2),{flag:'wx',mode:0o600});console.log(JSON.stringify(result));
}catch{console.error('Deployment continuation failed during '+phase+'; private details withheld');process.exit(1);}
