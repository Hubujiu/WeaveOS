import {execFileSync} from 'node:child_process';
import {appendFileSync,readFileSync,statSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {acmePlan} from './acme.mjs';
import {credentialsFromEnv} from './credentials.mjs';
import {receiveCodes} from '../runtime/monitor.mjs';
export function runAcme({operation,credentials,command=execFileSync,log}={}) {
 const p=acmePlan();
 if(!['register','issue','install','renew'].includes(operation))throw Error('ACME operation rejected');
 const env={...process.env,...credentials};
 try{
  const output=command('/bin/sh',['/opt/weaveos-v010/acme-client/acme.sh',...p[operation]],{env,stdio:'pipe',timeout:900000,maxBuffer:4*1024*1024});
  if(log)log(output);
  return {operation,status:'passed',exitCode:0};
 }catch(error){
  if(log)log(Buffer.concat([error.stdout??Buffer.alloc(0),error.stderr??Buffer.alloc(0)]));
  return {operation,status:'failed',exitCode:Number.isInteger(error.status)&&error.status>0?error.status:1};
 }
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
 process.umask(0o077);
 const root='/opt/weaveos-v010';
 try{
  const file=acmePlan().credentials,s=statSync(file);
  if(s.uid!==0||(s.mode&0o777)!==0o600)throw Error('Private credential permissions rejected');
  const credentials=credentialsFromEnv(readFileSync(file,'utf8'));
  const log=bytes=>appendFileSync(root+'/acme-private.log',bytes,{mode:0o600});
  const result=runAcme({operation:process.argv[2],credentials,log});
  if(result.exitCode)receiveCodes(root+'/alerts.jsonl',['CERTIFICATE']);
  console.log(JSON.stringify(result));process.exitCode=result.exitCode;
 }catch{
  receiveCodes(root+'/alerts.jsonl',['CERTIFICATE']);
  console.error('ACME operation failed; inspect private operational log');process.exitCode=1;
 }
}
