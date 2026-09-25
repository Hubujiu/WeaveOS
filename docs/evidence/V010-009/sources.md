# 来源审查与独立预期

读取时间：2026-09-25，本任务连续执行，时区Asia/Shanghai（+08:00）；最终记录时11:22。通过已授权Notion fetch逐页读取，不使用离线快照替代实时读取。原始工具响应带page_last_edited_at；均返回闭合page/content，没有truncated/unknown_block标记（这些计数字段未提供，不把未提供写成0）。工作台verification=unverified不等于ADR未接受；决策状态来自页面properties及正文。

| 来源 | 来源更新时间（UTC） | 实际状态/适用内容 |
| --- | --- | --- |
| [项目](https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773) | 2026-09-24T12:20:56.215Z | 正确项目，PRD/Architecture/Database入口 |
| [PRD入口](https://app.notion.com/p/3e52f5a9e6488037975bc303bee464b5) | 2026-09-24T05:09:24.618Z | 底账与迭代分工、DoR/DoD |
| [当前PRD全文](https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd) | 2026-09-24T12:15:05.462Z | v0.1.0，需求编写中，未冻结；DoR已勾，当前基线FR-001..018，DoD未完成 |
| [Architecture](https://app.notion.com/p/3e52f5a9e64881b9b005dc78a252c08f) | 2026-09-24T12:15:20.999Z | 001/002已接受，003/004拟议，数据待评审；不改状态 |
| [ADR-001全文](https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93) | 2026-09-24T09:52:34.410Z | 已接受；Go本地认证、Redis、Cookie、3600秒滑动、禁用/退出失效、Bootstrap |
| [ADR-002全文](https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a) | 2026-09-24T10:26:13.400Z | 已接受且补充不替代001；四键、错误表、Session challenge、CSRF、DTO拒绝未知写字段、OpenAPI3.2.1 |
| [登录底账记录](https://app.notion.com/p/3e52f5a9e64880a3aaa1caa116d73996) | 2026-09-24T02:37:59.106Z | 迭代改造中；正文blank，不假称已读线上规则 |
| [身份认证底账记录](https://app.notion.com/p/3e52f5a9e64881428779fb73a2c8fc51) | 2026-09-24T02:49:47.628Z | 迭代改造中；正文blank；测试以当前PRD和已接受ADR为依据 |
| [Database入口](https://app.notion.com/p/3e52f5a9e64881cb8c39c16e2f3c4c53) | 2026-09-24T13:03:20.178Z | 设计待评审；未部署 |
| [数据设计全文](https://app.notion.com/p/3e52f5a9e64881e2b759d67ef982dd01) | 2026-09-24T12:06:01.239Z | 物理模型、账号规范化、重置撤销与审计策略为拟议，停止相关物理实现 |
| [Redis设计全文](https://app.notion.com/p/3e52f5a9e6488184b345d41309068308) | 2026-09-24T12:17:34.288Z | key/字段/Lua/版本撤销方案拟议；测试不锁定这些字段或脚本 |

ADR-003/004仅从Architecture核对状态，**未读取各自全文**，未引入候选库或运行部署配置。数据对象/字段、ER/DDL记录本轮未继续下钻：设计未评审且本任务未实现物理观察器/迁移，不能声称这些已审核。恢复对应实现时必须逐记录读取。

Figma实际读取了[登录26:423](https://www.figma.com/design/9UE263yT1anH3KG9qBjXiT?node-id=26-423)及[注册26:439](https://www.figma.com/design/9UE263yT1anH3KG9qBjXiT?node-id=26-439)结构元数据；注册节点显示确认密码但未列邀请码。未声称已完成画布视觉验收。用户本任务明确答复“四项都保留”，因此测试按账号、密码、确认密码、邀请码；不修改Figma原件或Notion审批。

组件库实际读取[AGENTS](https://github.com/Hubujiu/React-/blob/main/AGENTS.md)、docs/consumer.md、docs/contract.md与批准记录顶部范围，列出registry相关输入/按钮登记。42组件获批不代表本任务已逐项验收采用；本任务未消费或修改任何组件，也未把原型组合当作已批准实现。

## 纠正旧测试的依据（先于本轮RED）

1. 已使用邀请码与同码并发失败从400改为409、INVITATION_ALREADY_USED：ADR-002§5.2明确规定，旧api.test.mjs断言有误；不是根据实现调整（当前实现只有404）。
2. 取消“任何登录JSON必须返回csrfToken”“同源写入缺token一定403”的强制策略：ADR-002§5.6默认来源校验+SameSite+Content-Type；只有需要的跨源/来源不可验证路径补token。改测恶意Origin以及无任何来源/CSRF证明。http.mjs可读取可选token仅是待评审适配器，不把数据库草案双提交方案当既成决策。
3. 密码强度由强制progressbar改为可访问的密码强度反馈：PRD要求反馈但未指定DOM role；可访问名称仍为006待确认绑定，不用DOM快照自证。
4. 注册填写确认密码：用户本任务明确保留四项，新增不一致拒绝场景。

## 未决问题

账号规范化/长度、重置撤销所有旧会话、审计保留策略已通过异步问题提示用户，截至交付尚未获得决策答复，保持BLOCKED。不把等待时间当批准。完整缺口及owner见[覆盖表](../../acceptance/coverage.md)。
