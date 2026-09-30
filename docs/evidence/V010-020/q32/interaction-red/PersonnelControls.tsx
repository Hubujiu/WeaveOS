import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react';

// Page-local controls. Q32 defines the appearance; personnel state owns all values.
type Choice = { value: string; label: string };
export function PersonnelSelect({label,value,options,onChange,className='',disabled=false}:{label:string;value:string;options:Choice[];onChange:(value:string)=>void;className?:string;disabled?:boolean}) {
 const id=useId();const root=useRef<HTMLSpanElement>(null);const trigger=useRef<HTMLButtonElement>(null);const panel=useRef<HTMLDivElement>(null);
 const [open,setOpen]=useState(false);const [mounted,setMounted]=useState(false);const [active,setActive]=useState(0);const [position,setPosition]=useState<CSSProperties>({});const [above,setAbove]=useState(false);
 const search=useRef({text:'',at:0});const selected=options.findIndex(o=>o.value===value);
 function close(){setOpen(false);}
 function show(index=selected>=0?selected:0){if(disabled||!options.length)return;setActive(index);setMounted(true);setOpen(true);}
 function choose(index:number){const choice=options[index];if(!choice)return;onChange(choice.value);close();trigger.current?.focus();}
 useLayoutEffect(()=>{
  if(!open)return;const button=trigger.current!;const b=button.getBoundingClientRect();const desired=Math.min(options.length*32+10,330);
  const below=innerHeight-b.bottom-16,over=b.top-16;const up=below<desired&&over>below;const height=Math.max(1,Math.min(desired,up?over:below));
  const width=Math.min(b.width,innerWidth-16),left=Math.max(8,Math.min(b.left,innerWidth-width-8));
  setAbove(up);setPosition({left,top:up?b.top-8-height:b.bottom+8,width,height,'--menu-height':height+'px'} as CSSProperties);
  panel.current?.showPopover();
 },[open,options.length]);
 useEffect(()=>{
  if(open||!mounted)return;const timer=window.setTimeout(()=>setMounted(false),matchMedia('(prefers-reduced-motion:reduce)').matches?120:420);return()=>clearTimeout(timer);
 },[open,mounted]);
 useEffect(()=>{
  if(!open)return;
  function outside(event:PointerEvent){if(!root.current?.contains(event.target as Node))close();}
  function shifted(event:Event){if(!panel.current?.contains(event.target as Node))close();}
  window.addEventListener('pointerdown',outside,true);window.addEventListener('resize',close);document.addEventListener('scroll',shifted,true);
  return()=>{window.removeEventListener('pointerdown',outside,true);window.removeEventListener('resize',close);document.removeEventListener('scroll',shifted,true);};
 },[open]);
 useLayoutEffect(()=>{if(open)panel.current?.querySelector<HTMLElement>('[data-active=true]')?.scrollIntoView({block:'nearest'});},[open,active]);
 function keys(event:KeyboardEvent<HTMLButtonElement>){
  const key=event.key;
  if(key==='Escape'&&open){event.preventDefault();event.stopPropagation();close();return;}
  if(key==='Tab'){close();return;}
  if(['ArrowDown','ArrowUp','Home','End','Enter',' '].includes(key)){
   event.preventDefault();
   if(key==='Enter'||key===' '){if(open)choose(active);else show();return;}
   if(!open){show(key==='End'?options.length-1:key==='Home'?0:selected>=0?selected:key==='ArrowUp'?options.length-1:0);return;}
   setActive(i=>key==='Home'?0:key==='End'?options.length-1:Math.max(0,Math.min(options.length-1,i+(key==='ArrowDown'?1:-1))));return;
  }
  if(key.length===1&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
   event.preventDefault();const now=performance.now();search.current={text:(now-search.current.at<700?search.current.text:'')+key.toLocaleLowerCase(),at:now};
   const index=options.findIndex(o=>o.label.toLocaleLowerCase().startsWith(search.current.text));if(index>=0){if(open)setActive(index);else show(index);}
  }
 }
 return <span ref={root} className={'personnel-select '+className}>
  <button ref={trigger} type="button" className="personnel-select-trigger" role="combobox" aria-label={label} aria-haspopup="listbox" aria-expanded={open} aria-controls={mounted?id:undefined} aria-activedescendant={open?id+'-'+active:undefined} disabled={disabled||!options.length} onClick={()=>open?close():show()} onKeyDown={keys} data-side={above?'above':'below'}>
   <span>{options[selected]?.label||'请选择'}</span><svg className="personnel-select-arrow" width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m4.5 6.25 3.5 3.5 3.5-3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/></svg>
  </button>
  {mounted&&<div ref={panel} id={id} popover="manual" role="listbox" aria-label={label} aria-hidden={!open} className="personnel-select-menu" data-state={open?'open':'closed'} data-side={above?'above':'below'} style={position}>
   {options.map((option,index)=><div key={option.value} id={id+'-'+index} role="option" aria-selected={option.value===value} data-active={active===index} className="personnel-select-option" style={{'--option-delay':50+index*35+'ms'} as CSSProperties} onPointerMove={()=>setActive(index)} onPointerDown={e=>e.preventDefault()} onClick={event=>{event.preventDefault();event.stopPropagation();choose(index);}}>{option.label}</div>)}
  </div>}
 </span>;
}

export function PersonnelTabs<T extends string>({value,values,onChange}:{value:T;values:readonly T[];onChange:(value:T)=>void}) {
 const track=useRef<HTMLDivElement>(null);const [indicator,setIndicator]=useState({x:4,width:0});const id=useId();
 useLayoutEffect(()=>{
  const node=track.current!;function measure(){const selected=node.querySelector<HTMLButtonElement>('[aria-selected=true]');if(selected){setIndicator({x:selected.offsetLeft,width:selected.offsetWidth});selected.scrollIntoView({block:'nearest',inline:'nearest'});}}
  measure();const resize=new ResizeObserver(measure);resize.observe(node);for(const child of node.querySelectorAll('button'))resize.observe(child);return()=>resize.disconnect();
 },[value,values]);
 function keys(event:KeyboardEvent<HTMLButtonElement>,index:number){
  const key=event.key;if(!['ArrowLeft','ArrowRight','Home','End'].includes(key))return;event.preventDefault();
  const next=key==='Home'?0:key==='End'?values.length-1:(index+(key==='ArrowRight'?1:-1)+values.length)%values.length;
  const button=track.current?.querySelectorAll('button')[next];button?.focus();button?.scrollIntoView({block:'nearest',inline:'nearest'});
 }
 return <nav className="personnel-tabs" role="tablist" aria-label="人员管理视图"><div ref={track} className="personnel-tabs-track">
  <span className="personnel-tab-indicator" aria-hidden="true" style={{width:indicator.width,transform:`translateX(${indicator.x}px)`}}/>
  {values.map((tab,index)=><button key={tab} id={id+'-'+index} type="button" role="tab" aria-selected={value===tab} aria-controls="personnel-panel" onKeyDown={event=>keys(event,index)} onClick={()=>onChange(tab)}>{tab}</button>)}
 </div></nav>;
}
