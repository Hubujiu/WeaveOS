# WeaveOS · 新 Agent 从这里接手

当前版本目标：v0.1.0 邀请码注册、账号密码登录与 Web Session。**整版尚未完成。**

## 立即读取

根 AGENTS → docs/workflow.md → docs/tasks/index.md → 当前任务 docs/tasks/V010-NNN.md → 目标目录AGENTS → Notion适用正文。重新检查远程main、任务分支、PR、CI和本机worktree，不能只信本页静态状态。

基础任务：V010-001，PR #2，分支task/V010-001-foundation。是否已验收，以远程main同名任务文档全部完成 + PR #2已合入main为准；清理结果在PR评论核对。已清理的分支/worktree不存在不代表任务丢失。

已建立的工程职责：React/Vite挂载与构建；Go BFF进程和HTTP平台；健康检查；任务/发布门禁；CI的真实浏览器smoke；产品验收测试入口。没有将空登录页、待评审SQL或未连接的Redis伪装成业务完成。

## 下一任务与分工

V010-002先核对逐接口契约/技术与数据未决项，验证OpenAPI3.2.1工具。随后003数据、004Session、005认证业务；006前端可在契约确认后与后端并行；007整体验收；008发布/运维。各任务的来源、branch/worktree、允许路径、依赖、验收和接手动作均已记录。

没有替这些角色启动后台Agent。待办项不能因底座CI绿了就勾选。领取时用 `node scripts/task.mjs start V010-002` 检查，再明确--apply。

## 常用验证

```sh
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
node scripts/verify-repo.mjs
node scripts/check-tasks.mjs
(cd services/bff && go test -race ./... && go vet ./...)
pnpm install --frozen-lockfile --ignore-scripts
pnpm typecheck && pnpm build
pnpm exec playwright install --with-deps
pnpm exec playwright test tests/e2e
```

`tests/e2e`当前是底座smoke；`tests/acceptance`是产品验收，不能混称。`node scripts/check-release.mjs`当前预期失败，因为整版自动/人工验收尚未完成；不删除或降低门禁。生产部署目标与授权尚未提供。
