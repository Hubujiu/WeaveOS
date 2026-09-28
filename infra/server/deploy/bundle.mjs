import {createHash} from 'node:crypto';
import {openSync,writeSync,closeSync} from 'node:fs';
import {join} from 'node:path';
import {validateCompatibility,candidateCompose,publicNginx} from './policy.mjs';
export const hash=value=>createHash('sha256').update(value).digest('hex');
export const canonical=value=>value.replaceAll('\r\n','\n');
export function validateEnvelope(e) {
 if(e.protocol!==1)throw Error('Unknown protocol');
 validateCompatibility({runId:0,migrations:{}},e);
 const expected=['compose.json','nginx.conf',...Object.keys(e.migrations)].sort();
 if(JSON.stringify(Object.keys(e.files).sort())!==JSON.stringify(expected))throw Error('Unexpected release files');
 for(const [name,bytes] of Object.entries(e.files))if(typeof bytes!=='string'||bytes.includes('\0')||hash(bytes)!==(e.config[name]??e.migrations[name]))throw Error('Release content hash mismatch');
 for(const name of ['bff','web']){
  const a=e.images[name];
  if(!a||!Number.isSafeInteger(a.size)||a.size<1||a.size>512*1024*1024||!/^sha256:[a-f0-9]{64}$/.test(a.imageID)||!/^([a-f0-9]{64})$/.test(a.sha256)||!Array.isArray(a.layers)||!a.layers.length||a.layers.some(l=>!/^sha256:[a-f0-9]{64}$/.test(l)))throw Error('Image metadata invalid');
 }
 candidateCompose(JSON.parse(e.files['compose.json']),{bff:e.images.bff.imageID,web:e.images.web.imageID});
 publicNginx(e.files['nginx.conf']);
 return true;
}
export async function receiveBundle(stream,dir) {
 const iterator=stream[Symbol.asyncIterator]();let pending=Buffer.alloc(0);
 async function take(n){
  const chunks=[];let size=0;
  while(size<n){
   if(!pending.length){const next=await iterator.next();if(next.done)throw Error('Incomplete release');pending=Buffer.from(next.value);}
   const count=Math.min(n-size,pending.length);chunks.push(pending.subarray(0,count));pending=pending.subarray(count);size+=count;
  }
  return Buffer.concat(chunks,n);
 }
 const length=(await take(4)).readUInt32BE();if(length<2||length>1024*1024)throw Error('Header size rejected');
 const e=JSON.parse((await take(length)).toString('utf8'));validateEnvelope(e);
 for(const name of ['bff','web']){
  const fd=openSync(join(dir,name+'.docker.tar'),'wx',0o600),digest=createHash('sha256');
  try{for(let left=e.images[name].size;left;){const bytes=await take(Math.min(left,65536));writeSync(fd,bytes);digest.update(bytes);left-=bytes.length;}}
  finally{closeSync(fd);}
  if(digest.digest('hex')!==e.images[name].sha256)throw Error('Image bytes changed');
 }
 if(pending.length||!(await iterator.next()).done)throw Error('Unexpected trailing content');
 return e;
}
