# V030-017 阶段一：记录安全合同

来源：[V015 ADR §8/11/14](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14)、[V017 PRD §2–3](https://app.notion.com/p/3ee2f5a9e64881cca5e6eb314c6ec899)、[V017 ADR §4–6](https://app.notion.com/p/3ee2f5a9e6488179816fc7f286376d5e)。V014 只读候选 `59e8c1b906b5dfb4e2fb46b828a171667de6f921` 的窄 renderer/leave 接口不是最终 approved 整合 SHA。

**RED 1**：源码 `cdfe345a2ebc372c82f69b85a7bc22f8863afc13`，命令 `pnpm exec playwright test records.component.spec.ts -c apps/web/playwright.component.config.ts --project chromium --reporter=line`，exit 1，四个目标失败：最小 record/draft 回执未接受；runtime 安全投影为空；history read/history 交集为假；允许 delta 的标签未返回。占位代码成功加载，断言实际执行。

**GREEN 1**：同命令 4 passed，`pnpm typecheck` exit 0。中间一次实现拼写导致解析失败，修复后重跑；解析失败不算 RED 或 GREEN。DTO 守卫仅在 V012 请求层确认 HTTP 状态/envelope 后检查 `data`；严格拒绝附加业务值。

**RED 2**：源码 `8725cb926a17d95d3fd0e31c18f37542fc18f2cd`，新建 record 身份用例到达断言，空 `clientDraftId` 与预期 UUID 不符，exit 1。

**GREEN 2**：同 suite 5 passed，`pnpm typecheck` exit 0。未保存 record 使用每实例新 UUID、actor/app/view/clientDraftId/field 的稳定 renderer key；只有校验已确认最小 MutationResult 后转换为真实 recordId scope。clientDraftId 从不当作服务端 ID。

边界：这里只测消费者合同与纯安全映射；没有 V013 完整 HTTP、V012 actor guard/operation 状态、V014 最终 renderer 或真实 PG/Redis。same-record refresh 的候选缓存失效由 V014 owner 修复；V017 未来挂载 FieldRenderer/ReferenceSelector 必须使用本合同 key。
