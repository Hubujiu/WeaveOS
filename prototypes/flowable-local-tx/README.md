# B3 Flowable 本地事务 proof

本包完成 PRD/ADR009 文末 B3 有限 PLAN 的受控 Java 原型。Spring Boot 以 WebApplicationType.NONE 创建测试 context，内嵌 Flowable，真实 PostgreSQL 只在专用 Docker 内网测试库运行。所有表定义及引擎建表配置只在 src/test 中；没有生产启动入口或 HTTP/gRPC 服务。

`LocalCommandExecutor` 在同一个底层 DataSource/transactionManager 的 REQUIRED 事务内依次登记 command ledger、执行 task complete、写 result outbox。提交前任一点抛错全部回滚；同 ID 的并发请求由 PostgreSQL 唯一键串行裁决，已提交的完整相同夹具命令重放持久结果。完整命令内容包含 taskId、固定布尔条件、opaque payload；这不是产品 JSON 规范化、哈希或 wire 契约。

受控 BPMN：start → 人工 approval → 固定布尔纯条件 → 人工 review／end。发布夹具后绑定精确 processDefinitionId。固定条件表达式不是用户 UEL 扩展，夹具不包含脚本、listener、timer、async 或外部副作用。该集合只界定实验，不削减尚未冻结的产品触发器范围。

执行（在选定云环境，需要 Docker、Python3、bash；工具仅放本包忽略的 .work）：

```sh
prototypes/flowable-local-tx/prepare-build.sh
prototypes/flowable-local-tx/run-proof.sh verify
```

prepare-build 从固定 Maven/Temurin 镜像提取测试工具，沿用云环境已有代理与只读信任库获取公共依赖；私有代理 settings 在退出时删除，未改系统或产品安全配置。run-proof 使用离线缓存、专用 internal Docker 网络、无发布端口、内存临时 PG 数据目录；退出时移除本次容器及网络。只有测试夹具常量账户，没有正式凭据。首次缓存下载失败属于环境问题，不是 RED。

实际验证组合：

| 项 | 实际版本 |
| --- | --- |
| Java 编译／运行 | Temurin 17.0.17+10，release17，class major61 |
| Maven / Compiler / Surefire | 3.9.11 / 3.14.1 / 3.5.4 |
| Flowable spring / engine | 8.0.0 |
| Spring Boot / Framework | 4.0.2 / 7.0.3 |
| PostgreSQL / JDBC | 18.6 / 42.7.9 |
| JUnit Jupiter / Platform launcher | 6.0.2 / 6.0.2 |
| Jackson 3 / MyBatis | 3.0.4 / 3.5.19 |
| SLF4J / Logback | 2.0.17 / 1.5.25 |

Flowable [固定源码 dependencies POM](https://github.com/flowable/flowable-engine/blob/0779d68e5a3385b74d8acb8bc37901ff54513249/modules/flowable-dependencies/pom.xml) 声明 JDBC42.7.8、JUnit6.0.1、Jackson3.0.1；本包 Boot4.0.2 BOM 分别解析为42.7.9、6.0.2、3.0.4。Boot BOM 的 Jackson2.20.2 没有进入本包实际测试 classpath。生产版本／资源预算仍须 root 冻结。

有效 RED：15项、14目标失败、0 error/skip；同底层连接与 REQUIRED 检查通过。GREEN：同一测试源码15/15通过，退出0，测试9.366秒、verify11.917秒。并发使用两条真实数据库事务；响应丢失发生在本地 commit 之后，重启 engine/Boot context 再读同一 PostgreSQL 的 ledger/outbox，重放原回执且不推进新任务。没有真实 OS kill 或跨库消息投递的验证。

永久 RED 源码快照、哈希、原始日志、JUnit报告、实际依赖树及92个测试 classpath jar哈希见 [证据](../../docs/evidence/V030-004/README.md)。本包隔离测试 jar 总大小26,160,520字节，包含测试依赖；这不是服务 RSS 或产品资源预算。RED测试与占位提交 f5ad180b869f9f7cf0b1a3bee7c8ad5e6d609823；原始日志提交4d4c06899c6ca7899dc44a7f35d104416b632fcb。

复杂度估计（未经负载压测）：命令唯一索引登记／读取约 O(log C)，完整 payload 比较 O(P)，ledger/outbox 持久空间 O(C×P)；引擎路径成本随节点／数据库写入增长，本夹具路径有固定少量节点。相同 ID 的竞争会等待唯一键所属事务，当前没有产品级超时、保留、清理或容量策略。测试时长不能当吞吐／延迟 SLA。

待 root 集成／NOT RUN：本轮已消费 B0 的 V010/V030编号兼容补丁并建立正式 [任务记录](../../docs/tasks/V030-004.md)；Java CI仍需root统一接入；OpenAPI、proto、迁移编号、Go/pnpm配置均未改。产品权限／身份、协议字段、可见性屏障、应用 fence/inbox/投影/audit、取消 tombstone、乱序回执、两库恢复、完整14故障场景、OS进程崩溃及负载／资源／漏洞扫描均未验证。Go/web产品回归未跑（相关代码未改）。本地 receipt 仅证明引擎库结果，不代表产品 SUCCESS、跨库 ACID 或完整后端验收。
