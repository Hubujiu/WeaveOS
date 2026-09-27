# V010-002 · v0.1.0 实施决定与未决项

核对日期：2026-09-25 Asia/Shanghai。来源：本轮用户答复、[当前 PRD](https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd)、已接受的 [ADR-001](https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93) / [ADR-002](https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a)、待评审的 [数据设计](https://app.notion.com/p/3e52f5a9e64881e2b759d67ef982dd01) / [DDL](https://app.notion.com/p/3e52f5a9e6488155bfcef8a7cfde4550) / [Redis Session](https://app.notion.com/p/3e52f5a9e6488184b345d41309068308)，及仍为拟议中的 [ADR-003](https://app.notion.com/p/3e52f5a9e6488111adbbe968326a23f0) / [ADR-004](https://app.notion.com/p/3e52f5a9e64881acafe1fe79376a9c91)。本页是任务实施记录，不修改 Notion 审批状态。

## 已确认

- 当前交付限于 Web 邀请码注册、账号密码登录、Redis Session、Bootstrap Admin 和最小认证日志；产品 PRD 仍是“需求编写中/未冻结”，不能标记上线。
- 本轮用户批准现有数据、DDL 与 Redis Session 设计建议，但修订账号规则：去首尾普通空格、不转小写，账号中不允许空格；`Alice` 与 `alice` 为不同账号。前端拦截空格，服务端仍独立验证。其余已批准建议包括重置密码后旧 Session 全部失效；审计保留期另定。
- V010-003 实施前须把草案的 `lower(account)` 生成列、唯一约束、`lower(btrim($1))` 登录查找及未知账号审计指纹的 lowercase 输入改成区分大小写的规范化输入，并先为注册唯一性、登录查找、并发和审计去敏写真实失败测试。不要编辑原草案或把它当已部署结构。
- API 采用 `contracts/openapi/openapi.json` 的七个操作、四字段 JSON Envelope、Cookie Session、安全来源校验和真实 HTTP 状态；测试侧 `tests/acceptance/bindings.json` 的五条路由与该契约一致。V010-009 已写测试仍需随真实服务检查 DTO 和全部错误分支，当前 404/fixture 阻塞不能算通过。

## 待决定 / 阻塞的范围

- ADR-003 整体仍标“拟议中”。用户已被询问是否采用 PostgreSQL 18、pgx/sqlc 和 Goose，答复尚未记录；V010-003 在得到决定前不按这些候选技术写迁移或持久化实现。已经合并的底座选型只证明底座存在，不反推这三项获批。
- 最小认证日志的保留期、归档/删除负责人尚未确定。可为注册/登录/退出等事件建立最小字段与追加行为；V010-008 真实数据发布前必须确定保留与清理策略，不能凭空填写一个时长。
- ADR-004 的生产运行方案仍拟议，真实域名、TLS、备份目标、RPO/RTO、告警接收者和安全债务处置没有授权环境与签署。V010-008 不能把开发构建/本机演练当生产验收。
- 账号大小写修订是本轮用户指令，Notion 数据字典仍记草案旧规则。实施与测试以本次明确修订为准，后续同步 Notion 需另获写入授权；审查者应看到这处差异。

本页为来源状态/文档映射，TDD:N/A；不含本地相对链接，任务结构检查记录在任务文档。行为实现均须分别执行 RED→GREEN。
