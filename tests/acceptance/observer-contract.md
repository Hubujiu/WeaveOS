# 真实存储观察器契约（V010-009 已装配）

`storage-observer.mjs` 已连接同一隔离 PostgreSQL18 / Redis8.2；`integration.test.mjs` 的19项本机实际通过，纳入 `infra/acceptance/run.mjs`。7项原生控制与1项透明传输屏障资格测试先运行，再串行运行19项产品断言。最终PR以最终head远端CI为准，不能以字符串声明替代真实结果。证据见 `docs/evidence/V010-009/store-validation.md`。

V010-003/004 已评审字典、迁移和ADR为绑定来源；本轮重新读取相关对象/字段。模块路径 `WEAVEOS_ACCEPTANCE_OBSERVER`，export async `open({baseURL})`。模块仅在测试进程中装配，不给生产HTTP新增后门。必须提供 `WEAVEOS_ACCEPTANCE_PROJECT`、私有fixture路径；可选 `WEAVEOS_ACCEPTANCE_COMPOSE` 仅用于本机独立端口的同项目配置。校验Docker项目/服务标签、私有端口、BFF实际数据库/会话代次与HTTP Origin一致。

返回对象必须使用当前BFF连接的同一隔离PostgreSQL/Redis，`storage = 'isolated-postgresql-and-redis'` 只是声明；评审必须读取驱动与真实运行记录，不能把字符串当证明。每个open拥有独立命名空间/可恢复故障，`close()`必须恢复网络/故障并只清理自己的数据。禁止全库FLUSH或连接生产环境。

| 方法 | 原始观察/操作约定 |
| --- | --- |
| sessionPTTL(authCookie) | 读取真实Redis PTTL毫秒值，-2表示key不存在；不得模拟TTL或调用被测续期实现计算结果 |
| setSessionPTTL(authCookie, ms) | 仅改变本case Redis key的native TTL；不伪造返回值 |
| expireSession(authCookie) | 用真实Redis原生过期让key过期并确认，不用内存删除代替 |
| setSessionCreationAge(authCookie, ms) | 仅修改当前case创建时刻；保持认证身份、原生TTL有效，不伪造续期 |
| setUserStatus(id, status) | 通过已评审存储/用例修改独立用户状态；不得改全局user fixture |
| disconnectApplication(redis或postgresql) | 断开应用到测试依赖的真实连接，保留观察连接；返回async恢复函数；HTTP仍须可达 |
| failNextRegistrationBeforeCommit(account) | 测试装配点在写入开始后、提交前注入一次真实事务失败；返回解除函数；禁止改HTTP响应掩盖已提交状态 |
| holdNextRenewal(authCookie) | 返回waitUntilHeld/release异步屏障；只暂停当前case下一次真实读请求的touch提交，其他请求包括退出正常运行；超时失败不能伪装RED |
| countUsersByExactAccount(account) | 独立只读SQL COUNT，不能调用被测查用户方法 |
| countCredentialsByExactAccount(account) | 独立SQL计数，不借用被测注册逻辑 |
| countInvitationUses(code) | 独立查询该码消费事实；物理摘要映射须按已评审字典实现 |
| readPasswordHash(account) | 返回原始凭据哈希，仅在内存断言，不日志输出 |
| dumpOwnedAuthenticationState(accounts) | 返回这些case拥有的数据的完整字段文本；仅内存去敏检查，不落盘/artifact |
| readLoginEvents({userId,since}) | 原始审计查询，字段映射为userId/accountIdentifier/ip/userAgent/time/result；result映射规则独立核对字典，不由被测函数计算expected |
| readApplicationLogs({since}) | 从隔离进程捕获原始日志，只供内存泄露断言，禁止上传 |
| snapshotIsolatedStorage(authCookie) | 全量隔离PG加密备份与本case Redis原生DUMP；私钥/密文留在受限.work、Redis二进制仅内存，不上传；返回本observer专属handle |
| restoreThroughRecoveryProcedure(handle) | 复用recoverRuntime的停服→恢复空库和旧Redis DUMP→切换generation→恢复服务；原生旧key确实存在，不能删旧key伪造通过 |
| recoveryEvidence(handle) | 独立核对新generation、旧key确已恢复、PG账号集合等于备份且备份后新增账号消失 |

STORE-01/02使用原生PTTL和实测请求耗时校验3600秒，无长sleep。STORE-05证明真实Redis过期拒绝，不声称覆盖精确t=3599999/3600000逻辑分支；确定性时钟边界及退出/续期指定交错仍须004用真实存储+调度屏障测试。HTTP-22只有并发压力，不替代该证明。

PG故障仅临时拒绝该BFF容器IP并终止旧连接，保留docker exec本地观察连接；Redis故障断开容器网络并终止旧连接，随后恢复原服务别名。提交故障用限定随机账号的延迟约束触发器在真实COMMIT失败，close解除。续期屏障只缓冲该key的一次EVAL，其他命令和Redis响应透明转发；业务镜像/Session算法无测试开关。

历史“待确认”的账号/DDL/哈希/审计规则已由002–008确认并验收；本模块不重新定义它们。19项证明指定存储行为，不替代完整迁移/权限/保留期或生产部署验收。STORE-16现在通过真实HTTP与Redis传输屏障验证确定性交错；毫秒边界仍由004的独立真实存储测试覆盖。
