# V010-003 数据来源与测试预期审查

2026-09-25 12:05 Asia/Shanghai，使用授权 Notion 连接器读取 [当前 PRD](https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd)、[ADR-001](https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93)、[ADR-002](https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a)、[ADR-003](https://app.notion.com/p/3e52f5a9e6488111adbbe968326a23f0)、[数据库入口](https://app.notion.com/p/3e52f5a9e64881cb8c39c16e2f3c4c53)、[版本设计](https://app.notion.com/p/3e52f5a9e64881e2b759d67ef982dd01)、[ER](https://app.notion.com/p/3e52f5a9e648815ebf88ccc3aa9a5bac)、[DDL/事务](https://app.notion.com/p/3e52f5a9e6488155bfcef8a7cfde4550)、[Redis Session](https://app.notion.com/p/3e52f5a9e6488184b345d41309068308)，查询对象与字段数据源，并逐页读取 5 个对象、40 个字段记录。来源均未提示截断/未知块。PRD 仍需求编写中；ADR-001/002 已接受，ADR-003 拟议；数据库记录状态为设计待评审。

本轮用户批准数据库设计、DDL 与 Redis Session 草案，明确修订账号规则：去首尾空格，拒绝中间空格，不转小写；`Alice` 与 `alice` 为不同账号。重置后旧 Session 全部失效。审计保留期未定。这个明确修订优先于原草案中的 `lower(account)` 生成列、`lower(btrim(input))` 登录查找和以小写账号生成的未知账号指纹；如采用 `account_key`，其值须保留大小写并由数据库唯一约束保证。服务端同样拒绝不合法空格，不能只靠前端。

## 待写的独立用例映射

| 来源 | 隔离测试必须观察的结果 |
| --- | --- |
| PRD FR-002、用户账号修订、字典 `auth.users.account/account_key` | 首尾去空格后的同大小写账号重复注册被唯一约束拒绝；`Alice` 与 `alice` 可分别存在且精确查找；账号内部空格、控制字符、越界长度被拒绝。 |
| PRD FR-003、DDL 第 4 节 | 同一码 20 个并发事务最多一个成功；账号冲突、凭据/审计插入失败回滚整个事务，失败者不消耗邀请码。 |
| PRD FR-016、DDL 第 7/8 节 | Bootstrap Seed 重跑不改密码、版本、状态；同名普通用户不提升；最多一个 Bootstrap Admin。 |
| PRD FR-014、字典认证事件 | 匿名失败可无用户外键；登录事件保留 IP/UA/时间/结果，密码、邀请码原文、完整 SID 不落库；运行身份只追加事件。 |
| 用户确认重置撤销、设计 `auth_version` | 密码重置与版本递增同一事务；旧版本 Session 不被接受；不把旧密码验证后的登录签发为新版本。 |
| DDL 第 8 节 | 空库迁移后用 catalog 验证字段、约束、外键、索引、运行时权限；破坏性 Down 仅在隔离可销毁库验证。 |

上表是来源到预期的映射，尚未写或执行测试，不能算 RED。技术选型确认与真实 PG 环境就绪后，先保存测试源码/基线快照，运行目标断言失败，再写对应实现。来源审查文本 TDD:N/A；不涉及行为实现。
