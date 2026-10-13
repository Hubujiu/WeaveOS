import {parseRuntimeView,parseRecordItem} from '../recordReadContracts';
import type {HistoricalField} from './historyContract';
export type ComparisonSnapshot={recordVersion:number;updatedAt:string;fields:HistoricalField[]};
export function parseComparisonSnapshot(rawView:unknown,rawRecord:unknown,location:{appId:string;viewId:string;recordId:string},actorId:string):ComparisonSnapshot|null{
 try{
  const view=parseRuntimeView(rawView,location),record=parseRecordItem(rawRecord,view,actorId);
  if(record.id!==location.recordId)return null;
  const fields:HistoricalField[]=view.fields.filter(field=>Object.hasOwn(record.values,field.id)).map(field=>{
   const value=record.values[field.id],valueLabels:HistoricalField['valueLabels']={};
   const selected=Array.isArray(value)?value:typeof value==='string'?[value]:[];
   if(field.kind==='member'||field.kind==='department'){
    for(const id of selected){const label=record.referenceDisplays[field.id]?.[id];if(label)valueLabels[id]={label:label.label,deleted:label.deleted};}
   }else if(field.kind==='single_select'||field.kind==='multi_select'){
    for(const id of selected){const option=field.input.options?.find(option=>option.id===id);if(option)valueLabels[id]={label:option.label,deleted:false};}
   }
   return {fieldId:field.id,fieldName:field.name,fieldKind:field.kind,value:Array.isArray(value)?[...value]:value,valueLabels};
  });
  return {recordVersion:record.recordVersion,updatedAt:record.updatedAt,fields};
 }catch{return null;}
}
