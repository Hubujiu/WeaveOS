import {expect,test} from '@playwright/test';

const url='/src/applications/forms/renderer-harness.html';

test('deleted reference remains visible as a disabled tombstone without enumeration',async({page})=>{
  await page.goto(url);
  await page.getByRole('button',{name:'切换只读'}).click();
  await expect(page.getByText('已离职成员')).toBeVisible();
  await expect(page.getByText('已删除')).toBeVisible();
  await expect(page.getByRole('button',{name:'选择成员'})).toHaveCount(0);
  expect(await page.evaluate(()=>(window as Window&{__referenceRequests?:unknown[]}).__referenceRequests?.length)).toBe(0);
});

test('editable reference loads distinct cursor pages and only selects active candidates',async({page})=>{
  await page.goto(url);
  await expect(page.getByText('已离职成员')).toBeVisible();
  await page.getByRole('button',{name:'选择成员'}).click();
  await expect(page.getByRole('button',{name:'王甲'})).toBeVisible();
  await page.getByRole('button',{name:'加载更多'}).click();
  await expect(page.getByRole('button',{name:'王乙'})).toBeVisible();
  await page.getByRole('button',{name:'王乙'}).click();
  await expect(page.getByTestId('selected')).toHaveText('member-b');
  expect(await page.evaluate(()=>(window as Window&{__referenceRequests?:{pageToken:string|null}[]}).__referenceRequests?.map(item=>item.pageToken))).toEqual([null,'next-page']);
});

test('reference search is scoped to its query and does not reuse a prior cursor',async({page})=>{
  await page.goto(url);
  await page.getByRole('button',{name:'选择成员'}).click();
  await expect(page.getByRole('button',{name:'王甲'})).toBeVisible();
  await page.getByRole('searchbox',{name:'搜索成员'}).fill('  林  ');
  await expect(page.getByRole('button',{name:'林海'})).toBeVisible();
  await expect(page.getByRole('button',{name:'王甲'})).toHaveCount(0);
  const requests=await page.evaluate(()=>(window as Window&{__referenceRequests?:{q:string;pageToken:string|null}[]}).__referenceRequests);
  expect(requests?.at(-1)).toEqual({q:'林',pageToken:null});
});

test('record leave scopes retain another actual resource after one unregisters',async({page})=>{
  await page.goto(url);
  expect(await page.evaluate(()=>(window as Window&{__scopeIsolation?:()=>boolean}).__scopeIsolation?.())).toBe(true);
});

test('separate unsaved record client draft scopes do not overwrite each other',async({page})=>{
  await page.goto(url);
  expect(await page.evaluate(()=>(window as Window&{__clientDraftIsolation?:()=>boolean}).__clientDraftIsolation?.())).toBe(true);
});

test('nullable boolean keeps unset, false, and true distinct with an explicit clear action',async({page})=>{
  await page.goto(`${url}?mode=boolean`);
  await expect(page.getByTestId('selected')).toHaveText('null');
  await expect(page.getByText('未设置')).toBeVisible();
  await page.getByRole('button',{name:'切换只读'}).click();
  await expect(page.getByText('未设置')).toBeVisible();
  await page.getByRole('button',{name:'切换只读'}).click();
  const checkbox=page.getByRole('checkbox',{name:'是否生效'});
  await checkbox.check();
  await expect(page.getByTestId('selected')).toHaveText('true');
  await expect(page.getByText('是',{exact:true})).toBeVisible();
  await checkbox.uncheck();
  await expect(page.getByTestId('selected')).toHaveText('false');
  await expect(page.getByText('否',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'清空 是否生效'}).click();
  await expect(page.getByTestId('selected')).toHaveText('null');
  await expect(page.getByText('未设置')).toBeVisible();
});

test('read-only reference never shows an unsaved selected label without current display authority',async({page})=>{
  await page.goto(url);
  await page.getByRole('button',{name:'选择成员'}).click();
  await page.getByRole('button',{name:'王甲'}).click();
  await expect(page.getByText('王甲')).toBeVisible();
  await page.getByRole('button',{name:'撤销引用显示'}).click();
  await expect(page.getByText('引用信息不可用（需修复）')).toBeVisible();
  await expect(page.getByText('王甲')).toHaveCount(0);
});

test('closing reference chooser aborts an in-flight later cursor page',async({page})=>{
  await page.goto(url);
  await page.evaluate(()=>(window as Window&{__holdMore?:()=>void}).__holdMore?.());
  await page.getByRole('button',{name:'选择成员'}).click();
  await page.getByRole('button',{name:'加载更多'}).click();
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__moreState?:()=>{started:boolean}}).__moreState?.().started)).toBe(true);
  await page.getByRole('button',{name:'选择成员'}).click();
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__moreState?:()=>{aborted:boolean}}).__moreState?.().aborted)).toBe(true);
  await page.evaluate(()=>(window as Window&{__releaseMore?:()=>void}).__releaseMore?.());
  await page.getByRole('button',{name:'选择成员'}).click();
  await expect(page.getByRole('button',{name:'王乙'})).toHaveCount(0);
});

test('reference scope remount clears local selected label and prior candidate page',async({page})=>{
  await page.goto(url);
  await page.getByRole('button',{name:'选择成员'}).click();
  await page.getByRole('button',{name:'王甲'}).click();
  await expect(page.getByText('王甲')).toBeVisible();
  await page.getByRole('button',{name:'切换引用作用域'}).click();
  await expect(page.getByText('引用信息不可用（需修复）')).toBeVisible();
  await expect(page.getByText('王甲')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'选择成员'})).toHaveAttribute('aria-expanded','false');
});
