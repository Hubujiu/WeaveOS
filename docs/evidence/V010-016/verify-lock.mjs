// Local operator check; no product tests or fault injection on the server.
import {spawn,spawnSync} from 'node:child_process';
const host='43.133.34.48',privateDir='D:/Workspace/WeaveOS-runtime/V010-016/';
const hold=spawn('ssh',['-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=10',host,"flock -n /opt/weaveos-v010/deploy.lock sh -c 'echo LOCKED; sleep 8'"],{stdio:['ignore','pipe','pipe']});
const started=await new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(Error('Lock not acquired')),15000);hold.stdout.once('data',data=>{clearTimeout(timer);resolve(data.toString().includes('LOCKED'));});hold.once('error',reject);hold.once('exit',code=>{if(code)reject(Error('Lock acquisition failed'));});});
if(!started)throw Error('Missing lock evidence');
const result=spawnSync('ssh',['-F','none','-T','-o','BatchMode=yes','-o','IdentitiesOnly=yes','-o','StrictHostKeyChecking=yes','-o','UserKnownHostsFile='+privateDir+'known_hosts','-o','ConnectTimeout=10','-i',privateDir+'deploy_ed25519','root@'+host],{input:Buffer.alloc(0),encoding:'utf8',timeout:15000});
await new Promise(resolve=>{if(hold.exitCode!==null)resolve();else hold.once('exit',resolve);});
if(result.status!==1||result.stdout!==''||result.stderr!=='')throw Error('Concurrent deployment was not rejected by flock');
console.log(JSON.stringify({realServerLock:'passed',concurrentDedicatedKeyExit:result.status,receiverNotStarted:true}));
