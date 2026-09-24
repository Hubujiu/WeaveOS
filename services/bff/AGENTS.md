# Go Web BFF

先读根AGENTS、HANDOFF、任务文档、Notion路由与测试规范；认证改动读当前PRD/ADR-001，HTTP加ADR-002，选库/边界加ADR-003，持久化加已评审数据资料。

先写独立需求测试并实际RED，再实现用例。HTTP适配、认证/邀请码行为、存储边界分开；认证模块可访问自己拥有的数据，不跨领域写库，不在Handler堆SQL。不为未来架构造空服务。

只有BFF认证边界处理密码，业务仅消费IdentityContext。Redis服务端Session与安全Cookie不能偷偷切成浏览器JWT；无法确认身份时不放行，外部身份头不可信。

邀请码事务/并发、退出和续期/撤销竞态须真实隔离存储验收，mock不证明事务/TTL；时钟可控，不靠sleep等待一小时。先确认撤销语义，不硬编码未经评审的数据规则。Bootstrap特例不演变成通用权限，日志不泄露凭据。

当前cmd/bff与internal/platform/httpserver是已建立的平台宿主；go.mod已存在。`go test -race ./...`、`go vet ./...`和构建由CI运行。认证、Session、持久化仍由V010-003/004/005实现。默认readiness未装配返回503，不能改成无条件成功。

每任务使用独立branch/worktree，更新同ID任务文档；squash与远程main验收/清理遵循根规则。不要在其他模块任务里抢改共享HTTP装配。
