// Invoke previously verified backup and monitor operations; no build/test entry.
import {serverContext} from './context.mjs';
import {sampleServer} from './probe.mjs';
import {receiveAlarms} from '../runtime/monitor.mjs';
import {backupDatabase} from '../runtime/backup.mjs';
const c=serverContext();
const operation=process.argv[2];
if(operation==='monitor'){
 const codes=receiveAlarms(c.dir+'/alerts.jsonl',await sampleServer(c));
 if(codes.length)console.log(JSON.stringify({codes}));
}else if(operation==='backup'){
 const stamp=new Date().toISOString().replaceAll(':','-');
 for(const database of ['weaveos_runtime','weaveos_cold_archive']){
  const result=backupDatabase({container:c.container('postgres'),database,user:'weaveos_backup',keyFile:c.dir+'/secrets/backup.key',backupFile:c.dir+`/backups/${database}-${stamp}.enc`,alertFile:c.dir+'/alerts.jsonl'});
  console.log(JSON.stringify({database,...result}));
 }
}else throw new Error('Usage: operations.mjs monitor|backup');
