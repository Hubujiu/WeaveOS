# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: q36-b2.component.spec.ts >> Root Q36 outside tab click after reopening records native event delivery 100ms
- Location: apps/web/src/q36-b2.component.spec.ts:398:40

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: getByLabel('搜索身份', { exact: true })
Expected: visible
Timeout: 3000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" getByLabel('搜索身份', { exact: true }) with timeout 3000ms
  - waiting for getByLabel('搜索身份', { exact: true })

```

```yaml
- complementary "工作区快捷入口":
  - button "返回主页"
  - button "应用中心"
  - button "设置"
  - button "账号": q3
- navigation "应用导航":
  - text: WaveOS
  - paragraph: 我的工作区
  - button "首页"
  - paragraph: 应用
  - button "全部应用"
  - button "人员管理"
- banner:
  - navigation "全局应用标签":
    - button "首页"
    - button "全部应用"
- main "人员管理":
  - heading "人员管理" [level=1]
  - button "草稿箱"
  - button "保存" [disabled]
  - button "退出"
  - tablist "人员管理视图":
    - tab "成员与部门" [selected]
    - tab "身份"
    - tab "权限模板"
    - tab "操作记录"
  - button "刷新查询"
  - tabpanel "成员与部门":
    - heading "成员与部门" [level=2]
    - button "新建部门"
    - button "邀请成员"
    - heading "部门 0" [level=3]
    - button "全部成员 100"
    - button "重命名部门" [disabled]
    - button "删除部门" [disabled]
    - textbox "搜索成员"
    - button "自定义筛选":
      - img
      - text: 自定义筛选
    - text: 100 位成员
    - combobox "筛选身份": 全部身份
    - table "成员":
      - rowgroup:
        - row "选择当前页成员 拖动以调整 account 列顺序 成员 Resize account column 拖动以调整 departments 列顺序 部门 Resize departments column 拖动以调整 identities 列顺序 身份 Resize identities column 拖动以调整 personnelManage 列顺序 人员管理 Resize personnelManage column 拖动以调整 actions 列顺序 操作 Resize actions column":
          - columnheader "选择当前页成员":
            - checkbox "选择当前页成员"
          - columnheader "拖动以调整 account 列顺序 成员 Resize account column":
            - button "拖动以调整 account 列顺序":
              - img
            - text: 成员
            - button "Resize account column"
          - columnheader "拖动以调整 departments 列顺序 部门 Resize departments column":
            - button "拖动以调整 departments 列顺序":
              - img
            - text: 部门
            - button "Resize departments column"
          - columnheader "拖动以调整 identities 列顺序 身份 Resize identities column":
            - button "拖动以调整 identities 列顺序":
              - img
            - text: 身份
            - button "Resize identities column"
          - columnheader "拖动以调整 personnelManage 列顺序 人员管理 Resize personnelManage column":
            - button "拖动以调整 personnelManage 列顺序":
              - img
            - text: 人员管理
            - button "Resize personnelManage column"
          - columnheader "拖动以调整 actions 列顺序 操作 Resize actions column":
            - button "拖动以调整 actions 列顺序":
              - img
            - text: 操作
            - button "Resize actions column"
      - rowgroup:
        - row "1 选择成员：q36-admin q36-admin 正常 未分组 Bootstrap Admin 默认拥有 配置身份 调整分组":
          - cell "1 选择成员：q36-admin":
            - text: "1"
            - checkbox "选择成员：q36-admin"
          - cell "q36-admin 正常":
            - strong: q36-admin
            - text: 正常
          - cell "未分组"
          - cell "Bootstrap Admin"
          - cell "默认拥有"
          - cell "配置身份 调整分组":
            - button "配置身份"
            - button "调整分组"
    - text: 每页条数
    - combobox "每页条数": "20"
    - text: 1 - 20 / 共 100 项
    - navigation "Pagination":
      - button "上一页" [disabled]:
        - img
      - button "Page 1": "1"
      - button "Page 2": "2"
      - button "Page 3": "3"
      - button "Page 4": "4"
      - button "Page 5": "5"
      - button "下一页":
        - img
      - text: 跳至页
      - spinbutton "跳至页": "1"
      - button "跳转到指定页":
        - img
