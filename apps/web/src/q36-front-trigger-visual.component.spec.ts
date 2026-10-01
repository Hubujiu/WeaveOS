import {test,expect,type Page,type TestInfo} from '@playwright/test';

async function triggerPixels(page:Page,testInfo:TestInfo,label:string) {
  const trigger=page.getByRole('button',{name:/自定义筛选/});
  await page.mouse.move(0,0);
  await trigger.focus();
  const image=await trigger.screenshot();
  await testInfo.attach(label,{body:image,contentType:'image/png'});
  return image;
}

for(const mode of ['native','fallback','reduced','quick','fallback-quick','reduced-quick'] as const) {
  test(`Q36 ${mode} close restores trigger pixels, visible text/icon and reopen`,async({page},testInfo)=>{
    if(mode.startsWith('fallback'))await page.addInitScript(()=>Object.defineProperty(document,'startViewTransition',{value:undefined,configurable:true}));
    if(mode.startsWith('reduced'))await page.emulateMedia({reducedMotion:'reduce'});
    await page.addInitScript(()=>{
      const native=document.startViewTransition?.bind(document);
      const state={count:0,pending:0};
      Object.defineProperty(window,'__q36VisualTransition',{value:state});
      if(native)document.startViewTransition=((...args:Parameters<Document['startViewTransition']>)=>{
        state.count++;state.pending++;
        const transition=native(...args);
        transition.finished.finally(()=>state.pending--);
        return transition;
      }) as Document['startViewTransition'];
    });
    await page.goto('/src/q36-front-fixture.html');
    if(mode==='native'||mode==='quick')test.skip(!await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.'),'native shared transition unavailable or unsafe in this engine');
    await page.evaluate(()=>document.fonts.ready);
    const before=await triggerPixels(page,testInfo,'initial-trigger');
    const trigger=page.getByRole('button',{name:/自定义筛选/});
    const dialog=page.getByRole('dialog',{name:'自定义筛选'});
    await trigger.click();
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    if(mode.endsWith('quick')){
      await trigger.click();
      await expect(dialog).toBeVisible();
      await page.keyboard.press('Escape');
    }
    await expect(dialog).not.toBeVisible();
    if(mode==='native'||mode==='quick'){
      await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36VisualTransition:{count:number}}).__q36VisualTransition.count)).toBeGreaterThanOrEqual(2);
      await page.waitForFunction(()=>(window as unknown as {__q36VisualTransition:{pending:number}}).__q36VisualTransition.pending===0);
    }
    await page.evaluate(()=>Promise.allSettled(document.getAnimations().filter(a=>a.effect instanceof KeyframeEffect&&a.effect.target instanceof Element&&a.effect.target.closest('.q36-filter-popup')).map(a=>a.finished)));
    await page.waitForTimeout(1000);
    const content=trigger.locator('.q36-filter-trigger-content');
    await expect(content).toBeVisible();
    await expect(content).toHaveCSS('opacity','1');
    await expect(content).toHaveCSS('visibility','visible');
    await expect(content.locator('svg')).toBeVisible();
    await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
    const after=await triggerPixels(page,testInfo,'closed-trigger');
    expect(after.equals(before),'closed trigger must render the same icon and text pixels as its initial state').toBe(true);
    await trigger.click();
    await expect(dialog).toBeVisible();
    await page.getByRole('heading',{name:'人员管理'}).click();
    await expect(dialog).not.toBeVisible();
    await page.waitForTimeout(1000);
    expect((await triggerPixels(page,testInfo,'outside-closed-trigger')).equals(before),'outside close must restore stable trigger pixels').toBe(true);
  });
}

test('Q36 native shell shares geometry with 300ms open and 220ms close',async({page})=>{
  await page.addInitScript(()=>{
    const native=document.startViewTransition?.bind(document);
    const state={motions:[] as {duration:unknown;shared:boolean}[][]};
    Object.defineProperty(window,'__q36ShellTiming',{value:state});
    if(native)document.startViewTransition=((...args:Parameters<Document['startViewTransition']>)=>{
      const transition=native(...args);
      transition.ready.then(()=>state.motions.push(document.getAnimations().filter(a=>a.effect instanceof KeyframeEffect&&a.effect.pseudoElement?.includes('q36-filter-shell')).map(a=>({duration:a.effect?.getTiming().duration??null,shared:a.effect instanceof KeyframeEffect&&a.effect.pseudoElement?.startsWith('::view-transition-group(')||false}))));
      return transition;
    }) as Document['startViewTransition'];
  });
  await page.goto('/src/q36-front-fixture.html');
  test.skip(!await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.'),'native shared transition unavailable or unsafe in this engine');
  await page.getByRole('button',{name:/自定义筛选/}).click();
  await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36ShellTiming:{motions:{duration:number;shared:boolean}[][]}}).__q36ShellTiming.motions[0])).toContainEqual({duration:300,shared:true});
  await page.keyboard.press('Escape');
  await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36ShellTiming:{motions:{duration:number;shared:boolean}[][]}}).__q36ShellTiming.motions.at(-1))).toContainEqual({duration:220,shared:true});
});
