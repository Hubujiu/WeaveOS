# 2026-09-28 Figma 来源变化复核

来源：当前 [Login13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2) 与 [Register40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2) 的高保真上下文和截图。两个根节点均为满宽网格，顶栏高72px、左右内边距32px、仅下方22px圆角；顶栏后18px间距，页面底部22px。卡片仍在12列中的第5–8列，宽度预期不变。Figma工具未提供更新时间。PRD仍指定这两个节点为已确认原型。

需求→用例：当前外框高度/内边距对应`auth.component.spec.ts`的login shell和`responsive.component.spec.ts`的两页三种桌面宽度；内容中心由顶栏72+间距18及底边距22独立计算。紧凑屏沿用既有可用性适配。

RED：在修改CSS前保存`figma-0928-style-before.css`、`figma-0928-responsive-red.spec.ts`、`figma-0928-auth-red.spec.ts`。Windows/Node/Playwright Chromium执行`pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'Figma responsive|login shell'`，退出1，7/7失败；首项及其余六项均在目标断言期望72px、实际94px失败。原始输出见`figma-0928-red.txt`。pnpm配置读取有警告但运行器加载并执行到断言，不是RED原因。

GREEN：仅将桌面顶栏改为72px/水平内边距32px，将内容最小高度按72+18+22重新计算。原命令重跑7/7通过、退出0，见`figma-0928-green.txt`。全部Chromium组件35/35通过，见`figma-0928-components.txt`。`pnpm typecheck`与`pnpm build`均退出0。Firefox/WebKit响应式24/24通过，见`figma-0928-cross-green.txt`。没有更改静态资产、React结构或认证行为。

跨浏览器首轮使用旧证据目录内的`cross.config.ts`，因该目录没有ES module包边界，Node在载入配置时以`import.meta`语法错误退出1，见`figma-0928-cross.txt`；没有运行用例，这不属于需求RED。改用`apps/web`内同等的一次性配置后通过全部24项。一次性配置在验证后删除，测试源码和原始记录保留。

最终head远端完整CI/产品验收仍须重新运行并核对。不能借用2026-09-27旧head的CI。
