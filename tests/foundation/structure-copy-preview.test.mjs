import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
async function load(url){let s=stripTypeScriptTypes(await readFile(url,'utf8'),{mode:'transform'});for(const m of [...s.matchAll(/from ['"]([^'"]+)['"]/g)])s=s.replace(m[0],`from '${await load(new URL(m[1]+'.ts',url))}'`);return 'data:text/javascript;base64,'+Buffer.from(s).toString('base64');}
const {validStructureCopyPlan:plan,validStructureCopyPreflight:pre}=await import(await load(new URL('../../apps/web/src/applications/templates/copyPreview.ts',import.meta.url)));
const id=n=>'cbcbcbcb-cbcb-4bcb-8bcb-'+String(n).padStart(12,'0');
const manifest={format:'weaveos.structure-template',version:1,application:{id:id(1),name:'原应用'},directories:[{id:id(2),name:'目录',parentId:null,position:0}],tables:[{id:id(3),name:'表',directoryId:id(2),position:0,fields:[{id:id(4),name:'人员',kind:'member',required:false,default:id(8),config:{},presentation:{helpText:null,displayTimeZone:null}},{id:id(5),name:'部门',kind:'department',required:false,default:id(8),config:{},presentation:{helpText:null,displayTimeZone:null}}]}],forms:[{id:id(6),name:'表单',tableId:id(3),directoryId:id(2),position:0,layout:{version:1,items:[]}}],workflows:[],permissionGroups:[{id:id(7),name:'组',enabled:true,memberIds:[id(9)],grants:[]}]};
const input={source:{appId:id(1),kind:'form',id:id(6)},target:{appId:id(10),directoryId:null,name:'副本'}};
const result={sourceFingerprint:'a'.repeat(64),targetStructureVersion:0,targetPolicyRevision:1,manifest,externalReferences:{userIds:[id(8),id(9)],departmentIds:[id(8)]}};
const counts={directories:1,tables:1,fields:2,forms:1,workflows:0,permissionGroups:1};
test('P22 plan consumes original scoped dependencies and typed references without allocating IDs or mutating input',()=>{
 const before=JSON.stringify([result,input]);assert.equal(plan(result,input),true);assert.equal(JSON.stringify([result,input]),before);
 for(const [kind,n] of [['directory',2],['table',3],['form',6]])assert.equal(plan(result,{...input,source:{...input.source,kind,id:id(n)}}),true);
 assert.equal(plan({...result,targetStructureVersion:0,targetPolicyRevision:0},{source:{appId:id(1),kind:'application',id:id(1)},target:{appId:null,directoryId:null,name:'副本'}}),true);
});
test('P22 plan refuses wrong source, missing roots, false target versions or dependency lists',()=>{
 for(const edit of [{sourceFingerprint:'A'.repeat(64)},{targetStructureVersion:-1},{targetStructureVersion:9007199254740992},{targetPolicyRevision:0},{recordCount:0},{externalReferences:{userIds:[id(8)],departmentIds:[id(8)]}},{externalReferences:{userIds:[id(9),id(8)],departmentIds:[id(8)]}},{externalReferences:{userIds:[id(8),id(9),id(9)],departmentIds:[id(8)]}},{externalReferences:{...result.externalReferences,emails:[]}},{manifest:{...manifest,application:{...manifest.application,id:id(11)}}},{manifest:{...manifest,forms:[]}},{manifest:{...manifest,records:[]}}])assert.equal(plan({...result,...edit},input),false);
 for(const key of Object.keys(result)){const bad={...result};delete bad[key];assert.equal(plan(bad,input),false,key);}
});
test('P22 mapped preflight remains a preview with exact counts and source identity',()=>{
 const mapped=structuredClone(manifest);mapped.tables[0].fields[0].default=id(12);mapped.permissionGroups[0].name='新组';
 const value={valid:true,counts,mappedManifest:mapped},before=JSON.stringify(value);assert.equal(pre(value,input),true);assert.equal(JSON.stringify(value),before);
 for(const key of Object.keys(counts))assert.equal(pre({...value,counts:{...counts,[key]:counts[key]+1}},input),false,key);
 for(const edit of [{valid:false},{appId:id(13)},{counts:{...counts,records:0}},{mappedManifest:{...mapped,application:{id:id(13),name:'新应用'}}},{mappedManifest:{...mapped,tables:null}}])assert.equal(pre({...value,...edit},input),false);
});
test('P22 malformed and oversized copy previews cannot enable confirmation',()=>{
 for(const bad of [null,{},[],{...result,manifest:{...manifest,application:{id:id(1),name:'x'.repeat(1048576)}}}])assert.equal(plan(bad,input),false);
 for(const bad of [null,{},[],{valid:true,counts}, {valid:true,counts,mappedManifest:null}])assert.equal(pre(bad,input),false);
});
