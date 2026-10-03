import { useEffect, useId, useRef, type ReactNode } from 'react';
import {dialogMotion} from '../motion/dialogMotion';

export function FormsDialog({title,onClose,children,busy=false}:{
  title:string;onClose:()=>void;children:ReactNode;busy?:boolean;
}) {
  const dialog=useRef<HTMLDialogElement>(null);
  const animation=useRef<ReturnType<typeof dialogMotion>|null>(null);
  const onCloseRef=useRef(onClose);
  onCloseRef.current=onClose;
  const heading=useId();
  useEffect(()=>{
    const origin=document.activeElement instanceof HTMLElement?document.activeElement:null;
    dialog.current?.showModal();
    if(dialog.current)animation.current=dialogMotion(dialog.current,origin,()=>onCloseRef.current());
    return()=>{animation.current?.dispose();animation.current=null;dialog.current?.close();if(origin?.isConnected)origin.focus();};
  },[]);
  const close=()=>{if(!busy)animation.current?.close();};
  return <dialog ref={dialog} className="forms-dialog" aria-labelledby={heading}
    onClickCapture={event=>{if((event.target as HTMLElement).closest('[data-forms-close]')){
      event.preventDefault();event.stopPropagation();close();}}}
    onCancel={event=>{event.preventDefault();close();}}>
    <div className="forms-dialog-heading"><h2 id={heading}>{title}</h2>
      <button type="button" className="forms-icon-button" aria-label="关闭弹窗"
        disabled={busy} onClick={close}>×</button></div>
    <div className="forms-dialog-body">{children}</div>
  </dialog>;
}
