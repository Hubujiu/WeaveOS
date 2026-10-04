# V030-020 当前工作树交接清单

分支 `task/V030-020-workflow-http`，工作树 `../WeaveOS-worktrees/V030-020`。最近提交：Root合同测试 `4724b87`、冷库fixture修正 `96e044b`。初始RED时序与测试SHA保存在 `root-workflow-http-red*`。

## 实现

- `services/bff/internal/appworkflows/http.go`：严格流程定义管理HTTP、当前owner/bootstrap与候选审批人授权、幂等事务、生命周期与ADR精确六键审计reason/action写入。
- `services/bff/internal/applications/http.go`、`services/bff/cmd/bff/config.go`：精确宿主路由分流及BFF装配。
- hot `db/migrations/00016_workflow_management_operations.sql`、cold `db/archive-migrations/00005_workflow_management_audit.sql`：有界增加workflow审计CHECK与operation kinds，Down遇历史按55000拒绝。未改hot00001–15/cold00001–4。
- OpenAPI、errors、compatibility hash和任务元数据已更新。Root测试未改断言；只机械gofmt，Root归档fixture仅补数据库必填id/occurred_at。

## 验证

- 13项Root HTTP + hot/cold约束 + archive复制冲突，共15项专项全绿；完整 `internal/audit` 回归全绿。
- `go test -race -p 1 -count=1 ./...`全后端通过；`go vet ./...`、`CGO_ENABLED=0 go build ./...`和gofmt检查通过。
- OpenAPI Redocly lint exit 0、12条既有warning；治理/契约测试294/294通过；backup.test 4/4通过。
- hot00016与cold00005空库up/down/up通过，带workflow审计历史的Down均以SQLSTATE 55000拒绝。
- `final-regression.md`与相邻`final-*`文件记录长回归与日志。迁移/备份及TDD细节在 `migration-16-lifecycle.md`、`audit-migration-guards.md`、专项证据中。

## 后续

已通过d570556整合精确V018 P2d checkpoint `b9f4579c2b7ec0c3d38f963d910a5f1c7adcd26f`，保留两侧OpenAPI/errors及`workflowConflicts/409`，更新P2d README顶部当前状态并保留历史失败原文。整合后适用检查已全部通过，详情见merged-validation.md。下一步推送分支，创建以`task/V030-018-approval-commands`为base的draft PR并更新任务PR字段，交Root审查。不合入main、不部署。本包不代表完整审批后端或Flowable回执接线。
