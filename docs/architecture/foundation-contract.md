# V010-001 工程底座验收契约

来源：2026-09-24 用户要求先搭建架构、CI/CD、验收/E2E、每任务独立分支与工作树、squash 合并及跨 Agent 交接；继承 Notion ADR-001/002，不批准 ADR-003/004 的全部候选方案。

本契约先于实现。它定义工程底座，不声称 v0.1.0 登录业务完成。

## FND-01 可运行 Go BFF

GET /health/live 返回 200 和 {"status":"alive"}；HEAD 同状态但无正文；其他方法 405，Allow: GET, HEAD。这是运维端点，不套业务 Envelope。
GET /health/ready 仅在已装配的 readiness 检查成功时返回 200、status=ready；检查缺失/失败/请求取消时 503、status=not_ready，不泄露错误详情。不允许空底座伪报产品 ready。
所有响应有 Cache-Control: no-store、X-Content-Type-Options: nosniff，及本入口独立生成的非空 X-Request-Id，不信任外部同名头。健康检查不设置 Session Cookie。
未知 API 返回 404 和 code/message/data/meta 四键：code=API_NOT_FOUND，data=null，meta.requestId 与响应头一致；不回退到 SPA 或伪造认证成功。

## FND-02 分支/任务/工作树生命周期

任务正文内有唯一 `<!-- task-meta ... -->` JSON 区，包含 id、title、branch、worktree、pr、owner、dependsOn、allowedPaths、deliveryState。id 为 V010-三位数字；branch 为 task/<id>-<slug>；worktree 为 ../WeaveOS-worktrees/<id>；不得跨任务复用。
任务必须有 Scope、Sources、Acceptance、Progress、Handoff 章节及至少一个验收 checkbox。验收项全部完成，只能将 deliveryState 置为 ready，不能在合并前宣称 accepted。
已验收是派生事实：已重新读取 GitHub 远程 main 同路径同 id 任务文档，所有验收项完成，deliveryState=ready，且对应 PR 已合入本仓库 main。文件仅存在/标题相同/本地 main/本地全绿均不足。
清理还须 PR head 与当前待删分支 tip 完全一致，squash commit 已在远程 main 历史，工作树无未提交/未跟踪/未审查忽略文件。PR 未合并、文档未完成、任务不匹配、脏工作树或 merge 后多出提交均拒绝。因 squash 不要求 task 分支 HEAD 成为 main 的祖先。
先 dry-run，明确 --apply 后只清理该任务；不得 force-remove 工作树、rm -rf 或删除 main。Squash 后仅在上述证据成立时允许删除不再有用的本地分支。离线或 GitHub 不可达必须失败关闭。

## FND-03 CI 与发布分层

必需工程 CI 包括仓库/任务规范测试、Go test -race/vet/gofmt/build、前端 TypeScript 与 Vite 构建、真实浏览器启动/HTTP 联通 smoke。依赖锁文件冻结安装；测试失败不 continue-on-error，不以 --passWithNoTests、skip 或 only 掩盖。
产品验收单独运行真实 API、真实浏览器与真实存储测试；未实现项明确失败或阻塞整个版本验收，不能把底座 smoke 当登录 E2E。GitHub Actions 支持浏览器和服务容器；人工项只因需要产品审阅/真实生产条件而保留，不把尚未写的自动测试说成平台不支持。
CD 当前交付可追溯构建制品，不部署未经指定的生产服务器；未来发布必须通过全部版本验收和人工门禁。

## FND-04 前端构建边界

React + Vite + TypeScript 空应用只提供构建/挂载点，不自写未批准的登录组件；浏览器加载 title=WeaveOS，#root 存在且无脚本异常；同源 /health 请求代理至真实 Go BFF。UI 任务须先读取 Figma 与组件批准登记。
