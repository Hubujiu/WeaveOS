# WeaveOS

面向中小企业的管理平台。产品需求、架构取舍和数据设计由 Notion 管理；可执行契约、测试、迁移及实现由 Git 管理。

> 当前为仓库初始化，不是可运行的登录系统。首版目标为 v0.1.0：邀请码注册、账号密码登录与 Web 登录态。

## 开始工作

先读 [AGENTS.md](AGENTS.md)，再按 [Notion 文档路由](docs/notion-router.md) 主动读取本任务的权威资料。开发必须先从需求设计测试、观察预期失败，再编写实现；不得依据已有实现倒推验收测试。

- [Notion 项目](https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773)
- [贡献流程](CONTRIBUTING.md) · [测试规范](docs/testing.md) · [任务证据模板](docs/templates/task-record.md)
- [当前基线与未决事项](docs/project-baseline.md) · [安全边界](SECURITY.md)

## 目录

```text
apps/web/           React + Vite 前端边界，尚未创建业务应用
services/bff/       Go Web BFF / 本地认证边界，尚未创建业务服务
contracts/          OpenAPI / Proto / 错误码的版本控制入口
db/migrations/     待评审通过后提交的数据库迁移
infra/              待评审通过后落地的运行配置
scripts/            无第三方依赖的仓库结构检查
tests/governance/  仓库检查器自身的测试
docs/              文档路由、工程规则、任务证据；不复制整套 Notion
```

目录中目前只有边界说明与规范；没有伪造可运行的服务、接口、SQL 或依赖锁文件。

## 已可执行的验证

```sh
node --test tests/governance/policy.test.mjs
node scripts/verify-repo.mjs
```

检查器仅需要 Node.js 标准库。本次在 Node.js 22.16.0 执行；这不代表产品工具链已选为 Node 22。产品 Node / pnpm / Go 版本仍须依 ADR 评审并锁定。

检查器验证规范文件、硬规则标识、文档路由结构和覆盖指令文件，**不能证明 AI 真读过 Notion、测试先于实现编写，或业务已验收**。这些仍需任务证据、PR 审查和后续真实测试。

尚无前端构建、Go 测试、数据库集成或浏览器 E2E 命令；不得将它们报告为通过。尚未选择开源许可证。
