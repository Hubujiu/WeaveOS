# Figma 错误状态 RED → GREEN

来源为 Q12 后实际读取的 Figma Login 13:2 高保真错误状态：标题“邮箱或密码不正确”，辅助文案“请检查您的密码后重新输入。为方便您，邮箱地址已保留。”，已输入账号保留，账号输入背景 #f4f7fd。

先提交测试 `7a98078`，永久源码 credential-display-red.spec.ts / login-before-credential-display.tsx。原始命令 `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'wrong credentials'` 退出 1，实际“账号或密码错误”不匹配源文案，有效 RED。随后只调整登录 401 的展示及该状态背景，完整 Chromium 组件 suite 14/14 GREEN。网络/503 错误文案保持区别；不是将所有失败都当错误凭据。

全栈及人工视觉签署仍未完成。
