# Q36 查询与草稿契约交接（阶段 A）

批准依据：[V010-020-PLAN](../docs/tasks/V010-020-PLAN.md)。OpenAPI 3.2.1 是 HTTP 契约；[前端 DTO](../apps/web/src/query-contracts.ts) 与 [Go DTO](../services/bff/internal/personnel/query_contracts.go) 仅声明，不安装新业务行为。既有 `PersonnelTable` 尚未移除，现有 HTTP handler 尚未实现本页新增字段。

## 前端可直接依赖的边界

| 操作 | 请求变化 | 成功响应变化 |
| --- | --- | --- |
| `POST /personnel/members/search` | JSON body含page/pageSize/search/快捷条件、可选 `filter` 对象和 `queryVersion`；继续查询/跳页必须带上下文；不接受排序字段 | 原 `items/total/page/pageSize` + `queryVersion`、`sort:null` |
| `POST /personnel/events/search` | 同上；body仅成对 `sortBy:"occurredAt"`、`sortDirection:"asc"或"desc"`；时间范围仍为from/to | 原分页 + `queryVersion`、`sort`、绝对半开时间 `range`；每条记录增加后端 `display` |
| 成员身份 PUT / 分组 POST | 保留原字段和 `version`，必填 `queryVersion`，可选 `draftRef` | 保持原对象响应 |
| 部门 POST / PUT | 保留原字段，必填 `queryVersion`，可选 `draftRef` | 保持原对象响应 |
| 部门 DELETE | 保留 `version` 参数，增加必填 `queryVersion` 参数 | 原 204 |
| 身份/模板 POST / PUT | 保留原对象版本工作流，增加可选 `draftRef` | 原对象响应；卡片不强制变表格 |
| 部门/身份/模板/权限辅助 GET | 可选成员 `queryVersion`，精确核验原查询；无关选项变化不直接判过期 | 保持原响应 |
| 邀请 POST | 可选成员 `queryVersion`；不支持草稿 | 仍只显示一次邀请码，非表格调用保持支持 |

表内操作先保留旧 `queryVersion` 校验旧结果；修改筛选/排序时在此基础上建立新上下文，不能把新条件对比旧指纹。显式刷新从第 1 页不带旧上下文发起，按 PLAN 保留界面条件/列状态、清选择。记录的“最近 7 天”在本轮查询固定；分页沿用已生效 range，显式刷新再解析相对范围。已有合法页的空结果行为保持；不会偷偷跳到其他页。

`filter` 是 POST JSON body 中的对象，不经过 URL 百分号编码。两个 POST 都保留 Session/CSRF 与来源检查；只有有限 Redis 上下文更新，不产生正式业务修改、审计或数据 revision。整个原始 body 上限 65536 字节，filter规范化后仍最多16384字节；普通Unicode/date转义JSON在两项限额内正常接收。例：

```json
{"operator":"and","children":[{"field":"identityIds","operator":"neq","value":"00000000-0000-4000-8000-000000000001"},{"operator":"or","children":[{"field":"status","operator":"eq","value":"active"},{"field":"account","operator":"eq","value":"Alice"}]}]}
```

根组为第 1 层，最多 3 层、全树 20 叶、规范 UTF-8 JSON 16384 字节，无条件不传 filter。OpenAPI 展开三层结构；`x-max-leaves` 和 `x-max-canonical-bytes` 需要业务校验器执行。关系 `neq` 为 NOT EXISTS；普通单值 neq 不匹配 NULL；显式 null 只允许 eq/neq。日期值为 `{date,timeZone}`，按实际当地日边界计算；绝对时刻按 PostgreSQL 微秒精度，拒绝有损的更细输入。文本不参与大小排序。

旧 GET /personnel/members、/personnel/events 仅兼容原简单参数，标记 deprecated，复用同一核心授权和查询校验；不新增复杂filter或自定义排序。旧 GET 没有上下文时是独立当前查询，不声称验证历史基线；有queryVersion时必须验证。全部受保护业务写仍必填并验证queryVersion，新UI仅用POST，不混用旧入口。

## 草稿接口

共同前缀 `/api/v1/personnel/drafts`：GET 列表（仅摘要，最多 20），POST 新建（201 + Location）；`/{draftId}` 支持 GET、PUT 更新、DELETE（204）。PUT body 必须含草稿 `version`；DELETE query 必须含 `version`。所有操作实时校验 Session 和人员管理权限，写操作带 CSRF。

新建请求 `kind/targetId/baseVersion/payload` 全部必填。新部门/身份/模板的 targetId/baseVersion 为 null；已有成员 baseVersion 最小 0，已有部门/定义最小 1。服务器从 Session 取 owner，不接收 ownerId。更新只接收 `version/payload`，不能改 kind、targetId 或最初 baseVersion。摘要含 `id/kind/targetId/baseVersion/version/createdAt/updatedAt`，详情另含 payload；没有 expiresAt。

