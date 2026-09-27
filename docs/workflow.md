# 任务、验收、squash 与工作树

每任务固定V010-NNN编号、docs/tasks/V010-NNN.md、task/V010-NNN-topic分支和../WeaveOS-worktrees/V010-NNN工作树。规划只登记；领取时创建，不伪造运行中的Agent。每阶段写完成/待完成、来源、owner、允许路径、PR、RED/GREEN、阻塞与下一条命令，并推送，不仅留在聊天。

## 开始或接手

先git fetch --prune origin；读远程main的AGENTS/HANDOFF/任务索引和目标文档，同时检查远程任务分支、PR及git worktree list。未合并分支中更新更近的文档是任务现场，再按Notion路由核对适用正文。

```sh
node scripts/task.mjs start V010-002
node scripts/task.mjs start V010-002 --apply
```

默认dry-run；--apply创建独立工作树和分支并推送。需要真实clone、Git、已登录GitHub CLI和网络。已有branch/worktree时拒绝重建，应恢复原任务；依赖未验收就不能开始依赖它的实现。

## 唯一验收含义

合并前deliveryState=ready只表示待验收；不设置一个只能合并后勾选的递归“PR已合并”复选框。任务清单写本PR可交付、可验证的事项。

**GitHub远程main同路径、同任务ID文档存在，所有事项完成，并且对应PR已经合入本仓库main，才代表已验收。** 本地main、同名不同目录、分支全勾、PR仅closed或本地测试通过均不足。accepted是实际查询结果，不是Agent可预填状态。

所有新任务用squash merge。核对最终PR head的适用CI/验收后，执行`gh pr merge <number> --squash --match-head-commit <verified-head>`。不能忽略失败检查，不能把底座smoke当整版验收。main分支保护/必需检查尚需管理员在服务端配置，CODEOWNERS/清单不等于已强制启用。

## squash后保留TDD证据

工作分支保持真实RED→GREEN；main的docs/evidence/<task>/保留独立需求、测试/最小接口占位源码的补丁或归档、命令、退出码、结果和哈希。不能只引用会删除的分支或会到期的Actions artifact；回放结果不得冒充原始执行时间。

## 两阶段安全清理

用户2026-09-27已明确授权并要求每次PR完成后清理本地和GitHub分支、测试文件与工作树。**合并验收后两阶段清理都是必须完成的收尾动作**：Agent在本轮主动执行，不能只说远程Actions已清理，或将本地清理长期留在下一任务。以下dry-run仍用于核验；条件满足后直接显式--apply，无须重复询问。清理授权不等于批准合并开放PR或删除未交付工作。

先保全main中的任务/永久RED→GREEN证据，核对无活动进程或别的任务引用，再审查并清除本任务可丢弃的node_modules/dist、测试报告/临时截图/日志、一次性工具、测试缓存与.work测试目录。隔离测试容器/网络/卷仅按任务身份逐项核对，无活动依赖且确属可丢弃测试数据后清理，不用全局prune。Windows每个删除目标须解析到明确的仓库/任务目录内部；不跟随junction/symlink删除其目标，不对未知目录递归删除。

开放PR、未交付代码、在用worktree/浏览器/隧道以及必要凭据/恢复资料须先保留并报告具体原因；不得为满足“无残留”删未合并分支、运行数据或永久证据。完成后重新查询GitHub分支、本地branch/worktree、目标目录和测试资源，记录实际已清理/仍保留/阻塞以及下一条命令；没有核验就不能说全部清理。

### 远程引用

`.github/workflows/task-cleanup.yml`在任务PR合并后执行，只检出main，调用scripts/cleanup-remote-task.mjs。它重新读取GitHub PR和远程main同ID任务文档，确认全部事项完成、同仓库PR merged到main、squash-style提交在main、远程分支仍等于已验收PR head。用预期SHA的条件push删除该远程引用；已删除则幂等成功，新提交/未合并/未完成/不可联网则拒绝。

该Actions只管理远程branch，不能看到或删除开发者机器的工作树，不能把远程删除记作本地也已清理。远程验收文档与PR保留，本地新增工作不会因远程引用删除而自动消失。

### 本地分支和工作树

```sh
node scripts/task.mjs status V010-002
node scripts/task.mjs cleanup V010-002
node scripts/task.mjs cleanup V010-002 --apply
```

status查询验收事实，即使branch/worktree已删除也可确认已验收。cleanup默认dry-run；--apply前再次联网读取remote main/PR，要求squash commit在main，本地task tip等于验收PR head且工作树完全干净。远程分支若尚在，必须同SHA；若已由Actions删除，不阻碍本地核验。

任何未交付本地提交、未提交/未跟踪/未审查忽略文件都阻止本地删除。先审查node_modules/dist等本机产物，不对未知目录rm -rf。远程分支尚在则条件删除，再非强制worktree remove，最后删除已经证实交付的本地branch。不会删除main，不force-remove脏工作树，发生并发或联网失败即停止。

Squash改变提交身份，不能要求task HEAD必须成为main祖先。脚本检测单父提交和对应PR，但这不是对任意历史合并方法的通用鉴别器；每次合并仍须明确使用squash参数并记录真实结果。

临时connector源码快照不含完整远程历史时，工具应阻塞；可下载已授权GitHub CI的source bundle恢复真实Git对象，再通过连接器重新核对最新remote main/PR/分支SHA后完成同等检查。不得伪造origin/main或把旧快照当刚刚fetch的结果。清理结果记录在PR评论，保留main任务和证据文档。
