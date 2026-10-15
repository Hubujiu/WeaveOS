import {useEffect,useRef,useState} from 'react';
import {useLocation} from 'react-router';
import {applicationApi,ApplicationError} from '../api';
import {FormsDialog} from '../forms/FormsDialog';
import {decodeTemplateFile} from './fileInput';

export function TemplateExport({actorId,appId,name,onUnauthorized,onIdentityMismatch}:{actorId:string;appId:string;name:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [open,setOpen]=useState(false),[pending,setPending]=useState(false),[closing,setClosing]=useState(false),[error,setError]=useState('');
 const current=useRef<AbortController|null>(null),location=useLocation();
 const abort=()=>{current.current?.abort();current.current=null;};
 useEffect(()=>{setOpen(false);return()=>{current.current?.abort();current.current=null;};},[location.key]);
 const cancelRead=()=>{abort();setClosing(true);};
 const exportTemplate=async()=>{
  if(current.current||closing)return;
  const controller=new AbortController();current.current=controller;setPending(true);setError('');
  try{
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/structure-template`,'GET',undefined,controller.signal);
   if(current.current!==controller||controller.signal.aborted)return;
   const json=JSON.stringify(raw),manifest=decodeTemplateFile(new TextEncoder().encode(json));
   if(!manifest||(manifest.application as Record<string,unknown>).id!==appId)throw Error('模板响应不完整或不属于当前应用，未生成下载文件。');
   const url=URL.createObjectURL(new Blob([json],{type:'application/json;charset=utf-8'}));
   try{const link=document.createElement('a');link.href=url;link.download=`weaveos-template-${appId}.json`;document.body.append(link);link.click();link.remove();}
   finally{setTimeout(()=>URL.revokeObjectURL(url),0);}
  }catch(cause){
   if(current.current!==controller||controller.signal.aborted)return;
   if(cause instanceof ApplicationError&&cause.status===401)onUnauthorized();
   else if(cause instanceof ApplicationError&&cause.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
   else setError(cause instanceof Error?cause.message:'无法导出模板，请稍后重试。');
  }finally{if(current.current===controller){current.current=null;setPending(false);}}
 };
 return <div className="template-export-control" onKeyDownCapture={event=>{if(event.key==='Escape')cancelRead();}} onClickCapture={event=>{
  if(open&&(event.target as HTMLElement).closest('[data-forms-close],.forms-icon-button'))cancelRead();
 }}><button type="button" className="admin-button" onClick={()=>{setPending(false);setClosing(false);setError('');setOpen(true);}}>导出应用模板</button>
 {open&&<FormsDialog title="导出应用模板" onClose={()=>{abort();setOpen(false);}}><div className="template-export-confirm">
  <h3>{name}</h3><p>导出当前应用已保存的完整配置，本地未保存的修改不会写入模板。</p>
  <ul><li>目录、表、字段及表单布局</li><li>流程候选配置与触发条件</li><li>权限组配置及待映射的人员、部门引用</li></ul>
  <p className="workflow-publication-caption">不包含业务记录、附件、审批实例、日志、密码或会话。导入后流程保持关闭。</p>
  {error&&<p role="alert">{error}</p>}
  <div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" aria-label="导出模板" disabled={pending||closing} onClick={()=>void exportTemplate()}>{pending?'导出中…':'导出模板'}</button></div>
 </div></FormsDialog>}</div>;
}
