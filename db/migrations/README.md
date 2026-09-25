# 数据库迁移

首个 Goose SQL migration 为 `00001_auth.sql`，只建立已确认的 v0.1.0 认证 schema。来源为 2026-09-25 用户修订并批准的 Notion 数据设计、DDL 和逐列字典；账号唯一键大小写敏感，拒绝内部普通空格。

在**新建且隔离**的 PostgreSQL 18 测试数据库运行：

```sh
goose -dir db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up
```

该迁移不含自动破坏性 Down。真实数据回退优先切回兼容制品或编写经评审的前向修复，不运行 `DROP SCHEMA ... CASCADE`。已发布迁移不可改写；尚需 Linux/CI、约束、权限和恢复测试，当前本机一次 GREEN 不代表整版验收。
