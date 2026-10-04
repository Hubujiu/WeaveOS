import {gsap} from 'gsap';

/** A dialog grows from the element that opened it and returns to that element. */
export function dialogMotion(dialog:HTMLDialogElement,origin:HTMLElement|null,onClosed:()=>void){
  const reduced=window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if(reduced||!origin?.isConnected){dialog.dataset.motionReady='true';
    return {close:onClosed,dispose:()=>{}};}
  const from=origin.getBoundingClientRect(),to=dialog.getBoundingClientRect();
  if(!from.width||!from.height||!to.width||!to.height){dialog.dataset.motionReady='true';
    return {close:onClosed,dispose:()=>{}};}
  const x=from.left+from.width/2-(to.left+to.width/2);
  const y=from.top+from.height/2-(to.top+to.height/2);
  const scaleX=Math.max(.18,Math.min(1,from.width/to.width));
  const scaleY=Math.max(.18,Math.min(1,from.height/to.height));
  dialog.dataset.motionReady='false';
  let timeline:gsap.core.Timeline;
  const context=gsap.context(()=>{
    timeline=gsap.timeline({paused:true,onComplete:()=>{dialog.dataset.motionReady='true';},
      onReverseComplete:onClosed})
      .fromTo(dialog,{x,y,scaleX,scaleY,opacity:.55,transformOrigin:'50% 50%'},
        {x:0,y:0,scaleX:1,scaleY:1,opacity:1,duration:.28,ease:'power2.out'});
    timeline.play();
  },dialog);
  return {
    close:()=>{dialog.dataset.motionReady='false';
      if(timeline.reversed()||timeline.progress()===0){onClosed();return;}
      timeline.timeScale(timeline.progress()<.5?2:1).reverse();},
    dispose:()=>context.revert(),
  };
}
