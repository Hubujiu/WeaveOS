import type {RuntimeView} from './contracts';
import type {ResourceFilterField} from '../../QueryFilterState';
import type {BusinessColumn} from '../../TablePresetState';
const systemColumns:BusinessColumn[]=[{id:'id',name:'记录 ID',system:true},{id:'createdBy',name:'创建人',system:true},{id:'createdAt',name:'创建时间',system:true},{id:'updatedAt',name:'更新时间',system:true},{id:'recordVersion',name:'记录版本',system:true}];
export function presetDescriptors(view:RuntimeView):{fields:ResourceFilterField[];columns:BusinessColumn[]}{
 const readable=view.fields.filter(field=>field.access.read!=='none');
 return {
  columns:[...readable.map(field=>({id:field.id,name:field.name})),...systemColumns.map(column=>({...column}))],
  fields:[...readable.filter(field=>field.query.operators.length>0).map(field=>({id:field.id,label:field.name,kind:field.kind,operators:[...field.query.operators],optionIds:field.input.options?.map(option=>option.id),choices:field.input.options?.map(option=>({value:option.id,label:option.label}))})),
   ...(['createdAt','updatedAt'] as const).map(id=>({id,label:id==='createdAt'?'创建时间':'更新时间',kind:'datetime' as const,operators:['eq','neq','gt','gte','lt','lte'] as const}))],
 };
}
