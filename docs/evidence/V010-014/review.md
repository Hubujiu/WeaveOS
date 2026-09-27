# V010-014 · Figma 布局更新验证

## 来源与变更

用户2026-09-27更新并指定 [Login13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2)，实际重读 [Register40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2) 同步改变。来源完整性、Notion版本/状态/时间、组件库授权和范围见任务文档。

两页页面外边距由顶22/左右30改为0，仅保留底22；顶栏仅下方圆角；内容区宽度随满宽页面扩大，四列卡片宽度同步增加20px。内容区起点由134变112，剩余高度重新居中。移动端顶栏也贴顶满宽，保留既有70px高度、卡片12px侧距和可滚动访问。未修改React/认证/资产/依赖/部署。

## RED → GREEN

RED提交15317e0先于CSS实现提交并已推送。原始命令：

`pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'responsive|login shell'`

Windows / Playwright1.63.0 / Chromium，退出1，13失败。原始 [red.txt](red.txt) 记录顶栏x30/y22、宽1860而非1920，卡片598.65625而非618.667，圆角及中心偏移。可恢复快照为responsive-red.spec.ts、auth-red.spec.ts、style-before.css；复制回各自原位置即可重放，不把重放时间当原始时间。sha256-lf.json记录LF规范化哈希，避免Git换行转换改变恢复校验。

GREEN：同配置全部组件测试，35/35通过，退出0，见green.txt。`pnpm typecheck`、`pnpm build`通过。`node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`为81/81，退出0，见governance.txt；verify-repo、check-tasks、git diff --check通过。这些结构检查不代替产品验收。

跨浏览器首轮23/24通过，唯一失败：Firefox注册1920的DOMRect高度55.999969482421875，旧精确浮点比较要求56。原始cross-browser-first.txt保留。Figma仍要求56px，没有改需求或CSS；修正错误测量断言为精确computed CSS `height:56px`，同时要求DOMRect在0.0005px内，避免坐标浮点相减引发伪失败。重跑结果见cross-browser.txt。临时cross.config.ts只复用正式组件配置、指定responsive测试和Firefox/WebKit、显式Vite工作目录；没有skip/retry/门槛降低。

## 视觉与资产

已实际打开并核对以下浏览器截图：

- login-1920.png / register-1920.png：1920×1080，顶栏满宽贴顶，卡片居中；同Figma94/18/22布局。
- login-error-1920.png：隔离401 mock显示错误状态；输入仅合成样例，不是真实账号凭据。
- login-390.png / register-390.png：390×844，可见顶栏、字段、按钮和导航，无水平溢出。

沿用已有静态SVG，local-assets.json列非空本地文件，App.tsx既有import和调用槽位未改。asset-geometry.json记录1920/390及错误状态全部img加载成功、自然尺寸及实际几何。DOM图片序号对应：登录0品牌62×46、1光晕1260×1260、2账号24×24、3锁24×24、4眼24×24、5勾22×22、6–8第三方24×24；错误状态在序号2增加26×26错误图标；注册0品牌、1光晕、2账号、3–6锁/眼、7邀请码24×24。与本轮Figma槽位一致，无新增临时Figma URL或替代图标。

已存在的内容区细节差异（如错误后密码保留、第三方按钮间距16px与Figma15px）不属于本轮外框更新，未修改认证或内部表单行为。此次不宣称全页逐像素一致。开发预览favicon.ico 404为既有问题；错误截图的401为有意模拟。CLI多行参数曾解析失败，改用临时代码文件成功执行；不把该工具错误计入产品RED。

## 交付边界

本机视觉/隔离组件验证不冒充真实全栈。最终PR CI（含真实产品验收）需查询最终head。本轮未合并、未部署；服务器仍运行此前版本。开放PR对应分支/工作树保留，临时浏览器/预览/测试产物在核验后停止并清理，永久证据保留。

最终跨浏览器：24/24通过，退出0。永久文本日志仅去除行尾空白，保留失败及执行内容；哈希按整理后的LF文本生成。PR15首轮pr=null身份检查失败在登记真实PR编号后修正，未改变门禁。最终head CI按 https://github.com/Hubujiu/WeaveOS/pull/15/checks 查询。

最终head1602f2c的真实产品验收通过，后续安全单测25/26；唯一失败是Gitleaks将sha256-lf.json第2行的auth-red.spec.ts校验哈希识别成generic-api-key。同版本同digest扫描器在本机git archive上复现退出1，私有去敏报告仅这一命中；重新计算全部4个文件LF哈希逐个相等，确认是本轮生成的证据摘要，不是凭据。现将摘要对象改为明确path/sha256字段数组；不改扫描器/规则/白名单或业务代码。下一条命令：对修正后的HEAD归档运行原Gitleaks检查。
