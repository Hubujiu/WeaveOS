# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: q36-front-motion.component.spec.ts >> Q36 delayed native opening commit cannot undo a later trusted Escape close
- Location: apps\web\src\q36-front-motion.component.spec.ts:56:1

# Error details

```
Error: expect(locator).not.toBeVisible() failed

Locator:  getByRole('dialog', { name: '自定义筛选' })
Expected: not visible
Received: visible
Timeout:  3000ms

Call log:
  - Expect "not toBeVisible" getByRole('dialog', { name: '自定义筛选' }) with timeout 3000ms
  - waiting for getByRole('dialog', { name: '自定义筛选' })
    10 × locator resolved to <div id="_r_0_" role="dialog" tabindex="-1" data-closed="" data-side="bottom" data-align="start" aria-label="自定义筛选" aria-hidden="false" data-ending-style="" data-instant="dismiss" class="q36-filter-popup" data-base-ui-focusable="" aria-labelledby="base-ui-_r_b_" aria-describedby="base-ui-_r_c_">…</div>
       - unexpected value "visible"

```

```yaml
- dialog "自定义筛选":
  - heading "自定义筛选" [level=2]
  - button "关闭筛选"
  - paragraph: 搜索、快捷条件与本筛选共同生效。最多 3 层分组、20 个条件。
  - group "筛选条件"
  - button "重置条件"
  - text: 0 / 20
  - button "应用筛选"
```

# Test source

