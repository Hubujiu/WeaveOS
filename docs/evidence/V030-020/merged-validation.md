# V030-020 P2d整合后的验证

2026-10-04，merge提交d570556的第二父提交精确为b9f4579c2b7ec0c3d38f963d910a5f1c7adcd26f。P2d生产源码及Root schema测试与该提交逐字节相同；共享OpenAPI冲突只合并两侧新增schema。全部244个两侧契约变更叶值及全部错误登记均保留，见merged-contract-preservation.txt。P2d README顶部已更新当前结果，初始无效HTTP400准备失败仍完整保留并标记resolved。

- 15项Root专项通过：merged-specialized.txt及exit-code。
- 完整串行Go race、vet与build通过：merged-backend.txt及exit-code。命令为go test -race -p 1 -count=1 ./...、go vet ./...、CGO_ENABLED=0 go build ./...。其中包含7项P2d schema Root用例和完整audit回归。
- BFF全目录gofmt检查无输出。原Root HTTP测试仅机械gofmt，精确等于d557260源码经gofmt输出；证明见merged-root-http-format-proof.txt。未改断言。最终SHA见merged-sources.sha256。
- 契约/治理294项通过；OpenAPI3.2.1 lint通过，12条既有warning；verify-repo/check-tasks通过。日志见merged-contract-governance.txt、merged-openapi.txt、merged-repo-checks.txt。
- Goose v3.28.0：新cold库up1–5/down4/up5、新hot库up1–16/down15/up16均通过，见merged-migration-lifecycle.txt。写入合法workflow审计后两边Down均exit1、SQLSTATE55000按设计拒绝，见merged-hot-down-guard.txt与merged-cold-down-guard.txt。迁移哈希保持兼容清单登记值。
- 专用network-none PostgreSQL18.6备份容器执行backup.test，4/4通过，见merged-backup.txt。首次readiness命中初始化临时PG而失败的原日志保留为merged-backup-initialization-failure.txt；等待最终PG启动后重跑通过，测试与断言不改。专用backup容器已停止并删除。

所有数据库均为合成隔离数据，凭据与加密backup测试产物只留在私有.work。没有合入main或部署。本包交付管理HTTP和schema兼容接线；实际Flowable受控部署/RPC和审批推进仍由后续包交付。

Draft PR #31：https://github.com/Hubujiu/WeaveOS/pull/31，base task/V030-018-approval-commands，head task/V030-020-workflow-http。任务元数据同步PR编号；最终远端SHA及CI结果由该PR/GitHub Actions核验。

Recoverable Root RED source snapshots are preserved in red-source/ with their SHA-256 manifest. They were copied byte-for-byte from the verified original commits on 2026-10-04; original RED execution times remain in the original logs. These .txt snapshots add no tests or changed assertions.
