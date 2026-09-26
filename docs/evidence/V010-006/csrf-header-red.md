# Q7 浏览器 Cookie-to-header RED → GREEN

独立预期来自 2026-09-26 回写/重新读取的 Q7、Redis Session §3 与 ADR-002：前端只读取非 HttpOnly `__Host-csrf`，写请求提交 `X-CSRF-Token`；不读登录 JSON token，不存储会话凭据。

测试提交 `305212f`，永久源码 csrf-header-red.spec.ts / login-before-csrf-header.tsx。初次 Cookie 环境配置因 http URL + Secure 被 Chrome DevTools 拒绝，非 RED；改为 https URL 添加 Secure Cookie 后，Chromium 的 loopback 测试页面能实际读取该 Cookie（前置断言通过）。原始 `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep Q7` 退出 1，DELETE header 实际 undefined、预期合成 Cookie 值，构成有效 RED。

随后 api helper 仅在写请求读取 Cookie 并设置 header，完整15项隔离组件测试 GREEN。该测试使用合成凭据与 mocked API，仅证明浏览器提交行为；真实 HTTPS Host Cookie、三浏览器与服务端 Session 绑定仍须全栈验收。
