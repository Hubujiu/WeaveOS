# Q12 禁用控件 RED → GREEN

来源：2026-09-26 用户 Q12“当前没实现的可以直接置灰。不允许点即可”；已同步并重新读取 PRD、登录/身份认证功能底账，Figma Login 13:2 的五个根控件灰度、opacity 0.5、无点击交互，并读取高保真代码与截图。

本机 `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep Q12` 退出 1，目标断言 `记住账号` checkbox 不存在，构成有效 RED。测试提交 `b38b9ce`；永久源码 `disabled-controls-red.spec.ts`、旧 App `login-before-disabled.tsx`。

实现原生 disabled checkbox/button，记住账号只是 Figma 的禁用已选外观，无存储行为；忘记密码与三个社交按钮均无处理器。下载 Figma 灰度 check/Google/Microsoft/GitHub SVG，未修改资产。完整组件命令退出 0，14 passed；pnpm typecheck/build 退出 0。

1920×1080 截图 `waveos-login-disabled.png` 使用合成 401 API 边界（非产品 E2E）。实际卡片 x660.5/y236/w599/h720；十个 img 全部 loaded，顺序对应品牌62×46、光晕1260×1260、错误26×26、账号/密码/可见图标24×24、check22×22、三个社交24×24。已与来源截图对照。剩余凭据错误文案/错误账号背景差异待独立复现；完整真实后端与人工视觉签署未完成。
