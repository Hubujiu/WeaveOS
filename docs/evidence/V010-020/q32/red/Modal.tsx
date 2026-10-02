import { useEffect, useId, useRef, type ReactNode } from 'react';

export function Modal({title,children,onClose,busy=false}:{title:string;children:ReactNode;onClose:()=>void;busy?:boolean}){
 const ref=useRef<HTMLDialogElement>(null);const id=useId();
 useEffect(()=>{const previous=document.activeElement as HTMLElement|null;ref.current?.showModal();return()=>{previous?.focus();};},[]);
 return <dialog ref={ref} className="personnel-dialog" aria-labelledby={id} onCancel={e=>{e.preventDefault();if(!busy)onClose();}}>
  <div className="dialog-heading"><h2 id={id}>{title}</h2><button type="button" className="text-button" aria-label="关闭" onClick={onClose} disabled={busy}>×</button></div>{children}
 </dialog>;
}