```

# Test source

```ts
  309 |    }
  310 |    return timeout(handler,delay,...args);
  311 |   }) as typeof window.setTimeout;
  312 |   const focus=HTMLElement.prototype.focus;
  313 |   HTMLElement.prototype.focus=function(...args){state.focusCalls.push(this.getAttribute('aria-label')??this.tagName);return focus.apply(this,args);};
  314 |  },mode);
  315 | }
  316 | async function waitHeldExit(page:Page){await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.finished)).toBe(true);}
  317 | async function releaseHeldExit(page:Page){await page.evaluate(async()=>{(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.release();await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));});}
  318 | async function focusFixture(page:Page){
  319 |  await fixture(page);await page.route('**/personnel/members/search',r=>r.fulfill({json:envelope(paging([member],r.request().postDataJSON()))}));await admin(page);
  320 | }
  321 | 
  322 | for(const mode of ['engine','fallback','reduced'] as const)for(const close of ['button','Escape'] as const)test(`Q36 B2 ${mode} ${close} restores focus once after an ordinary close`,async({page})=>{
  323 |  if(mode==='fallback')await page.addInitScript(()=>Object.defineProperty(document,'startViewTransition',{value:undefined,configurable:true}));
  324 |  if(mode==='reduced')await page.emulateMedia({reducedMotion:'reduce'});
  325 |  await page.addInitScript(()=>{const state={calls:0};Object.defineProperty(window,'__q36NormalCloseFocus',{value:state});const focus=HTMLElement.prototype.focus;HTMLElement.prototype.focus=function(...args){if(this.classList.contains('q36-filter-trigger'))state.calls++;return focus.apply(this,args);};});
  326 |  await focusFixture(page);const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  327 |  await trigger.click();await expect(panel).toBeVisible();await expect(panel.getByRole('button',{name:'新增筛选',exact:true})).toBeEnabled();
  328 |  await panel.getByRole('button',{name:'新增筛选',exact:true}).focus();await expect(panel.getByRole('button',{name:'新增筛选',exact:true})).toBeFocused();
  329 |  await page.evaluate(()=>(window as unknown as {__q36NormalCloseFocus:{calls:number}}).__q36NormalCloseFocus.calls=0);
  330 |  if(close==='button')await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();else await page.keyboard.press('Escape');
  331 |  await expect(panel).toBeHidden();await expect(trigger).toBeFocused();
  332 |  await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-preset-transition-active')&&document.getAnimations().every(a=>a.playState!=='running'||!(a.effect instanceof KeyframeEffect)||(!(a.effect.target as Element|null)?.classList?.contains('q36-filter-shell')&&!a.effect.pseudoElement?.startsWith('::view-transition'))));
  333 |  expect(await page.evaluate(()=>(window as unknown as {__q36NormalCloseFocus:{calls:number}}).__q36NormalCloseFocus.calls)).toBe(1);
  334 | });
  335 | 
  336 | for(const mode of ['engine','fallback'] as const)test(`Q36 B2 ${mode} late close honors newer focus even after that target is removed`,async({page})=>{
  337 |  await holdFilterExit(page,mode);await focusFixture(page);const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  338 |  await trigger.click();await expect(panel).toBeVisible();await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await waitHeldExit(page);
  339 |  await page.getByLabel('搜索成员',{exact:true}).focus();await expect(page.getByLabel('搜索成员',{exact:true})).toBeFocused();
  340 |  await page.evaluate(()=>{document.activeElement?.remove();});expect(await page.evaluate(()=>document.activeElement===document.body)).toBe(true);
  341 |  await page.evaluate(()=>(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.focusCalls=[]);
  342 |  await releaseHeldExit(page);await expect(trigger).not.toBeFocused();
  343 |  expect(await page.evaluate(()=>(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.focusCalls)).not.toContain('自定义筛选');
  344 |  await expect(panel).toBeHidden();
  345 | });
  346 | 
  347 | test('Q36 B2 removal-induced body focus still restores after Escape',async({page})=>{
  348 |  await page.addInitScript(()=>Object.defineProperty(document,'startViewTransition',{value:undefined,configurable:true}));await focusFixture(page);
  349 |  const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await trigger.click();await expect(panel).toBeVisible();
  350 |  await panel.getByRole('button',{name:'新增筛选',exact:true}).focus();expect(await page.evaluate(()=>{document.activeElement?.remove();return document.activeElement===document.body;})).toBe(true);
  351 |  await page.keyboard.press('Escape');await expect(panel).toBeHidden();await expect(trigger).toBeFocused();
  352 | });
  353 | 
  354 | for(const mode of ['engine','fallback'] as const)test(`Q36 B2 ${mode} stale exit cannot focus the trigger after reopen or unmount`,async({page})=>{
  355 |  await holdFilterExit(page,mode);await focusFixture(page);const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  356 |  await trigger.click();await expect(panel).toBeVisible();await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await waitHeldExit(page);
  357 |  await trigger.click();await expect(panel).toBeVisible();const first=panel.getByRole('button',{name:'新增筛选',exact:true});await first.focus();await releaseHeldExit(page);await expect(first).toBeFocused();await expect(panel).toBeVisible();
  358 |  await page.getByRole('tab',{name:'身份',exact:true}).click();await expect(page.getByLabel('搜索身份',{exact:true})).toBeVisible();await expect(panel).toBeHidden();
  359 |  const search=page.getByLabel('搜索身份',{exact:true});await search.focus();await expect(search).toBeFocused();await expect(trigger).toHaveCount(0);
  360 | });
  361 | 
  362 | for(const mode of ['engine','fallback'] as const)test(`Q36 B2 ${mode} outside pointer and keyboard navigation keep their new focus`,async({page})=>{
  363 |  if(mode==='fallback')await page.addInitScript(()=>Object.defineProperty(document,'startViewTransition',{value:undefined,configurable:true}));await focusFixture(page);
  364 |  const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true}),search=page.getByRole('button',{name:'刷新查询',exact:true});
  365 |  await trigger.click();await expect(panel).toBeVisible();
  366 |  // The new centered panel covers the table-header checkbox. Use a real outside control.
  367 |  const outsideBox=await search.boundingBox(),panelBox=await panel.boundingBox();expect(outsideBox).not.toBeNull();expect(panelBox).not.toBeNull();
  368 |  const outsideX=outsideBox!.x+outsideBox!.width/2,outsideY=outsideBox!.y+outsideBox!.height/2;
  369 |  expect(outsideX<panelBox!.x||outsideX>panelBox!.x+panelBox!.width||outsideY<panelBox!.y||outsideY>panelBox!.y+panelBox!.height).toBe(true);
  370 |  await search.click();await expect(panel).toBeHidden();await expect(search).toBeFocused();
  371 |  await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-preset-transition-active')&&document.getAnimations().every(a=>a.playState!=='running'||!(a.effect instanceof KeyframeEffect)||(!(a.effect.target as Element|null)?.classList?.contains('q36-filter-shell')&&!a.effect.pseudoElement?.startsWith('::view-transition'))));
  372 |  await expect(search).toBeFocused();await trigger.click();await expect(panel).toBeVisible();await page.keyboard.press('Escape');await expect(trigger).toBeFocused();await page.keyboard.press('Tab');await expect(trigger).not.toBeFocused();
  373 | });
  374 | 
  375 | for(const mode of ['engine','fallback'] as const)test(`Q36 B2 ${mode} completion delivered after unmount cannot move focus`,async({page})=>{
  376 |  await holdFilterExit(page,mode);await focusFixture(page);const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  377 |  await trigger.click();await expect(panel).toBeVisible();await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await waitHeldExit(page);
  378 |  await page.getByRole('tab',{name:'身份',exact:true}).click();const search=page.getByLabel('搜索身份',{exact:true});await expect(search).toBeVisible();await search.focus();await expect(trigger).toHaveCount(0);
  379 |  await page.evaluate(()=>(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.focusCalls=[]);
  380 |  await releaseHeldExit(page);await expect(search).toBeFocused();
  381 |  expect(await page.evaluate(()=>(window as unknown as {__q36HeldFilterExit:ExitFocusState}).__q36HeldFilterExit.focusCalls)).not.toContain('自定义筛选');
  382 |  await page.getByRole('tab',{name:'成员与部门',exact:true}).click();await expect(trigger).toBeVisible();await expect(trigger).toHaveAttribute('aria-expanded','false');
  383 | });
  384 | 
  385 | test.describe('Q36 B2 focus review evidence',()=>{
  386 |  test('manager, editor and closed keyboard focus remain accessible',async({page,browserName},testInfo)=>{
  387 |   await focusFixture(page);const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),manager=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  388 |   async function settle(){await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-preset-transition-active')&&document.getAnimations().every(a=>a.playState!=='running'||!(a.effect instanceof KeyframeEffect)||(!(a.effect.target as Element|null)?.classList?.contains('q36-filter-shell')&&!a.effect.pseudoElement?.startsWith('::view-transition'))));}
  389 |   async function capture(name:string){await settle();const path=testInfo.outputPath(name);await page.screenshot({path});await testInfo.attach(name,{path,contentType:'image/png'});}
  390 |   await trigger.click();await expect(manager).toBeVisible();const first=manager.getByRole('button',{name:'新增筛选',exact:true});await expect(first).toBeEnabled();await first.focus();await expect(first).toBeFocused();await capture(browserName+'-manager-focus.png');
  391 |   await first.click();const editor=page.getByRole('dialog',{name:'新增自定义筛选',exact:true}),name=editor.getByLabel('自定义筛选名称',{exact:true});await expect(name).toBeFocused();await capture(browserName+'-editor-focus.png');
  392 |   await page.keyboard.press('Escape');await expect(editor).toBeHidden();await expect(trigger).toBeFocused();await capture(browserName+'-closed-trigger-focus.png');
  393 |   await trigger.click();await expect(manager).toBeVisible();await page.keyboard.press('Escape');const all=page.getByLabel('选择当前页成员',{exact:true});await all.focus();await settle();await expect(all).toBeFocused();await page.keyboard.press('Space');await expect(all).toBeChecked();await expect(manager).toBeHidden();await capture(browserName+'-checkbox-space-focus.png');
  394 |  });
  395 | });
  396 | 
  397 | // Root diagnostic: observe native event delivery without changing timing or handlers.
  398 | for(const pressDuration of [0,100,300])test(`Root Q36 outside tab click after reopening records native event delivery ${pressDuration}ms`,async({page})=>{
  399 |  await holdFilterExit(page,'engine');await focusFixture(page);
  400 |  await page.evaluate(()=>{
  401 |   const events:unknown[]=[];Object.defineProperty(window,'__q36OutsideClickEvidence',{value:events});
  402 |   const describe=(node:EventTarget|null)=>node instanceof Element?{tag:node.tagName,id:node.id,role:node.getAttribute('role'),label:node.getAttribute('aria-label'),text:node.textContent?.slice(0,45)}:null;
  403 |   for(const type of ['pointerdown','mousedown','pointerup','mouseup','click','focusin'])for(const capture of [true,false])document.addEventListener(type,event=>events.push({type,capture,time:performance.now(),target:describe(event.target),active:describe(document.activeElement),prevented:event.defaultPrevented,rootClass:document.documentElement.className,duration:document.documentElement.style.getPropertyValue('--q36-filter-shell-duration')}),capture);
  404 |  });
  405 |  try{
  406 |   const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});
  407 |   await trigger.click();await expect(panel).toBeVisible();await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await waitHeldExit(page);
  408 |   await trigger.click();await expect(panel).toBeVisible();const first=panel.getByRole('button',{name:'新增筛选',exact:true});await first.focus();await releaseHeldExit(page);await expect(first).toBeFocused();await expect(panel).toBeVisible();
> 409 |   await page.getByRole('tab',{name:'身份',exact:true}).click({delay:pressDuration});await expect(page.getByLabel('搜索身份',{exact:true})).toBeVisible();await expect(panel).toBeHidden();
      |                                                                                                                                      ^ Error: expect(locator).toBeVisible() failed
  410 |  }finally{const path=test.info().outputPath('native-outside-click.json');await writeFile(path,JSON.stringify(await page.evaluate(()=>(window as unknown as {__q36OutsideClickEvidence:unknown[]}).__q36OutsideClickEvidence),null,2));await test.info().attach('native-outside-click.json',{path,contentType:'application/json'});}
  411 | });
  412 | 
```