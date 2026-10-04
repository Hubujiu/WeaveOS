# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: applications.form-shell.component.spec.ts >> Root R26 nested long form actions stay inside the tree and remain clickable at 1280
- Location: apps/web/src/applications.form-shell.component.spec.ts:237:2

# Error details

```
Error: Every tree action must fit its panel and receive normal pointer input

Every tree action must fit its panel and receive normal pointer input

expect(received).toBe(expected) // Object.is equality

Expected: true
Received: false

Call Log:
- Timeout 5000ms exceeded while waiting on the predicate
```

# Page snapshot

```yaml
- generic [ref=e4]:
  - banner [ref=e5]:
    - generic [ref=e6]: WaveOS
    - navigation "全局应用标签" [ref=e8]:
      - button "首页" [ref=e9] [cursor=pointer]
      - generic [ref=e10]:
        - button "请假应用" [ref=e11] [cursor=pointer]
        - button "关闭应用：请假应用" [ref=e12] [cursor=pointer]: ×
      - button "全部应用" [ref=e13] [cursor=pointer]
    - generic [ref=e14]:
      - button "设置" [ref=e15] [cursor=pointer]
      - button "账号" [ref=e16] [cursor=pointer]
  - navigation "应用导航" [ref=e17]:
    - generic [ref=e18]:
      - paragraph [ref=e19]: 请假应用
      - button "全部应用" [ref=e20] [cursor=pointer]
      - paragraph [ref=e21]: 应用个人导航
    - button "应用中心" [ref=e23] [cursor=pointer]
  - main "应用工作台" [ref=e24]:
    - generic [ref=e25]:
      - heading "请假应用" [level=1] [ref=e26]
      - button "权限管理" [ref=e27] [cursor=pointer]
    - tablist "应用表单" [ref=e28]:
      - tab "应用目录" [selected] [ref=e29] [cursor=pointer]
    - region "目录与视图管理" [ref=e30]:
      - generic [ref=e31]:
        - generic [ref=e32]:
          - strong [ref=e33]: 应用目录与视图
          - paragraph [ref=e34]: 管理目录、逻辑表和表单视图
        - generic [ref=e35]:
          - button "新建目录" [ref=e36] [cursor=pointer]
          - button "新建表单" [ref=e37] [cursor=pointer]
      - generic [ref=e38]:
        - navigation [ref=e39]:
          - tree "应用目录" [ref=e40]:
            - treeitem "业务目录" [expanded] [ref=e41]:
              - generic [ref=e42]:
                - button "目录 业务目录" [ref=e43] [cursor=pointer]
                - button "移动目录 业务目录" [ref=e44] [cursor=pointer]: 移动
              - group [ref=e45]:
                - treeitem "财务分组" [expanded] [ref=e46]:
                  - generic [ref=e47]:
                    - button "目录 财务分组" [ref=e48] [cursor=pointer]
                    - button "移动目录 财务分组" [ref=e49] [cursor=pointer]: 移动
                  - group [ref=e50]:
                    - treeitem "审批视图 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" [ref=e51]:
                      - generic [ref=e52]:
                        - button "表单 审批视图 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" [ref=e53] [cursor=pointer]
                        - button "打开表单 审批视图 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" [ref=e54] [cursor=pointer]
                        - button "配置表单 审批视图 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" [ref=e55] [cursor=pointer]
        - region "目录与视图详情" [ref=e56]:
          - heading "选择目录或视图" [level=2] [ref=e57]
          - paragraph [ref=e58]: 从左侧选择项目，查看详情并管理位置。
```

# Test source

