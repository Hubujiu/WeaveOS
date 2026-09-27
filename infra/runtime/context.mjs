import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
export function runtimeContext(){
 const root=resolve('.'),file=process.env.WEAVEOS_RUNTIME_CONTEXT;
 if(!file)throw new Error('Isolated runtime record required');
 const c=JSON.parse(readFileSync(file,'utf8'));
 if(!/^weaveos-v010-008-[0-9]+$/.test(c.project)||!resolve(c.dir).startsWith(resolve(root,'.work')+'/')&&!resolve(c.dir).startsWith(resolve(root,'.work')+'\\'))throw new Error('Refusing non-isolated runtime');
 const env={...process.env,WEAVEOS_RUNTIME_DIR:c.dir,WEAVEOS_BFF_IMAGE:c.artifacts.bff.imageID,WEAVEOS_WEB_IMAGE:c.artifacts.web.imageID};
 const args=['compose','-p',c.project,'-f',resolve('infra/runtime/compose.json')];
 const command=(bin,args,options={})=>execFileSync(bin,args,{env,stdio:'pipe',...options});
 const compose=(...a)=>command('docker',[...args,...a]);
 const container=name=>command('docker',['inspect',compose('ps','-aq',name).toString().trim(),'--format','{{.Name}}']).toString().trim().slice(1);
 const sql=(database,text,user='weaveos_owner')=>command('docker',['exec','-i',container('postgres'),'psql','-X','-At','-v','ON_ERROR_STOP=1','-U',user,'-d',database],{input:text}).toString().trim();
 const cli=(name,input,extra=[])=>command('docker',['run','--rm','-i','--network',`${c.project}_default`,'--env-file',resolve(c.dir,name==='audit-read'?'reader.env':'maintenance.env'),'--entrypoint',`/app/${name}`,c.artifacts.bff.imageID,...extra],{input,timeout:10000});
 return {...c,env,args,command,compose,container,sql,cli};
}
