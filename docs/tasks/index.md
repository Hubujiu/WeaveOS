# v0.1.0 任务索引

状态以每个任务文件和GitHub PR/远程main为准，索引不复制第二份可能漂移的完成状态。

| 任务 | 工作内容 | 依赖 | 分支 / 工作树 |
| --- | --- | --- | --- |
| [V010-001](V010-001.md) | 工程底座、CI、测试框架、交接治理 | 无 | task/V010-001-foundation / ../WeaveOS-worktrees/V010-001 |
| [V010-002](V010-002.md) | 接口契约与未决项 | 001 | task/V010-002-contracts / ../WeaveOS-worktrees/V010-002 |
| [V010-003](V010-003.md) | 数据迁移、持久化、验收Seed | 002 | task/V010-003-persistence / ../WeaveOS-worktrees/V010-003 |
| [V010-004](V010-004.md) | Session、CSRF、身份 | 002、003 | task/V010-004-session / ../WeaveOS-worktrees/V010-004 |
| [V010-005](V010-005.md) | 注册登录、邀请、管理员、审计 | 002、003、004 | task/V010-005-auth / ../WeaveOS-worktrees/V010-005 |
| [V010-006](V010-006.md) | 登录/注册UI | 002；可与后端并行 | task/V010-006-web / ../WeaveOS-worktrees/V010-006 |
| [V010-007](V010-007.md) | 真实全栈集成和E2E | 003、004、005、006 | task/V010-007-acceptance / ../WeaveOS-worktrees/V010-007 |
| [V010-008](V010-008.md) | 发布、恢复、安全债务验收 | 007 | task/V010-008-release / ../WeaveOS-worktrees/V010-008 |
| [V010-011](V010-011.md) | 已验收成品服务器运行（SSH隧道） | 008 | task/V010-011-server / ../WeaveOS-worktrees/V010-011 |
| [V010-010](V010-010.md) | Notion 阻塞问题集中答复与来源规则 | 002 | task/V010-010-source-truth / ../WeaveOS-worktrees/V010-010 |
| [V010-012](V010-012.md) | Figma 自适应布局与手工浏览器视口修复 | 006 | task/V010-012-responsive / ../WeaveOS-worktrees/V010-012 |
| [V010-013](V010-013.md) | PR 收尾规则与遗留工作树清理 | 001 | task/V010-013-cleanup / ../WeaveOS-worktrees/V010-013 |

只领取依赖已经验收的任务。后续任务尚未启动，分支和工作树在领取时创建，不伪造Agent运行。共享契约由002维护；存储归003；Session归004；领域/HTTP装配归005；视觉/组件归006；跨层验证归007；运行配置归008。变更他人目录先登记协调，不跨任务覆盖。

新增用户授权的测试先行任务：[V010-009](V010-009.md)，依赖已验收001；task/V010-009-test-first / ../WeaveOS-worktrees/V010-009。用户2026-09-27追加授权完成PR #3：扩展HTTP/浏览器已通过，剩余19项真实STORE已本机通过并纳入CI；等待最终head完整远端复验，不提前accepted。正式来源和002–008验收不被本任务替代。
