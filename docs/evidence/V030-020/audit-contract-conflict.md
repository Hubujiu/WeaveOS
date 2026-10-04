# V030-020 审计合同冲突记录

检查时间：2026-10-04 UTC。运行环境为隔离 PostgreSQL 18.6、Redis 8.2.10；迁移 1–16 已应用。具体镜像标签、Go/Goose版本与专项初始 RED 结果见 `root-workflow-http-red-*`。

冻结的 V030-020 ADR 要求 `application_structure_changed` 的 `change_summary` 含 `appId,flowId,operationId,revision,state,action`。已发布 `db/migrations/00007_app_structure.sql`（SHA-256 `080b6daf0aacf0763d54dfadcbcffc2135486835700895b08010c77cb5d12531`）定义 `ck_auth_events_summary`，要求该事件键集合精确为 `appId,operationId,structureVersion,schemaVersion,viewVersion,changeCount`。直接以 ADR 字段插入真实 PostgreSQL 被 CHECK 拒绝，SQLSTATE `23514`。迁移合同同时限定 00016 只扩展 `ck_operation_kind` 并保留其他 CHECK。

将审计降为 00007 的六个既有键后，`TestRootWorkflowHTTP` 12 项全部通过；但此临时写法没有 ADR 要求的 `flowId,revision,state,action`，因此只证明 HTTP 行为通过，不计作合同 GREEN。当前实现不得将此结果表述成审计合同已满足。等待 Root 对迁移 00016 能否扩展 `ck_auth_events_summary`，或正式豁免 ADR 字段要求作出裁定。

被测 Root 测试文件 SHA-256：`ed98bd4398ba20bfd12b7ad0f117995c6220e831e9f3f83ac83dbb53d4e059f3`；未修改。初始真实 RED 已保存在 `root-workflow-http-red.txt` 及其源码 SHA/基线文件中。
