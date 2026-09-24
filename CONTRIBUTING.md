# 贡献流程

先执行AGENTS、HANDOFF和docs/workflow.md；按Notion路由实际读取需求。每任务独立分支/worktree，使用docs/tasks/V010-NNN.md记录来源、范围、验收和交接。

先需求→测试→实际RED，再最小实现GREEN/重构/回归。配置、脚本、迁移也适用；禁止根据实现倒推expected、弱化断言或伪造执行。真实证据在squash后仍须从main恢复。

提交使用docs/test/feat/fix/refactor/chore前缀并含任务ID。只stage本任务文件，不盲目git add -A。每个PR更新自己的任务文档；检查允许路径、对应PR、实际验证与未完成项。

任务交付项全部验证后最多标ready，最终head通过适用检查后squash merge到main。只有远程main同名同ID任务文档全部完成且PR已合入main才算验收；按工作流删除该任务分支/worktree，保留文档与证据。不能提前填accepted，也不需要合并后再改“PR已合并”复选框。

规则、门禁或范围变更须说明依据和影响，不得悄悄弱化TDD、组件批准或Notion状态约束。当前CODEOWNERS只提供审查路由；服务端分支保护/必需审批尚未配置。生产部署和破坏性操作另需授权。
