# V010-008 真实执行证据

2026-09-26 13:18 +08:00：Go1.25.7 Docker，PostgreSQL18.0，隔离weaveos_ops_test/独立weaveos_ci_archive_test。命令：`go test -count=1 -v ./internal/audit`；退出1，TestColdArchiveSchema：Q9冷库表缺失；TestRuntimeRolesRestrictAuditAndBootstrap：受限应用角色缺失。两项到达目标断言。永久测试/无行为占位快照：schema-red-test.go、archive-before.sql、roles-before.sql。此前Goose需auto toolchain及测试源相对路径错误均环境/测试加载失败，不计产品RED。

来源：Q9及审计对象/12字段、DDL角色权限；ADR004仅已授权本地模拟适用。正式页面状态未改。本轮GREEN与后续操作尚未执行；不能以本页文字代替实际结果。

第一组schema/roles GREEN：真实相同隔离PG环境 `go test -count=1 -v ./internal/audit` exit0，2/2。测试只在表不存在时应用初始DDL，随后每次重新校验真实目录，避免直接重复已发布CREATE；迁移账本重跑仍须运行装配证明。

第二组 RED：无行为Maintain/Reader声明可加载，`go test -count=1 -v ./internal/audit` exit1，五项目标失败：历史月份未移出、冷库冲突未拒绝、已提交副本重试未删除热副本、闰年年度到期未删除、有效Bootstrap无法读取热事件；另 `go test -count=1 -v ./internal/audit -run TestAuditReadDenies` exit1，普通/禁用/旧版本/匿名四次均未拒绝。schema/roles同时仍GREEN。源码保存在maintenance-red-test.go/read-red-test.go/maintenance-before.go/read-before.go，工作分支786d4d7先于实现。

第二组 GREEN：`go test -race -count=1 -v ./internal/audit` exit0，8/8；真实PG18 + Redis8.2.1 DB14，隔离数据库同上。维护使用UTC calendar interval、100行批次、受控advisory lock、先确认冷库commit并核对完整事件再删除热库；普通查询仅热库且复核当前用户。当前只证明模块真实用例，CLI/自动调度/受限身份实际执行/备份恢复/制品与用户签署仍未完成。

自动维护/冷角色：`go test -count=1 -v ./internal/audit -run 'TestScheduled|TestColdMaintenance'` exit1：无调度执行，cold SELECT/INSERT/DELETE缺失；先e3c4c5d快照，再实现，每小时维护立即执行+周期执行，测试注入10ms与固定领域时钟，不等待年度/月份；race10/10 exit0。

受限身份：真实SET ROLE auth_maintenance后Maintain SQLSTATE42501行锁权限缺失，`go test -count=1 -v ./internal/audit -run TestControlled` exit1，45cf50c保留测试与原roles。最小grant UPDATE(id)仅受控维护者满足PG FOR UPDATE列权限，不扩权auth_app；同用例race exit0，auth_reader真实受限读取同时通过。fresh测试显式装配角色，不依赖另一个测试先执行。

