# B2 执行证据 · 2026-10-02 UTC

范围依据为 [PRD](https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa)／[ADR009](https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f) 文末 B2 有限授权，root 于本会话明确释放文档门禁。基线 main `6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5`；只读核对 PR21 `74cc5824ee4336dd76c72874f0cc4cf38fe9e889`，未改 PR21。最终 ADR 回读页更新时间为 `2026-10-02T13:47:57.476Z`，B2 PLAN 与开工时逐字相同。

## 环境与隔离

保存的 WeaveOS 云环境 `/workspace`，独立工作树 `/workspace/WeaveOS-worktrees/V030-003`，分支 `task/V030-003-schema-save`。固定编译器 `/workspace/.weaveos-tools/go/bin/go`，实际版本 `go1.27.1 linux/amd64`，沿用现有 pgx/v5 5.9.2，无新增依赖。

本任务新建 PostgreSQL 18.6 容器 `weaveos-b2-01a0fcbd-pg`，仅供合成测试，`--network none`，无主机端口，Unix socket 目录 `/tmp/weaveos-b2-01a0fcbd/socket`，专用 DB `weaveos_b2_isolated_test`。夹具只创建随机 `b2_*` schema 并在每用例结束删除。停止旧 onboarding 容器、真实业务数据库和生产资源均未操作。

最终核查专用数据库剩余 `b2_*` schema 数为 0。测试容器已停止并移除，其专属匿名数据卷 `93e1a1e5459d47288364c8d55fb23f3d02f3369510d7507b5bb4385c709d385d`、socket 和 `/tmp/weaveos-b2-01a0fcbd` 临时日志／profile 目录均已移除；永久证据已先复制。独立工作树／分支为 root 审查保留，未合入 main，不能执行任务分支清理。

启动第一次 `pg_isready` 返回 no response（退出 2）；随后实际 readiness 接受连接再运行 RED。此启动状态不是目标 RED。

## 真实时序与命令

1. `20534e1`：先写最小接口／无行为占位及独立需求测试，实际执行两次 RED（`red-initial.txt`、`red-complete.txt`），退出 1。第二轮 38 条测试／子测试条目在目标断言失败，真实 DB 已连接；不是加载／编译／依赖错误。源码与占位保存在 `red-source.tar.gz`，哈希见 `red-sha256.txt`。快照保存时刻实际观察为 `2026-10-02T13:30:58Z`；原测试日志未包含起始时刻，未补造执行时间。
2. `129fed1`：SQL 语句超时／上下文取消用例先 RED，退出 1，见 `red-timeout.txt` 与对应源码归档、哈希。
3. `a045f0b`：首次实现后 41 个测试／子测试条目 GREEN，退出 0，见 `green-first.txt`。
4. `f567b3c`：新增“PostgreSQL error 不等于确定回滚”缺陷复现先 RED，退出 1，见 `red-commit-classification.txt`、`red-commit-source.tar.gz` 和哈希。SQLSTATE 40003 在真实提交之后被夹具注入，不是假装实际网络断开。
5. 之后最小修正：只有 `pgx.ErrTxCommitRollback` 构成本包采用的明确回滚证明，其余提交错误保留 `ErrCommitUnknown`／原因且不重试。最终 race 26 个顶层测试、16 个子测试（42 条）全部通过，覆盖率 87.7%，见 `green-race.txt`。覆盖率只作观察，不充当验收标准。

实际测试环境设置如下（此 URL 仅为本包无网络测试 socket，不含生产凭据）：

```sh
export WEAVEOS_B2_TEST_DATABASE_URL='postgres:///weaveos_b2_isolated_test?host=/tmp/weaveos-b2-01a0fcbd/socket&user=postgres'
export GOMODCACHE=/workspace/.weaveos-tools/go-mod
export GOCACHE=/workspace/.weaveos-tools/go-cache
export GOTOOLCHAIN=local
cd /workspace/WeaveOS-worktrees/V030-003/services/bff
/workspace/.weaveos-tools/go/bin/go test ./internal/appschema -count=1 -v
/workspace/.weaveos-tools/go/bin/go test ./internal/appschema -run TestSaveStatementTimeoutAndCancellationRollBack -count=1 -v
/workspace/.weaveos-tools/go/bin/go test ./internal/appschema -run TestSaveCommitServerErrorWithoutRollbackProofIsUnknown -count=1 -v
/workspace/.weaveos-tools/go/bin/go test -race ./internal/appschema -count=1 -v -coverprofile=/tmp/weaveos-b2-01a0fcbd/coverage.out
/workspace/.weaveos-tools/go/bin/go vet ./internal/appschema
/workspace/.weaveos-tools/go/bin/go vet ./...
/workspace/.weaveos-tools/go/bin/go build ./...
```

vet／build 都退出 0，原空输出日志仍保存，并在 `verification.json` 单独记录命令与退出码。最终源码哈希见 `green-sha256.txt`。源归档用于未来标记为 REPLAY 的复验，不冒充新的原始 RED。

## 验收映射与边界

AP-FR-02／B2 PLAN 对应稳定 ID、纯计划、真实物理表、默认／NULL、必填补齐、改型整次回滚、确认当前删除影响和依赖引用阻断。AP-FR-03 对应 Save 生效与 metadata 事务钩子，AP-FR-08 对应默认拒绝缺失 guard／依赖保护。用例独立断言数据库字段类型、旧值、物理列、revision 和 SQLSTATE。

实际 flow 和应用 metadata 未接入；`fixture_metadata` 的配置 JSONB 及 `fixture_references` 是本包私有测试适配器，不是 root 产品 schema。业务行只有真实 text／boolean 列；未使用 JSONB/EAV 业务记录。实际权限与确认 token 均需要 root 的领域适配；模拟允许 guard 仅存在于测试夹具。

复杂度及锁扫描成本见 [包说明](../../../services/bff/internal/appschema/README.md)。结构 Save 在 ACCESS EXCLUSIVE 锁内完成；默认常量通常免重写，改型、补齐和非空验证会扫描／写入，多个改型目前逐列 ALTER。没有后台 DDL 或另行发布阶段，未运行 10k／100k／1m 样本，不声称生产性能。

现有 `node scripts/verify-repo.mjs` 和 legacy `check-tasks.mjs` 通过，但后者没有验证 V030；显式 `validateTask` 本文件返回 `[TASK] invalid id`，这是 root 共享治理接线阻塞。未改脚本、未开 PR、未合并、未部署。

NOT RUN：全部已有 PG／Redis 产品回归；业务数字精度／时区／枚举与成员部门类型；真实 B1/B3 适配；生产最小 DDL 权限；共享迁移及备份升级恢复；公共 API、前端删除确认和完整幂等／恢复协议；最终远程 CI。已完成仅为本包的可隔离执行器与真实事务证明。
