import {appendFileSync} from 'node:fs';
// Local simulation thresholds are adjustable operator warnings, not an SLA.
export function alarms(s){
 const codes=[];
 for(const [key,code] of [['ready','READINESS'],['live','LIVENESS'],['database','DATABASE'],['redis','REDIS']])if(s[key]!==true)codes.push(code);
 if(!(s.redisLimitBytes>0)||s.redisUsedBytes/s.redisLimitBytes>=0.8)codes.push('REDIS_CAPACITY');
 if(!Array.isArray(s.containerMemoryRatios)||s.containerMemoryRatios.some(r=>!Number.isFinite(r)||r>=0.8))codes.push('MEMORY');
 if(!Number.isFinite(s.diskFreeRatio)||s.diskFreeRatio<0.1)codes.push('DISK');
 if(!Number.isFinite(s.certificateRemainingMs)||s.certificateRemainingMs<3*86400000)codes.push('CERTIFICATE');
 if(s.backupFailed!==false)codes.push('BACKUP');
 return codes;
}
export function receiveAlarms(file,sample){
 const codes=alarms(sample);
 return receiveCodes(file,codes);
}
export function receiveCodes(file,codes){
 const allowed=new Set(['READINESS','LIVENESS','DATABASE','REDIS','REDIS_CAPACITY','MEMORY','DISK','CERTIFICATE','BACKUP']);
 if(!Array.isArray(codes)||codes.some(c=>!allowed.has(c)))throw new Error('Only fixed operational codes may reach the receiver');
 if(codes.length)appendFileSync(file,JSON.stringify({at:new Date().toISOString(),receiver:'local operator review',codes})+'\n',{mode:0o600});
 return codes;
}