```ts
  162 |  await guard.getByRole('button', { name: '放弃修改' }).click();
  163 |  release();
  164 |  await finished;
  165 |  await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '$'));
  166 |  await page.waitForLoadState('networkidle');
  167 |  expect(writes).toBe(0);
  168 | });
  169 |
  170 | test('V030-012 Shell re-confirms a changed preflight outcome and retains the original unknown write', async ({ page }) => {
  171 |  await fixture(page);
  172 |  let release!: () => void; const gate = new Promise<void>(resolve => { release = resolve; });
  173 |  let started!: () => void; const entered = new Promise<void>(resolve => { started = resolve; });
  174 |  const writes: Record<string, unknown>[] = [];
  175 |  await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', async route => {
  176 |   started(); await gate; await route.fulfill({ json: ok(preflight) }).catch(() => {});
  177 |  });
  178 |  await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  179 |   if (route.request().method() === 'GET') return route.fulfill({ json: ok(definition) });
  180 |   writes.push(route.request().postDataJSON());
  181 |   return route.abort('failed');
  182 |  });
  183 |  await page.goto('/app/applications/' + appId);
  184 |  await page.getByRole('button', { name: '配置表单 请假申请' }).click();
  185 |  await page.getByRole('button', { name: '文本', exact: true }).click();
  186 |  await page.getByRole('button', { name: '保存', exact: true }).click();
  187 |  await entered;
  188 |  await page.goBack();
  189 |  const guard = page.getByRole('dialog', { name: '有未保存的修改' });
  190 |  await expect(guard).toContainText('尚未发送的预检将取消');
  191 |  release();
  192 |  await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
  193 |  await guard.getByRole('button', { name: '放弃修改' }).click();
  194 |  await expect(guard).toContainText('状态已变化');
  195 |  await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '/forms/' + viewId + '/design$'));
  196 |  await expect(guard).toContainText('已发送的操作会保留原请求');
  197 |  await guard.getByRole('button', { name: '放弃修改' }).click();
  198 |  await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '$'));
  199 |  await page.getByRole('button', { name: '配置表单 请假申请' }).click();
  200 |  await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
  201 |  expect(writes).toHaveLength(1);
  202 |  expect(typeof writes[0].operationId).toBe('string');
  203 | });
  204 |
  205 | test('V030-012 forced form 401 routes to login and restores only the same actor draft', async ({ page }) => {
  206 |  await fixture(page);
  207 |  let writes = 0;
  208 |  await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', route => route.fulfill({ status: 401, json: { code: 'AUTH_UNAUTHENTICATED', data: null } }));
  209 |  await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  210 |   if (route.request().method() === 'PUT') writes++;
  211 |   return route.fallback();
  212 |  });
  213 |  await page.route('**/api/v1/sessions', route => route.fulfill({ status: 200, json: ok(actor) }));
  214 |  await page.goto('/app/applications/' + appId);
  215 |  await page.getByRole('button', { name: '配置表单 请假申请' }).click();
  216 |  await page.getByRole('button', { name: '文本', exact: true }).click();
  217 |  await page.getByLabel('字段名称').fill('复登保留字段');
  218 |  await page.getByRole('button', { name: '保存', exact: true }).click();
  219 |  await expect(page).toHaveURL(/\/login$/);
  220 |  expect(writes).toBe(0);
  221 |  await expect(page.getByRole('heading', { name: '登录', exact: true })).toBeVisible();
  222 |  const account = page.getByRole('textbox', { name: '账号', exact: true });
  223 |  await account.fill(actor.account);
  224 |  const password = page.getByLabel('密码', { exact: true });
  225 |  await password.fill('test-password');
  226 |  await expect(account).toHaveValue(actor.account);
  227 |  await expect(password).toHaveValue('test-password');
  228 |  await page.getByRole('button', { name: '登录' }).click();
  229 |  await expect(page).toHaveURL(/\/app$/);
  230 |  await page.getByRole('button', { name: '打开应用中心' }).click();
  231 |  await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
  232 |  await page.getByRole('button', { name: '配置表单 请假申请' }).click();
  233 |  await expect(page.getByRole('region', { name: '表单画布' })).toContainText('复登保留字段');
  234 | });
  235 |
  236 | for (const width of [1280, 1920]) {
  237 |  test('Root R26 nested long form actions stay inside the tree and remain clickable at ' + width, async ({page}, info) => {
  238 |   await page.setViewportSize({width,height:1080});
  239 |   await fixture(page);
  240 |   const parent='00000000-0000-4000-8000-000000000191', child='00000000-0000-4000-8000-000000000192';
  241 |   const longName='审批视图 '+ 'A'.repeat(90);
  242 |   const nested={...structure,directories:[
  243 |    {id:parent,appId,name:'业务目录',parentId:null,position:0},
  244 |    {id:child,appId,name:'财务分组',parentId:parent,position:0},
  245 |   ],tables:[{...table,name:longName,directoryId:child}],forms:[{...form,name:longName,directoryId:child}]};
  246 |   await page.route('**/api/v1/applications/'+appId+'/structure',route=>route.fulfill({json:ok(nested)}));
  247 |   await page.goto('/app/applications/'+appId);
  248 |   const tree=page.locator('.forms-structure-tree');
  249 |   const row=tree.getByRole('treeitem',{name:longName,exact:true}).locator('.forms-tree-row');
  250 |   await expect(row).toBeVisible();
  251 |   const actions=row.getByRole('button');
  252 |   await expect(actions).toHaveCount(3);
  253 |   for(let i=0;i<3;i++){
  254 |    const action=actions.nth(i);
  255 |    await action.scrollIntoViewIfNeeded();
  256 |    await expect.poll(()=>action.evaluate(node=>{
  257 |     const panel=node.closest('.forms-structure-tree');
  258 |     if(!panel)return false;
  259 |     const a=node.getBoundingClientRect(),p=panel.getBoundingClientRect();
  260 |     const hit=document.elementFromPoint(a.x+a.width/2,a.y+a.height/2);
  261 |     return a.width>=24&&a.height>=24&&a.left>=p.left&&a.right<=p.right&&!!hit&&node.contains(hit);
> 262 |    }),{message:'Every tree action must fit its panel and receive normal pointer input'}).toBe(true);
      |                                                                                          ^ Error: Every tree action must fit its panel and receive normal pointer input
  263 |   }
  264 |   await page.screenshot({path:info.outputPath('nested-form-tree-'+width+'.png'),fullPage:true});
  265 |   await row.getByRole('button',{name:'配置表单 '+longName,exact:true}).click();
  266 |   await expect(page).toHaveURL(new RegExp('/forms/'+viewId+'/design$'));
  267 |   await expect(page.getByRole('region',{name:'表单设计器',exact:true})).toBeVisible();
  268 |  });
  269 | }
  270 | test('Root R26 money configuration exposes a unique rounding control and preserves its selected rule',async({page})=>{
  271 |  await fixture(page);
  272 |  await page.goto('/app/applications/'+appId+'/forms/'+viewId+'/design');
  273 |  await page.getByRole('button',{name:'金额',exact:true}).click();
  274 |  await page.getByLabel('字段名称',{exact:true}).fill('报销金额');
  275 |  await page.getByLabel('总精度',{exact:true}).fill('20');
  276 |  await page.getByLabel('小数位数',{exact:true}).fill('2');
  277 |  await page.getByLabel('处理位数',{exact:true}).fill('2');
  278 |  const rounding=page.getByRole('combobox',{name:/^舍入规则/});
  279 |  await expect(rounding).toHaveCount(1);
  280 |  await rounding.selectOption('HALF_EVEN');await expect(rounding).toHaveValue('HALF_EVEN');
  281 |  await rounding.selectOption('HALF_UP');await expect(rounding).toHaveValue('HALF_UP');
  282 |  await expect(page.getByLabel('总精度',{exact:true})).toHaveValue('20');
  283 |  await expect(page.getByLabel('处理位数',{exact:true})).toHaveValue('2');
  284 | });
  285 |
```