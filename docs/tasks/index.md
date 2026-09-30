# v0.1.0 任务索引

后台外壳修复：[V010-020](V010-020.md)，依赖已验收019；task/V010-020-admin-shell / ../WeaveOS-worktrees/V010-020。用户2026-09-30要求全视口背景、图标水平折叠及arca-ui动效参考，单Agent。

后端精简剩余需求：[V010-018](V010-018.md)，依赖已验收016/017；task/V010-018-backend-simplification / ../WeaveOS-worktrees/V010-018，[PR19](https://github.com/Hubujiu/WeaveOS/pull/19)。用户2026-09-29确认采用剩余全部方案，单Agent实施；日志留存参数见任务文档及Notion Q20。

公网入口：[V010-015](V010-015.md)，依赖已验收011；task/V010-015-public-https / ../WeaveOS-worktrees/V010-015。用户2026-09-28明确部署授权，ADR004已接受；单Agent实施，不触碰014界面工作树。

自动交付：[V010-016](V010-016.md)，依赖已验收008/011；task/V010-016-auto-deploy / ../WeaveOS-worktrees/V010-016。用户2026-09-28授权main通过检查后自动部署，包含已验收兼容迁移与配置。单Agent独立工作树。

后端边界：[V010-017](V010-017.md)，依赖已验收005；task/V010-017-auth-boundary / ../WeaveOS-worktrees/V010-017。[PR18](https://github.com/Hubujiu/WeaveOS/pull/18)实现用户已确认的同进程Web适配与认证用例分离，不新增微服务或改变部署。

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
| [V010-014](V010-014.md) | 同步更新后的 Figma 登录注册布局 | 012 | task/V010-014-figma-refresh / ../WeaveOS-worktrees/V010-014 |

只领取依赖已经验收的任务。后续任务尚未启动，分支和工作树在领取时创建，不伪造Agent运行。共享契约由002维护；存储归003；Session归004；领域/HTTP装配归005；视觉/组件归006；跨层验证归007；运行配置归008。变更他人目录先登记协调，不跨任务覆盖。

新增用户授权的测试先行任务：[V010-009](V010-009.md)，依赖已验收001；task/V010-009-test-first / ../WeaveOS-worktrees/V010-009。用户2026-09-27追加授权完成PR #3：扩展HTTP/浏览器已通过，剩余19项真实STORE已本机通过并纳入CI；等待最终head完整远端复验，不提前accepted。正式来源和002–008验收不被本任务替代。

本轮完整接入：[V010-019](V010-019.md)，依赖已验收018；task/V010-019-complete-app / ../WeaveOS-worktrees/V010-019。用户要求补齐最新登录注册、Home导航与已批准人员管理R3；Q24确认按Figma原版复刻，组件优化后置。单Agent。

最新尺寸、后台动画与Arca控件：[V010-020](V010-020.md)，依赖已验收019；task/V010-020-admin-shell / ../WeaveOS-worktrees/V010-020，[PR21](https://github.com/Hubujiu/WeaveOS/pull/21)。Q31按用户审核移除顶栏收缩、缩短侧栏菜单间距并消除页签多余纵滚条；先同步Figma唯一视觉/交互源，再RED→GREEN。固定56顶栏、176/72侧栏，保留图标纵坐标与正文必要滚动。Q32追加六处Arca Select样式/分离动效与四页签Pill，先同步Notion/Figma再真实RED→GREEN；106完整组件、96三引擎、类型/构建和132治理通过，强化反向采样再三引擎3项通过。实际状态与验证以任务正文及PR最终head为准；主Agent代码唯一写者，子Agent只读核对与独立Figma同步。未合并/验收/部署。
