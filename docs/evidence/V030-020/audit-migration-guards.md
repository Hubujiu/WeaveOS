# V030-020 审计迁移回退保护

在独立 PostgreSQL 18.6 测试环境中，最新 hot00016/cold00005 都已应用。

- 在 hot 测试数据库插入一个合法、非零actor的 `WORKFLOW_DEFINITION_SAVE` 审计事件后，Goose hot00016 Down 因持久审计历史以 SQLSTATE `55000` 拒绝；约束与操作枚举保留。
- 在独立 cold 测试数据库插入一个合法流程审计事件后，Goose cold00005 Down 以 SQLSTATE `55000` 拒绝；归档历史与约束保留。

初次未插入 actor 的 probe 因既有 `fk_authentication_events_actor` 被拒绝，随后已用独立合成用户完成 hot 验证。该probe未改业务代码或迁移。
- 空cold库 `weaveos_workflow_archive_m16check` 从1–5 up到5，Down回4，再up回5成功；证明无新审计历史时可安全回退/重升。
- 空hot库 `weaveos_workflow_audit_m16check` 从1–16 up，hot00016 Down回15，再up回16成功。
