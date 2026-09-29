import {execFileSync} from 'node:child_process';
export function serverContext(directory='/opt/weaveos-v010',execute) {
 if(directory!=='/opt/weaveos-v010')throw new Error('Refusing unauthorized deployment directory');
 const command=execute??((bin,args,options={})=>execFileSync(bin,args,{cwd:directory,stdio:'pipe',timeout:30000,...options}));
 const args=['compose','--env-file',directory+'/.env','-p','weaveos-v010-011','-f',directory+'/compose.json'];
 const compose=(...a)=>command('docker',[...args,...a]);
 const container=name=>command('docker',['inspect',compose('ps','-aq',name).toString().trim(),'--format','{{.Name}}']).toString().trim().replace(/^\//,'');
 const sql=(database,input,user='weaveos_owner')=>command('docker',['exec','-i',container('postgres'),'psql','-X','-At','-v','ON_ERROR_STOP=1','-U',user,'-d',database],{input}).toString().trim();
 return {dir:directory,args,command,compose,container,sql};
}
