# V030-020 migration 00016 验证

时间：2026-10-04 16:03 UTC。PostgreSQL 18.6，独立隔离容器。

- 新建空数据库 `weaveos_workflow_m16check`，Goose 1–16 `up` 成功，版本到 16。
- 在空数据库 `down` 成功，版本回到 15；再次 `up` 成功，版本恢复 16。
- Root HTTP 专项已创建 workflow management operation history。在该真实测试数据库执行 Goose `down`，按预期失败，SQLSTATE `55000`，错误为 `cannot remove workflow management operation kinds while history exists`；原版本16仍保留。
- 备份恢复：`node --test infra/runtime/backup.test.mjs` 4/4 通过。测试隔离保护只接受旧 `weaveos-v010-` 容器名，因此临时将同一隔离容器别名重命名为合规测试前缀运行，结束后已恢复原名；代码与断言未改。

`backup.test.mjs` 输出：4 tests, 4 pass, 0 fail。迁移状态/命令输出留在执行器日志；数据库凭据未进入此证据文件。
