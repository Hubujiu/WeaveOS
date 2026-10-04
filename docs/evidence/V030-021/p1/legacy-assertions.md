# Legacy visual assertions for Root review

These tests are unchanged. New V030-021 geometry supersedes the older Shell oracle; only Root may update tests.


## personnel.component.spec.ts

```ts
105: // Fixed 176px sidebar on every viewport; no collapse entry or hidden navigation.
109:  await expect.soft(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
110:  await expect.soft(page.locator('.admin-header')).toHaveCSS('height','56px');
114:  expect.soft({x:menu.x,y:menu.y,width:menu.width,height:menu.height}).toEqual({x:16,y:116,width:144,height:48});
122:   await page.setViewportSize({width,height:844});await expect.soft(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
132:  await expect.soft(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
267: test('Q31 Home navigation stays 56px with no collapse control and retains account actions',async({page})=>{
269:  await expect.soft(page.locator('.home-header')).toHaveCSS('height','56px');
272:  await expect.soft(page.getByRole('button',{name:/收起顶栏|展开顶栏/})).toHaveCount(0);await expect(page.locator('.account-menu')).toBeVisible();await expect(page.locator('.home-header')).toHaveCSS('height','56px');
275: test('Q33 admin uses fixed 56px top and 176px sidebar geometry across resize',async({page})=>{
281:   await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');await expect(page.locator('.admin-header')).toHaveCSS('height','56px');
283:   const menu=(await page.locator('.personnel-nav').boundingBox())!;expect({x:menu.x,y:menu.y,width:menu.width,height:menu.height}).toEqual({x:16,y:116,width:144,height:48});
315:  return page.locator('.admin-material').evaluate(async(node)=>{
327:  await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
330:  await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
346:   for(const width of [390,1000,320,1920]){await page.setViewportSize({width,height:844});await expect(page.locator('.personnel-nav')).toHaveCSS('width','144px');await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');expect((await page.locator('.personnel-nav-icon').boundingBox())!.y).toBe(130);await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('保留输入');}
354:  await expect(page.locator('.home-header')).toHaveCSS('height','56px');
368:   await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');await expect(page.locator('.admin-header')).toHaveCSS('height','56px');
410:  await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width:390,height:844});await admin(page);await expect(page.locator('.admin-sidebar')).toHaveCSS('transition-duration','0s');await page.getByRole('tab',{name:'身份',exact:true}).focus();await page.keyboard.press('Enter');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
568:   await page.setViewportSize(viewport);const material=page.locator('.admin-material');await expect.poll(async()=> (await materialAlpha(page)).every(a=>a>=190&&a<=200)).toBe(true);
570:   await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');await expect(page.locator('.admin-header')).toHaveCSS('height','56px');
590:  await admin(page);await expect(page.locator('.admin-header')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');await expect(page.locator('.admin-sidebar')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
601:   await page.setViewportSize({width,height:1080});await expect(page.locator('.admin-material')).toHaveCSS('left','0px');await expect(page.locator('.admin-material')).toHaveCSS('top','0px');
603:   await expect(page.locator('.admin-header')).toHaveCSS('height','56px');
613:  await page.setViewportSize({width:320,height:844});await admin(page);const menu=page.getByRole('button',{name:'人员管理',exact:true});await expect(menu).toHaveCSS('width','144px');const box=(await menu.boundingBox())!;expect({width:box.width,height:box.height,y:box.y}).toEqual({width:144,height:48,y:116});
616:  await page.setViewportSize({width:1920,height:1080});await admin(page);await page.setViewportSize({width:390,height:844});await page.reload();await expect(page.locator('.admin-material')).toHaveCSS('top','0px');await expect(page.locator('.admin-sidebar')).toHaveCSS('width','176px');
```


## applications.component.spec.ts

```ts
38:   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
53:   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
76:   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
91:   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
732:  await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
```

## personnel-arca-keyboard.component.spec.ts

Line 58 expects the outer page heading color rgb(16,32,68), while frozen P1 uses rgb(37,37,37). The original Table color and smoothing assertions before it passed. Root review required; test unchanged.
