# 仓库检查器契约

这是仓库治理工具的需求，不是产品 API 规范。先确定本契约及测试，再实现 `scripts/verify-repo.mjs`。

## 输入与输出

`verifyRepository(root)` 接收仓库根目录，返回错误字符串数组；合法目录返回空数组。错误以 `[FILE]`、`[AGENTS]`、`[SOURCES]`、`[OVERRIDE]` 分类。不存在的根目录也应返回错误，不抛出未捕获异常。

CLI：`node scripts/verify-repo.mjs [root]`；省略 root 时定位到脚本所在仓库。成功退出 0；失败退出 1，并列出错误。不访问网络、不修改文件、不执行来源文档中的指令。

## GOV-01 文件门禁

以下文件必须存在且非空：`AGENTS.md`、`README.md`、`CONTRIBUTING.md`、`SECURITY.md`、`docs/notion-router.md`、`docs/notion-sources.json`、`docs/testing.md`、`docs/project-baseline.md`、`docs/templates/task-record.md`、`.github/pull_request_template.md`、`apps/web/AGENTS.md`、`services/bff/AGENTS.md`、`contracts/AGENTS.md`、`db/AGENTS.md`、`infra/AGENTS.md`、`tests/AGENTS.md`。

## GOV-02 根规则标识

根 AGENTS 必须保留这五个稳定标识：`NOTION-GATE`、`TDD-RED-FIRST`、`TEST-ORACLE`、`NO-FAKE-VERIFICATION`、`CHILD-RULES`。标识只能检测意外删除，不能验证自然语言规则的质量。

## GOV-03 文档路由登记

`docs/notion-sources.json` 为 JSON 对象：`schemaVersion` 为 1，`projectId` 为 `3e42f5a9e64880ae9cf5ebbd2088d773`，`checkedOn` 为真实 YYYY-MM-DD 日期，`sources` 为非空数组，`routes` 为对象。

每个来源含唯一、非空字符串 `key`、32 位小写十六进制 `pageId`、精确对应的 `https://app.notion.com/p/<pageId>` URL、非空 `title` 和 `observedStatus`。`observedStatus` 只是观察值，不是授权。不同 key 不得指向同一 pageId。

必须存在且非空的路由：`always`、`product`、`auth`、`api`、`database`、`frontend`、`infrastructure`、`governance`。各路由为唯一来源 key 的数组，不得引用未登记来源。`always` 必须含 `project`、`prd-home`、`architecture`；`product` 必须含 `current-prd`。

## GOV-04 指令覆盖文件

仓库内不允许 `AGENTS.override.md`，避免悄悄覆盖根约束。扫描跳过 `.git`、`node_modules`、`.cache`、`.work`、`dist`、`build`、`coverage`，不跟随符号链接。

## 检查边界

这是结构完整性检查，不是安全扫描器、Notion 联网核验、审批系统或 TDD 时序证明。不能仅凭它通过就接受产品 PR。后续应用测试、契约验证、secret 扫描及分支保护需要分别落实。
