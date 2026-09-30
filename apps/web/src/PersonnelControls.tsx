import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react';

// Page-local controls. Q32 defines the appearance; personnel state owns all values.
type Choice = { value: string; label: string };
const menuHeightEase = 'linear(0,0.01376,0.04981,0.10143,0.16323,0.23095,0.30127,0.37169,0.44033,0.50585,0.56734,0.62424,0.67625,0.72326,0.76534,0.80267,0.83549,0.86412,0.88890,0.91016,0.92827,0.94356,0.95638,0.96701,0.97575,0.98287,0.98858,0.99312,0.99666,0.99937,1.00139,1.00286,1.00387,1.00451,1.00487,1.00501,1.00498,1.00483,1.00459,1.00430,1)';
const menuGapEase = 'linear(0,0.04052,0.14458,0.28769,0.44868,0.61049,0.76043,0.88999,0.99446,1.07228,1.12435,1.15334,1.16297,1.15753,1.14135,1.11845,1.09234,1.06588,1.04119,1.01972,1.00229,0.98920,0.98034,0.97529,0.97346,0.97415,0.97666,0.98032,0.98455,0.98887,0.99293,0.99649,0.99939,1.00159,1.00310,1.00398,1.00432,1.00424,1.00385,1.00327,1)';
const easeOut = 'cubic-bezier(.16,1,.3,1)';
export function PersonnelSelect({label,value,options,onChange,className='',disabled=false}:{label:string;value:string;options:Choice[];onChange:(value:string)=>void;className?:string;disabled?:boolean}) {
 const id=useId();const root=useRef<HTMLSpanElement>(null);const trigger=useRef<HTMLButtonElement>(null);const panel=useRef<HTMLDivElement>(null);
 const [open,setOpen]=useState(false);const [mounted,setMounted]=useState(false);const [active,setActive]=useState(0);const [position,setPosition]=useState<CSSProperties>({});const [above,setAbove]=useState(false);
 const search=useRef({text:'',at:0});const selected=options.findIndex(o=>o.value===value);
 const motion=useRef<{node:HTMLDivElement;animations:Animation[]}|null>(null);
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
 useLayoutEffect(()=>{
  const node=panel.current;if(!node||typeof position.height!=='number')return;
  const previous=motion.current;const continuing=previous?.node===node;
  // Read the rendered frame before cancelling, so reversal starts at that frame.
  const style=getComputedStyle(node);const height=continuing?style.height:'0px';
  const opacity=continuing?style.opacity:'0';const transform=continuing?style.transform:`translateY(${above?8:-8}px)`;
  const first=above?'borderBottomLeftRadius':'borderTopLeftRadius';const second=above?'borderBottomRightRadius':'borderTopRightRadius';
  const radius=continuing?style[first]:'0px';const button=trigger.current!;const triggerFirst=above?'borderTopLeftRadius':'borderBottomLeftRadius';const triggerSecond=above?'borderTopRightRadius':'borderBottomRightRadius';
  const triggerRadius=continuing?getComputedStyle(button)[triggerFirst]:'0px';
  previous?.animations.forEach(animation=>animation.cancel());
  const reduce=matchMedia('(prefers-reduced-motion:reduce)').matches;
  const animations:Animation[]=[];
  function animate(target:HTMLElement,frames:Keyframe[],duration:number,easing:string,delay=0){animations.push(target.animate(frames,{duration,easing,delay,fill:'both'}));}
  animate(node,[{height},{height:open?position.height+'px':'0px'}],reduce?120:open?420:260,reduce?'ease-out':open?menuHeightEase:easeOut,reduce||open?0:140);
  animate(node,[{opacity},{opacity:open?1:0}],reduce?120:open?180:160,reduce?'ease-out':easeOut,reduce||open?0:120);
  if(!reduce){
   animate(node,[{transform},{transform:open?'none':`translateY(${above?8:-8}px)`}],open?600:300,open?menuGapEase:menuHeightEase,open?120:0);
   animate(node,[{[first]:radius,[second]:radius},{[first]:open?'12px':'0px',[second]:open?'12px':'0px'}],open?300:160,easeOut,open?140:0);
   animate(button,[{[triggerFirst]:triggerRadius,[triggerSecond]:triggerRadius},{[triggerFirst]:'0px',[triggerSecond]:'0px',offset:open?.4:.5},{[triggerFirst]:'12px',[triggerSecond]:'12px'}],open?600:420,easeOut);
  }
  motion.current={node,animations};
 },[open,mounted,position.height,above]);
 useEffect(()=>()=>motion.current?.animations.forEach(animation=>animation.cancel()),[]);
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
 useLayoutEffect(()=>{
  const node=panel.current;const option=node?.querySelector<HTMLElement>('[data-active=true]');if(!open||!node||!option)return;
  // Scroll only this list, never the dialog or page containing its trigger.
  const top=option.offsetTop-4,bottom=top+option.offsetHeight;
  const viewport=typeof position.height==='number'?position.height-10:node.clientHeight;
  if(top<node.scrollTop)node.scrollTop=top;else if(bottom>node.scrollTop+viewport)node.scrollTop=bottom-viewport;
 },[open,active,position.height]);
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
   {options.map((option,index)=><div key={option.value} id={id+'-'+index} role="option" aria-selected={option.value===value} data-active={active===index} data-entering={index<Math.ceil((Number(position.height)||330)/32)} className="personnel-select-option" style={{'--option-delay':50+index*35+'ms'} as CSSProperties} onPointerMove={()=>setActive(index)} onPointerDown={e=>e.preventDefault()} onClick={event=>{event.preventDefault();event.stopPropagation();choose(index);}}>{option.label}</div>)}
  </div>}
 </span>;
}

