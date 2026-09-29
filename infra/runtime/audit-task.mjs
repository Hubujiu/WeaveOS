import {writeFileSync,renameSync} from 'node:fs';
import {join} from 'node:path';
import {receiveCodes} from './monitor.mjs';
import {configuredLog} from './log-policy.mjs';

// Same entry for the hourly host schedule, manual operations and isolated drills.
// PostgreSQL advisory locking remains authoritative across all callers.
export function runAuditTask(c) {
 const started=Date.now();
 let status='complete';
 try{c.command('docker',[...c.args,'--profile','maintenance','run','--rm','--no-deps','-T','audit-maintenance'],{timeout:330000,maxBuffer:1024*1024});}
 catch{status='failed';receiveCodes(join(c.dir,'alerts.jsonl'),['AUDIT_MAINTENANCE']);}
 const result={at:new Date().toISOString(),operation:'audit',status,elapsedMs:Date.now()-started};
 // Unique staging files avoid competing manual/scheduled writers corrupting JSON.
 const temporary=join(c.dir,`audit-status.${process.pid}.next`);
 writeFileSync(temporary,JSON.stringify(result)+'\n',{mode:0o600});
 renameSync(temporary,join(c.dir,'audit-status.json'));
 configuredLog(c,'operations',result);
 return result;
}
