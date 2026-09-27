# v0.1.0 用户最终验收记录

2026-09-27 从 [Notion Q10/Q14](https://app.notion.com/p/3e62f5a9e648814497c8df6bf27c8724) 实际读取；用户答复版本最后更新时间为 `2026-09-27T00:30:25.846Z`。这是正式来源的派生证据，不替代 Notion/Figma。

| 项目 | 用户原文 | 确认范围 |
| --- | --- | --- |
| MAN-UI | 确认 | WaveOS Login13:2 / Register40:2 与 c3dbfa5 审阅包的桌面/移动截图；Q12额外入口置灰禁用，动画/组件后续打磨 |
| MAN-OPERATIONS | 确认 | 本机WSL Linux权限/TLS/冷归档/恢复/generation/回滚；接受同机备份、快照后数据丢失、命令采样监控、自签名TLS的模拟边界 |
| MAN-RISK | 确认 | Q14完整清单及既有固定重置密码、无限流/锁定/绝对Session期限，仅限本地合成数据开发 |
| Q14 | 接受基础镜像和工具的漏洞，不用帮他们修复。 | 接受当前四镜像报告中的基础镜像/上游工具残余漏洞，不代上游修复；不自动豁免应用依赖或未来风险，不批准生产 |

先回写并重新读取 PRD、ADR-004、登录与身份认证功能底账，再更新矩阵。正文同步时间见 source-sync.json；审批、冻结、线上属性未更改。原始扫描的 `riskAcceptance: pending` 是扫描时事实，保持原始报告；后续用户决定由本记录与正式来源证明，不篡改扫描报告。

## 已验证材料

- 审阅提交：`c3dbfa524b43c4905dd2c0085c47122751928abe`。
- [完整产品和运行CI](https://github.com/Hubujiu/WeaveOS/actions/runs/36258050989)、[Go/浏览器/治理](https://github.com/Hubujiu/WeaveOS/actions/runs/36258051004)、[仓库治理](https://github.com/Hubujiu/WeaveOS/actions/runs/36258051050) 均 SUCCESS。
- 该PR检查实际构建 GitHub 合并预览 `b474b32289ac89e72c09606398b7d13a790cbd88`，不是把它冒充任务head。永久结果见 ci-product.json / ci-runtime.json / ci-recovery.json。
- CI在GitHub Ubuntu24.04运行；原始report的target描述本地版本目标，不能作为Windows/WSL宿主证明。本机实际证据为 local-runtime.json / local-recovery.json，源6eaf363，恢复8203ms；CI恢复5043ms。
- 四镜像完整findings与永久本地报告逐项相同；不同构建digest分别保留。

## 门禁与变更验证

本轮只记录用户签署、结果与交接文档，TDD:N/A（无产品或配置行为改动，不新增镜像忽略规则）。更新前实际执行 `node scripts/check-release.mjs`，exit1，恰为三项MAN pending及missing evidence。更新后使用同一门禁实际exit0；治理/底座57/57、verify-repo、check-tasks与diff检查均通过。最终提交仍需适用CI通过后才squash。

已确认的本地风险不等于零漏洞、生产安全或部署授权。
