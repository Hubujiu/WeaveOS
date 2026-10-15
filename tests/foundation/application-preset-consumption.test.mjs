import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/records/presetContract.ts',import.meta.url),'utf8');
const {validApplicationPreset:valid,validApplicationPresets:list,validPresetMutationRequest:request,validPresetMutationReceipt:receipt}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
// Independent V071 ADR closed DTO, scope and limits. Not schema/authorization proof.
const id=n=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const scope={appId:id(1),viewId:id(2)};
const leaf={fieldId:id(4),operator:'eq',value:'9007199254740993.01'};
const and={operator:'and',children:[leaf]};
const item={id:id(3),...scope,name:'我的方案',version:1,invalid:false,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-11T00:00:00Z',filter:null,sort:null,hiddenColumnIds:[],columnOrder:[],columnWidths:{}};
const invalid={id:id(3),name:'失效方案',version:2,invalid:true,reason:'PERMISSION_CHANGED'};
for(const [name,value]of [
 ['empty configuration',item],['AND',{...item,filter:and}],['OR of AND',{...item,filter:{operator:'or',children:[and,{operator:'and',children:[{...leaf,value:null}]}]}}],
 ['boolean',{...item,filter:{operator:'and',children:[{...leaf,value:false}]}}],['set equality',{...item,filter:{operator:'and',children:[{...leaf,value:[id(9),id(8)]}]}}],
 ['layout and numeric sorting',{...item,sort:{fieldId:id(4),direction:'desc'},hiddenColumnIds:[id(5),'id'],columnOrder:['createdAt',id(4)],columnWidths:{[id(4)]:9007199254740991,createdAt:1}}],
 ['20 leaves',{...item,filter:{operator:'and',children:Array.from({length:20},(_,i)=>({...leaf,fieldId:id(100+i)}))}}],
 ['100 Unicode codepoints',{...item,name:'😀'.repeat(100)}],
 ...['FIELD_UNAVAILABLE','PERMISSION_CHANGED','DEFINITION_CHANGED'].map(reason=>[reason,{...invalid,reason}]),
])test('preset wire accepts '+name,()=>assert.equal(valid(value,scope,id(3)),true));
for(const [name,change]of [
 ['extra owner',{ownerId:id(9)}],['other app',{appId:id(9)}],['other view',{viewId:id(9)}],['zero version',{version:0}],['unsafe version',{version:9007199254740992}],
 ['empty name',{name:''}],['NUL name',{name:'a\0b'}],['unpaired surrogate',{name:'\ud800'}],['long name',{name:'😀'.repeat(101)}],
 ['bad timestamp',{createdAt:'2026-02-30T00:00:00Z'}],['zero id',{id:'00000000-0000-0000-0000-000000000000'}],
 ['missing settings',{columnWidths:undefined}],['negative width',{columnWidths:{[id(4)]:-1}}],['fractional width',{columnWidths:{[id(4)]:1.5}}],['unsafe width',{columnWidths:{[id(4)]:9007199254740992}}],
 ['helper column',{hiddenColumnIds:['selection']}],['unknown system column',{columnOrder:['updatedBy']}],['duplicate hidden',{hiddenColumnIds:[id(4),id(4)]}],['duplicate order',{columnOrder:[id(4),id(4)]}],
 ['oversize columns',{hiddenColumnIds:Array.from({length:206},(_,i)=>id(100+i))}],['extra sort field',{sort:{fieldId:id(4),direction:'asc',owner:id(9)}}],['unknown sort',{sort:{fieldId:'id',direction:'asc'}}],
 ['empty filter',{filter:{operator:'and',children:[]}}],['OR leaves',{filter:{operator:'or',children:[leaf]}}],['nested AND',{filter:{operator:'and',children:[and]}}],
 ['extra leaf field',{filter:{operator:'and',children:[{...leaf,sql:'private'}]}}],['unknown operator',{filter:{operator:'and',children:[{...leaf,operator:'contains'}]}}],
 ['missing value',{filter:{operator:'and',children:[{fieldId:id(4),operator:'eq'}]}}],['numeric coercion',{filter:{operator:'and',children:[{...leaf,value:9007199254740992}]}}],
 ['oversize leaves',{filter:{operator:'and',children:Array.from({length:21},()=>leaf)}}],['filter byte limit',{filter:{operator:'and',children:[{...leaf,value:'中'.repeat(6000)}]}}],
])test('preset wire rejects '+name,()=>assert.equal(valid({...item,...change},scope,id(3)),false));
for(const extra of [{filter:and},{createdAt:item.createdAt},{columnOrder:[id(4)]},{columnWidths:{[id(4)]:50}},{reason:'SQL_INTERNAL'},{appId:id(1)}])test('invalid preset rejects leaked or unknown payload '+JSON.stringify(extra),()=>assert.equal(valid({...invalid,...extra},scope),false));
test('preset detail binds the selected id',()=>assert.equal(valid(item,scope,id(99)),false));
test('preset list accepts empty and mixed current configurations',()=>{assert.equal(list({items:[]},scope),true);assert.equal(list({items:[item,{...invalid,id:id(9)}]},scope),true);});
for(const value of [{items:[item,item]},{items:Array.from({length:21},(_,i)=>({...item,id:id(100+i)}))},{items:[],total:0},{items:[{...item,viewId:id(9)}]},{items:[{...invalid,filter:and}]}])test('preset list fails closed for bad scope limits or payload',()=>assert.equal(list(value,scope),false));
for(const [name,value]of [
 ['literal reason',{...invalid,reason:['PERMISSION_CHANGED']}],
 ['literal sort direction',{...item,sort:{fieldId:id(4),direction:['asc']}}],
 ['literal comparison operator',{...item,filter:{operator:'and',children:[{...leaf,operator:['eq']}]}}],
 ['hour 24',{...item,createdAt:'2026-10-10T24:00:00Z'}],
 ['config byte limit',{...item,filter:{operator:'and',children:[{...leaf,value:'x'.repeat(15000)}]},hiddenColumnIds:Array.from({length:200},(_,i)=>id(100+i)),columnOrder:Array.from({length:200},(_,i)=>id(100+i)),columnWidths:Object.fromEntries(Array.from({length:200},(_,i)=>[id(100+i),999999999999999]))}],
])test('preset rejects '+name,()=>assert.equal(valid(value,scope),false));

const state={name:'方案',filter:and,sort:null,hiddenColumnIds:[],columnOrder:[],columnWidths:{}};
const resource={kind:'preset',...scope,id:id(3)};
const creating={kind:'preset',...scope,creationNonce:id(10)};
const path=`applications/${id(1)}/forms/${id(2)}/table-presets`;
const create={path,method:'POST',expectedStatus:201,body:{...state,expectedSchemaVersion:2}};
const update={path:path+'/'+id(3),method:'PUT',expectedStatus:200,body:{...state,expectedSchemaVersion:2,expectedVersion:3}};
const remove={path:path+'/'+id(3),method:'DELETE',expectedStatus:204,expectedVersion:3};
for(const [name,res,req]of [['create',creating,create],['update',resource,update],['remove',resource,remove]])test('private preset request accepts '+name,()=>assert.equal(request(res,req),true));
for(const [name,res,req]of [
 ['other resource',{...resource,kind:'record'},update],['other id',resource,{...update,path:path+'/'+id(8)}],['other view',resource,{...update,path:update.path.replace(id(2),id(9))}],
 ['query injection',resource,{...update,path:update.path+'?owner='+id(9)}],['fragment',resource,{...update,path:update.path+'#fragment'}],
 ['dual identity',{...creating,id:id(3)},create],['no creation nonce',{kind:'preset',...scope},create],['create on existing',resource,create],
 ['wrong create status',creating,{...create,expectedStatus:200}],['wrong update method',resource,{...update,method:'PATCH'}],['wrong update status',resource,{...update,expectedStatus:201}],
 ['missing CAS',resource,{...update,body:{...state,expectedSchemaVersion:2}}],['unsafe CAS',resource,{...update,body:{...update.body,expectedVersion:9007199254740992}}],
 ['caller operation',resource,{...update,body:{...update.body,operationId:id(12)}}],['caller owner',resource,{...update,body:{...update.body,ownerUserId:id(1)}}],
 ['invalid config',resource,{...update,body:{...update.body,columnWidths:{[id(4)]:0}}}],['no schema',creating,{...create,body:state}],
 ['delete missing CAS',resource,{...remove,expectedVersion:undefined}],['delete body',resource,{...remove,body:{expectedVersion:3}}],['wrong delete status',resource,{...remove,expectedStatus:200}],
])test('private preset request rejects '+name,()=>assert.equal(request(res,req),false));
const result={operationId:id(20),id:id(3),version:4};
test('private preset minimal result binds operation and selected id',()=>{assert.equal(receipt(result,id(20),id(3)),true);assert.equal(receipt(result,id(20)),true);});
for(const [name,value,op,preset]of [
 ['wrong operation',result,id(21),id(3)],['wrong resource',result,id(20),id(9)],['zero version',{...result,version:0},id(20),id(3)],
 ['unsafe version',{...result,version:9007199254740992},id(20),id(3)],['private config',{...result,filter:and},id(20),id(3)],['record result',{operationId:id(20),id:id(3),recordVersion:4},id(20),id(3)],
])test('private preset minimal result rejects '+name,()=>assert.equal(receipt(value,op,preset),false));
