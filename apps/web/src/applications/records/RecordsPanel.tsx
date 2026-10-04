import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';
import type {FieldValue,RecordItem,RecordPage,RecordSort,RuntimeField,RuntimeView,UUID} from './contracts';
import {projectRuntimeFields,scopeAllows} from './runtimeModel';

export type RecordsPanelProps={view:RuntimeView;actorId:UUID;page:RecordPage;loading:boolean;error:string|null;hiddenColumnIds:readonly string[];columnWidths:Record<string,number>;columnOrder:string[];selectedRowIds:string[];onSelectionChange:(ids:string[])=>void;onPageChange:(page:number)=>void;onPageSizeChange:(size:number)=>void;onSortChange:(sort:RecordSort)=>void;onColumnWidthsChange:(widths:Record<string,number>)=>void;onColumnOrderChange:(order:string[])=>void;onOpenRecord:(record:RecordItem)=>void};

function displayValue(row:RecordItem,field:RuntimeField,value:FieldValue|undefined):string{
 if(value===undefined||value===null)return '—';
 if(field.kind==='member'||field.kind==='department'){
  const ids=Array.isArray(value)?value:typeof value==='string'?[value]:[];
  return ids.map(id=>{
   const display=row.referenceDisplays[field.id]?.[id];
   return display?(display.deleted?`${display.label}（已删除）`:display.label):'引用信息不可用';
  }).join('、')||'—';
 }
 if(Array.isArray(value)){
  const labels=new Map(field.input.options?.map(option=>[option.id,option.label]));
  return value.map(id=>labels.get(id)??'选项不可用').join('、')||'—';
 }
 if(typeof value==='boolean')return value?'是':'否';
 if(field.kind==='single_select')return field.input.options?.find(option=>option.id===value)?.label??'选项不可用';
 // Decimal values remain strings. Numeric and time ordering belongs to the server.
 return value;
}

export function RecordsPanel({view,actorId,page,loading,error,hiddenColumnIds,columnWidths,columnOrder,selectedRowIds,onSelectionChange,onPageChange,onPageSizeChange,onSortChange,onColumnWidthsChange,onColumnOrderChange,onOpenRecord}:RecordsPanelProps){
 const visibleRows=page.items.filter(row=>row.appId===view.appId&&row.tableId===view.tableId&&row.viewId===view.viewId&&scopeAllows(view.capabilities.read,actorId,row.createdBy));
 const columns:TableColumn<RecordItem>[]=view.fields.map(field=>({
  key:field.id,header:field.name,
  dataType:field.kind==='number'||field.kind==='money'?'number':field.kind==='date'||field.kind==='datetime'?'time':'text',
  sortable:field.query.sortable,
  cell:row=>{
   const projected=projectRuntimeFields(view,'read',actorId,row).find(item=>item.field.id===field.id);
   if(!projected?.readable)return <span aria-label="不可读取">—</span>;
   const text=displayValue(row,field,projected.value);
   return <button className="text-button table-cell" type="button" onClick={()=>onOpenRecord(row)} aria-label={`打开记录：${field.name} ${text}`}>{text}</button>;
  }
 }));
 const sort=page.sort?{key:page.sort.fieldId,direction:page.sort.direction}:null;
 return <section className="member-table surface" aria-label="记录列表">
  {error?<p role="alert">{error}</p>:null}
  <div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="记录" data={visibleRows} columns={columns} hiddenColumnIds={hiddenColumnIds} getRowId={row=>row.id} rowHeight={40} fillViewport paginated manualPagination manualSorting recordCount={page.total} page={page.page} pageSize={page.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={onPageChange} onPageSizeChange={onPageSizeChange} sort={sort} onSortChange={next=>onSortChange(next?{fieldId:next.key,direction:next.direction}:null)} resizable columnWidths={columnWidths} onColumnWidthsChange={onColumnWidthsChange} reorderable columnOrder={columnOrder} onColumnOrderChange={onColumnOrderChange} selectable selectedRowIds={selectedRowIds} onSelectionChange={onSelectionChange} selectAllLabel="选择当前页记录" getRowLabel={row=>`选择记录：${row.id}`} loading={loading} emptyState="暂无记录"/></div>
 </section>;
}
