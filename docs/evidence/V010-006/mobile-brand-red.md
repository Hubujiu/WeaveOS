# 移动端品牌层级 RED → GREEN

独立预期：WaveOS Figma 13:2 品牌文本颜色 #102044。390×844 页面上的装饰光晕不应改变该颜色。

测试提交 45e6ccc；可恢复测试 mobile-brand-red.spec.ts、旧样式 style-before-mobile-stack.css、失败截图 waveos-login-mobile-before-stack.png。等待字体加载后对品牌文本区域真实截图统计目标颜色像素。原命令 `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'brand text'` 退出1：匹配像素0，预期大于0，实际观察有效RED。

仅给品牌 header 添加 position:relative / z-index:1，使其高于装饰光晕。完整相同配置18项测试退出0，pnpm typecheck、pnpm build退出0。此组件测试不替代真实后端、三浏览器和最终人工视觉验收。
