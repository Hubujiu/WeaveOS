import {test,expect,type BrowserContext} from '@playwright/test';
import {readFileSync} from 'node:fs';
import {validateDefinitionFixture} from './product-browser-contract.mjs';

test('real definition creation, publication, manual start and approval',async({browser},info)=>{
 let f:ReturnType<typeof validateDefinitionFixture>;
 try{f=validateDefinitionFixture(JSON.parse(readFileSync(process.env.WEAVEOS_PRODUCT_BROWSER_FIXTURE!,'utf8')),process.env.WEAVEOS_PRODUCT_HEAD,info.project.name);}catch{throw new Error('Private isolated definition fixture unavailable');}
 const contexts:BrowserContext[]=[];
 async function context(other=false){
  const context=await browser.newContext({baseURL:f.origin,ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});contexts.push(context);
  await context.addCookies([{name:'__Host-session',value:other?f.otherSession:f.session,url:f.origin,httpOnly:true,secure:true,sameSite:'Lax'},{name:'__Host-csrf',value:other?f.otherCsrf:f.csrf,url:f.origin,httpOnly:false,secure:true,sameSite:'Lax'}]);return context;
 }
 try{
  const own=await context(),page=await own.newPage(),name='Browser-created approval',saved='browser corrected '+info.project.name;
  const form='/app/applications/'+f.appId+'/forms/'+f.viewId;
  await page.goto(form+'/workflows');await page.getByRole('button',{name:'新建流程',exact:true}).click();
  const canvas=page.getByRole('region',{name:'流程图',exact:true});await canvas.getByRole('button',{name:'开始',exact:true}).click();await page.getByRole('button',{name:'在此节点后添加审批节点',exact:true}).click();
  const picker=page.getByRole('dialog',{name:'选择审批人',exact:true});await picker.getByRole('checkbox',{name:'root-http-task-'+f.actorId,exact:true}).check();await picker.getByRole('button',{name:'确认',exact:true}).click();
  await canvas.getByRole('button',{name:'审批节点',exact:true}).click();await page.getByLabel('节点名称（最多100字）',{exact:true}).fill('浏览器财务复核😀');await expect(canvas.getByRole('button',{name:'浏览器财务复核😀',exact:true})).toBeVisible();await page.getByLabel('通过规则',{exact:true}).selectOption('all');await page.getByRole('checkbox',{name:'Reason',exact:true}).check();
  await page.getByRole('tab',{name:'触发与设置',exact:true}).click();await page.getByRole('textbox',{name:'流程名称',exact:true}).fill(name);await page.getByRole('checkbox',{name:'手动发起',exact:true}).check();
  await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByRole('heading',{name,exact:true})).toBeVisible();await expect(page.getByText('尚未发布',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'发布',exact:true}).click();await page.getByRole('dialog',{name:'发布流程',exact:true}).getByRole('button',{name:'确认发布',exact:true}).click();
  const published=page.getByRole('dialog',{name:'流程已发布',exact:true});
  await expect.poll(async()=>{if(await published.isVisible())return true;const query=page.getByRole('button',{name:'核查发布结果',exact:true});if(await query.isVisible()&&await query.isEnabled())await query.click();return published.isVisible();},{timeout:15000}).toBe(true);
  await expect(published).toContainText('当前状态：已关闭');await published.getByRole('button',{name:'继续编辑',exact:true}).click();await page.getByRole('tab',{name:'触发与设置',exact:true}).click();await page.getByRole('button',{name:'启用流程',exact:true}).click();await expect(page.getByText('已启用',{exact:true})).toBeVisible();
  await page.goto(form);await page.getByRole('button',{name:'打开记录：Reason original',exact:true}).click();await page.getByRole('button',{name:'发起流程',exact:true}).click();const start=page.getByRole('dialog',{name:'发起流程',exact:true});await start.getByRole('radio',{name,exact:true}).check();await start.getByRole('button',{name:'确认发起',exact:true}).click();await expect(start.getByRole('status')).toContainText('已接受发起请求');
  await page.goto('/app/approvals');const task=page.getByRole('button',{name:'打开待办：'+name,exact:true});
  await expect.poll(async()=>{if(await task.isVisible())return true;const refresh=page.getByRole('button',{name:'刷新待办',exact:true});if(await refresh.isVisible()&&await refresh.isEnabled())await refresh.click();return task.isVisible();},{timeout:15000}).toBe(true);await task.click();await expect(page).toHaveURL(new RegExp('/app/approvals/'+[f.appId,f.viewId,f.recordId].join('/')+'/[0-9a-f-]{36}/[0-9a-f-]{36}$'));const taskPath=new URL(page.url()).pathname;
  await expect(page.getByLabel('Reason',{exact:true})).toHaveValue('original');await page.getByLabel('Reason',{exact:true}).fill(saved);await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'修改已保存'})).toBeVisible();await expect(page.getByText('最新 v2',{exact:true})).toBeVisible();await page.getByRole('button',{name:'同意',exact:true}).click();
  await expect.poll(async()=>{if(await page.getByRole('status').filter({hasText:'审批已同意'}).isVisible())return true;const query=page.getByRole('button',{name:'核查审批结果',exact:true});if(await query.isVisible()&&await query.isEnabled())await query.click();return page.getByRole('status').filter({hasText:'审批已同意'}).isVisible();},{timeout:15000}).toBe(true);
  await page.goto('/app/approvals/completed');await page.getByRole('button',{name:'查看已审批：'+name,exact:true}).click();await expect(page.getByText(saved,{exact:true})).toBeVisible();await page.reload();await expect(page.getByText(saved,{exact:true})).toBeVisible();await expect(page.getByText(/审批时记录 v2/)).toBeVisible();await info.attach('real-created-published-started-approved-'+info.project.name,{body:await page.screenshot(),contentType:'image/png'});
  const other=await(await context(true)).newPage();await other.goto(taskPath);await expect(other.getByRole('alert')).toBeVisible();await expect(other.getByText(saved,{exact:true})).toHaveCount(0);
 }finally{for(const context of contexts)await context.close();}
});
