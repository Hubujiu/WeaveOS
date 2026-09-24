# 基础设施

先读根AGENTS、HANDOFF、任务文档、ADR-004/003当前状态；影响认证网络/Cookie/故障时加ADR-001。拟议拓扑不是生产部署授权。

配置会改变行为，先独立预期和失败验证再实现；不得借YAML/CI逃避测试，不使用假成功步骤。当前CI为工程检查/真实浏览器smoke，acceptance为尚待业务接入的产品验收，delivery仅开发制品，不得混称上线。

只引入实际需要的组件；不顺手部署MQ、搜索、对象存储、Kubernetes、服务网格或额外网关。开发默认loopback，数据库/Redis/内部管理端口不默认公开。Secret外部注入，测试隔离，权限最小，恢复/回滚分别真实验证。

当前没有授权生产主机/域名/Secrets，也没有生产Compose/Docker部署。固定重置密码和无限流属于已知安全债务，不能宣称适合公网生产。

V010-007可装配infra/acceptance中的同源HTTPS与测试证书，保留Secure Cookie；V010-008负责真实运行保障。工作流权限不足时报告，不绕过授权。每任务独立branch/worktree、文档和squash验收后清理。
