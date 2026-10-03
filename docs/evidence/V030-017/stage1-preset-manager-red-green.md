# V030-017 阶段一：唯一筛选管理器泛化

来源：[V017 PRD §2–3](https://app.notion.com/p/3ee2f5a9e64881cca5e6eb314c6ec899)、[V017 ADR §2–3](https://app.notion.com/p/3ee2f5a9e6488179816fc7f286376d5e)、[V015 ADR §14.4](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14)。隔离 fixture 的私人 repository 只验证组件对注入端口的行为，不冒充 V013 持久化。

执行环境：2026-10-03 UTC；`PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers`；Chromium 与 Vite fixture。第一次尝试设置私有 `XDG_CACHE_HOME` 后缺浏览器二进制，属于环境失败，未计入 RED。固定浏览器路径后重跑取得以下有效结果。

**RED 1**：`793edfe9f1311f45d8cbf079e03ebf34aabfc6bf`，运行 `pnpm exec playwright test table-preset-generic.component.spec.ts -c apps/web/playwright.component.config.ts --project chromium --reporter=line`，exit 1；已有3个绿，新加2个失败：`presetBlocks` 读取 `fieldId` 得空键；resource 方案验证返回空名和 null filter。随后针对同一用例 GREEN：5 passed，`pnpm typecheck` exit 0。

**RED 2**：`d8772fc374d5b18868d243b551e251a6bb0e2abc`，同命令带 `PLAYWRIGHT_BROWSERS_PATH`，exit 1；已有5个绿，新加2个在目标行为失败：管理器调用人员预设而不读注入的私人 repository，应用字段选项不存在。保存测试初版 locator 对“值／值类型”匹配不精确，按真实标签改为 `exact:true`；验证预期不变。

**GREEN**：同命令 exit 0，7 passed (5.1s)。`pnpm typecheck` exit 0。旧 `filter-manager.component.spec.ts`＋`q36-b2.component.spec.ts` 在同一 Chromium fixture 环境下 57 passed (2.8m)，包含 members/events wire、Q36 旧 token、相对日期、失效引用、focus 与 reduced-motion 回归。

边界：没有 V013 应用预设 API／迁移、真实 Session／CSRF／actor、V012 accepted 请求层或 V014 引用候选；不宣称私人方案持久化完成。
