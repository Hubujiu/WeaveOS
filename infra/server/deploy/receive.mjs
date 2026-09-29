// Installed, pinned receiver. SSH key is restricted to this command under flock.
// Server operations only; never builds, tests, scans or executes uploaded programs.
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,existsSync,renameSync,unlinkSync,statfsSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {request} from 'node:https';
import {receiveBundle} from './bundle.mjs';
import {validateCompatibility,promote} from './policy.mjs';
import {candidateCompose,publicNginx} from './configuration.mjs';
import {applyMigrations} from './migrate.mjs';
import {serverContext} from '../context.mjs';
const root='/opt/weaveos-v010',deploy=root+'/deploy-state';
process.umask(0o077);
const c=serverContext(),run=(bin,args,options={})=>execFileSync(bin,args,{stdio:'pipe',timeout:120000,maxBuffer:4*1024*1024,...options});
const json=path=>JSON.parse(readFileSync(path,'utf8'));
function atomic(path,value){writeFileSync(path+'.next',JSON.stringify(value,null,2),{mode:0o600});renameSync(path+'.next',path);}
function replace(path,value){writeFileSync(path+'.next',value,{mode:0o600});renameSync(path+'.next',path);}
async function health(){
 for(let i=0;i<45;i++){
  const ok=await Promise.all([['/health/ready',200],['/login',200],['/api/v1/sessions/current',401]].map(([path,status])=>new Promise(resolve=>{
   const req=request({hostname:'weave.hubujiu.site',servername:'weave.hubujiu.site',port:443,path,lookup:(_h,_o,cb)=>cb(null,'127.0.0.1',4),family:4},res=>{res.resume();resolve(res.statusCode===status);});req.on('error',()=>resolve(false));req.setTimeout(2000,()=>{req.destroy();resolve(false);});req.end();
  })));
  if(ok.every(Boolean))return;
  await new Promise(r=>setTimeout(r,1000));
 }
 throw Error('Public TLS readiness failed');
}
const up=()=>c.compose('up','-d','--no-deps','--pull','never','--force-recreate','bff','audit-maintenance','nginx');
let phase='receive',stage;
const timeout=setTimeout(()=>{console.error('Deployment receiver timed out; journal retained');process.exit(1);},15*60*1000);
try{
 if(!existsSync(deploy+'/current.json'))throw Error('Receiver is not initialized');
 if(existsSync(deploy+'/journal.json'))throw Error('Interrupted promotion requires recovery before another release');
 const disk=statfsSync(root);if(disk.bavail*disk.bsize<3*1024**3)throw Error('Insufficient release/backup headroom');
 stage=mkdtempSync(deploy+'/incoming-');
 const e=await receiveBundle(process.stdin,stage),current=json(deploy+'/current.json');
 const ledger=json(deploy+'/ledger.json');
 let previous;
 const result=await promote({
  validate:async()=>{
   phase='validate';validateCompatibility(ledger,e);
   const compose=candidateCompose(JSON.parse(e.files['compose.json']),{bff:e.images.bff.imageID,web:e.images.web.imageID});
   const existing=json(root+'/compose.json');
   // Storage engines/volumes have a separate upgrade procedure, never replace them
   // implicitly while promoting application/config changes.
   for(const name of ['postgres','redis'])if(JSON.stringify(compose.services[name])!==JSON.stringify(existing.services[name]))throw Error('Storage configuration requires separate reviewed upgrade');
   for(const name of ['bff','web']){
    run('docker',['load','--input',stage+'/'+name+'.docker.tar']);
    const image=JSON.parse(run('docker',['image','inspect',e.images[name].imageID],{encoding:'utf8'}))[0];
    if(image.Id!==e.images[name].imageID||image.Config.Labels?.['org.opencontainers.image.revision']!==e.commit||JSON.stringify(image.RootFS.Layers)!==JSON.stringify(e.images[name].layers))throw Error('Loaded image identity differs');
   }
   for(const [name,bytes] of Object.entries(e.files).filter(([name])=>name.includes('/'))){mkdirSync(stage+'/'+name.split('/')[0],{recursive:true});writeFileSync(stage+'/'+name,bytes,{mode:0o600});}
   writeFileSync(stage+'/compose.json',JSON.stringify(compose,null,2));
   writeFileSync(stage+'/public-nginx.conf',publicNginx(e.files['nginx.conf']));
   previous={compose:readFileSync(root+'/compose.json','utf8'),nginx:readFileSync(root+'/public-nginx.conf','utf8'),current};
   atomic(stage+'/previous.json',previous);atomic(stage+'/manifest.json',e);
   // Parse config without emitting interpolated secrets.
   run('docker',['compose','--env-file',root+'/.env','-p','weaveos-v010-011','-f',stage+'/compose.json','config','--quiet']);
  },
  backup:async()=>{phase='backup';run('/usr/bin/flock',['-w','180',root+'/backup.lock','/usr/local/bin/node',root+'/infra/server/operations.mjs','backup'],{timeout:300000});},
  migrate:async()=>{
   phase='migrate';
   // Conservatively freeze every attempted migration, including a partial failure.
   atomic(deploy+'/ledger.json',{runId:e.runId,commit:e.commit,migrations:e.migrations});
   for(const [directory,env] of [['migrations','migration.env'],['archive-migrations','cold-migration.env']])applyMigrations({container:c.container('postgres'),goose:root+'/tools/goose',directory:stage+'/'+directory,env:readFileSync(root+'/'+env,'utf8')});
  },
  activate:async()=>{
   phase='activate';atomic(deploy+'/journal.json',{stage,previous});
   replace(root+'/compose.json',readFileSync(stage+'/compose.json'));
   replace(root+'/public-nginx.conf',readFileSync(stage+'/public-nginx.conf'));
   up();
  },
  health:async()=>{phase='health';await health();},
  record:async()=>{phase='record';atomic(deploy+'/current.json',{runId:e.runId,commit:e.commit,images:e.images,migrations:e.migrations,stage,deployedAt:new Date().toISOString()});unlinkSync(deploy+'/journal.json');},
  restore:async()=>{replace(root+'/compose.json',previous.compose);replace(root+'/public-nginx.conf',previous.nginx);up();atomic(deploy+'/current.json',previous.current);},
  healthPrevious:async()=>{await health();if(existsSync(deploy+'/journal.json'))unlinkSync(deploy+'/journal.json');}
 });
 atomic(stage+'/result.json',result);
 console.log(JSON.stringify({...result,commit:e.commit,runId:e.runId}));
 if(result.status!=='deployed')process.exitCode=1;
}catch{console.error(JSON.stringify({status:'failed',phase,details:'Private diagnostics withheld; inspect deployment journal and release metadata'}));process.exitCode=1;}
finally{clearTimeout(timeout);}