export function PersonnelTabs<T extends string>({value,values,onChange}:{value:T;values:readonly T[];onChange:(value:T)=>void}) {
 const track=useRef<HTMLDivElement>(null);const [indicator,setIndicator]=useState({x:4,width:0});const id=useId();
 useLayoutEffect(()=>{
  const node=track.current!;function measure(){const selected=node.querySelector<HTMLButtonElement>('[aria-selected=true]');if(selected)setIndicator({x:selected.offsetLeft,width:selected.offsetWidth});}
  measure();const resize=new ResizeObserver(measure);resize.observe(node);for(const child of node.querySelectorAll('button'))resize.observe(child);return()=>resize.disconnect();
 },[value]);
 function reveal(button:HTMLButtonElement){
  const bar=track.current!.parentElement!;const left=button.offsetLeft,right=left+button.offsetWidth;
  if(left<bar.scrollLeft)bar.scrollLeft=left;else if(right>bar.scrollLeft+bar.clientWidth)bar.scrollLeft=right-bar.clientWidth;
 }
 useLayoutEffect(()=>{const selected=track.current?.querySelector<HTMLButtonElement>('[aria-selected=true]');if(selected)reveal(selected);},[value]);
 function keys(event:KeyboardEvent<HTMLButtonElement>,index:number){
  const key=event.key;if(!['ArrowLeft','ArrowRight','Home','End'].includes(key))return;event.preventDefault();
  const next=key==='Home'?0:key==='End'?values.length-1:(index+(key==='ArrowRight'?1:-1)+values.length)%values.length;
  const button=track.current?.querySelectorAll('button')[next];if(button){button.focus({preventScroll:true});reveal(button);}
 }
 return <nav className="personnel-tabs" role="tablist" aria-label="人员管理视图"><div ref={track} className="personnel-tabs-track">
  <span className="personnel-tab-indicator" aria-hidden="true" style={{width:indicator.width,transform:`translateX(${indicator.x}px)`}}/>
  {values.map((tab,index)=><button key={tab} id={id+'-'+index} type="button" role="tab" aria-selected={value===tab} aria-controls="personnel-panel" onKeyDown={event=>keys(event,index)} onClick={()=>onChange(tab)}>{tab}</button>)}
 </div></nav>;
}
