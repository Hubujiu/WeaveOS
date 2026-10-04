# V030-020 最终本地回归检查点

日期：2026-10-04。当前执行工作树 `/workspace/WeaveOS-worktrees/V030-020`，分支 `task/V030-020-workflow-http`。Go 命令在 `golang:1.27.1` 容器执行；真实集成测试使用专用 PostgreSQL 18.6、Redis 8.2.10 与独立 appschema Unix-socket PostgreSQL fixture。凭据留在私有 `.work/v030-020`。

## 通过项

- 15 项Root专项：13项`TestRootWorkflowHTTP`、hot/cold约束用例、归档完整复制/冲突保留用例均通过。各原始日志、退出码和Root测试SHA见`root-workflow-http-green*`、`audit-hot-cold-green*`、`audit-archive-green*`。
- 完整BFF：`go test -race -p 1 -count=1 ./...`，exit 0。串行运行是项目隔离PG fixtures的规定；appscha专用socket按同一容器路径挂载。所有Go包通过。
- 静态检查和构建：`go vet ./...`及`CGO_ENABLED=0 go build ./...`通过；原始命令输出见`final-vet-build.txt`，退出码为0。
- 格式：对本任务Go源码及Root新增测试运行`gofmt -d`，无差异。并将两份Root测试与其授权提交中的源码经gofmt后逐字节比较，结果一致。Root提供的归档fixture仅补齐必填id与occurred_at，断言未改。
- 契约/治理：Redocly OpenAPI lint exit 0（12条既有warning，无error）；`node --test contracts/*.test.mjs tests/governance/*.test.mjs tests/foundation/*.test.mjs tests/acceptance/topology.test.mjs`共294项通过。完整日志见`final-openapi-lint.txt`、`final-governance-contract-tests.txt`。
- 迁移：hot00016空库up 1–16、down到15、再up通过；cold00005空库up 1–5、down到4、再up通过。热、冷均在workflow审计历史存在时按SQLSTATE `55000`拒绝Down。细节见`migration-16-lifecycle.md`与`audit-migration-guards.md`。
- 备份回归：`node --test infra/runtime/backup.test.mjs` 4/4通过，测试和断言未改；环境兼容步骤及结果见`migration-16-lifecycle.md`。
- 迁移兼容清单中的hot00016/cold00005 SHA-256已与当前文件核对，见`infra/server/deploy/compatibility.json`。

## 执行环境备注

当前执行器没有Go SDK可直接调用：`/usr/bin/go`是无关同名程序且没有`gofmt`。使用仓库锁定的`golang:1.27.1`镜像执行检查，并将宿主CA bundle只读挂入容器。Go module下载成功。首次并行全量测试因共享数据库fixtures互相干扰而失败；后续按仓库验收脚本使用`-p 1`串行重跑，完整退出码为0。appschema首次容器检查未挂载宿主socket时失败，随后按固定fixture路径挂载，串行全量检查通过。

## 尚未完成

精确V030-018 P2d checkpoint尚未整合；整合后需核对共同OpenAPI/error edits并保留`workflowConflicts/409`，更新P2d README顶部当前状态，再按合并后最终源码重跑专项、race/vet/build、OpenAPI/治理、迁移/备份契约检查。之后推送任务分支并创建以`task/V030-018-approval-commands`为base的draft PR，更新任务PR字段并提交Root审查。不合入main、不部署。本包不代表完整审批后端或真实Flowable回执接线。
