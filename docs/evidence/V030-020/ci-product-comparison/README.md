# Product权限撤销用例对照

目标用例：`forms.component.spec.ts:1083`，`cached structure dialog is masked after permission revocation`。按Root要求单独核对product中的同一用例，browser job通过不替代该核对。

本次提交仅记录证据和任务状态，TDD:N/A；运行既有测试，无生产逻辑或断言改动。`verify-repo`、`check-tasks`、`git diff --check`通过。

| 源码head | 证据与实际结果 |
| --- | --- |
| 863ab00ff089ca226d0267b0231c652e7e3c9e06 | [product job](https://github.com/Hubujiu/WeaveOS/actions/runs/37220565110/job/111489869612)失败。该用例在1092行等待5s后dialog为1，预期0；suite为484 passed / 1 failed / 1 skipped。完整相关断言见863ab00-failure.txt。 |
| 66e2f1ea2df89ad0391d1a6a30d1f05d16f9d988 | [product job](https://github.com/Hubujiu/WeaveOS/actions/runs/37220779031/job/111490496138)完整success；该用例列于[54/486]，对应component suite为485 passed / 1 skipped，见66e2f1e-case-and-summary.txt和下载的product artifact result。 |
| f6a5daf40b6f06ad408421d75e52f50ff8b0be23 | 该现有用例在本地Chromium连续执行5次，5 passed / 0 skipped / 0 unexpected；保存原始日志、JSON stats与每次trace中的原断言结果。远端[product job](https://github.com/Hubujiu/WeaveOS/actions/runs/37222483460/job/111495466679)在read-time.txt所示时刻仍in_progress，尚不能声称最终远端该用例通过。 |

三个head中的该Root既有前端测试源码SHA256相同：`b436b7e51a6756b9a6b9037fc86d01dcd00aeb4b332e09aac92da741882ea919`。没有修改前端生产文件、测试或配置，也没有取消/跳过远端CI。当前后端身份边界修复与正式回归见../identity-boundary-validation.md。

## Trace与复跑

旧863ab00 product artifact已实际下载检查，只包含result.json（列表见863ab00-artifact-files.txt），未包含错误上下文或trace。既有component配置未开启trace，acceptance仅上传public摘要。因此无法取得旧失败的trace，不推测超时原因。

当前f6a5daf用例复跑开启CLI `--trace on`，原测试/配置不变；命令见f6a5daf-local-case-command.txt。代表性完整trace永久保存为f6a5daf-local-case.trace.zip；5次trace的SHA及三个原断言的实际执行结果见f6a5daf-local-trace-assertions.json。均使用本地harness及测试内合成API回复。第一次选择器错误使用了完整标题锚点而匹配不到用例，原输出单独保留为local-selector-setup-failure.*，不是产品行为RED。

## 待核对

当前代码head的完整远端product日志需待job完成后读取：`gh run view 37222483460 --job 111495466679 --log`。最终PR head仍需`gh pr checks 31`及Root审查。如再次出现同一权限撤销断言失败，将精确日志交Root安排前端修复；本包不修改前端或原测试。文档证据提交会产生新PR head，f6a5daf是本次实际后端代码及上述本地复跑的源码head。
