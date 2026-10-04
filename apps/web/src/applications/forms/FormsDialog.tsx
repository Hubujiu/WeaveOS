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
    const element=dialog.current;
    element?.showModal();
    if(element)animation.current=dialogMotion(element,origin,()=>onCloseRef.current());
    // Passive cleanup runs after React detaches refs. Close the captured top-layer
    // element before restoring focus, including quick-close and unmount paths.
    return()=>{animation.current?.dispose();animation.current=null;element?.close();if(origin?.isConnected)origin.focus();};
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
