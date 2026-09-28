import {execFileSync} from 'node:child_process';
import {randomBytes} from 'node:crypto';
// No shell evaluation, no DSN in argv/logs, and Goose Up only.
export function applyMigrations({container,goose,directory,env}) {
 if(!/^weaveos-v010-[a-zA-Z0-9_-]+$/.test(container))throw Error('Unexpected database container');
 const values=Object.fromEntries(env.split(/\r?\n/).filter(Boolean).map(line=>{const i=line.indexOf('=');return [line.slice(0,i),line.slice(i+1)];}));
 if(values.GOOSE_DRIVER!=='postgres'||!values.GOOSE_DBSTRING?.startsWith('postgres://'))throw Error('Migration credentials invalid');
 const path='/tmp/weaveos-deploy-'+randomBytes(8).toString('hex');
 const run=(args,options={})=>execFileSync('docker',args,{stdio:'pipe',timeout:120000,...options});
 try{
  run(['exec',container,'mkdir','-m','700',path]);
  run(['cp',goose,container+':'+path+'/goose']);
  run(['exec',container,'chmod','700',path+'/goose']);
  run(['cp',directory,container+':'+path+'/migrations']);
  run(['exec','-e','GOOSE_DRIVER','-e','GOOSE_DBSTRING',container,path+'/goose','-dir',path+'/migrations','up'],{env:{...process.env,GOOSE_DRIVER:values.GOOSE_DRIVER,GOOSE_DBSTRING:values.GOOSE_DBSTRING}});
 }catch{throw Error('Compatible migration failed');}
 finally{run(['exec',container,'rm','-rf','--',path]);}
}
