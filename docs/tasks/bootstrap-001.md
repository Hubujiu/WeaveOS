# BOOTSTRAP-001 · WeaveOS 仓库初始化

状态：本地验证完成；托管 CI 结果以本初始化 PR 的真实检查记录为准。
授权：用户要求初始化 Hubujiu/WeaveOS，特别约束主动读 Notion、先测试后实现、禁止依据实现写测试。空仓库创建初始 README，剩余改动在 `chore/repository-initialization` 完成。

## 范围

包含根/子 AGENTS、文档路由、来源登记、贡献/测试/安全规则、任务/PR 模板、目录边界、仓库结构检查器及其独立测试、治理 CI。

不包含产品业务代码、依赖选型批准、API 端点定义、生产 SQL、Figma 修改、组件导入、Notion 状态变更、部署、分支保护或开源许可选择。

## 本次实际读取

读取日期：2026-09-24 UTC，本会话；请求未全部提供精确读取时刻，不补造到秒的时间。来源 URL 在 `docs/notion-sources.json` 按下列 key 登记。

| 来源 key | 来源最后编辑时间（UTC） | 本次使用部分与状态 |
| --- | --- | --- |
| project | 2026-09-24T12:20:56.215Z | 项目入口/子页 |
| prd-home | 2026-09-24T05:09:24.618Z | 文档职责、Ready/Done、版本冻结 |
| current-prd | 2026-09-24T12:15:05.461Z | 范围、流程、原型链接；需求编写中/未冻结 |
| architecture | 2026-09-24T12:15:20.999Z | 当前 ADR、拟议项、数据库边界 |
| adr-001 | 2026-09-24T09:52:34.410Z | 已接受的本地认证/Session/安全债务 |
| adr-002 | 2026-09-24T10:26:13.400Z | 已接受的 API 格式/协议/契约优先基线 |
| adr-003 | 2026-09-24T11:27:38.676Z | 拟议状态、技术栈和组件复用约束 |
| adr-004 | 2026-09-24T11:26:40.286Z | 拟议状态、最小运行边界 |
| db-home | 2026-09-24T13:03:20.178Z | 导航、字典维护；待评审/未部署 |
| db-design | 2026-09-24T12:06:01.373Z | 四表职责、数据归属；待评审建议 |

组件库实际读取：`Hubujiu/React-/AGENTS.md`，blob SHA `60492f2c28069ba77e27debd5ad0b5274aa96a4c`。未导入或修改组件。

部分长 PRD/ADR/数据页的工具展示被截断；本次只使用已返回且覆盖初始化需要的范围/状态段落，不宣称全页审查或业务契约验收。其余下级 ER/DDL/Redis/对象/字段/功能记录仅登记已读入口中的链接，实施时必须读取，不能将它们标为已读。Figma 仅定位 PRD 链接，未读画布。

独立外部核对：OpenAPI 官方 3.2.1；Codex 官方 AGENTS 加载说明；GitHub 官方 actions/checkout 和 actions/setup-node v4 tag 指向的 commit（工作流已固定 SHA）。没有据此改变项目批准状态。

## 实现前验收契约

`docs/repository-checks.md` 先于实现确定 GOV-01 文件、GOV-02 根规则标识、GOV-03 来源/路由结构、GOV-04 禁止指令覆盖文件，以及 CLI 退出语义。测试从这些要求设计，不读取实现常量生成 expected。

## RED

测试提交：`b9344da4da980a35b25e9985580df16e897ea3ca`。
测试路径：`tests/governance/policy.test.mjs`。
环境：Node.js v22.16.0 / Linux；命令 `node --test tests/governance/policy.test.mjs`。
日志文件时间（UTC）：2026-09-24T13:17:25.518125+00:00。
结果：18 tests，3 pass，15 fail，0 skip，退出码 1。

失败因校验行为尚未实现：删除规范文件、删除硬规则、损坏来源 JSON/日期/URL/路由、增加覆盖文件及非法目录等未被拒绝。运行器正常到达断言，不是依赖/语法/网络错误。RED 仅有返回空错误数组的接口占位，不含校验实现。

## GREEN

保持测试及测试需求契约不变，之后才实现 `scripts/verify-repo.mjs`。GREEN 实现提交为包含本记录的初始化完成提交。

日志文件时间（UTC）：2026-09-24T13:23:44.309070+00:00。
同一测试命令结果：18 tests，18 pass，0 fail，0 skip，退出码 0。
`node scripts/verify-repo.mjs`：实际仓库结构检查通过，退出码 0。

RED/GREEN 两阶段均保持：
- 测试 SHA-256：`bf34d5341302c5f3e94e7dbfffc8032eca143e471fb19e01ed3ef27323437cd1`。
- 测试契约 SHA-256：`d9f7066a6764bfe4948030fb793bdaa3fe4d37a56824927ddd8c09d98f7727b4`。

可检出 RED 提交重现预期失败，再检出初始化完成提交重现 GREEN。无需私有生产数据或外部服务。

## CI 与边界

`.github/workflows/governance.yml` 已成功提交；只读权限，固定 action commit，Node 24 执行上述治理测试及结构检查。这里记录的是配置与本地结果，不预先宣称 GitHub Actions 已绿；以 PR 的真实运行结果为准。

没有产品构建、Go 测试、存储集成、浏览器 E2E、部署或恢复演练。没有修改仓库分支保护/必需审批设置。CODEOWNERS 与清单不等于服务端强制执行；结构检查也不能证明 Agent 真实阅读或 TDD 时序。
