import {useState} from 'react';
import {Modal} from '../../Modal';
import type {RuntimeView} from './contracts';
import {presetDescriptors} from './presetDescriptors';

type Props={view:RuntimeView;hiddenColumnIds:string[];onApply:(ids:string[])=>void};
export function RecordColumnVisibility({view,hiddenColumnIds,onApply}:Props){
 const [draft,setDraft]=useState<string[]|null>(null);
 const {columns}=presetDescriptors(view);
 const visibleBusiness=columns.filter(column=>!column.system&&!draft?.includes(column.id)).length;
 const close=()=>setDraft(null);
 return <><button className="admin-button" type="button" onClick={()=>setDraft([...hiddenColumnIds])}>显示字段</button>{draft&&<Modal title="显示字段" onClose={close}>
  <div className="record-column-visibility"><div className="record-column-options">{columns.map(column=>{
   const checked=!draft.includes(column.id),lastBusiness=checked&&!column.system&&visibleBusiness===1;
   return <label className="forms-checkbox" key={column.id}><input type="checkbox" checked={checked} disabled={lastBusiness} onChange={event=>setDraft(current=>event.target.checked?current!.filter(id=>id!==column.id):[...current!,column.id])}/>{column.name}</label>;
  })}</div><div className="record-column-actions"><button className="admin-button" type="button" onClick={close}>取消</button><button className="admin-button primary" type="button" disabled={!visibleBusiness} onClick={()=>{onApply([...draft]);close();}}>应用</button></div></div>
 </Modal>}</>;
}
