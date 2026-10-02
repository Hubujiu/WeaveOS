import {chromium,firefox,expect} from '@playwright/test';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {rename} from 'node:fs/promises';
import {resolve} from 'node:path';
import {createHash} from 'node:crypto';
const base=process.env.WEAVEOS_WEB_URL;
if(!base?.startsWith('https://')||!['localhost','127.0.0.1'].includes(new URL(base).hostname))throw Error('Actual isolated loopback HTTPS required');
const fixture=JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES,'utf8'));
const out=resolve('docs/evidence/V010-020/q36-b2/motion-race/final');mkdirSync(out,{recursive:true});
const raw=resolve('.work/q36-motion-formal-video');mkdirSync(raw,{recursive:true});
async function login(context){const r=await context.request.post(base+'/api/v1/sessions',{headers:{Origin:base},data:fixture.admin});expect(r.status(),'private synthetic prerequisite login').toBe(201);}
function observe(){
 const state={native:0,pending:0,escapes:0,busySlices:0,durations:[]};Object.defineProperty(window,'__q36FormalMotion',{value:state});
 document.addEventListener('keydown',event=>{if(event.isTrusted&&event.key==='Escape')state.escapes++;},true);
 const native=document.startViewTransition?.bind(document);if(native)document.startViewTransition=(...args)=>{state.native++;state.pending++;const vt=native(...args);vt.ready.then(()=>{for(const a of document.getAnimations())if(a.effect instanceof KeyframeEffect&&a.effect.pseudoElement?.includes('q36-filter-shell'))state.durations.push(a.effect.getTiming().duration);},()=>{});vt.finished.then(()=>state.pending--,()=>state.pending--);return vt;};
}
async function ready(page){await page.goto(base+'/app/admin');await expect(page.locator('.member-table tbody tr[data-row-id]').first()).toBeVisible();await page.evaluate(()=>document.fonts.ready);expect(await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.')).toBe(true);}
async function settled(page){await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-filter-transition-active')&&window.__q36FormalMotion.pending===0);}
async function shot(page,name){await settled(page);await page.screenshot({path:resolve(out,name+'.png'),fullPage:false});}
async function pixels(page,trigger,name){await page.mouse.move(0,0);await trigger.focus();const image=await trigger.screenshot();writeFileSync(resolve(out,name+'.png'),image);return image;}
async function closed(page,trigger){await expect(page.getByRole('dialog',{name:'自定义筛选',exact:true})).not.toBeVisible();await expect(trigger).toBeFocused();await settled(page);await expect(trigger).toHaveAttribute('aria-expanded','false');await expect(trigger.locator('.q36-filter-trigger-content')).toHaveCSS('visibility','visible');await expect(trigger.locator('svg')).toBeVisible();}
const report={sourceComponentSHA256:createHash('sha256').update(readFileSync('apps/web/src/QueryFilterPanel.tsx')).digest('hex'),target:'real HTTPS/BFF/PG/Redis synthetic GREEN instance; API login before recorded page; no credential-bearing page/trace',engines:{}};
for(const[name,engine]of[['chromium',chromium],['firefox',firefox]]){
 const browser=await engine.launch();
 const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:900},recordVideo:{dir:raw,size:{width:1440,height:900}}});await login(context);await context.addInitScript(observe);
 const page=await context.newPage();await ready(page);await page.keyboard.press('Tab');const trigger=page.getByRole('button',{name:'自定义筛选',exact:true});const before=await pixels(page,trigger,name+'-initial-trigger');await page.waitForTimeout(800);
 await trigger.click();const dialog=page.getByRole('dialog',{name:'自定义筛选',exact:true});await expect(dialog).toBeVisible();await dialog.getByRole('button',{name:'组 1 添加分组',exact:true}).click();await dialog.getByLabel('组 1.1 匹配方式',{exact:true}).selectOption('or');await shot(page,name+'-members-open-desktop');
 await page.keyboard.press('Escape');await closed(page,trigger);await page.waitForTimeout(1100);expect((await pixels(page,trigger,name+'-normal-closed-trigger')).equals(before),'formal normal close restores exact trigger pixels').toBe(true);
 await trigger.click();await expect(dialog).toBeVisible();
 await page.evaluate(()=>{const until=performance.now()+1400;function busy(){const end=performance.now()+35;while(performance.now()<end){}window.__q36FormalMotion.busySlices++;if(performance.now()<until)setTimeout(busy,40);}setTimeout(busy,0);});
 await page.keyboard.press('Escape');await trigger.click();await page.keyboard.press('Escape');await closed(page,trigger);await page.waitForTimeout(1100);expect((await pixels(page,trigger,name+'-quick-closed-trigger')).equals(before),'formal CPU-loaded reversal restores exact trigger pixels').toBe(true);await shot(page,name+'-members-closed-desktop');
 await trigger.click();await expect(dialog).toBeVisible();await shot(page,name+'-members-reopened-desktop');await page.keyboard.press('Escape');await closed(page,trigger);await page.waitForTimeout(1100);
 const desktop=await page.evaluate(()=>window.__q36FormalMotion);expect(desktop.escapes).toBe(4);expect(desktop.busySlices).toBeGreaterThan(0);expect(desktop.durations).toContain(300);expect(desktop.durations).toContain(220);
 const video=page.video();await context.close();const videoPath=resolve(out,name+'-formal-open-close-reversal.webm');if(!video)throw Error('Real video missing');await rename(await video.path(),videoPath);
 const views=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:390,height:844}});await login(views);await views.addInitScript(observe);const narrow=await views.newPage();await ready(narrow);await narrow.keyboard.press('Tab');const narrowTrigger=narrow.getByRole('button',{name:'自定义筛选',exact:true});const narrowBefore=await pixels(narrow,narrowTrigger,name+'-narrow-initial-trigger');
 await narrowTrigger.click();await expect(narrow.getByRole('dialog',{name:'自定义筛选',exact:true})).toBeVisible();await shot(narrow,name+'-members-open-narrow');await narrow.keyboard.press('Escape');await closed(narrow,narrowTrigger);expect((await pixels(narrow,narrowTrigger,name+'-narrow-closed-trigger')).equals(narrowBefore),'formal narrow close restores exact trigger pixels').toBe(true);await shot(narrow,name+'-members-closed-narrow');
 await narrow.setViewportSize({width:1440,height:900});await narrow.getByRole('tab',{name:'操作记录',exact:true}).click();await expect(narrow.locator('.activity-table tbody tr[data-row-id]').first()).toBeVisible();const eventTrigger=narrow.getByRole('button',{name:'自定义筛选',exact:true});await eventTrigger.click();await expect(narrow.getByRole('dialog',{name:'自定义筛选',exact:true})).toBeVisible();await shot(narrow,name+'-events-open-desktop');await narrow.keyboard.press('Escape');await closed(narrow,eventTrigger);await shot(narrow,name+'-events-closed-desktop');
 report.engines[name]={desktop,views:await narrow.evaluate(()=>window.__q36FormalMotion),video:videoPath.split(/[\\/]/).at(-1),triggerPixelsIdentical:{normal:true,quickCpuLoad:true,narrow:true},desktopClosedHoldsMs:[1100,1100,1100]};
 await views.close();await browser.close();console.log('Formal '+name+': desktop/narrow/events, trusted rapid reversal under CPU load, exact restored pixels, focus and 300/220ms timings verified; actual video saved.');
}
writeFileSync(resolve(out,'metrics.json'),JSON.stringify(report,null,2)+'\n');
