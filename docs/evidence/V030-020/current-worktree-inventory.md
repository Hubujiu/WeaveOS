# V030-020 当前工作树交接清单

检查时间：2026-10-04 16:xx UTC（待 Root 增补 00016 审计合同）。分支 `task/V030-020-workflow-http`，HEAD 仍为基线 `d55726078ec42e1ab7ebcca47cfa979c44321a9d`；当前未提交变更请完整保留。

## 未提交实现

- `services/bff/internal/appworkflows/http.go`：流程定义 HTTP、严格 DTO/JSON、读写权限、幂等 manager transaction、候选审批人权限检查、生命周期管理与审计写入。**当前审计摘要仍为 00007 六键临时兼容写法；Root 新增 migration16 审计合同后需改为 ADR 字段并完成专项回归。**
- `services/bff/internal/applications/http.go`、`services/bff/cmd/bff/config.go`：宿主精确路由分流与真实服务装配。
- `db/migrations/00016_workflow_management_operations.sql`：当前只扩展 operation kind；Root 后续合同可能需叠加 audit CHECK 的增量扩展，完成后更新 compatibility hash。
- `contracts/openapi/openapi.json`、`contracts/errors/codes.json`、`infra/server/deploy/compatibility.json`：HTTP schema/errors/migration 16登记。兼容清单中 migration16 当前 SHA-256 为 `484e1cacd03558632d86a77662f7f77f203d0392fe41fd8d445ce485fe4e1a09`，迁移变化后必须重算。
- 测试文件 `services/bff/internal/appstructure/root_workflow_http_test.go` 未修改，其 SHA 与最初RED记录相同。

## 已有验证结果（非最终合同 GREEN）

- 有效初始 RED：12 项真实 Root HTTP 测试，所有占位行为失败；源码/测试/环境/exit code见 `root-workflow-http-red*`。
- 当前临时六键审计 shape 下，同一专项曾通过 12/12；这不符合 ADR 审计字段，不可作为最终 GREEN。没有按本次交接要求重跑。
- `pnpm exec redocly lint contracts/openapi/openapi.json` exit 0；12 条既有警告，无 schema error。
- migration16 空库 up 1–16/down 到15/up16通过；存在 workflow operation history 时 down 以 SQLSTATE `55000` 拒绝。
- `node --test infra/runtime/backup.test.mjs` 4/4通过；`backup.test.mjs` 与断言均未改，测试时临时给隔离 PG 容器使用其既有 v010 测试名前缀并恢复容器原名。
- 初始审计CHECK失败的 PostgreSQL SQLSTATE `23514` 诊断保存在 `audit-check-rejection.txt`。相关说明见 `audit-contract-conflict.md`。

## 后续接续点

Root 补齐 migration16审计CHECK与精确合同后，先读取其新增迁移/测试，按 ADR 生成完整审计摘要；校验 migration/check/hash/compatibility/OpenAPI，然后只运行正式专项测试并保存最终GREEN及SHA。专项GREEN后推检查点给Root审查，再继续 race/vet/gofmt/build、治理回归，最终再创建以 `task/V030-018-approval-commands` 为base的draft PR并更新任务元数据。不得合入main或部署。