| kind | payload 的允许字段（全部显式提交） |
| --- | --- |
| member-identities | identityIds |
| member-groups | operation、departmentId、sourceDepartmentId（未选 ID 可为 null） |
| department | name、parentId（未选可为 null） |
| identity | name、description、templateIds、permissionCodes |
| template | name、description、permissionCodes |

允许未完成输入，但不允许任意字段或凭据。草稿路由整个原始body限131072字节，且规范payload同时限65536字节，两者取交集；正常JSON.stringify UTF-8合法payload加wrapper应通过，过度Unicode转义或空白使raw超限必须拒绝，即使规范payload未超限。不改全局限制。payload 最大 65536 个规范 UTF-8 JSON 字节；每账号最多 20 份，超限报错、不淘汰、无自动过期。恢复读最新业务对象展示冲突，不能自行替换原 baseVersion。成功业务提交可带 `draftRef:{id,version}`，同事务只清理确切提交的版本，更晚保存的版本保留。

| 错误 | HTTP | 行为 |
| --- | --- | --- |
| COMMON_QUERY_CHANGED | 409 | 相关完整结果有差异，中止当前操作，要求显式刷新 |
| COMMON_QUERY_CONTEXT_EXPIRED | 409 | 上下文缺失/到期/LRU 淘汰；重新查询，不声称数据变化 |
| COMMON_SERVICE_UNAVAILABLE + meta.reason=QUERY_BUSY | 503 | 锁外预校验重试仍繁忙，未执行领域写；不是变化错误 |
| PERSONNEL_CONFLICT | 409 | 保留既有对象版本/引用冲突语义 |
| PERSONNEL_DRAFT_CONFLICT | 409 | 草稿版本冲突，不覆盖另一标签页 |
| PERSONNEL_DRAFT_LIMIT_REACHED | 409 | 已有 20 份，不替用户删除 |
| COMMON_INVALID_ARGUMENT | 400 | 无效协议输入或草稿超出 64 KiB |

## 数据结构与下一阶段责任

新迁移 `00003_query_drafts.sql` 创建三个 `query_revisions` 行和 `drafts`。revision 为内部 bigint，不向浏览器输出；草稿业务/草稿版本限定 JS 安全整数。每账号 slot 1–20 + UNIQUE(owner,slot) 实施并发容量硬上限；服务端负责分配空 slot、唯一冲突重试与明确容量错误。

草稿 owner 有 auth.users 外键；target 没有外键，防止目标删除带走草稿。payload 保存规范 JSON text，数据库约束对象形状和字节数，服务端仍必须验证字段允许清单、规范编码、实时授权和 CAS。不得把 JSONB 格式化后的空格字节当作用户 payload 大小。草稿不会触发业务 revision。仅 Up/Down 已在一次性隔离 PG 验证；Down 删除草稿，不是线上回退授权。

下一阶段必须完成，不能由阶段 A 绿灯代替：

- 查询解析、全结果规范投影/流式 SHA256、同一个短 RR 事务、Redis session 归属/30 分钟空闲 TTL/20 context LRU/CAS，以及业务错误映射。
- 全部实际写路径的固定 revision 锁顺序、锁后鉴权/版本复验、同事务 revision 更新；真实无变化不递增。新增函数/触发器使用后续新 migration，不能改写 00003。
- 草稿路由、所有权/权限、quota 分配、版本 CAS、恢复差异与精确版本清理。列表按 updated_at DESC,id DESC；不读取其他账号草稿。
- 最小运行角色授权与 migration compatibility 审核登记由唯一整合者接续。阶段 A 未改 `infra/runtime/roles.sql`，未给应用任意 revision 写权限，未扩大原始认证审计读取。
- 统一原版 Table、查询状态/迟到响应处理、筛选/草稿/动效界面、真实 PG/Redis 全链路与截图录屏。禁止创建另一适配层。

契约新增必填字段是发布前协调变更，旧浏览器必须刷新。现有 handler 不满足新响应契约，草稿路由也尚不存在；不要把这个中间提交单独部署或宣称全栈验收通过。

## 阶段 A 实际证据

[RED/GREEN 证据](../docs/evidence/V010-020/q36-contracts/)保存了先失败的测试源码、日志和哈希。契约结构/有限响应 oracle 测试不能证明业务权限；真实 PG 约束测试不能证明完整并发工作流。验证明细见同目录 green-manifest.json；全栈、Redis、截图和最终 PR head CI 留待后续实现与根验收。

POST修订已由用户 messageSentinel_7282e2dbf5d881919b08d2691246ad57 批准，见PLAN §9。本轮只修改契约/类型与有限响应测试oracle；真实Nginx大中文POST到BFF测试尚未运行，固定镜像拉取仍受Docker Hub限流，HTTP接线未实现。此前GET复杂filter设计已被本修订替代。

根已明确选择草稿128KiB raw与64KiB canonical独立双上限，替代前次1MiB raw说明；58,355B规范payload但338,420B raw的转义样例应被拒绝。历史q36-post/q36-types证据保留为当时观测，当前以此说明和OpenAPI为准。
