# V010-008 真实执行证据

2026-09-26 13:18 +08:00：Go1.25.7 Docker，PostgreSQL18.0，隔离weaveos_ops_test/独立weaveos_ci_archive_test。命令：`go test -count=1 -v ./internal/audit`；退出1，TestColdArchiveSchema：Q9冷库表缺失；TestRuntimeRolesRestrictAuditAndBootstrap：受限应用角色缺失。两项到达目标断言。永久测试/无行为占位快照：schema-red-test.go、archive-before.sql、roles-before.sql。此前Goose需auto toolchain及测试源相对路径错误均环境/测试加载失败，不计产品RED。

来源：Q9及审计对象/12字段、DDL角色权限；ADR004仅已授权本地模拟适用。正式页面状态未改。本轮GREEN与后续操作尚未执行；不能以本页文字代替实际结果。

第一组schema/roles GREEN：真实相同隔离PG环境 `go test -count=1 -v ./internal/audit` exit0，2/2。测试只在表不存在时应用初始DDL，随后每次重新校验真实目录，避免直接重复已发布CREATE；迁移账本重跑仍须运行装配证明。

第二组 RED：无行为Maintain/Reader声明可加载，`go test -count=1 -v ./internal/audit` exit1，五项目标失败：历史月份未移出、冷库冲突未拒绝、已提交副本重试未删除热副本、闰年年度到期未删除、有效Bootstrap无法读取热事件；另 `go test -count=1 -v ./internal/audit -run TestAuditReadDenies` exit1，普通/禁用/旧版本/匿名四次均未拒绝。schema/roles同时仍GREEN。源码保存在maintenance-red-test.go/read-red-test.go/maintenance-before.go/read-before.go，工作分支786d4d7先于实现。

第二组 GREEN：`go test -race -count=1 -v ./internal/audit` exit0，8/8；真实PG18 + Redis8.2.1 DB14，隔离数据库同上。维护使用UTC calendar interval、100行批次、受控advisory lock、先确认冷库commit并核对完整事件再删除热库；普通查询仅热库且复核当前用户。当前只证明模块真实用例，CLI/自动调度/受限身份实际执行/备份恢复/制品与用户签署仍未完成。
