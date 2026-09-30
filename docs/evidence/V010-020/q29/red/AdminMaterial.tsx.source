import { useId, useLayoutEffect, useRef } from 'react';

// Q28 adapts the Figma L surface to the live shell geometry. Only the surface
// endpoints change; the 22px corners, translucent fill, stroke and shadow retain
// their source values. One observer keeps it aligned during resize and motion.
export function AdminMaterial() {
 const svg=useRef<SVGSVGElement>(null);const filterId=useId();
 useLayoutEffect(()=>{
  const node=svg.current!;const shell=node.parentElement!;
  const header=shell.querySelector<HTMLElement>('.admin-header')!;
  const sidebar=shell.querySelector<HTMLElement>('.admin-sidebar')!;
  const paint=()=>{
   const {width:w,height:h}=shell.getBoundingClientRect();
   const top=header.getBoundingClientRect().height;const side=sidebar.getBoundingClientRect().width;
   const r=22;const a=r/3;
   const edge=`M${w} ${top-r} C${w} ${top-a} ${w-a} ${top} ${w-r} ${top} H${side+r} C${side+a} ${top} ${side} ${top+a} ${side} ${top+r} V${h-r} C${side} ${h-a} ${side-a} ${h} ${side-r} ${h}`;
   node.setAttribute('width',String(w));node.setAttribute('height',String(h));
   node.querySelector('.material-fill')!.setAttribute('d',`M0 0 H${w} V${top-r} ${edge.slice(edge.indexOf(' C')+1)} H0 Z`);
   node.querySelector('.material-edge')!.setAttribute('d',edge);
  };
  paint();const observer=new ResizeObserver(paint);for(const target of [shell,header,sidebar])observer.observe(target);
  return ()=>observer.disconnect();
 },[]);
 return <svg ref={svg} className="admin-material" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" focusable="false">
  <defs><filter id={filterId} x="-10%" y="-10%" width="120%" height="120%" colorInterpolationFilters="sRGB"><feDropShadow dx="8" dy="8" stdDeviation="15" floodColor="#0f172a" floodOpacity="0.045"/></filter></defs>
  <path className="material-fill" fill="white" fillOpacity="0.76" filter={`url(#${filterId})`}/>
  <path className="material-edge" fill="none" stroke="#dce3ec" strokeOpacity="0.42"/>
 </svg>;
}
