# WeaveOS

企业管理平台。Notion维护产品需求、架构决策和数据设计；Git维护可执行契约、测试、实现与任务验收记录。

**当前交付是v0.1.0工程底座，不是已完成的登录系统。** 前端只有React挂载点，不包含临时或假登录UI；Go BFF提供平台宿主，业务readiness尚未装配，返回503。

## 开始与接手

[AGENTS.md](AGENTS.md) → [HANDOFF.md](HANDOFF.md) → [任务索引](docs/tasks/index.md)。每任务独立branch/worktree，实际读取Notion，先需求测试RED后实现GREEN，验收后squash到main并安全清理。详见[工作流](docs/workflow.md)。

## 结构与运行

```text
apps/web/          React + Vite + TypeScript，构建/挂载宿主
services/bff/      Go BFF进程、HTTP平台，后续认证领域在内部按职责装配
contracts/         逐接口OpenAPI/Proto由V010-002完成
 db/migrations/    待评审后由V010-003实施的数据迁移
infra/             按验收任务落实运行环境，不默认部署全套中间件
tests/foundation/  工程、任务、发布规则测试
tests/e2e/         三浏览器底座smoke
tests/acceptance/  产品验收独立预期、测试绑定及待完成集成
 docs/tasks/       任务、依赖、分工、进度和接手记录
```

工具版本见.node-version、.go-version和packageManager/锁文件。本地安装pnpm后：

```sh
pnpm install --frozen-lockfile --ignore-scripts
pnpm typecheck
pnpm build
# 两个终端分别运行
cd services/bff && go run ./cmd/bff
pnpm --filter @weaveos/web dev
```

Go默认127.0.0.1:8080；可通过受控BFF_ADDR配置。`/health/live`用于宿主存活；`/health/ready`未接入真实依赖时不报成功。Vite preview只用于CI/development，不是生产服务器。

## CI、验收与交付

`.github/workflows/ci.yml`对PR/main执行仓库与任务检查、Go format/vet/race/build、冻结依赖/类型/构建以及Chromium/Firefox/WebKit smoke，保留测试报告和覆盖率。

`acceptance.yml`是手动/可复用的完整产品验收：真实PG/Redis、隔离Seed、HTTP、浏览器与发布矩阵。当前业务和Seed未完成，不能PASS。[验收说明](docs/acceptance/README.md)区分已写用例、待补自动化与真正需要人工/目标环境的项目。

`delivery.yml`经CI后构建带提交SHA和校验和的开发制品，仅main可产出；没有生产部署、没有v0.1.0发布声明。尚未配置main分支保护/必需审批，也未选择开源许可证。
