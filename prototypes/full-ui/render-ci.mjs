import {firefox} from '@playwright/test';
import {mkdir,writeFile,readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
const dir=resolve('prototypes/full-ui'),out=dir+'/output';await mkdir(out,{recursive:true});
const browser=await firefox.launch({headless:true});const errors=[],results=[];
try{
 const page=await browser.newPage({viewport:{width:1440,height:960},deviceScaleFactor:2,reducedMotion:'reduce'});
 page.on('pageerror',e=>errors.push(e.message));
 const url=pathToFileURL(dir+'/WeaveOS-Full.html').href;
 async function go(route){await page.goto(url+'#'+route,{waitUntil:'load'});await page.waitForFunction(r=>window.__readyRoute===r,route.split('?')[0]);await page.evaluate(()=>document.fonts.ready);}
 await go('home');const screens=await page.evaluate(()=>window.PROTOTYPE_SCREENS);
 for(let i=0;i<screens.length;i++){
  const {id,title}=screens[i];await page.setViewportSize({width:1440,height:960});await go(id);await page.mouse.move(1400,950);
  const extra=await page.evaluate(()=>{const b=document.querySelector('.body');return b&&!b.classList.contains('editor-main')?Math.max(0,b.scrollHeight-b.clientHeight):0});
  if(extra>0)await page.setViewportSize({width:1440,height:960+extra+12});
  const metrics=await page.evaluate(()=>({horizontal:document.documentElement.scrollWidth>innerWidth,bodyOverflow:!!document.querySelector('.body')&&document.querySelector('.body').scrollWidth>document.querySelector('.body').clientWidth+1,main:document.querySelector('.main')?.getBoundingClientRect().toJSON(),mainRadius:document.querySelector('.main')?getComputedStyle(document.querySelector('.main')).borderTopLeftRadius:null,visibleText:document.body.innerText.length}));
  assert.equal(metrics.horizontal,false,id+' document overflow');assert.equal(metrics.bodyOverflow,false,id+' content overflow');assert.ok(metrics.visibleText>80,id+' empty');
  if(metrics.main){assert.equal(metrics.main.right,1440);assert.equal(metrics.main.top,0);assert.equal(metrics.main.bottom,page.viewportSize().height);assert.equal(metrics.mainRadius,'18px');assert.ok(await page.locator('.profile').evaluate(e=>e.getBoundingClientRect().bottom<=innerHeight),id+' account menu clipped');}
  const filename=String(i+1).padStart(2,'0')+'-'+id+'.png';await page.screenshot({path:out+'/'+filename});results.push({id,title,filename,metrics});
 }
 await page.setViewportSize({width:1440,height:960});
 for(const [index,id,title,hash] of [[25,'filters','自定义筛选管理','records?modal=filters'],[26,'filter-editor','AND/OR条件编辑','records?modal=filter-editor'],[27,'columns','字段显隐','records?modal=columns'],[28,'import','应用模板导入','apps?modal=import'],[29,'evidence','审批依据','history?modal=evidence'],[30,'return','退回节点','approval?modal=return']]){
  await go(hash);assert.equal(await page.locator('dialog').isVisible(),true);const filename=index+'-'+id+'.png';await page.screenshot({path:out+'/'+filename});results.push({id,title,filename});await page.keyboard.press('Escape');assert.equal(await page.locator('dialog').isVisible(),false);
 }
 await go('records');await page.getByRole('button',{name:'收起侧边栏',exact:true}).click();
 assert.equal(await page.locator('.sidebar').evaluate(e=>e.getBoundingClientRect().width),64);
 await page.mouse.move(900,300);assert.equal(await page.locator('.expand-glyph').evaluate(e=>getComputedStyle(e).opacity),'0');
 await page.getByRole('button',{name:'展开侧边栏',exact:true}).hover();assert.equal(await page.locator('.expand-glyph').evaluate(e=>getComputedStyle(e).opacity),'1');
 await page.getByRole('button',{name:'展开侧边栏',exact:true}).click();assert.equal(await page.locator('.sidebar').evaluate(e=>e.getBoundingClientRect().width),236);
 await page.locator('#localSearch').fill('梁宇凡');assert.equal(await page.locator('tbody tr').count(),2);await page.locator('#localSearch').fill('');assert.equal(await page.locator('tbody tr').count(),10);
 await page.getByRole('button',{name:'筛选',exact:true}).click();await page.getByRole('button',{name:'应用筛选',exact:true}).click();assert.equal(await page.locator('tbody tr').count(),1);
 await go('records');await page.getByRole('button',{name:'编辑表单',exact:true}).click();await page.waitForFunction(()=>window.__readyRoute==='form-builder');
 assert.equal(await page.locator('.editor-center h1,.editor-center h2').count(),0);assert.equal(await page.locator('.properties').isVisible(),true);
 const before=await page.locator('.design-field').count();await page.locator('[data-add-field="单行文字"]').click();assert.equal(await page.locator('.design-field').count(),before+1);
 await page.getByRole('button',{name:'退出',exact:true}).click();await page.waitForFunction(()=>window.__readyRoute==='records');
 await go('approval');await page.getByRole('textbox',{name:'报销金额',exact:true}).fill('2500.00');await page.getByRole('button',{name:'同意',exact:true}).click();assert.equal(await page.getByRole('heading',{name:'先保存修改'}).isVisible(),true);await page.keyboard.press('Escape');
 await go('records?modal=filter-editor');assert.equal(await page.locator('.filter-row').first().locator('select').nth(1).locator('option').count(),2);await page.keyboard.press('Escape');
 await page.emulateMedia({reducedMotion:'no-preference'});await go('records');await page.locator('[data-modal="filters"]').click();await page.waitForFunction(()=>document.querySelector('dialog').getAnimations().every(a=>a.playState==='finished'));await page.getByRole('button',{name:'取消',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('dialog').open);await page.emulateMedia({reducedMotion:'reduce'});
 await go('audit');
 assert.equal(await page.locator('.crumbs button').count(),0);
 await page.getByRole('button',{name:'账户菜单',exact:true}).click();
 assert.equal(await page.locator('#accountMenu').evaluate(e=>e.matches(':popover-open')),true);
 assert.equal(await page.evaluate(()=>window.__readyRoute),'audit');
 await page.screenshot({path:out+'/31-account-menu.png'});
 await page.keyboard.press('Escape');
 assert.equal(await page.locator('#accountMenu').evaluate(e=>e.matches(':popover-open')),false);
 assert.equal(await page.locator('#accountTrigger').evaluate(e=>document.activeElement===e),true);
 await page.getByRole('button',{name:'账户菜单',exact:true}).click();await page.getByRole('menuitem',{name:'退出登录',exact:true}).click();await page.waitForFunction(()=>window.__readyRoute==='login');
 await go('audit');await page.getByRole('button',{name:'收起侧边栏',exact:true}).click();await page.getByRole('button',{name:'账户菜单',exact:true}).click();
 assert.equal(await page.getByRole('menuitem',{name:'设置',exact:true}).isVisible(),true);await page.keyboard.press('Escape');
 await page.screenshot({path:out+'/32-audit-collapsed.png'});
 const iconWeight=await page.locator('.nav svg').first().evaluate(e=>getComputedStyle(e).strokeWidth);assert.equal(iconWeight,'1.75px');
 await go('form-builder');assert.equal(await page.locator('[data-icon]').count()>20,true);
 assert.deepEqual(errors,[]);
 // Browser-rendered overview boards: actual page screenshots, six per board.
 for(let start=0;start<results.length;start+=6){
  const board=await browser.newPage({viewport:{width:1480,height:1710},deviceScaleFactor:1});
  const subset=results.slice(start,start+6);let cards='';
  for(const [n,r]of subset.entries()){const data=await readFile(out+'/'+r.filename);cards+=`<article><header><b>${String(start+n+1).padStart(2,'0')}</b>${r.title}</header><img src="data:image/png;base64,${data.toString('base64')}"></article>`}
  await board.setContent(`<html><meta charset="utf-8"><style>*{box-sizing:border-box}body{margin:0;background:#f5f5f7;padding:32px;font:16px 'Noto Sans CJK SC',sans-serif;color:#45414c}h1{font-size:24px;font-weight:500;margin:0 0 26px}.grid{display:grid;grid-template-columns:1fr 1fr;gap:26px 24px}article{background:white;border:1px solid #e5e3eb;border-radius:12px;overflow:hidden}header{height:48px;display:flex;align-items:center;gap:12px;padding:0 17px;font-size:14px}b{font-size:12px;color:#b1a9bd;font-weight:400}img{display:block;width:100%}</style><h1>WeaveOS <span style="color:#a9a2b2;font-size:16px;margin-left:12px">界面预览 ${Math.floor(start/6)+1} / 5</span></h1><div class="grid">${cards}</div></html>`);
  await board.evaluate(()=>Promise.all([...document.images].map(img=>img.decode())));await board.screenshot({path:out+'/overview-'+(Math.floor(start/6)+1)+'.png',fullPage:true});await board.close();
 }
 await writeFile(out+'/checks.json',JSON.stringify({browser:browser.version(),viewport:{width:1440,height:960},deviceScaleFactor:2,pages:results,interactions:{sidebar:true,logoHover:true,search:true,singleFilter:true,fieldAdd:true,returnToSource:true,dialogEscape:true,accountMenu:true,accountEscapeFocus:true,accountLogoutExplicit:true,accountCollapsed:true,uniformIconWeight:true,breadcrumbActionsRemoved:true},errors},null,2));
 await writeFile(out+'/WeaveOS-Full.html',await readFile(dir+'/WeaveOS-Full.html'));
 console.log(JSON.stringify({screens:results.length,errors}));
}finally{await browser.close()}
