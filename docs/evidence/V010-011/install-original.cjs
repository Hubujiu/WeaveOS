const {execFileSync}=require('node:child_process');
const {readFileSync,writeFileSync,mkdirSync,chmodSync,existsSync}=require('node:fs');
const {createHash}=require('node:crypto');
const dir='/opt/weaveos-v010';let phase='unpack';
const run=(cmd,args,options={})=>execFileSync(cmd,args,{cwd:dir,stdio:'pipe',timeout:300000,maxBuffer:32*1024*1024,...options});
const sha=b=>createHash('sha256').update(b).digest('hex');
const compose=(...args)=>run('docker',['compose','--env-file',dir+'/.env','-p','weaveos-v010-011','-f',dir+'/compose.json',...args]);
try{
 if(existsSync(dir+'/INSTALLED.json'))throw Error('existing deployment');
 run('tar',['-xf','transfer.tar']);run('tar',['-xf','ops.tar']);
 run('chmod',['-R','go-rwx',dir]);chmodSync(dir+'/tools/goose',0o700);run('chown',['999:999',dir+'/redis.conf']);
 mkdirSync(dir+'/backups',{mode:0o700});
 phase='artifact verification';
 if(sha(readFileSync(dir+'/original-artifact.zip'))!=='4f8594b022be9cc5ee7c97dca50d52dea56b41fd5f47e5085a029209f78a5019')throw Error('original ZIP mismatch');
 mkdirSync(dir+'/original-oci',{mode:0o700});
 run('python3',['-c','import zipfile; z=zipfile.ZipFile("original-artifact.zip"); names=["BUILD.json","bff.oci.tar","web.oci.tar"]; assert set(z.namelist())==set(names); [z.extract(n,"original-oci") for n in names]']);
 const transfer=JSON.parse(readFileSync(dir+'/TRANSFER.json'));
 const source=JSON.parse(readFileSync(dir+'/original-oci/BUILD.json'));
 if(source.commit!==transfer.source)throw Error('source mismatch');
 const results={};
 for(const name of ['bff','web']){
  const a=transfer.dockerArchives[name],o=source[name],file=dir+`/${name}.docker.tar`,oci=dir+`/original-oci/${name}.oci.tar`;
  if(sha(readFileSync(file))!==a.sha256||sha(readFileSync(oci))!==o.archiveSHA256)throw Error('archive mismatch');
  const manifestBytes=run('tar',['-xOf',oci,`blobs/sha256/${o.manifestDigest.slice(7)}`]);
  if('sha256:'+sha(manifestBytes)!==o.manifestDigest)throw Error('OCI manifest mismatch');
  const manifest=JSON.parse(manifestBytes),configBytes=run('tar',['-xOf',oci,`blobs/sha256/${manifest.config.digest.slice(7)}`]);
  if('sha256:'+sha(configBytes)!==manifest.config.digest)throw Error('OCI config mismatch');
  run('docker',['load','-i',file]);
  const image=JSON.parse(run('docker',['image','inspect',a.tag],{encoding:'utf8'}))[0];
  if(image.Id!==manifest.config.digest||image.Config.Labels['org.opencontainers.image.revision']!==source.commit||JSON.stringify(image.RootFS.Layers)!==JSON.stringify(JSON.parse(configBytes).rootfs.diff_ids))throw Error('loaded image differs from original OCI');
  results[name]={configID:image.Id,originalManifest:o.manifestDigest,source:source.commit};
 }
 console.log('Original ZIP, OCI manifest/config and loaded image layers verified');
 phase='database start';compose('up','-d','--wait','postgres','redis');
 const container=compose('ps','-aq','postgres').toString().trim();
 const sql=(db,text)=>run('docker',['exec','-i',container,'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',db],{input:text});
 phase='database migration';sql('postgres','CREATE DATABASE weaveos_cold_archive;');
 for(const [file,target] of [['tools/goose','/tmp/weaveos-goose'],['migrations','/tmp/migrations'],['archive-migrations','/tmp/archive-migrations'],['migration.env','/tmp/migration.env'],['cold-migration.env','/tmp/cold-migration.env']])run('docker',['cp',dir+'/'+file,container+':'+target]);
 run('docker',['exec',container,'sh','-ec','set -a; . /tmp/migration.env; /tmp/weaveos-goose -dir /tmp/migrations up; . /tmp/cold-migration.env; /tmp/weaveos-goose -dir /tmp/archive-migrations up']);
 phase='bootstrap initialization';sql('weaveos_runtime',readFileSync(dir+'/bootstrap.sql'));
 phase='restricted roles';sql('weaveos_runtime',readFileSync(dir+'/roles.sql'));sql('weaveos_cold_archive',readFileSync(dir+'/cold-roles.sql'));sql('postgres',readFileSync(dir+'/login-roles.sql'));
 run('docker',['exec',container,'rm','-f','/tmp/migration.env','/tmp/cold-migration.env','/tmp/weaveos-goose']);
 phase='service start';compose('up','-d','bff','audit-maintenance','nginx');
 phase='readiness';let ready=false;
 for(let i=0;i<60;i++){
  try{run('curl',['--fail','--silent','--cacert',dir+'/tls/cert.pem','https://localhost:19443/health/ready']);ready=true;break;}catch{run('sleep',['1']);}
 }
 if(!ready)throw Error('readiness unavailable');
 const state={source:source.commit,images:results,project:'weaveos-v010-011',installedAt:new Date().toISOString(),access:'SSH tunnel only; 127.0.0.1:19443',productTestsOnServer:false,buildsOnServer:false};
 writeFileSync(dir+'/INSTALLED.json',JSON.stringify(state,null,2),{flag:'wx',mode:0o600});
 console.log(JSON.stringify(state));
}catch{console.error('Deployment failed during '+phase+'; private details withheld');process.exit(1);}
