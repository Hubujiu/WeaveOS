# B3 · 独立 Flowable 本地事务 proof

## Scope

Owner: B3 云执行线程 01a0fcbd-d8d3-72a2-b4c2-d50df7930030。
分支 task/V030-004-flowable-tx-proof；工作树 /workspace/WeaveOS-worktrees/V030-004。
允许路径：prototypes/flowable-local-tx/**、docs/evidence/V030-004/**。
仅同库事务原型和测试。没有产品服务、HTTP/gRPC/API、凭据管理、应用库投影或生产集成。
本包不定义产品触发器、权限、可见性屏障或完整分布式协议，不宣称跨库 ACID。

## Sources

- [PRD](https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa)：2026-10-02 实际读取，更新时间 13:14:33.192 UTC，待评审、未整体冻结；文末 B3 有限授权。
- [ADR009](https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f)：实际读取，更新时间 13:14:39.459 UTC，整页拟议，文末 B3 有限授权。D03 的同 DataSource/transactionManager REQUIRED 仅作为本包试验证明目标。
- root 本线程消息明确解除文档门禁，要求开始 RED/GREEN、包外集成文件不写。
- 已读仓库 AGENTS、HANDOFF、workflow、testing、任务索引、Notion 路由和 tests/AGENTS。/workspace/.agents 为空，没有 .agents/skills。
- Flowable 官方 8.0.0/source 0779d68e5a3385b74d8acb8bc37901ff54513249，已读取 flowable-dependencies、flowable-spring、SpringProcessEngineConfiguration 源码/POM；发布日期版本组合不是生产冻结。
- 远端 main 6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5；PR21 OPEN，head 74cc5824ee4336dd76c72874f0cc4cf38fe9e889，五项检查 SUCCESS。未修改 PR21。

## Acceptance

- [ ] 在真实独立 PostgreSQL 测试库证明底层 DataSource/transactionManager 相同、REQUIRED。
- [ ] ledger、complete、outbox 在各提交前故障点与外层事务回滚时全部回滚。
- [ ] 重复与并发相同命令只推进一次；同 ID 不同完整夹具 payload 明确冲突。
- [ ] 引擎提交后响应丢失，重启引擎/Boot context 后回放原持久回执。
- [ ] 受控人工任务、固定布尔纯条件、结束两路径；没有外部副作用。
- [ ] 实际 RED/GREEN、永久源码快照、命令及结果、精确依赖和未跑项齐全。

## Progress

有效 RED 已于 2026-10-02T13:35:41Z 完成：run-proof.sh verify 退出1，15项、14目标断言失败、0 error、0 skip；同底层 DataSource/transactionManager 与 REQUIRED 夹具核查通过。尚未实现 GREEN。
永久证据见 docs/evidence/V030-004/red.log、red-junit.xml、red-source.tar.gz 与 SHA256。
测试数据库：专用无发布端口 Docker 网络，PG18.6，随机 b3_* schema；Flowable 仅在该随机测试 schema 建引擎表。测试账户仅为隔离夹具常量，未创建正式凭据或连接已有数据库。
构建：独立 Maven POM，Boot 4.0.2 parent、Flowable spring 8.0.0、release17；无 web starter。
Boot BOM 声明 Framework7.0.3、JDBC42.7.9、JUnit6.0.2、Jackson2.20.2/3.0.4；实际解析依赖树待运行。
构建镜像 Maven3.9.11/Temurin17.0.17+10，amd64 manifest sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b。
公共 Maven Central HTTP429；仅本包 settings 使用公共 Google Central mirror，未改 root 配置。

## Handoff

root 集成需求：正式任务编号/任务索引、Java CI、最终服务版本/资源/安全与 API/proto 均由 root 冻结。现有 task-policy 只识别 V010-NNN；本包不扩写该共享配置或抢占编号。
本包记录保留在本 TASK.md 与 docs/evidence/V030-004，由 root 统一纳入正式任务治理；不冒充现有 check-tasks 已覆盖 v0.3.0。
下一条：run-proof.sh verify，必须到达目标行为断言后才记 RED；依赖/编译错误不是 RED。
