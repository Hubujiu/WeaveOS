import {test,expect} from '@playwright/test';

test('Q36 supported browser uses React native shared ViewTransition for open and close',async({page})=>{
  await page.addInitScript(()=>{
    const native = document.startViewTransition.bind(document);
    const state={count:0};
    Object.defineProperty(window,'__q36NativeViewTransitions',{value:state});
    document.startViewTransition = ((...args:Parameters<Document['startViewTransition']>)=>{
      state.count++;
      return native(...args);
    }) as Document['startViewTransition'];
  });
  await page.goto('/src/q36-front-fixture.html');
  await page.getByRole('button',{name:/自定义筛选/}).click();
  await expect(page.getByRole('dialog',{name:'自定义筛选'})).toBeVisible();
  await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36NativeViewTransitions:{count:number}}).__q36NativeViewTransitions.count)).toBeGreaterThanOrEqual(1);
  await page.getByRole('dialog',{name:'自定义筛选'}).getByRole('button',{name:'关闭筛选'}).click();
  await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();
  await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36NativeViewTransitions:{count:number}}).__q36NativeViewTransitions.count)).toBeGreaterThanOrEqual(2);
});
