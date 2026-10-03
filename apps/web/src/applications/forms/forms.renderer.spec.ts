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
