import {appendFileSync,existsSync,lstatSync,mkdirSync,readdirSync,unlinkSync} from 'node:fs';
import {resolve,parse,join} from 'node:path';

const kinds=['access','application','operations','metrics'];
const messages=new Set(['starting BFF host','BFF host failed','BFF authentication configuration invalid','BFF authentication initialization failed','authentication audit persistence failed','session renewal failed after committed write']);
export const approvedRetention=policy=>policy.timeZone==='Asia/Shanghai'&&policy.keepDays===30&&policy.deleteEnabled===true;
function day(at,timeZone){
 if(typeof timeZone!=='string'||!timeZone||!Number.isFinite(new Date(at).getTime()))throw Error('Explicit log timezone and valid timestamp required');
 return new Intl.DateTimeFormat('en-CA',{timeZone,year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date(at));
}
// Operator-owned directories only. Reject links in every existing path segment,
// including junctions, before writing or enumerating approved log children.
function directory(path,create=false){
 const absolute=resolve(path),root=parse(absolute).root;
 let current=root;
 for(const segment of absolute.slice(root.length).split(/[\\/]/).filter(Boolean)){
  current=join(current,segment);
  if(!existsSync(current)){
   if(create)mkdirSync(current,{mode:0o700});else return false;
  }
  const stat=lstatSync(current);
  if(stat.isSymbolicLink()||!stat.isDirectory())throw Error('Log directory links rejected');
 }
 return true;
}
function regular(file){
 const stat=lstatSync(file);
 if(stat.isSymbolicLink()||!stat.isFile()||stat.nlink!==1)throw Error('Log file links rejected');
}
const number=value=>typeof value==='number'&&Number.isFinite(value)?value:null;
export function appendDaily(root,kind,record,policy){
 if(!kinds.includes(kind))throw Error('Unapproved log kind');
 const stamp=day(record.at,policy.timeZone),safe={at:record.at};
 if(kind==='access'){
  if(!/^[a-f0-9]{32}$/.test(record.request_id)||!Number.isInteger(record.status)||record.status<100||record.status>599)throw Error('Invalid access event');
  const method=['GET','HEAD','POST','PUT','PATCH','DELETE','OPTIONS'].includes(record.method)?record.method:'OTHER';
  const path=String(record.path??'').split('?')[0];
  // Unknown paths may themselves contain credential material. Keep route shapes.
  const route=/^\/(?:login|register|health\/(?:ready|live)|api\/v1\/(?:sessions(?:\/current)?|registrations|invitations))$/.test(path)?path:
   /^\/api\/v1\/users\/[a-f0-9-]{36}\/password-reset$/.test(path)?'/api/v1/users/:id/password-reset':path.startsWith('/assets/')?'/assets/*':'[unmatched]';
  Object.assign(safe,{request_id:record.request_id,method,path:route,status:record.status,bytes:number(record.bytes)});
 }else if(kind==='application'){
  safe.message=messages.has(record.msg)?record.msg:'unrecognized application event';
  safe.level=['INFO','WARN','ERROR'].includes(record.level)?record.level:'INFO';
  if(/^[a-f0-9]{32}$/.test(record.request_id))safe.request_id=record.request_id;
 }else if(kind==='operations'){
  if(!['audit','monitor','backup','logs','retention','acme'].includes(record.operation)||!['complete','failed','passed'].includes(record.status))throw Error('Invalid operation event');
  Object.assign(safe,{operation:record.operation,status:record.status,elapsedMs:number(record.elapsedMs)});
 }else{
  for(const key of ['ready','live','database','redis'])safe[key]=record[key]===true;
  for(const key of ['redisUsedBytes','redisLimitBytes','diskFreeRatio','certificateRemainingMs'])safe[key]=number(record[key]);
  safe.containerMemoryRatios=Array.isArray(record.containerMemoryRatios)?record.containerMemoryRatios.slice(0,4).map(number):[];
 }
 const folder=join(resolve(root),kind);directory(folder,true);
 const file=join(folder,stamp+'.jsonl');if(existsSync(file))regular(file);
 appendFileSync(file,JSON.stringify(safe)+'\n',{mode:0o600});
 return `${kind}/${stamp}.jsonl`;
}
export function cleanDaily(root,policy,{now=new Date(),apply=false}={}){
 // Q20 (2026-09-29) authorizes exactly this window for the four runtime kinds.
 if(apply&&!approvedRetention(policy))throw Error('Retention deletion disabled for nonapproved parameters');
 if(!Number.isSafeInteger(policy.keepDays)||policy.keepDays<1||policy.keepDays>3650)throw Error('Explicit approved retention days required');
 const today=day(now,policy.timeZone);
 const cutoff=new Date(Date.parse(today+'T00:00:00Z')-(policy.keepDays-1)*86400000).toISOString().slice(0,10);
 const selected=[];
 if(!directory(root))return selected;
 for(const kind of kinds){
  const folder=join(resolve(root),kind);if(!directory(folder))continue;
  for(const name of readdirSync(folder)){
   if(!/^\d{4}-\d{2}-\d{2}\.jsonl$/.test(name))continue;
   regular(join(folder,name));
   const date=name.slice(0,10);
   if(!Number.isFinite(Date.parse(date+'T00:00:00Z'))||new Date(date+'T00:00:00Z').toISOString().slice(0,10)!==date)throw Error('Invalid log date');
   if(date<cutoff)selected.push(kind+'/'+name);
  }
 }
 selected.sort();
 // Preflight every candidate before the first unlink; never recurse or remove
 // directories, backups, credentials, unknown files or business audit data.
 if(apply)for(const relative of selected){const file=join(resolve(root),relative);regular(file);unlinkSync(file);}
 return selected;
}
