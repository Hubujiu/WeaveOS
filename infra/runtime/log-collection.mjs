import {spawnSync} from 'node:child_process';
import {existsSync,readFileSync,writeFileSync,renameSync} from 'node:fs';
import {join} from 'node:path';
import {appendDaily} from './daily-logs.mjs';

export function collectLogs(c,policy,{now=new Date(),readLogs=spawnSync}={}){
 const cursor=join(c.dir,'log-cursor.json');
 const until=now.toISOString();
 const since=existsSync(cursor)?JSON.parse(readFileSync(cursor,'utf8')).until:new Date(now.getTime()-60000).toISOString();
 if(!Number.isFinite(Date.parse(since))||Date.parse(since)>now.getTime())throw Error('Invalid log collection window');
 let written=0;
 for(const [name,kind] of [['nginx','access'],['bff','application']]){
  const result=readLogs('docker',['logs','--timestamps','--since',since,'--until',until,c.container(name)],{env:c.env??process.env,encoding:'utf8',stdio:'pipe',timeout:10000,maxBuffer:64*1024*1024});
  if(result.status!==0||result.error)throw Error('Docker log collection failed; diagnostics withheld');
  for(const line of `${result.stdout??''}\n${result.stderr??''}`.split('\n')){
   const split=line.indexOf(' ');if(split<0)continue;
   const at=line.slice(0,split),stamp=Date.parse(at);
   if(!Number.isFinite(stamp)||stamp<Date.parse(since)||stamp>=now.getTime())continue;
   let event;try{event=JSON.parse(line.slice(split+1));}catch{continue;}
   if(!event||typeof event!=='object'||Array.isArray(event))continue;
   // Timestamp is from the Docker envelope, not from arbitrary log fields.
   appendDaily(join(c.dir,'logs'),kind,{...event,at},policy);written++;
  }
 }
 // Retry may duplicate a prefix after a crash; never advance past a failed stream.
 writeFileSync(cursor+'.next',JSON.stringify({until})+'\n',{mode:0o600});renameSync(cursor+'.next',cursor);
 return {status:'complete',written};
}
