// Supplementary actual-page pixels after the full E2E has populated synthetic drafts.
// Uses only the explicitly selected isolated instance and private fixture file.
import {chromium,expect} from '@playwright/test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
const base=process.env.WEAVEOS_WEB_URL;
if(!base?.startsWith('https://')||!['localhost','127.0.0.1'].includes(new URL(base).hostname)||!process.env.WEAVEOS_ACCEPTANCE_FIXTURES)throw Error('isolated HTTPS instance and private synthetic fixtures required');
const fixture=JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES,'utf8'));
const browser=await chromium.launch();
const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:390,height:844}});
try{
 const login=await context.request.post(base+'/api/v1/sessions',{headers:{Origin:base},data:fixture.admin});expect(login.status()).toBe(201);
 const page=await context.newPage();await page.goto(base+'/app/admin');
 await page.getByRole('button',{name:'草稿箱',exact:true}).click();
 const row=page.getByRole('dialog',{name:'草稿箱',exact:true}).locator('.personnel-draft-item').filter({has:page.getByText('权限模板',{exact:true})}).first();
 await row.getByRole('button',{name:'恢复草稿',exact:true}).click();
 const resolveTarget=page.getByRole('button',{name:'保留当前输入，使用最新对象版本',exact:true});await expect(resolveTarget).toBeVisible();
 await resolveTarget.scrollIntoViewIfNeeded();await page.evaluate(()=>document.fonts.ready);
 await page.screenshot({path:resolve('docs/evidence/V010-020/q36-b2/final/draft-target-conflict-narrow-detail.png')});
 const cookie=(await context.cookies(base)).find(c=>c.name==='__Host-csrf');
 const headers={Origin:base,'X-CSRF-Token':cookie.value};
 const listing=await (await context.request.get(base+'/api/v1/personnel/drafts')).json();const summary=listing.data.items.find(item=>item.kind==='template');
 const stored=(await (await context.request.get(base+'/api/v1/personnel/drafts/'+summary.id)).json()).data;
 const changed=await context.request.put(base+'/api/v1/personnel/drafts/'+stored.id,{headers,data:{version:stored.version,payload:stored.payload}});expect(changed.ok()).toBe(true);
 await page.getByRole('button',{name:'保存草稿',exact:true}).click();
 const adopt=page.getByRole('button',{name:'保留当前输入，采用最新草稿版本',exact:true});await expect(adopt).toBeVisible();await adopt.scrollIntoViewIfNeeded();
 await page.screenshot({path:resolve('docs/evidence/V010-020/q36-b2/final/draft-cas-conflict-narrow-detail.png')});
 console.log('Actual narrow target/CAS conflict controls captured; synthetic draft newer version retained.');
}finally{await context.close();await browser.close();}
