# V010-008 真实执行证据

2026-09-26 13:18 +08:00：Go1.25.7 Docker，PostgreSQL18.0，隔离weaveos_ops_test/独立weaveos_ci_archive_test。命令：`go test -count=1 -v ./internal/audit`；退出1，TestColdArchiveSchema：Q9冷库表缺失；TestRuntimeRolesRestrictAuditAndBootstrap：受限应用角色缺失。两项到达目标断言。永久测试/无行为占位快照：schema-red-test.go、archive-before.sql、roles-before.sql。此前Goose需auto toolchain及测试源相对路径错误均环境/测试加载失败，不计产品RED。

来源：Q9及审计对象/12字段、DDL角色权限；ADR004仅已授权本地模拟适用。正式页面状态未改。本轮GREEN与后续操作尚未执行；不能以本页文字代替实际结果。
