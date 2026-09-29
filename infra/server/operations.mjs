// Invoke previously verified backup and monitor operations; no build/test entry.
import {serverContext} from './context.mjs';
import {sampleServer} from './probe.mjs';
import {receiveAlarms} from '../runtime/monitor.mjs';
import {backupDatabase} from '../runtime/backup.mjs';
import {runAuditTask} from '../runtime/audit-task.mjs';
import {readLogPolicy,configuredLog} from '../runtime/log-policy.mjs';
import {collectLogs} from '../runtime/log-collection.mjs';
import {cleanDaily} from '../runtime/daily-logs.mjs';
const c=serverContext();
const operation=process.argv[2];
const started=Date.now();
try{
if(operation==='monitor'){
 const sample=await sampleServer(c);
 configuredLog(c,'metrics',{at:new Date().toISOString(),...sample});
 const codes=receiveAlarms(c.dir+'/alerts.jsonl',sample);
 if(codes.length)console.log(JSON.stringify({codes}));
}else if(operation==='backup'){
 const stamp=new Date().toISOString().replaceAll(':','-');
 for(const database of ['weaveos_runtime','weaveos_cold_archive']){
  const result=backupDatabase({container:c.container('postgres'),database,user:'weaveos_backup',keyFile:c.dir+'/secrets/backup.key',backupFile:c.dir+`/backups/${database}-${stamp}.enc`,alertFile:c.dir+'/alerts.jsonl'});
  console.log(JSON.stringify({database,...result}));
 }
}else if(operation==='audit'){
 const result=runAuditTask(c);
 console.log(JSON.stringify(result));
 if(result.status!=='complete')process.exitCode=1;
}else if(operation==='logs'){
 const policy=readLogPolicy(c);if(!policy)throw Error('Log policy must be installed explicitly');
 console.log(JSON.stringify(collectLogs(c,policy)));
}else if(operation==='retention'){
 const policy=readLogPolicy(c);if(!policy)throw Error('Log policy must be installed explicitly');
 const apply=process.argv.includes('--apply');
 console.log(JSON.stringify({operation:'retention',dryRun:!apply,files:cleanDaily(c.dir+'/logs',policy,{apply})}));
}else throw new Error('Unknown operation');
if(operation!=='audit')configuredLog(c,'operations',{at:new Date().toISOString(),operation,status:'complete',elapsedMs:Date.now()-started});
}catch{
 try{configuredLog(c,'operations',{at:new Date().toISOString(),operation,status:'failed',elapsedMs:Date.now()-started});}catch{}
 console.error(JSON.stringify({status:'failed',operation:['monitor','backup','audit','logs','retention'].includes(operation)?operation:'unknown'}));
 process.exitCode=1;
}
