# 真实存储观察器交接契约（测试侧提案）

`integration.test.mjs` 是测试代码，不是已运行的存储验收。当前没有观察器实现，所有场景运行应明确失败为 BLOCKED，不能skip、用内存fake替换、或声称已RED。

V010-003/004 在评审数据字典、实际ADR及迁移之后，提供本地模块路径 `WEAVEOS_ACCEPTANCE_OBSERVER`，export async `open({baseURL})`。此模块仅在测试进程中装配；不能给生产HTTP新增改时钟、改状态或故障注入后门。

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
| countUsersByExactAccount(account) | 独立只读SQL COUNT，不能调用被测查用户方法 |
| countCredentialsByExactAccount(account) | 独立SQL计数，不借用被测注册逻辑 |
| countInvitationUses(code) | 独立查询该码消费事实；物理摘要映射须按已评审字典实现 |
| readPasswordHash(account) | 返回原始凭据哈希，仅在内存断言，不日志输出 |
| dumpOwnedAuthenticationState(accounts) | 返回这些case拥有的数据的完整字段文本；仅内存去敏检查，不落盘/artifact |
| readLoginEvents({userId,since}) | 原始审计查询，字段映射为userId/accountIdentifier/ip/userAgent/time/result；result映射规则独立核对字典，不由被测函数计算expected |
| readApplicationLogs({since}) | 从隔离进程捕获原始日志，只供内存泄露断言，禁止上传 |
| snapshotIsolatedStorage() | 仅当前隔离实例的可恢复快照，不上传；返回不透明handle |
| restoreThroughRecoveryProcedure(handle) | 运行经评审的恢复/回滚流程（实现机制待确认），不能直接强行删旧Session伪造通过 |

STORE-01/02使用原生PTTL和实测请求耗时校验3600秒，无长sleep。STORE-05证明真实Redis过期拒绝，不声称覆盖精确t=3599999/3600000逻辑分支；确定性时钟边界及退出/续期指定交错仍须004用真实存储+调度屏障测试。HTTP-22只有并发压力，不替代该证明。

未接受的账号规范化、重置后全部会话撤销、物理DDL、哈希参数、Seed覆盖策略、审计保留期不在这里擅自决定。版本化schema全量约束和迁移测试须先完成002/003评审。没有任何这些项目被标为已覆盖。
