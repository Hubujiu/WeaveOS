// R2 AC01–08/13–16 + Q25: actual same-origin HTTPS, Cookie, PostgreSQL and Redis.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {randomUUID} from 'node:crypto';
import {assertResponseSchema} from './response-schema.mjs';
const base=process.env.WEAVEOS_API_URL??'https://localhost:19443';
const fixtures=()=>JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES,'utf8'));
async function send(path,method='GET',body,session){
 const response=await fetch(new URL('/api/v1/'+path,base),{method,signal:AbortSignal.timeout(10000),headers:{Origin:base,...(body===undefined?{}:{'Content-Type':'application/json'}),...(session?{Cookie:session.cookie,'X-CSRF-Token':session.csrf}:{})},...(body===undefined?{}:{body:JSON.stringify(body)})});
 await assertResponseSchema(response,'/api/v1/'+path,method);return response;
}
async function login(credentials){const response=await send('sessions','POST',credentials);assert.equal(response.status,201);const cookies=response.headers.getSetCookie().map(v=>v.split(';')[0]);const csrf=cookies.find(v=>v.startsWith('__Host-csrf='))?.slice('__Host-csrf='.length);assert.ok(csrf);return {cookie:cookies.join('; '),csrf};}
async function data(path,method,body,session,status=200){const response=await send(path,method,body,session);assert.equal(response.status,status,'declared personnel status');if(status===204)return null;return (await response.json()).data;}
test('R2/Q25 ordinary member reads own access and all management read/write families deny',async()=>{
 const f=fixtures(),s=await login(f.user);const own=await data('me/access','GET',undefined,s);assert.equal(own.personnelManage,false);assert.equal(own.bootstrapAdmin,false);
 for(const path of ['personnel/members','personnel/members/'+f.userId,'personnel/departments','personnel/identities','personnel/templates','personnel/permissions','personnel/events'])assert.equal((await send(path,'GET',undefined,s)).status,403);
 assert.equal((await send('personnel/identities','POST',{name:'forbidden',description:'',templateIds:[],permissionCodes:[]},s)).status,403);
});
test('R2/Q25 non-Root template manager, live revoke, versions, invitation and department invariance',async()=>{
 const f=fixtures(),root=await login(f.admin),label='p-'+randomUUID();
 const invitation=await data('invitations','POST',{},root,201);const credentials={account:label,password:'Synthetic@123'};
 const registered=await data('registrations','POST',{...credentials,invitationCode:invitation.invitationCode},undefined,201);const ordinary=await login(credentials);
 const t=await data('personnel/templates','POST',{name:label,description:'',permissionCodes:['personnel.manage']},root,201);
 const i=await data('personnel/identities','POST',{name:label,description:'',permissionCodes:[],templateIds:[t.id]},root,201);
 let member=await data('personnel/members/'+registered.id,'GET',undefined,root);assert.equal(member.version,0);
 member=await data('personnel/members/'+member.id+'/identities','PUT',{identityIds:[i.id],version:0},root);assert.equal(member.version,1);
 const own=await data('me/access','GET',undefined,ordinary);assert.equal(own.personnelManage,true);assert.equal(own.bootstrapAdmin,false);assert.equal(own.permissions.find(p=>p.code==='personnel.manage').sources[0].templateId,t.id);
 await data('personnel/members','GET',undefined,ordinary);
 const invited=await data('invitations','POST',{},ordinary,201);assert.equal(typeof invited.invitationCode,'string');assert.equal((await send('users/'+registered.id+'/password-reset','POST',{},ordinary)).status,403);
 const ds=await data('personnel/departments','GET',undefined,root),enterprise=ds.items.find(d=>d.isRoot);assert.ok(enterprise);assert.equal((await send('personnel/departments/'+enterprise.id+'?version='+enterprise.version,'DELETE',undefined,root)).status,409);
 const d1=await data('personnel/departments','POST',{name:label+'-1',parentId:enterprise.id},ordinary,201),d2=await data('personnel/departments','POST',{name:label+'-2',parentId:enterprise.id},ordinary,201),d3=await data('personnel/departments','POST',{name:label+'-3',parentId:enterprise.id},ordinary,201);
 for(const d of [d1,d3])member=await data('personnel/members/'+member.id+'/groups','POST',{operation:'add',departmentId:d.id,version:member.version},root);
 const before=member;member=await data('personnel/members/'+member.id+'/groups','POST',{operation:'move',departmentId:d2.id,sourceDepartmentId:d1.id,version:member.version},root);
 assert.deepEqual([...member.departmentIds].sort(),[d2.id,d3.id].sort());assert.deepEqual(member.identityIds,before.identityIds);assert.deepEqual(member.permissions,before.permissions);
 assert.equal((await send('personnel/members/'+member.id+'/identities','PUT',{identityIds:[],version:0},root)).status,409);
 const changed=await data('personnel/templates/'+t.id,'PUT',{name:label,description:'',permissionCodes:[],version:1},root);assert.equal(changed.version,2);
 assert.equal((await send('personnel/templates/'+t.id,'PUT',{name:label,description:'',permissionCodes:[],version:1},root)).status,409);
 assert.equal((await data('me/access','GET',undefined,ordinary)).personnelManage,false);assert.equal((await send('personnel/events','GET',undefined,ordinary)).status,403);assert.equal((await send('invitations','POST',{},ordinary)).status,403);
 const events=await data('personnel/events?search='+encodeURIComponent(label),'GET',undefined,root);assert.ok(events.items.some(e=>e.action==='TEMPLATE_UPDATED'));assert.ok(!JSON.stringify(events).includes(invited.invitationCode));
 member=await data('personnel/members/'+member.id+'/identities','PUT',{identityIds:[],version:member.version},root);
 for(const d of [d2,d3])member=await data('personnel/members/'+member.id+'/groups','POST',{operation:'remove',departmentId:d.id,version:member.version},root);
 await data('personnel/identities/'+i.id+'?version=1','DELETE',undefined,root,204);await data('personnel/templates/'+t.id+'?version=2','DELETE',undefined,root,204);
 for(const d of [d1,d2,d3])await data('personnel/departments/'+d.id+'?version=1','DELETE',undefined,root,204);
});
