# V030-017 阶段一：动态记录筛选合同

来源：[V017 PRD §2](https://app.notion.com/p/3ee2f5a9e64881cca5e6eb314c6ec899)、[V017 ADR §2](https://app.notion.com/p/3ee2f5a9e6488179816fc7f286376d5e)、[V015 ADR §8.4/8.6/11.2/14](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14)。独立样例使用超过 JS 安全整数精度的金额字符串、嵌套 AND/OR、失权字段、NULL／false／日期；不从实现推导期望。

环境：2026-10-03 UTC；任务工作树 `task/V030-017-records-frontend`，React 19 / Playwright 1.63 / Chromium。依赖以冻结锁文件安装，无新 package。

RED 源码提交：`bd67c18e2da4fca7669ba5a59fc1a234a4a505b3`。测试 SHA256 `c1df79f0ccb48866811aea97673b8b2332ba6cc3dc6c6e410e757d8b39f69947`；最小无行为声明 SHA256 `70f7b8f41b9eecd16b0a87dcc14348780e1ae0be632c8da8c6bc25206d57b656`。

执行命令：`XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache pnpm exec playwright test table-preset-generic.component.spec.ts -c apps/web/playwright.component.config.ts --project chromium --reporter=line`。RED exit 1，3 个测试均到达目标断言并失败：结果 `filter` 为 undefined；无效字段／值 `issues.length` 为 0；NULL／false／日期的合法 AST 未返回。非加载、语法或网络故障。

GREEN 同命令 exit 0，3 passed (1.7s)；`pnpm typecheck` exit 0。该用例是隔离合同测试，不证明 V013 HTTP、数据库权限或真实 UI。
