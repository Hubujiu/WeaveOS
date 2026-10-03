# V030-017 阶段一：共享查询生命周期

来源：[V017 PRD §2–3](https://app.notion.com/p/3ee2f5a9e64881cca5e6eb314c6ec899)、[V017 ADR §2/5](https://app.notion.com/p/3ee2f5a9e6488179816fc7f286376d5e)、[V015 ADR §11.2/14.4](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14)。fixture 只模拟查询传输以验证浏览器状态；不算后端查询证据。

RED 源码提交 `0dbf96a3bd23dab0d7a3c7064db5cce19901c634`，在 `PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers` 的 Chromium 执行：`pnpm exec playwright test table-preset-generic.component.spec.ts -c apps/web/playwright.component.config.ts --project chromium --grep 'shared query lifecycle' --reporter=line`，exit 1。断言到达：首次请求数预期 1，实得 0（公共 hook 无行为占位）。

GREEN 同命令 exit 0，1 passed (2.9s)；`pnpm typecheck` exit 0。旧人员与新组件组合命令 `pnpm exec playwright test filter-manager.component.spec.ts q36-b2.component.spec.ts table-preset-generic.component.spec.ts -c apps/web/playwright.component.config.ts --project chromium --reporter=line`，exit 0，65 passed (3.0m)。保持人员现有 POST wire、range 冻结和旧 token 行为。

旧 context 在条件改变时仍随 page1 请求；显式刷新或 actor/resource scope 更换后无旧 token。真实 RecordSearch 的 HTTP 联调仍待 V013，V012 正在修请求层；此结果仅验证注入传输下的 lifecycle。
