// Execute the real restore test's personnel setup verbatim, before any backup/restore.
// This targeted bridge proof is deliberately not a claim of full runtime recovery.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const base=process.env.WEAVEOS_API_URL;
if(!['https://localhost:19446','https://localhost:19447'].includes(base))throw Error('New isolated runtime-context RED/GREEN instance on 19446/19447 required');
const fixture=JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES,'utf8'));
async function api(path,method='GET',body,session){return fetch(base+'/api/v1'+path,{method,signal:AbortSignal.timeout(10000),headers:{Origin:base,'Content-Type':'application/json',...session?.headers},body:body===undefined?undefined:JSON.stringify(body)});}
async function login(credentials){const r=await api('/sessions','POST',credentials);assert.equal(r.status,201);const cookies=r.headers.getSetCookie().map(c=>c.split(';')[0]);const csrf=cookies.find(c=>c.startsWith('__Host-csrf='))?.slice(12);assert.ok(csrf);return {headers:{Cookie:cookies.join('; '),'X-CSRF-Token':csrf}};}
let queries=0,guardedWrites=0;
async function personnelData(path,method,body,session,status=200){const r=await api(path,method,body,session);assert.equal(r.status,status,'restore personnel prelude: '+method+' '+path);const data=(await r.json()).data;if(path==='/personnel/members/search'){assert.equal(typeof data.queryVersion,'string');assert.ok(data.queryVersion.length);assert.equal(body.page,1);assert.equal(body.queryVersion,undefined);queries++;}if(method!=='GET'&&(path==='/personnel/departments'||/\/members\/[^/]+\/(identities|groups)$/.test(path))){assert.ok(body.queryVersion);guardedWrites++;}return data;}
test('actual runtime restore personnel prelude obtains three fresh acting-session contexts',async()=>{
 const file=process.env.WEAVEOS_RUNTIME_PRELUDE_SOURCE??'infra/runtime/operations.test.mjs';
 const source=readFileSync(file,'utf8'),marker="test('encrypted hot/cold backup restores true state";
 const start=source.indexOf(' const invite=await personnelData',source.indexOf(marker)),end=source.indexOf(' const backupDir=',start);
 assert.ok(start>0&&end>start,'original restore setup source bounds');
 const prelude=source.slice(start,end);
 const admin=await login(fixture.admin);
 const execute=new Function('personnelData','admin',`return (async()=>{let personnelSnapshot;${prelude}\nreturn personnelSnapshot;})();`);
 const snapshot=await execute(personnelData,admin);assert.equal(queries,3);assert.equal(guardedWrites,3);
 const member=await personnelData('/personnel/members/'+snapshot.member.id,'GET',undefined,admin);
 assert.equal(member.version,2);assert.deepEqual(member.identityIds,[snapshot.identity.id]);assert.deepEqual(member.departmentIds,[snapshot.department.id]);assert.equal(member.permissions.find(p=>p.code==='personnel.manage').sources[0].templateId,snapshot.template.id);
 const manager=await login({account:'runtime-personnel-restore',password:'Synthetic@123'}),access=await personnelData('/me/access','GET',undefined,manager);
 assert.equal(access.personnelManage,true);assert.equal(access.bootstrapAdmin,false);
 console.log('Three actual guarded writes succeeded after explicit POST refreshes; resulting versions, memberships, grant source and non-Root access verified. No backup, restore, rollback or existing instance was touched.');
});
