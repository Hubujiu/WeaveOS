# 初始化基线 · 2026-09-24

这是初始化时的状态快照和阻塞清单，不是长期 PRD。每次任务仍须重新读取 Notion。

| 项目 | 已核对状态 | 对实施的含义 |
| --- | --- | --- |
| 产品 v0.1.0「完成用户登录态」 | 需求编写中；未冻结 | 初始化不代表需求已经全部验收就绪 |
| ADR-001 认证与会话 | 已接受 | Go BFF、本地认证、Redis Session、Cookie 方案生效 |
| ADR-002 API | 已接受 | OpenAPI 3.2.1；对外 REST、跨服务 gRPC；同进程不强制 RPC |
| ADR-003 技术栈细化 | 拟议中 | React + Vite、Go 为既有方向；新增库、精确版本、边界细化不得自动批准 |
| ADR-004 基础设施 | 拟议中 | 不在初始化阶段部署全套中间件或宣布环境就绪 |
| 数据库设计 | 待评审，未部署 | 4 张 PostgreSQL 表 + Redis Session 是设计；不复制为已批准迁移 |

首版 PostgreSQL 表设计为 `auth.users`、`auth.password_credentials`、`auth.invitations`、`auth.authentication_events`。本次仅建立 `db/migrations/` 入口，没有提交 CREATE TABLE 或运行迁移。

保留的产品边界：邀请码一次性、无时间过期限制、成功注册原子消费；注册后回到登录页重新认证；Session 为 1 小时滑动空闲 TTL，不加绝对最长生命周期；Bootstrap Admin 仅为本期特例。账号规范化、字段/索引和具体实现策略需按对应待评审设计确认。

本次不实现角色/ACL、多租户、部门、SSO、MFA、第三方登录、自助找回密码、设备中心、知识库、低代码或 AI 认证；不顺手引入 MQ、搜索、对象存储、Kubernetes 或额外 Node BFF。Go BFF 的认证模块可以拥有自己的数据，不得跨领域写其他服务数据库。

## 实现前必须解除的事项

确认本任务 PRD 验收条件和未决问题；按需要接受 ADR-003/004 的新增决策；评审数据库设计及事务/撤销策略；验证 OpenAPI 3.2.1 工具链而不是静默降版；确认组件消费批准范围；锁定实际依赖与开发/测试环境；单独审查已知认证安全债务与上线条件。

尚未建立：业务端点、OpenAPI/Proto 定义、错误码登记条目、可执行 migration、前端/BFF 工程及锁文件、业务 CI、部署环境。目录说明不是它们已经实现的证据。

来源入口和每页状态见 [notion-sources.json](notion-sources.json) 与 [Notion 路由](notion-router.md)。其中未读取的下级页明确标为“仅定位”，不能当作已审核材料。
