import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const data=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
async function load(url){let s=stripTypeScriptTypes(await readFile(url,'utf8'),{mode:'transform'});for(const m of [...s.matchAll(/from ['"]([^'"]+)['"]/g)])s=s.replace(m[0],"from '"+await load(new URL(m[1]+'.ts',url))+"'");return data(s);}
const {validWorkflowDefinitionRequest:valid,validWorkflowDefinitionReceipt:receipt}=await import(await load(new URL('../../apps/web/src/applications/workflows/definitionSaveContract.ts',import.meta.url)));
const id=n=>'dcdcdcdc-dcdc-4cdc-8cdc-'+String(n).padStart(12,'0'),resource={kind:'workflow-definition',appId:id(1),viewId:id(2),id:id(3)};
const graph={version:1,nodes:[{id:id(4),kind:'start'},{id:id(5),kind:'end'}],edges:[{from:id(4),to:id(5)}]};
const request={path:`applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}/definition`,method:'PUT',expectedStatus:200,body:{name:'财务审批',expectedRevision:4,expectedSchemaVersion:2,graph,allowWithdraw:true,triggers:[{event:'manual',condition:null}]}};
const result={operationId:id(9),id:id(3),revision:5,currentVersion:1,candidateVersion:2,state:'enabled'};
test('V020 existing definition uses exact PUT and preserves graph and explicit triggers',()=>{const copy=structuredClone(request);assert.equal(valid(resource,request),true);assert.deepEqual(request,copy);});
test('V020 first definition PUT uses client flow identity and 201, not record POST',()=>assert.equal(valid(resource,{...request,expectedStatus:201,body:{...request.body,expectedRevision:0}}),true));
for(const key of ['appId','viewId','id'])for(const bad of ['../escape','00000000-0000-0000-0000-000000000000',id(1).toUpperCase()])test('definition identity '+key+' '+bad,()=>assert.equal(valid({...resource,[key]:bad},request),false));
for(const [label,r,q]of [
 ['record resource',{...resource,kind:'record'},request],['extra resource',{...resource,creationNonce:id(6)},request],
 ['wrong flow',resource,{...request,path:request.path.replace(id(3),id(8))}],['query',resource,{...request,path:request.path+'?version=1'}],['fragment',resource,{...request,path:request.path+'#x'}],
 ['POST',resource,{...request,method:'POST'}],['existing 201',resource,{...request,expectedStatus:201}],['new 200',resource,{...request,body:{...request.body,expectedRevision:0}}],
 ['caller operation',resource,{...request,body:{...request.body,operationId:id(9)}}],['BPMN authority',resource,{...request,body:{...request.body,bpmn:'xml'}}],
 ['blank name',resource,{...request,body:{...request.body,name:' '}}],['unsafe revision',resource,{...request,body:{...request.body,expectedRevision:2**53}}],
 ['bad schema',resource,{...request,body:{...request.body,expectedSchemaVersion:0}}],['fake boolean',resource,{...request,body:{...request.body,allowWithdraw:'true'}}],
 ['hidden trigger omission',resource,{...request,body:{...request.body,triggers:undefined}}],['unsupported trigger',resource,{...request,body:{...request.body,triggers:[{event:'webhook',condition:null}]}}],
 ['invalid DAG',resource,{...request,body:{...request.body,graph:{...graph,edges:[]}}}],
])test('definition rejects '+label,()=>assert.equal(valid(r,q),false));
test('V020 closed minimal save receipt confirms persistence with matching operation and flow only',()=>assert.equal(receipt(result,id(9),id(3),4),true));
for(const patch of [{operationId:id(8)},{id:id(8)},{revision:4},{revision:2**53},{candidateVersion:0},{currentVersion:3},{state:'closing'},{graph},{deploymentId:'private'},{state:'disabled',currentVersion:-1}])test('definition receipt rejects '+JSON.stringify(patch),()=>assert.equal(receipt({...result,...patch},id(9),id(3),4),false));
