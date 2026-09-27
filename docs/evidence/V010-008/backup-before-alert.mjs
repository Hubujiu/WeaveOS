import { execFileSync } from 'node:child_process';
import { readFileSync, openSync, closeSync, writeFileSync } from 'node:fs';
import { seal, open } from './backup-crypto.mjs';
function valid(o){
 if(!/^weaveos-v010-[a-zA-Z0-9_-]+$/.test(o.container)||!/^weaveos_[a-zA-Z0-9_]+$/.test(o.database)||!/^weaveos_[a-zA-Z0-9_]+$/.test(o.user))throw new Error('Refusing non-isolated database operation');
 if(!o.keyFile||!o.backupFile||o.keyFile===o.backupFile)throw new Error('Separate backup/key files required');
}
export function privateFile(file,bytes){
 const fd=openSync(file,'wx',0o600);
 try{
  if(process.platform==='win32'){
   const identity=execFileSync('whoami',['/user','/fo','csv','/nh'],{encoding:'utf8',stdio:'pipe'});
   const sid=identity.match(/S-1-[0-9-]+/)?.[0];if(!sid)throw new Error('Private file identity unavailable');
   execFileSync('icacls',[file,'/inheritance:r','/grant:r',`*${sid}:F`],{stdio:'pipe'});
  }
  writeFileSync(fd,bytes);
 }finally{closeSync(fd);}
}
const docker=(args,options={})=>execFileSync('docker',args,{stdio:['pipe','pipe','pipe'],maxBuffer:64*1024*1024,timeout:120000,...options});
export function backupDatabase(o){
 valid(o);const start=Date.now();const key=readFileSync(o.keyFile);
 try{
  const dump=docker(['exec',o.container,'pg_dump','-U',o.user,'-d',o.database,'--format=custom','--no-owner','--no-privileges']);
  try{privateFile(o.backupFile,seal(dump,key));}finally{dump.fill(0);}
  return {elapsedMs:Date.now()-start,format:'pg_dump18 custom + AES256-GCM',result:'passed'};
 }catch{throw new Error('Encrypted database backup failed');}
 finally{key.fill(0);}
}
export function restoreDatabase(o){
 valid(o);const start=Date.now(),key=readFileSync(o.keyFile);let dump;
 try{
  // Authenticate before even opening a database connection. Never stream
  // unauthenticated plaintext into pg_restore.
  dump=open(readFileSync(o.backupFile),key);
  const existing=docker(['exec',o.container,'psql','-X','-At','-v','ON_ERROR_STOP=1','-U',o.user,'-d',o.database,'-c',"SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','S')"],{encoding:'utf8'}).trim();
  if(existing!=='0')throw new Error('Restore target must be empty');
  docker(['exec','-i',o.container,'pg_restore','-U',o.user,'-d',o.database,'--no-owner','--no-privileges','--exit-on-error','--single-transaction'],{input:dump});
  return {elapsedMs:Date.now()-start,result:'passed'};
 }catch{throw new Error('Isolated database restore failed');}
 finally{key.fill(0);dump?.fill(0);}
}
