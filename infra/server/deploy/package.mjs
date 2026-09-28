// CI only: promote the exact OCI artifacts that completed acceptance, never rebuild.
import {readFileSync,writeFileSync,readdirSync,mkdirSync,statSync,openSync,writeSync,closeSync,createReadStream} from 'node:fs';
import {resolve} from 'node:path';
import {execFileSync} from 'node:child_process';
import {importArtifacts} from '../../runtime/artifacts.mjs';
import {hash,canonical,validateEnvelope} from './bundle.mjs';
const [input,output]=process.argv.slice(2);
if(!input||!output)throw Error('Usage: package.mjs BUILD.json output-directory');
const record=importArtifacts(resolve(input));
if(record.commit!==process.env.GITHUB_SHA)throw Error('Artifact is not this workflow commit');
const out=resolve(output);mkdirSync(out,{recursive:true});
const compatibility=JSON.parse(readFileSync('infra/server/deploy/compatibility.json'));
const e={protocol:1,commit:record.commit,runId:Number(process.env.GITHUB_RUN_ID)*1000+Number(process.env.GITHUB_RUN_ATTEMPT??1),backwardCompatible:compatibility.backwardCompatible,approved:compatibility.migrations,config:compatibility.config,migrations:{},files:{},images:{}};
for(const [name,path] of [['compose.json','infra/runtime/compose.json'],['nginx.conf','infra/acceptance/nginx.conf']])e.files[name]=canonical(readFileSync(path,'utf8'));
for(const dir of ['migrations','archive-migrations'])for(const file of readdirSync('db/'+dir).filter(f=>f.endsWith('.sql'))){const name=dir+'/'+file;e.files[name]=canonical(readFileSync('db/'+name,'utf8'));e.migrations[name]=hash(e.files[name]);}
for(const name of ['bff','web']){
 const archive=resolve(out,name+'.docker.tar');
 execFileSync('docker',['save','--output',archive,record[name].tag],{stdio:'pipe',timeout:120000});
 const manifest=JSON.parse(execFileSync('tar',['-xOf',record[name].archive,'blobs/sha256/'+record[name].manifestDigest.slice(7)],{encoding:'utf8'}));
 const inspect=JSON.parse(execFileSync('docker',['image','inspect',record[name].tag],{encoding:'utf8'}))[0];
 e.images[name]={size:statSync(archive).size,sha256:hash(readFileSync(archive)),imageID:manifest.config.digest,layers:inspect.RootFS.Layers,ociDigest:record[name].manifestDigest,ociSHA256:record[name].archiveSHA256};
}
validateEnvelope(e);
const bytes=Buffer.from(JSON.stringify(e)),length=Buffer.alloc(4);length.writeUInt32BE(bytes.length);
if(bytes.length>1024*1024)throw Error('Release header too large');
const fd=openSync(resolve(out,'release.bin'),'wx',0o600);
try{writeSync(fd,length);writeSync(fd,bytes);for(const name of ['bff','web'])for await(const chunk of createReadStream(resolve(out,name+'.docker.tar')))writeSync(fd,chunk);}finally{closeSync(fd);}
writeFileSync(resolve(out,'manifest.json'),JSON.stringify(e,null,2));
console.log(JSON.stringify({commit:e.commit,runId:e.runId,images:e.images}));
