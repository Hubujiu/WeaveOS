import {useEffect,useRef,useState,type ReactNode} from 'react';
import type {FieldValue,MutationResult,RecordItem,RuntimeField,RuntimeView,UUID,Values} from './contracts';
import {isMutationResult} from './contracts';
import {confirmRecordIdentity,fieldRendererKey,type RecordEditorIdentity} from './recordState';
import {projectRuntimeFields,requiresRuntimeReview} from './runtimeModel';

export type FieldPort={field:RuntimeField;value:FieldValue|undefined;readOnly:boolean;onChange:(value:FieldValue)=>void;referenceDisplays:RecordItem['referenceDisplays'];instanceKey:string};
export type SaveOutcome={kind:'confirmed';result:unknown}|{kind:'unknown'}|{kind:'failed';message:string};
export type RecordSaveCommand={identity:RecordEditorIdentity;operationId:UUID;schemaVersion:number;viewVersion:number;expectedRecordVersion:number|null;values:Values};
export type RecordFormProps={view:RuntimeView;identity:RecordEditorIdentity;record?:RecordItem;mode:'create'|'edit'|'read';renderField:(port:FieldPort)=>ReactNode;onSave:(command:RecordSaveCommand)=>Promise<SaveOutcome>;onRecover:(operationId:UUID)=>Promise<SaveOutcome>;onConfirmed:(result:MutationResult,identity:RecordEditorIdentity)=>void;onDirtyChange:(dirty:boolean)=>void;onDiscard:()=>void};
function initialValues(view:RuntimeView,actorId:UUID,mode:RecordFormProps['mode'],record?:RecordItem):Values{
 return Object.fromEntries(projectRuntimeFields(view,mode,actorId,record).filter(port=>port.editable&&Object.hasOwn(port,'value')).map(port=>[port.field.id,port.value])) as Values;
}
export function RecordForm({view,identity,record,mode,renderField,onSave,onRecover,onConfirmed,onDirtyChange,onDiscard}:RecordFormProps){
 const startView=useRef(view),startValues=useRef(initialValues(view,identity.actorId,mode,record));
 const [values,setValues]=useState<Values>(startValues.current),[pending,setPending]=useState(false),[unknownOperation,setUnknownOperation]=useState<UUID|null>(null),[error,setError]=useState(''),[confirmed,setConfirmed]=useState(false);
 const dirty=JSON.stringify(values)!==JSON.stringify(startValues.current);
 const needsReview=requiresRuntimeReview(startView.current,view,dirty);
 useEffect(()=>{onDirtyChange(dirty&&!confirmed);},[dirty,confirmed,onDirtyChange]);
 const fields=projectRuntimeFields(startView.current,mode,identity.actorId,record);
 async function accept(outcome:SaveOutcome,operationId:UUID){
  if(outcome.kind==='unknown'){setUnknownOperation(operationId);setError('');return;}
  if(outcome.kind==='failed'){setUnknownOperation(null);setError(outcome.message);return;}
  const expected=identity.kind==='record'?identity.recordId:record?.id;
  if(!isMutationResult(outcome.result,operationId,expected)){setUnknownOperation(operationId);setError('确认回执不完整，请恢复原操作');return;}
  const next=identity.kind==='new'?confirmRecordIdentity(identity,outcome.result,operationId):identity;
  if(!next){setUnknownOperation(operationId);setError('记录身份未确认，请恢复原操作');return;}
  setUnknownOperation(null);setError('');setConfirmed(true);startValues.current=values;
  onConfirmed(outcome.result,next);
 }
 async function save(){
  if(pending||unknownOperation||confirmed||needsReview||mode==='read'||!dirty&&mode==='edit')return;
  const operationId=crypto.randomUUID();setPending(true);setError('');
  try{
   const allowed=new Set(fields.filter(port=>port.editable).map(port=>port.field.id));
   const changes:Values=Object.fromEntries(Object.entries(values).filter(([id,value])=>allowed.has(id)&&(mode==='create'||JSON.stringify(startValues.current[id])!==JSON.stringify(value))));
   const command:RecordSaveCommand={identity,operationId,schemaVersion:startView.current.schemaVersion,viewVersion:startView.current.viewVersion,expectedRecordVersion:record?.recordVersion??null,values:changes};
   await accept(await onSave(command),operationId);
  }catch{setUnknownOperation(operationId);setError('保存结果待确认，请恢复原操作');}
  finally{setPending(false);}
 }
 async function recover(){
  if(!unknownOperation||pending)return;const operationId=unknownOperation;setPending(true);
  try{await accept(await onRecover(operationId),operationId);}
  catch{setError('恢复暂不可用，请稍后继续恢复原操作');}
  finally{setPending(false);}
 }
 return <section aria-label="记录填写" className="surface record-form">
  {needsReview?<p role="alert">结构或权限已变化，请核对当前输入</p>:null}
  {error?<p role="alert">{error}</p>:null}
  {unknownOperation?<p role="status">保存结果待确认</p>:null}
  <div className="record-form-fields">{fields.map(port=><div key={fieldRendererKey(identity,port.field.id)}>{renderField({field:port.field,value:values[port.field.id],readOnly:mode==='read'||!port.editable||needsReview||!!unknownOperation||pending||confirmed,onChange:value=>setValues(old=>({...old,[port.field.id]:value})),referenceDisplays:record?.referenceDisplays??{},instanceKey:fieldRendererKey(identity,port.field.id)})}</div>)}</div>
  <footer><button type="button" onClick={onDiscard} disabled={pending||!!unknownOperation}>放弃填写</button>{unknownOperation?<button type="button" onClick={()=>void recover()} disabled={pending}>恢复保存结果</button>:null}<button type="button" onClick={()=>void save()} disabled={pending||!!unknownOperation||confirmed||needsReview||mode==='read'||mode==='edit'&&!dirty}>保存记录</button></footer>
 </section>;
}
