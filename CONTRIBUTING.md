# 贡献流程

进入任务先执行 [AGENTS.md](AGENTS.md) 和 [Notion 路由](docs/notion-router.md)。Notion 负责需求和决策，Git 负责实现与可执行契约；不要维护第二套手抄 PRD。

## 一个行为变更的最小闭环

1. 从当前 main 建立 `feat/<topic>`、`fix/<topic>` 或 `chore/<topic>` 分支；先检查工作区，不覆盖他人未提交内容。
2. 主动读相关 Notion 正文，创建 `docs/tasks/<task-id>.md`，记录来源、范围、验收用例和不做什么；未决问题阻塞相应行为。
3. 先写测试并实际 RED，提交测试或保存不可变测试快照。再最小实现到 GREEN，最后重构并回归；细则见 [测试规范](docs/testing.md)。
4. 更新本次涉及的 OpenAPI / Proto / 错误码 / migration。依据变更需先完成 PRD / ADR 评审，不能只把当前代码翻译成文档。
5. 执行适用验证、检查完整 diff 和敏感数据，使用 [PR 模板](.github/pull_request_template.md)。只 stage 本任务文件，不盲目 `git add -A`。
6. 通过审查和实际检查后，由有权限且获得授权的人合并。不要自动上线、自动改 Notion 审批状态或把“测试全绿”当成用户验收。

提交信息使用 `docs:`、`test:`、`feat:`、`fix:`、`refactor:`、`chore:`。保留可审计的 RED → GREEN 历史；若合并策略压缩提交，必须在任务/PR 中保留可访问的不可变 RED 证据，不能编造时序。

## 规则变更

修改根/子 AGENTS、TDD、来源权威规则、门禁或范围边界属于治理变更，需要明确说明原因及影响。不得在修业务时顺手弱化测试或审批；申请豁免不等于已获豁免。

`CODEOWNERS` 和 PR 清单本身不会开启强制审批。仓库管理员仍需配置 main 分支保护、必需检查及敏感规范文件的审查要求；本次初始化不声称这些服务端设置已启用。
