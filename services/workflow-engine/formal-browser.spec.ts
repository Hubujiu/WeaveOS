import {test,expect,type BrowserContext} from '@playwright/test';
import {readFileSync} from 'node:fs';
import {validateFixture} from './product-browser-contract.mjs';

test('real task save, approve, retained evidence and outsider isolation',async({browser},info)=>{
 const f=validateFixture(JSON.parse(readFileSync(process.env.WEAVEOS_PRODUCT_BROWSER_FIXTURE!,'utf8')),process.env.WEAVEOS_PRODUCT_HEAD,info.project.name);
 const contexts:BrowserContext[]=[];
 async function context(other=false){
  // Only this isolated synthetic TLS fixture, matching existing product tests.
  const context=await browser.newContext({baseURL:f.origin,ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});contexts.push(context);
  await context.addCookies([{name:'__Host-weaveos_session',value:other?f.otherSession:f.session,url:f.origin,httpOnly:true,secure:true,sameSite:'Lax'},{name:'__Host-weaveos_csrf',value:other?f.otherCsrf:f.csrf,url:f.origin,httpOnly:false,secure:true,sameSite:'Lax'}]);return context;
 }
 try{
  const own=await context(),page=await own.newPage(),saved='browser corrected '+info.project.name;
  await page.goto('/app/approvals');await page.getByRole('button',{name:'打开待办：Formal runtime approval',exact:true}).click();
  const taskPath='/app/approvals/'+[f.appId,f.viewId,f.recordId,f.instanceId,f.taskId].join('/');await expect(page).toHaveURL(f.origin+taskPath);
  await expect(page.getByLabel('Reason',{exact:true})).toHaveValue('original');await page.getByLabel('Reason',{exact:true}).fill(saved);await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'修改已保存'})).toBeVisible();await expect(page.getByText('最新 v2',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'同意',exact:true}).click();
  await expect.poll(async()=>{if(await page.getByRole('status').filter({hasText:'审批已同意'}).isVisible())return true;const query=page.getByRole('button',{name:'核查审批结果',exact:true});if(await query.isVisible()&&await query.isEnabled())await query.click();return page.getByRole('status').filter({hasText:'审批已同意'}).isVisible();},{timeout:15000}).toBe(true);
  await page.goto('/app/approvals/completed');await page.getByRole('button',{name:'查看已审批：Formal runtime approval',exact:true}).click();await expect(page.getByRole('heading',{name:'历史审批依据',exact:true})).toBeVisible();await expect(page.getByText(saved,{exact:true})).toBeVisible();await page.reload();await expect(page.getByText(saved,{exact:true})).toBeVisible();await expect(page.getByText(/审批时记录 v2/)).toBeVisible();
  const eventId=new URL(page.url()).pathname.split('/').at(-1)!;await info.attach('real-approved-evidence-'+info.project.name,{body:await page.screenshot(),contentType:'image/png'});
  const outsider=await context(true),other=await outsider.newPage();await other.goto(taskPath);await expect(other.getByRole('alert')).toBeVisible();await expect(other.getByText(saved,{exact:true})).toHaveCount(0);await other.goto('/app/approvals');await expect(other.getByText('暂无待办',{exact:true})).toBeVisible();
  const denied=await outsider.request.get(`/api/v1/applications/${f.appId}/forms/${f.viewId}/records/${f.recordId}/workflow-events/${eventId}`,{headers:{'X-Expected-Actor-Id':f.outsiderId}});expect([403,404]).toContain(denied.status());expect(await denied.text()).not.toContain(saved);
 }finally{for(const context of contexts)await context.close();}
});
