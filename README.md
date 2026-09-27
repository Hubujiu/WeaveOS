# WeaveOS

企业管理平台。Notion 与 Figma 定义产品、数据与设计规则；Git 保存派生契约、测试、实现和验收证据。

## v0.1.0 本地开发版

已实现邀请码注册、账号密码登录、一小时滑动 Web Session、Bootstrap Admin 邀请码/重置特例，以及最小认证日志、独立冷归档和自动到期删除。登录与注册使用已确认的 WaveOS Figma 原型。

产品自动测试与本机 WSL Docker Linux 运行演练已通过，用户于 2026-09-27 确认视觉、运行恢复及风险验收。实际最终交付以 [PR #11](https://github.com/Hubujiu/WeaveOS/pull/11)、远程 main 的 [V010-008](docs/tasks/V010-008.md) 和最终提交 CI 为准；材料见 [验收总览](docs/evidence/V010-008/review.md) 与 [用户签署](docs/evidence/V010-008/user-signoff.md)。

本版本仅用于本地合成数据开发。固定重置密码、无限流/锁定/绝对会话期限及已列基础镜像/工具漏洞按用户决定保留，完整报告可查；没有生产部署。

## 开始与接手

[AGENTS.md](AGENTS.md) → [HANDOFF.md](HANDOFF.md) → [任务索引](docs/tasks/index.md)。每任务独立 branch/worktree，实际读取 Notion/Figma，按需求先测试 RED 后实现 GREEN。最终 head 检查通过后 squash；验收与安全清理规则见 [工作流](docs/workflow.md)。

## 运行与测试

前置条件、私有配置、制品导入、启动/停止及恢复步骤见 [运行手册](infra/runtime/README.md)。要求 Docker Linux daemon；Windows 使用 WSL Docker Desktop。工具版本由 .node-version、.go-version、packageManager 和锁文件固定。

```sh
# 在没有已有私有现场的独立工作目录中执行
node infra/acceptance/run.mjs   # 真实 PostgreSQL/Redis/HTTPS 产品验收
node infra/runtime/run.mjs      # 不可变镜像、受限身份、恢复/回滚/告警/TLS及API/浏览器
node scripts/check-release.mjs # 全部自动与实际用户签署证据
```

这些命令执行验收流程并在结束时停止自建容器，保留私有现场。已有 .work/acceptance 或 .work/runtime 时不能覆盖重跑；按运行手册检查并保留旧目录。仅入口绑定 127.0.0.1:19443，不公开数据库或 Redis。私有 .work 中的密钥、随机账号、邀请码、Cookie 与备份禁止入仓库或公开上传。

## 结构

| 路径 | 职责 |
| --- | --- |
| apps/web/ | React/Vite 登录、注册与受保护页面 |
| services/bff/ | Go BFF、本地认证、Redis Session、审计与受控CLI |
| contracts/ | OpenAPI 3.2.1、DTO与错误契约 |
| db/migrations/、db/archive-migrations/ | 热库与独立冷库版本化迁移 |
| infra/acceptance/、infra/runtime/ | 隔离产品验收与真实运行/恢复/制品配置 |
| tests/foundation/、tests/governance/ | 工程、任务与门禁检查 |
| tests/e2e/ | 三浏览器底座 smoke |
| tests/acceptance/ | 真实 HTTPS API、三浏览器产品验收 |
| docs/tasks/、docs/evidence/ | 任务、永久 RED 快照、执行证据及交接 |

## CI 与交付

CI 检查 Go format/vet/race/build、前端类型与构建、三浏览器 smoke 及治理。产品流水线另执行真实迁移/事务/Session/认证、23组件、25 API、30三浏览器与3依赖故障测试，并验证受限运行、恢复、回滚和完整镜像扫描；分层说明见 [验收文档](docs/acceptance/README.md)。

手动 delivery 工作流通过 CI 和完整发布矩阵后，将同一次运行已验证的 OCI 文件交付为本地版本包，仅 main 可产出，不重新构建、不执行生产部署。同机备份、快照后数据丢失、命令采样告警和自签名证书的边界见运行手册。

分支保护/必需审批须在 GitHub 服务端独立配置；文档与 CODEOWNERS 不代表已强制启用。仓库尚未选择开源许可证。
