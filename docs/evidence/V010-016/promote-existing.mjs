// Operational promotion of the already accepted, already installed artifact.
// No build, conversion, product tests or scans on the server.
import {readFileSync,writeFileSync,openSync,writeSync,closeSync,createReadStream,statSync,mkdirSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const root='/opt/weaveos-v010',dir=root+'/releases/V010-016';
mkdirSync(dir,{recursive:true,mode:0o700});
const e=JSON.parse(readFileSync(dir+'/baseline-header.json'));
const record=JSON.parse(readFileSync(root+'/BUILD.json'));
const transfer=JSON.parse(readFileSync(root+'/TRANSFER.json'));
if(record.commit!=='6e946105f9cb8e77460c54ae2c74ef0d9f021302'||transfer.source!==record.commit)throw Error('Preserve the accepted installed source');
for(const name of ['bff','web']){
 const archive=root+'/'+name+'.docker.tar';
 const sha256=createHash('sha256').update(readFileSync(archive)).digest('hex');
 if(sha256!==transfer.dockerArchives[name].sha256)throw Error('Previously verified portable artifact changed');
 const inspect=JSON.parse(execFileSync('docker',['image','inspect',record[name].tag],{encoding:'utf8'}))[0];
 if(inspect.Config.Labels['org.opencontainers.image.revision']!==record.commit||JSON.stringify(inspect.RootFS.Layers)!==JSON.stringify(transfer.dockerArchives[name].layers))throw Error('Existing image/source mismatch');
 e.images[name]={size:statSync(archive).size,sha256,imageID:inspect.Id,layers:inspect.RootFS.Layers,ociDigest:record[name].manifestDigest,ociSHA256:record[name].archiveSHA256};
}
const bytes=Buffer.from(JSON.stringify(e)),length=Buffer.alloc(4);length.writeUInt32BE(bytes.length);
const fd=openSync(dir+'/baseline-release.bin','wx',0o600);
try{writeSync(fd,length);writeSync(fd,bytes);for(const name of ['bff','web'])for await(const chunk of createReadStream(root+'/'+name+'.docker.tar'))writeSync(fd,chunk);}finally{closeSync(fd);}
writeFileSync(dir+'/baseline-manifest.json',JSON.stringify(e,null,2),{flag:'wx',mode:0o600});
console.log(JSON.stringify({preparedExistingArtifact:true,commit:e.commit,sequence:e.runId}));
