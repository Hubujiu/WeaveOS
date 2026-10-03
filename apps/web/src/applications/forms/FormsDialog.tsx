import { useEffect, useId, useRef, type ReactNode } from 'react';

export function FormsDialog({title,onClose,children,busy=false}:{
  title:string;onClose:()=>void;children:ReactNode;busy?:boolean;
}) {
  const dialog=useRef<HTMLDialogElement>(null);
  const heading=useId();
  useEffect(()=>{
    const origin=document.activeElement instanceof HTMLElement?document.activeElement:null;
    dialog.current?.showModal();
    return()=>{dialog.current?.close();if(origin?.isConnected)origin.focus();};
  },[]);
  return <dialog ref={dialog} className="forms-dialog" aria-labelledby={heading}
    onCancel={event=>{event.preventDefault();if(!busy)onClose();}}>
    <div className="forms-dialog-heading"><h2 id={heading}>{title}</h2>
      <button type="button" className="forms-icon-button" aria-label="关闭弹窗"
        disabled={busy} onClick={onClose}>×</button></div>
    <div className="forms-dialog-body">{children}</div>
  </dialog>;
}
