import {test,expect} from '@playwright/test';
import {isDraftMutationResult,isMutationResult,type HistoryChange,type RecordItem,type RuntimeField,type RuntimeView} from './contracts';
import {allowedHistoryValueLabels,canOfferHistory,projectRuntimeFields,requiresRuntimeReview} from './runtimeModel';

const actor='11111111-1111-4111-8111-111111111111',other='22222222-2222-4222-8222-222222222222',app='33333333-3333-4333-8333-333333333333',viewId='44444444-4444-4444-8444-444444444444',fieldId='55555555-5555-4555-8555-555555555555',operation='66666666-6666-4666-8666-666666666666',recordId='77777777-7777-4777-8777-777777777777';
const field=(patch:Partial<RuntimeField>={}):RuntimeField=>({id:fieldId,name:'标题',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'own',create:true,edit:'own',history:'own'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true},...patch});
const runtime=(fields:RuntimeField[]):RuntimeView=>({appId:app,tableId:app,viewId,schemaVersion:2,viewVersion:3,policyRevision:4,fields,layout:[],capabilities:{create:true,read:'own',edit:'own',history:'own',search:true,draftCreate:true,draftEdit:true}});
const record:RecordItem={id:recordId,appId:app,tableId:app,viewId,createdBy:actor,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z',recordVersion:1,schemaVersion:2,values:{[fieldId]:''},referenceDisplays:{}};

test('confirmed record and draft receipts accept exact minimum only',()=>{
 const full={operationId:operation,id:recordId,recordVersion:1,schemaVersion:2,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'};
 expect(isMutationResult(full,operation,recordId)).toBe(true);
 expect(isMutationResult({...full,values:{[fieldId]:'secret'}},operation,recordId)).toBe(false);
 expect(isMutationResult(full,other,recordId)).toBe(false);
 const draft={operationId:operation,id:recordId,draftVersion:0};
 expect(isDraftMutationResult(draft,operation,recordId)).toBe(true);
 expect(isDraftMutationResult({...draft,values:{}},operation,recordId)).toBe(false);
 expect(isDraftMutationResult({...draft,draftVersion:Number.MAX_SAFE_INTEGER+1},operation,recordId)).toBe(false);
});

test('safe runtime projection keeps null, empty and false but never fills denied values or defaults',()=>{
 const restricted=field({id:other,name:'隐藏',default:'owner-only',access:{read:'none',create:false,edit:'none',history:'none'}});
 const allowed=field({default:'新建默认'});
 const create=projectRuntimeFields(runtime([allowed,restricted]),'create',actor);
 expect(create.map(f=>f.field.id)).toEqual([fieldId]);
 expect(create[0].value).toBe('新建默认');
 const edit=projectRuntimeFields(runtime([allowed,restricted]),'edit',actor,record);
 expect(edit.map(f=>f.field.id)).toEqual([fieldId]);
 expect(edit[0].value).toBe('');
 expect(projectRuntimeFields(runtime([allowed]),'edit',other,record)).toEqual([]);
 const booleanField=field({kind:'boolean',default:false});
 expect(projectRuntimeFields(runtime([booleanField]),'create',actor)[0].value).toBe(false);
});

test('history hint intersects current read and history; schema change flags dirty review',()=>{
 const allowed=field();
 expect(canOfferHistory(runtime([allowed]),allowed,actor,record)).toBe(true);
 expect(canOfferHistory(runtime([allowed]),allowed,other,record)).toBe(false);
 expect(canOfferHistory(runtime([field({access:{read:'own',create:true,edit:'own',history:'none'}})]),field({access:{read:'own',create:true,edit:'own',history:'none'}}),actor,record)).toBe(false);
 expect(requiresRuntimeReview(runtime([allowed]),{...runtime([allowed]),schemaVersion:3},true)).toBe(true);
 expect(requiresRuntimeReview(runtime([allowed]),{...runtime([allowed]),schemaVersion:3},false)).toBe(false);
});

test('history labels only use IDs in allowed delta and keep unavailable explicit',()=>{
 const id='88888888-8888-4888-8888-888888888888',unused='99999999-9999-4999-8999-999999999999';
 const change:HistoryChange={fieldId,fieldKind:'member',before:id,after:null,fieldLabel:'申请人',fieldDeleted:false,valueLabels:{[id]:{label:null,deleted:true,labelUnavailable:true},[unused]:{label:'private',deleted:false,labelUnavailable:false}}};
 expect(allowedHistoryValueLabels(change)).toEqual({[id]:{label:null,deleted:true,labelUnavailable:true}});
});
