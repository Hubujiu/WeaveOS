import type {ReactNode} from 'react';
import type {FieldValue,MutationResult,RecordItem,RuntimeField,RuntimeView,UUID,Values} from './contracts';
import type {RecordEditorIdentity} from './recordState';

export type FieldPort={field:RuntimeField;value:FieldValue|undefined;readOnly:boolean;onChange:(value:FieldValue)=>void;referenceDisplays:RecordItem['referenceDisplays'];instanceKey:string};
export type SaveOutcome={kind:'confirmed';result:unknown}|{kind:'unknown'}|{kind:'failed';message:string};
export type RecordSaveCommand={identity:RecordEditorIdentity;operationId:UUID;schemaVersion:number;viewVersion:number;expectedRecordVersion:number|null;values:Values};
export type RecordFormProps={view:RuntimeView;identity:RecordEditorIdentity;record?:RecordItem;mode:'create'|'edit'|'read';renderField:(port:FieldPort)=>ReactNode;onSave:(command:RecordSaveCommand)=>Promise<SaveOutcome>;onRecover:(operationId:UUID)=>Promise<SaveOutcome>;onConfirmed:(result:MutationResult,identity:RecordEditorIdentity)=>void;onDirtyChange:(dirty:boolean)=>void;onDiscard:()=>void};
export function RecordForm(_props:RecordFormProps){return null;}
