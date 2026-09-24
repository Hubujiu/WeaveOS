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

## 安全清理

```sh
node scripts/task.mjs status V010-002
node scripts/task.mjs cleanup V010-002
node scripts/task.mjs cleanup V010-002 --apply
```

status查询验收事实，即使分支/worktree已删除也仍可确认已验收。cleanup再次联网读取远程main/PR，要求squash commit已在main历史中、任务branch tip等于验收PR head、该工作树完全干净。任何新增提交、未提交/未跟踪/未审查忽略文件都阻止删除。先审查node_modules/dist等本机产物，不对未知目录rm -rf。

Squash改变提交身份，不能要求task HEAD必须是main祖先。工具使用预期SHA的条件远程删除，再非强制worktree remove，最后删除已证实交付的本地branch。不会删除main，不force-remove脏工作树。出现并发变更或联网失败即停止。

临时connector源码快照不含完整远程历史时，工具应阻塞；可下载已授权GitHub CI的source bundle恢复真实Git对象，再通过连接器重新核对最新远程main/PR/分支SHA后完成同等检查。不得伪造origin/main或把旧快照当刚刚fetch的结果。
