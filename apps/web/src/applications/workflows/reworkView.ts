import type {RuntimeView,RecordItem} from '../records/contracts';
import {scopeAllows} from '../records/runtimeModel';
import {validRoundPreview,type RoundPreview} from './roundContract';
export function reworkRuntimeView(view:RuntimeView,record:RecordItem,preview:RoundPreview,actorId:string):RuntimeView|null{
 if(!validRoundPreview(preview,preview.instanceId)||!preview.canRework||preview.latestInstanceId!==preview.instanceId||record.appId!==view.appId||record.viewId!==view.viewId||record.tableId!==view.tableId||view.schemaVersion!==preview.schemaVersion||record.schemaVersion!==preview.schemaVersion||record.recordVersion!==preview.recordVersion||!scopeAllows(view.capabilities.read,actorId,record.createdBy)||!scopeAllows(view.capabilities.edit,actorId,record.createdBy))return null;
 const mask=new Set(preview.editableFieldIds);
 if([...mask].some(id=>!view.fields.some(field=>field.id===id)))return null;
 const fields=view.fields.map(field=>({...field,access:{...field.access,edit:mask.has(field.id)&&scopeAllows(field.access.edit,actorId,record.createdBy)?field.access.edit:'none' as const}}));
 return fields.some(field=>field.access.edit!=='none')?{...view,fields}:null;
}