```ts
  1   | import {test,expect} from '@playwright/test';
  2   |
  3   | test('Q36 supported browser uses React native shared ViewTransition for open and close',async({page})=>{
  4   |   await page.addInitScript(()=>{
  5   |     const native = document.startViewTransition?.bind(document);
  6   |     const state={count:0};
  7   |     Object.defineProperty(window,'__q36NativeViewTransitions',{value:state});
  8   |     if(native)document.startViewTransition = ((...args:Parameters<Document['startViewTransition']>)=>{
  9   |       state.count++;
  10  |       return native(...args);
  11  |     }) as Document['startViewTransition'];
  12  |   });
  13  |   await page.goto('/src/q36-front-fixture.html');
  14  |   test.skip(!await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.'),'native shared transition unavailable or unsafe in this engine');
  15  |   await page.getByRole('button',{name:/自定义筛选/}).click();
  16  |   await expect(page.getByRole('dialog',{name:'自定义筛选'})).toBeVisible();
  17  |   await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36NativeViewTransitions:{count:number}}).__q36NativeViewTransitions.count)).toBeGreaterThanOrEqual(1);
  18  |   await page.getByRole('dialog',{name:'自定义筛选'}).getByRole('button',{name:'关闭筛选'}).click();
  19  |   await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();
  20  |   await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36NativeViewTransitions:{count:number}}).__q36NativeViewTransitions.count)).toBeGreaterThanOrEqual(2);
  21  | });
  22  |
  23  | test('Q36 unsupported native transition uses popup motion and restores trigger focus',async({page})=>{
  24  |   await page.addInitScript(()=>{
  25  |     Object.defineProperty(document,'startViewTransition',{value:undefined,configurable:true});
  26  |     const native=Element.prototype.animate;
  27  |     const state={count:0};
  28  |     Object.defineProperty(window,'__q36FallbackAnimations',{value:state});
  29  |     Element.prototype.animate=function(...args){state.count++;return native.apply(this,args);};
  30  |   });
  31  |   await page.goto('/src/q36-front-fixture.html');
  32  |   const trigger=page.getByRole('button',{name:/自定义筛选/});
  33  |   await trigger.click();
  34  |   const dialog=page.getByRole('dialog',{name:'自定义筛选'});
  35  |   await expect(dialog).toBeVisible();
  36  |   await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36FallbackAnimations:{count:number}}).__q36FallbackAnimations.count)).toBeGreaterThan(0);
  37  |   await page.keyboard.press('Escape');
  38  |   await expect(dialog).not.toBeVisible();
  39  |   await expect(trigger).toBeFocused();
  40  | });
  41  |
  42  | test('Q36 quick native reversal preserves last close intent and trigger focus',async({page})=>{
  43  |   await page.goto('/src/q36-front-fixture.html');
  44  |   test.skip(!await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.'),'native shared transition unavailable or unsafe in this engine');
  45  |   const trigger=page.getByRole('button',{name:/自定义筛选/});
  46  |   await trigger.click();
  47  |   await expect(page.getByRole('dialog',{name:'自定义筛选'})).toBeVisible();
  48  |   await page.keyboard.press('Escape');
  49  |   await trigger.click();
  50  |   await page.keyboard.press('Escape');
  51  |   await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();
  52  |   await expect(trigger).toBeFocused();
  53  |   await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
  54  | });
  55  |
  56  | test('Q36 delayed native opening commit cannot undo a later trusted Escape close',async({page})=>{
  57  |   await page.addInitScript(()=>{
  58  |     const native=document.startViewTransition?.bind(document);
  59  |     let release!:()=>void;
  60  |     const gate=new Promise<void>(resolve=>{release=resolve;});
  61  |     const state={waiting:false,release,escapes:0};
  62  |     Object.defineProperty(window,'__q36DelayedNativeCommit',{value:state});
  63  |     document.addEventListener('keydown',event=>{if(event.isTrusted&&event.key==='Escape')state.escapes++;},true);
  64  |     if(native)document.startViewTransition=((input:Parameters<Document['startViewTransition']>[0])=>{
  65  |       const update=typeof input==='function'?input:input?.update;
  66  |       const delayed=async()=>{state.waiting=true;await gate;await update?.();};
  67  |       return native(typeof input==='function'?delayed:{...input,update:delayed});
  68  |     }) as Document['startViewTransition'];
  69  |   });
  70  |   await page.goto('/src/q36-front-fixture.html');
  71  |   test.skip(!await page.evaluate(()=>typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.'),'native shared transition unavailable or unsafe in this engine');
  72  |   const trigger=page.getByRole('button',{name:/自定义筛选/});
  73  |   await trigger.click();
  74  |   await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36DelayedNativeCommit:{waiting:boolean}}).__q36DelayedNativeCommit.waiting)).toBe(true);
  75  |   await page.keyboard.press('Escape');
  76  |   expect(await page.evaluate(()=>(window as unknown as {__q36DelayedNativeCommit:{escapes:number}}).__q36DelayedNativeCommit.escapes)).toBe(1);
  77  |   await page.evaluate(()=>(window as unknown as {__q36DelayedNativeCommit:{release:()=>void}}).__q36DelayedNativeCommit.release());
> 78  |   await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();
      |                                                             ^ Error: expect(locator).not.toBeVisible() failed
  79  |   await expect(trigger).toBeFocused();
  80  |   await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
  81  |   await expect(trigger).toHaveAttribute('aria-expanded','false');
  82  |   await expect(page.locator('.q36-filter-popup')).toHaveAttribute('aria-hidden','true');
  83  | });
  84  |
  85  | test('Q36 WebKit edits nested groups through the safe animated fallback',async({page,browserName})=>{
  86  |   test.skip(browserName!=='webkit','WebKit crash regression');
  87  |   await page.addInitScript(()=>{
  88  |     const native=document.startViewTransition?.bind(document);
  89  |     const animate=Element.prototype.animate;
  90  |     const state={native:0,fallback:0};
  91  |     Object.defineProperty(window,'__q36WebKitMotion',{value:state});
  92  |     if(native)document.startViewTransition=((...args:Parameters<Document['startViewTransition']>)=>{
  93  |       state.native++;return native(...args);
  94  |     }) as Document['startViewTransition'];
  95  |     Element.prototype.animate=function(...args){state.fallback++;return animate.apply(this,args);};
  96  |   });
  97  |   await page.goto('/src/q36-front-fixture.html');
  98  |   await page.getByRole('button',{name:/自定义筛选/}).click();
  99  |   await expect.poll(()=>page.evaluate(()=>(window as unknown as {__q36WebKitMotion:{fallback:number}}).__q36WebKitMotion.fallback)).toBeGreaterThan(0);
  100 |   await page.locator('.q36-filter-group-actions button').nth(1).click();
  101 |   await expect(page.locator('.q36-filter-group')).toHaveCount(2);
  102 |   expect(await page.evaluate(()=>(window as unknown as {__q36WebKitMotion:{native:number}}).__q36WebKitMotion.native)).toBe(0);
  103 | });
  104 |
```
