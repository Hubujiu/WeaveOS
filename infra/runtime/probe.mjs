import {readFileSync,statfsSync} from 'node:fs';import {X509Certificate} from 'node:crypto';import {resolve} from 'node:path';
export async function sampleRuntime(c){
 const s={ready:false,live:false,database:false,redis:false,redisUsedBytes:0,redisLimitBytes:0,containerMemoryRatios:[],diskFreeRatio:NaN,certificateRemainingMs:NaN,backupFailed:false};
 const health=async path=>{try{return (await fetch('https://localhost:19443/health/'+path,{signal:AbortSignal.timeout(3000)})).status===200;}catch{return false;}};
 [s.ready,s.live]=await Promise.all([health('ready'),health('live')]);
 try{s.database=c.sql('postgres','SELECT 1;')==='1';}catch{}
 try{const info=c.compose('exec','-T','redis','redis-cli','INFO','memory').toString();s.redis=true;s.redisUsedBytes=Number(info.match(/^used_memory:(\d+)/m)?.[1]);s.redisLimitBytes=Number(info.match(/^maxmemory:(\d+)/m)?.[1]);}catch{}
 for(const name of ['nginx','bff','postgres','redis','audit-maintenance']){
  try{const raw=c.command('docker',['stats','--no-stream','--format','{{.MemPerc}}',c.container(name)]).toString().trim();s.containerMemoryRatios.push(Number(raw.replace('%',''))/100);}catch{s.containerMemoryRatios.push(NaN);}
 }
 try{const fs=statfsSync(c.dir);s.diskFreeRatio=fs.bavail/fs.blocks;}catch{}
 try{s.certificateRemainingMs=Date.parse(new X509Certificate(readFileSync(resolve(c.dir,'tls/cert.pem'))).validTo)-Date.now();}catch{}
 return s;
}