加密：backup-crypto三用例分别从Node/OpenSSL独立加/解密、nonce独立性、错钥/篡改/截断拒绝确定预期；初始声明3RED exit1→AES256-GCM/32随机byte key/12byte nonce/16byte tag完整认证后输出3GREEN。主来源[Node Crypto](https://nodejs.org/api/crypto.html)，仅用标准库。

配置：config.test.mjs独立运行exit1三项RED，缺独立archive测试环境、runtime Redis及自动维护进程；f60222f永久原配置。最小CI/runner装配独立cold、runtime配置后的config+crypto6/6 exit0。首次复合Shell末尾docker输出exit0不是RED退出码，随后明确独立Node exit1已观察。

真实备份：PostgreSQL18容器内pg_dump custom，独立空库pg_restore，synthetic状态/哈希/邀请消费字段作为领域样例。`WEAVEOS_BACKUP_TEST_CONTAINER=weaveos-v010-test-postgres node --test infra/runtime/backup.test.mjs` 初始3RED exit1（无备份文件、缺源未报错、错钥未拒绝）→3GREEN exit0；backup恢复密文仅在内存解密、完成认证后再连接目标，single-transaction/no-owner/no-privileges，拒绝有表目标。备份和key保留本机私有.work，不上传；这不是生产/异机恢复能力证明。参考[PostgreSQL18 pg_dump](https://www.postgresql.org/docs/18/app-pgdump.html)。

CLI：两个最小main无行为，真实编译执行后缺可信配置退出0；`node --test infra/runtime/cli.test.mjs` 2RED exit1→最小配置拒绝/私有stdin/安全日志2GREEN。此前mount的dst=/repo:ro导致Docker125，初始测试未识别该环境失败，明确不计GREEN或RED；已修正readonly选项并要求程序实际exit0/1，再观察真正RED保存快照。正常Bootstrap/维护CLI运行仍待完整runtime演练。

全量Go：首次用DB14启动全suite，auth既有测试要求DB15，exit1是环境隔离校验，未改守卫；改用DB15后 `go test -race -p1 -count=1 ./... && go vet ./...` 全部exit0。2026-09-26完整本机acceptance已实际启动；尚未取得结果，不能记PASS。

完整acceptance随后实际exit0，76/23/25/30/3各阶段通过，Go race/vet/build及类型/构建通过；运行期间infra后续改动，因此不宣称此本机结果验证单一不可变最终HEAD。PR11远程7d04fa3全部CI/product成功。

完整运行恢复RED：实际6项运行操作5通过，备份恢复失败；私有诊断确认pg_restore序列setval(0)越界，owner原始账本序列值2。独立regression `WEAVEOS_BACKUP_TEST_CONTAINER=weaveos-v010-test-postgres node --test infra/runtime/backup.test.mjs` exit1，3 GREEN/1 RED（受控角色序列恢复失败）；初次fixture缺auth表属加载故障不计RED。永久sequence-backup-red-test.mjs、roles-before-sequence.sql、cold-roles-before-sequence.sql、full-restore-red-test.mjs保存于实现前。依据[PG18序列视图权限](https://www.postgresql.org/docs/18/view-pg-sequences.html)：缺SELECT/USAGE时last_value为NULL。最小修复待执行。
2026-09-27 模块扫描预期审查：ADR003/004要求记录和处理已知漏洞，并未规定无关源码的零告警。最初模块级零告警预期错误地包含GO-2026-5932；官方 https://pkg.go.dev/vuln/GO-2026-5932 明确仅OpenPGP包不安全且无修复，不能通过替换Argon2来制造无关范围变更。保存最初测试后改为：所有可修复模块告警清零；唯一GO-2026-5932须以全应用go list -deps证明其所有包均未链接。修订后的测试仍实际RED，20个可修复公告未清除，exit1；之后才允许升级x/crypto。先前module模式带pattern、模块根无Go文件是扫描加载错误，不计RED。
本机新制品8a37688运行：6操作、1真实monitor、2TLS、4备份、3制品完整性/搬迁通过；之后25 API全部因TLS trust失败，浏览器未执行。续期测试还原文件后异步reload未确认原证书已加载，属测试环境清理失败，不计产品RED/通过。修复清理必须真实用原CA握手确认后才能启动后续API，不能关闭证书验证。原测试tls-before-cleanup.mjs保留。
`n2026-09-27：image-scan 两个测试在无行为声明下实际exit1（缺完整报告/实际扫描记录），42a42b0保存测试与占位；实现保留全部有/无修复漏洞，实际2/2 exit0。df15837镜像BFF227项（4 critical/52 high）、web333项（1 critical/66 high）；这不是零漏洞或用户接受。`n环境清理复核：dns-before-cleanup在故障后主入口ready503、live200，下一采样ready200，BFF重新建立数据依赖；原恢复固定IP还导致停止重启地址被维护容器占用。改回动态分配并等实际ready，DNS1/monitor1连续exit0。原测试已保留。CI36255794796模块/CLI加载失败，本机Linux Git所有权fixture复现VCS128；GOFLAGS=-buildvcs=false后CLI2+security4 exit0，不改Git safe.directory或业务预期。
