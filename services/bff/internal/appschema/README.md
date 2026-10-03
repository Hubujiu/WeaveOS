# B2 隔离 schema plan / Save 包

本包按 PRD／ADR009 文末 B2 有限授权实现纯结构计划和同步 PostgreSQL 事务执行器。当前物理原语为 text／boolean，拒绝未登记类型。完整业务字段字典、API、权限矩阵、Flowable 存储及生产配置由 root 冻结和接入。

## 事务契约

调用者显式提供命名空间、连接池、每条 SQL／每次锁等待的超时，以及四个事务适配边界：

- `SaveGuard.Check`：在同一事务验证真实操作者、表归属、权限和相关保护状态。本包不提供允许全部操作的默认 guard。
- `Metadata.Lock`：返回当前字段和不透明 revision，并保护逻辑表的结构操作及字段依赖登记至事务结束。新表也必须有可锁的身份门禁。`Metadata.Store` 在这个事务保存字段，接入方同时保存布局及审计；其失败回滚全部 DDL。
- `DependencyProtector.Protect`：删除／改型前返回启用流程和在途实例引用，并与依赖登记使用同一保护门禁。无实现或查询失败时拒绝操作。读一次远程 Flowable 状态不足以满足该契约。
- 每次删除命令的 `DeletionConfirmation.Verify`：在已锁住真实表的事务核验用户对当前受影响数据的确认。`ColumnImpact` 包含稳定字段 ID 和非 NULL 行数；接入方必须另外核验真实数据／recordVersion 证明，因为同数量的数据变化不会改变计数。包内测试使用私有夹具查询证明旧确认失效；最终令牌协议尚未冻结。

执行顺序为显式事务超时 → guard → 元数据门禁与 revision 检查 → 编译计划 → 真实表锁 → 依赖／必填旧行／当前删除影响检查 → DDL／补齐 → 元数据、布局与审计提交钩子 → 一次 COMMIT。展示重命名仅更新元数据，计划构建不访问数据库。

内部规范 UUID 映射 `t_`／`f_` 加 32 位十六进制，物理名长度为 34 字节。用户展示名不会参与 SQL。命名空间必须显式传入；未默认选择应用 schema。默认值是类型化常量，文本采用受控转义，补齐值使用参数绑定。本包不接受用户 SQL、任意表达式或脚本。`Backfills` 仅对无默认值的新列生效，不能覆盖既有列；复杂逐行补齐策略仍由 root 约定。

新增列使用 DEFAULT 或 NULL；旧记录上的新必填列必须 DEFAULT 或类型化补齐。补齐／改型／非空验证失败会回滚整个 Save。改型使用 PostgreSQL 原语 cast，完整业务兼容矩阵尚未确定。

提交期间出现错误而无 `pgx.ErrTxCommitRollback` 明确回滚证明时，返回 `ErrCommitUnknown` 并保留原因，不自动重试。接入方须核实 revision／命令结果后决定后续动作。本包未定义公共幂等键、命令 ledger 或 UI 恢复协议。底层 PostgreSQL 错误可能带有旧字段值；root 的 API／日志适配须按冻结契约去敏，不能直接对外序列化原始错误。

## 锁与成本

结构变化在当前表持有 `ACCESS EXCLUSIVE` 至事务结束，包含影响扫描、DDL 和元数据提交钩子；同表读写会被阻塞。纯显示改名只使用元数据门禁。锁等待和 SQL 时间有调用者显式配置，整次 Save 的总 deadline 由请求 context 提供；没有产品默认预算。

设 F 为旧／新字段总数、L 为默认文本总字节、N 为记录数、D 为删除列数、K 为改型列数、B 为补齐列数、J 为需验证的非空约束数：

- 计划时间 O(F+L)，额外空间 O(F)，不扫描业务记录。
- 删除影响计数合为一次 SQL 扫描，O(N·D)；确认适配器的数据／版本检查成本另计。
- 常量 DEFAULT／NULL 新增列通常免表重写；NOT NULL 验证可能扫描全表。
- 每个补齐列执行一次 UPDATE，O(B·N)，并产生 WAL／MVCC 写入；每个改型单独 ALTER，通常 O(K·N) 表重写，加上索引重建成本；非空验证最坏 O(J·N)。当前没有偷偷合并成后台迁移或第二阶段发布。
- 依赖登记、授权及 metadata 的查询／索引成本由其实际适配器决定。本包未声称这些外部边界是常数时间。

[PostgreSQL 18 ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html) 说明默认锁级别、常量默认值免重写、非空扫描及改型／索引重建。未运行 10k／100k／1m 容量实验，测试耗时不代表生产性能。索引策略及维护、运行 DDL 权限、备份／升级／恢复均待 root 集成；本包没有引入索引后台任务或 `CREATE INDEX CONCURRENTLY`。

## 验证与待接入

本包的真实存储测试要求 `WEAVEOS_B2_TEST_DATABASE_URL` 指向独立临时 `weaveos_b2_isolated_test`，且 host 为 `/tmp/weaveos-b2-*/socket` 私有 Unix socket。拒绝其他数据库，缺少环境时失败而非 skip。夹具 metadata 的 JSONB 仅用于测试配置；真实业务字段始终为 text／boolean 物理列。测试未改动既有 migration。

详细 RED／GREEN、不可变源码快照及命令见 [永久证据](../../../../docs/evidence/V030-003/README.md)。实际 B1／B3 adapter、完整字段 registry、前端 API 和删除确认协议均未接入。现有任务工具限定 V010，V030 分支的 PR governance 支持需 root 更新共享规则；本包不修改这些脚本或创建必然失败的 PR。
