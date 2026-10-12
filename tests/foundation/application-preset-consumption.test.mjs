import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/records/presetContract.ts',import.meta.url),'utf8');
const {validApplicationPreset:valid,validApplicationPresets:list}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
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
