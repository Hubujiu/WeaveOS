# WeaveOS · 新 Agent 从这里接手

当前版本目标：v0.1.0 邀请码注册、账号密码登录与 Web Session。**整版尚未完成。**

## 立即读取

根 AGENTS → docs/workflow.md → docs/tasks/index.md → 当前任务 docs/tasks/V010-NNN.md → 目标目录AGENTS → Notion适用正文。重新检查远程main、任务分支、PR、CI和本机worktree，不能只信本页静态状态。

基础任务：V010-001，PR #2，分支task/V010-001-foundation。是否已验收，以远程main同名任务文档全部完成 + PR #2已合入main为准；清理结果在PR评论核对。已清理的分支/worktree不存在不代表任务丢失。

已建立的工程职责：React/Vite挂载与构建；Go BFF进程和HTTP平台；健康检查；任务/发布门禁；CI的真实浏览器smoke；产品验收测试入口。没有将空登录页、待评审SQL或未连接的Redis伪装成业务完成。

## 当前任务

V010-001至007及010已合入；实际接受状态仍须读取远程main同ID任务与merged PR。当前V010-008在../WeaveOS-worktrees/V010-008、task/V010-008-release、PR #11实施。只有一个Agent，没有后台部署。

008已实现受限审计、独立冷库自动维护、加密恢复、同制品回滚、真实告警与安全补丁。6eaf363完整本机镜像运行通过（6操作、DNS、monitor、TLS、备份、完整性与四镜像扫描、25API、30浏览器），恢复8203ms；回滚85c2ee1已实际通过。PG18.6/Redis8.2.10及应用依赖补丁锁定。最新CI还须实际查询；此前磁盘不足已增加仅托管Linux的SDK清理，不在本机执行。视觉/运行/风险最终三项仍待用户按Q10/Q14签署；不能声称整版完成。读取docs/tasks/V010-008.md与docs/evidence/V010-008/review.md恢复真实head、命令及剩余事项。

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

`tests/e2e`是底座smoke；`node infra/acceptance/run.mjs`执行真实产品全栈；`node infra/runtime/run.mjs`在空工作目录创建仅本机可访问的Linux镜像环境，验证恢复/回滚/告警/TLS和同制品API/浏览器，结束后停止容器并保留私有卷/资料。不能在已存在.work/runtime的情况下覆盖重跑，先核对并保留现场。`node scripts/check-release.mjs`当前预期失败，因为整版自动/人工验收尚未完成；不删除或降低门禁。生产部署目标与授权尚未提供。
