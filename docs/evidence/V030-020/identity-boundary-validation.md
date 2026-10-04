# Root身份边界审查修复

来源：Root提交3b87fd2be0fb193160bc709e82ae1884bff47e08（本地cherry-pick为87bee60）与2026-10-04 17:47:16 UTC更新的Notion PRD：https://app.notion.com/p/3ef2f5a9e64881ec80d7fa30bb9aa8f8。

新增Root测试先在真实Session/CSRF、PG/Redis运行有效RED：5个非法审批人ID（bad、empty、zero、uppercase、trailing-space）误返503或403，见identity-boundary-red.txt、命令、退出码1、源码SHA与环境记录。原始Root测试和修复前HTTP源码均完整保存在identity-boundary-source/，不是事后生成测试或伪造RED。

最小生产修复只在decodeGraph的approval节点解析中用已有validID检查每个assigneeId，在任何授权SQL或operation写入前返回400 COMMON_INVALID_ARGUMENT；不trim、lowercase或宽松转换ID。authorizeAssignees及合法候选权限查询原样保留。OpenAPI同步小写canonical、非零UUID约束。Root测试只机械gofmt，逐字节比较授权原文经gofmt的输出一致，证明见identity-boundary-test-format-proof.txt。

修复后验证全部通过：

- 14项Root HTTP（含新测试5子例）及2项热冷CHECK/归档，共16项专项；定义/operation/audit计数不变断言均执行通过。见identity-boundary-specialized.txt，exit0。
- 完整go test -race -p 1 -count=1 ./...、go vet ./...、CGO_ENABLED=0 go build ./...；见identity-boundary-backend.txt，exit0。
- 294契约/治理、OpenAPI3.2.1 lint（12条既有warning）、全BFF gofmt、verify-repo/check-tasks均通过。见identity-boundary-contract-governance.txt、identity-boundary-openapi.txt、identity-boundary-gofmt.txt。
- migration/backup源码未改变，仍沿用merged-validation.md中的真实空库up/down/up、55000历史保护与backup4/4证据。

最终源码清单为identity-boundary-final-sources.sha256。旧66e2f1e CI状态快照与读取时间保存在identity-boundary-old-checkpoint-ci.*和-time.txt；旧head不能代替本次新增边界合同的验收。PR31同分支继续推送，最终head CI及Root复核为待确认项。不合入main、不部署。
