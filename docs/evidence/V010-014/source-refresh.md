# 2026-09-28 Figma 来源变化复核

来源：当前 [Login13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2) 与 [Register40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2) 的高保真上下文和截图。两个根节点均为满宽网格，顶栏高72px、左右内边距32px、仅下方22px圆角；顶栏后18px间距，页面底部22px。卡片仍在12列中的第5–8列，宽度预期不变。Figma工具未提供更新时间。PRD仍指定这两个节点为已确认原型。

需求→用例：当前外框高度/内边距对应`auth.component.spec.ts`的login shell和`responsive.component.spec.ts`的两页三种桌面宽度；内容中心由顶栏72+间距18及底边距22独立计算。紧凑屏沿用既有可用性适配。

RED：在修改CSS前保存`figma-0928-style-before.css`、`figma-0928-responsive-red.spec.ts`、`figma-0928-auth-red.spec.ts`。Windows/Node/Playwright Chromium执行`pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'Figma responsive|login shell'`，退出1，7/7失败；首项及其余六项均在目标断言期望72px、实际94px失败。原始输出见`figma-0928-red.txt`。pnpm配置读取有警告但运行器加载并执行到断言，不是RED原因。

GREEN：待实现后填写实际命令和结果。不能借用2026-09-27旧head的CI。
