# V010-002 契约 RED / GREEN 证据

2026-09-25 Asia/Shanghai，在独立 `task/V010-002-contracts` 工作树，Node v22.23.1。

| 阶段 | 源码状态 | 命令与退出码 | 可观察结果 |
| --- | --- | --- | --- |
| 首轮 RED | `red/openapi.contract.test.mjs`，SHA256 `DB83748A3CDDE9191661DA6F9A5ED41DFF0900B9956DE6D9A0FFFD467FCAD776`；提交 ba728ec | `node --test contracts/openapi.contract.test.mjs` → 1 | 6/6 目标断言失败；OpenAPI 文档、路径、Cookie 安全、响应状态、Envelope、错误映射缺失；运行器正常加载。 |
| 首轮 GREEN | 提交 f9d84be | 同命令 → 0 | 原六组 6/6 通过。 |
| 第二轮 RED | `red2/openapi.contract.test.mjs`，SHA256 `96BD6FE58D195E4DFF14C6D08CE0BB144349CC7659F9AF1E66F66F052D91A758`；提交 7570218 | 同命令 → 1 | 6/8 通过，账号保留大小写的机器可读规则及 `X-Request-Id` 响应头声明两组目标断言失败。 |
| 第二轮 GREEN | 当前契约实现，提交待记录 | 同命令 → 0 | 8/8 通过；正式工具与工程检查见任务记录。 |

来源：2026-09-25 通过授权 Notion 连接器直接获取项目、PRD、ADR-001/002 和相关设计正文。PRD 最新编辑 `2026-09-24T12:15:05.462Z`，ADR-001 `2026-09-24T09:52:34.410Z`、ADR-002 `2026-09-24T10:26:13.400Z`；两条 ADR 状态已接受，PRD 需求编写中。用户本轮批准数据设计并修订账号大小写/空格规则；Notion 草案状态未改。

测试源码副本保留在同目录，可在 squash 删除任务提交后恢复测试预期。此证据只证明契约编写顺序与所列运行结果，不能证明未实施的认证业务或真实存储通过。

形式工具：固定 `@redocly/cli@2.54.2` 对 3.2.1 文档 lint、bundle、build-docs、generate-client 均退出 0，生成 TypeScript 使用 5.9.3 编译通过；lint 有一项缺 license 元数据警告。浏览器分别打开 Redoc 2 静态页（可见操作但 React 控制台 14 错）和本地 `swagger-ui-dist@5.32.2`（七个操作、0 错 0 警）。生成客户端提示浏览器不能手写 Cookie header，未接入前端。未运行真实请求/响应校验，原因是产品 API 尚未实现；V010-007 保留该门禁。文档新增/排版为 TDD:N/A，已用链接与结构检查核对，不当作行为 GREEN。
