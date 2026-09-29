import {existsSync,readFileSync} from 'node:fs';
import {join} from 'node:path';
import {appendDaily} from './daily-logs.mjs';
export function readLogPolicy(c){
 const file=join(c.dir,'log-policy.json');
 if(!existsSync(file))return null;
 const policy=JSON.parse(readFileSync(file,'utf8'));
 if(typeof policy.timeZone!=='string'||!policy.timeZone)throw Error('Explicit log timezone required');
 new Intl.DateTimeFormat('en-CA',{timeZone:policy.timeZone});
 if(policy.keepDays!==null&&(!Number.isSafeInteger(policy.keepDays)||policy.keepDays<1||policy.keepDays>3650))throw Error('Invalid retention policy');
 if(policy.deleteEnabled!==false)throw Error('Log deletion is not approved');
 return policy;
}
export function configuredLog(c,kind,event){
 const policy=readLogPolicy(c);if(!policy)return false;
 appendDaily(join(c.dir,'logs'),kind,event,policy);return true;
}
