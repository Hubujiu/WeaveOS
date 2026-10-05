// Administrator-installed module and role policy, never release-uploaded code.
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const roleHash='5727b104f0f18dd9f9a1de400dd8b8bb9fe4fdd866ee2dc3351de146ef1666ab';
export function validateInstalledPersonnelRoles(sql){
 if(typeof sql!=='string'||createHash('sha256').update(sql.replace(/\r\n/g,'\n')).digest('hex')!==roleHash)throw Error('Installed personnel role policy differs from reviewed source');
}
export function applyPersonnelRoles({container,env,roleSQL}){
 validateInstalledPersonnelRoles(roleSQL);
 if(!/^weaveos-v010-[a-zA-Z0-9_-]+$/.test(container))throw Error('Unexpected database container');
 const values=Object.fromEntries(env.split(/\r?\n/).filter(Boolean).map(line=>{const i=line.indexOf('=');return [line.slice(0,i),line.slice(i+1)];}));
 let url;try{url=new URL(values.GOOSE_DBSTRING);}catch{throw Error('Role owner credentials invalid');}
 if(values.GOOSE_DRIVER!=='postgres'||url.protocol!=='postgres:'||!['127.0.0.1','localhost','[::1]'].includes(url.hostname)||decodeURIComponent(url.username)!=='weaveos_owner'||!/^\/weaveos_[a-z0-9_]+$/.test(url.pathname))throw Error('Role owner target invalid');
 const credentials={PGHOST:url.hostname.replace(/^\[|\]$/g,''),PGPORT:url.port||'5432',PGUSER:decodeURIComponent(url.username),PGPASSWORD:decodeURIComponent(url.password),PGDATABASE:url.pathname.slice(1)};
 try{execFileSync('docker',['exec','-i',...Object.keys(credentials).flatMap(name=>['-e',name]),container,'psql','-X','-1','-v','ON_ERROR_STOP=1'],{env:{...process.env,...credentials},input:roleSQL.replace(/\r\n/g,'\n'),stdio:'pipe',timeout:120000});}catch{throw Error('Pinned personnel role upgrade failed');}
}
