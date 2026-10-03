# Actual source updated 2026-10-03T02:21:59.281Z

### B5a.1 Capability、身份与目录登记
- 正式代码固定为 `applications.create`，中文“应用管理”，category＝system、app_id＝NULL。沿现有 `personnel.manage` 身份／权限模板规则分配，不改成 root-only；Bootstrap 全权
- 新 app ID 为服务端生成的 canonical UUID，固定 owner＝可信 actor，不接受客户端 owner；create-only 不获得其他应用全权或全局 capability 委托能力
- 每 app catalog code 固定为 `app.<uuid>.access`，app_id 为 canonical UUID 字符串，name 来自 app。此项只是登记，不加中央身份／template 二次 grant 门槛
### B5a.2 持久 schema 与本轮可授予资源
持久 schema 固定为 `applications`，包含 `apps`、`permission_groups`、`group_members`、`grants`、`grant_fields`、`menu_resources`、`operations`。apps 固定 `owner_user_id`，独立 `policy_revision` 初值为 1；相关关系使用同 app 复合外键／唯一约束，稳定 UUID 不随展示名变化。
每个 app 自动登记一个真实菜单资源：resourceKind＝application、resourceId＝app ID，代表该应用工作区入口；不是新建一个 Figma 页面，也不自动授权子菜单。form／view 菜单须未来真实元数据登记后才能 grant。本轮拒绝不存在／伪 app 资源，不接受任意 URL 或客户端自称资源存在。
本轮 HTTP 持久 grant 仅支持 `menu.enter`＋all＋空 fields。`data.read`／`data.edit` 核心保留，但尚无字段 registry，HTTP 写入这些 data grant 明确不支持；不得把本轮接线验收为完整数据权限矩阵。
### B5a.3 受限 catalog 登记函数与锁顺序
函数固定为 `applications.register_catalog_entry(appUUID)`：
- 使用 SECURITY DEFINER，search_path＝pg_catalog；所有 SQL 标识符 schema 限定，REVOKE PUBLIC，`auth_app` 仅取得 EXECUTE，没有任意 catalog CRUD
- 函数只从已存在的目标 app 行生成固定 code／app_id／name，不接受 code、category、owner 等自由参数；不能修改 system 项或其他 app 的登记
- 冲突仅在相同登记时幂等，其余拒绝
- caller 先遵 PR21 锁顺序：全局 query revision locks → auth source／账号授权依赖 → app policy 锁，再登记 catalog；函数不引入反序
- roles 代码中的授权仅作隔离验证，不实际应用到生产
### B5a.4 固定 HTTP 路由与配置权限
- POST／GET `/api/v1/applications`
- GET `/api/v1/applications/{appId}`
- GET `/api/v1/applications/{appId}/access`
- GET／POST `/api/v1/applications/{appId}/permission-groups`
- PUT `/api/v1/applications/{appId}/permission-groups/{groupId}`，仅 name／enabled 配置
- GET／PUT `/api/v1/applications/{appId}/permission-groups/{groupId}/members`
- GET／PUT `/api/v1/applications/{appId}/permission-groups/{groupId}/grants`
- GET `/api/v1/application-operations/{operationId}`
全写入执行强 Session、账号活跃／auth_version、Origin／CSRF 与 strict JSON 检查；route 检查真实 app 归属。所有组配置只允许 Bootstrap 或固定 owner，不实现委托、owner transfer、group delete 或候选成员搜索。
members／grants PUT 完整替换：缺少列表是错误，显式 empty 列表清空。成员 UUID 必须真实存在；登记 inactive 账号不使其恢复 access，操作者仍需可信活跃状态。
### B5a.5 固定 DTO 与输入限制
所有 write DTO 都带 `operationId`，为 client UUID，兼作幂等 key，不等同于 requestId；组配置修改另带 `expectedPolicyRevision`：
- POST app：\{name, operationId\}
- POST group：\{name, operationId, expectedPolicyRevision\}
- PUT group：\{name, enabled, operationId, expectedPolicyRevision\}
- PUT members：\{memberIds, operationId, expectedPolicyRevision\}
- PUT grants：\{grants, operationId, expectedPolicyRevision\}
name 先 TrimSpace，长度 1–100 Unicode codepoints，遵现有 varchar 风格；不强制显示名唯一，不允许 client owner。请求大小、重复 key 与严格解码沿用既有 personnel 限制，不新放宽。
### B5a.6 全写入 operation 幂等与未知结果
- actor UUID＋operationId 唯一；指纹为 operationKind、实际 route 参数及规范化 DTO 的 SHA-256，禁止同 key 跨方法／资源复用
- 通过唯一行并发裁决，operation result、配置与审计同事务。同 ID／同 payload 稳定重放，不重复 revision 或 audit；不同 payload 返回 409
- 新写入重新鉴权；重放只限可信活跃的同一 actor 且结果可读。持久化最小业务结果、HTTP status 和 Location，不存 Cookie／Session 或整套响应 headers
- operations 首版不后台删除；记录增长是当前限制，retention 后续另案
- commit unknown 返回 503／`APPLICATION_OPERATION_UNCONFIRMED`，明确提示结果未确认；GET 原 operation 核查。未找到不能作为回滚证明，重试保持原 key 才能安全，不生成新 key 盲重放
- 只有确认 COMMIT 后才发送成功；Session 续期失败不能伪造为业务已回滚
### B5a.7 固定错误与成功结果
- `APPLICATION_NOT_FOUND`：404
- `APPLICATION_FORBIDDEN`：403
- `APPLICATION_POLICY_CONFLICT`：409
- `APPLICATION_OPERATION_CONFLICT`：409
- `APPLICATION_OPERATION_UNCONFIRMED`：503
- `APPLICATION_RESOURCE_INVALID`：400
其余 415、CSRF、401 等沿公共契约，不重新定义。创建返回 201＋Location；修改返回 200、policyRevision 与必要结果。operation 查询严格 actor 隔离，不返回他人结果。
每个成功 policy 变更只加一次 revision；完整替换即使内容相同也按成功操作更新一次，operation 重放不得再增第二次。
### B5a.8 审计与冷热迁移精确契约
沿 `auth.authentication_events` 现有 15 列：
- event_type＝`application_changed`
- object_type＝`application` 或 `permission_group`，object_id 为目标 UUID
- reason_code 仅 `APPLICATION_CREATED`、`GROUP_CREATED`、`GROUP_UPDATED`、`GROUP_MEMBERS_REPLACED`、`GROUP_GRANTS_REPLACED`
- change_summary 白名单为 \{appId, operationId, beforePolicyRevision, afterPolicyRevision, changeCounts\}，只存最小摘要；不写凭据、Session 或 full member DTO，actor／request 沿用既有列规范
审计失败使同事务配置和 operation result 全部回滚。冷热 constraints 精确支持新 event／object 类型且保留旧类型，不新增列；冷 `00003_apps_audit.sql` 先于热 `00006_apps_policy.sql`。archive 的复制／校验／commit 后删 hot 与失败保留算法不变，旧 roles／兼容记录按 B5.3 保留。
### B5a.9 B0 checker 的最小兼容修复
已发现 B0 checker 不兼容 PR21 既有 `docs/tasks/V010-020-PLAN.md`。本轮允许 B5 专属修改 `scripts/check-tasks`、对应测试与 task scope：
- 仅对已核实的 legacy 辅助 PLAN 文件准确处理，保留 canonical task metadata／scope 判定
- 仍拒绝畸形／伪任务 ID，不忽略全部带连字符的辅助文件，也不放宽 cleanup
- 不重命名或删除 PR21 原文件来掩盖不兼容，保留原任务和证据
### B5a.10 首段状态与继续执行
已完成有限的 AccessForWrite 首段：local SHA a142d65f964458f02f89872504de8b8dc527c481；集成 baseline 08e8a97abc222ab3fd7fd99461d972b613950fa2，3 次普通 merge 无冲突。执行回报 453 个 BFF 测试事件、182 项治理、vet／build 通过；这是首段结果，完整应用 SQL／HTTP 尚未实现，不是 B5 完成。
本节是主负责人已冻结的技术契约。回读后执行者继续本地与隔离实现，无需再等待上述 B5.4／B5.6／B5.8 技术选项；本轮 scope 外业务仍不得自选。新 PR／push、生产 DDL／授权、merge main／deploy 继续禁止；整页 PRD 待评审、ADR-009 拟议中，未用户验收。
