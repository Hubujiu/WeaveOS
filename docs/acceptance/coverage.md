# V010-009 · 测试先行覆盖与剩余门禁

本任务按用户要求先写测试，**不代表全部测试已完整落地或整版验收通过**。97个HTTP用例、19个存储/故障用例、35个浏览器场景（三浏览器105项）已编写。测试不实现认证业务；缺少fixture/观察器时明确BLOCKED，不skip。精确执行结果见[证据](../evidence/V010-009/results.md)。

来源是当前PRD、已接受ADR-001/002和2026-09-25用户“四项都保留”确认；详情见[来源与预期纠正](../evidence/V010-009/sources.md)。HTTP路径、成功DTO、UI路由和可访问名称仍是测试绑定提案，必须由002/006核对。没有从当前404响应反向生成expected。

## HTTP Given / When / Then

代码：[api.test.mjs](../../tests/acceptance/api.test.mjs)，仅真实HTTP；失败响应不打印凭据。

| 测试组 | Given / When | Then / 来源 |
| --- | --- | --- |
| HTTP-01 | 匿名读取当前用户 | 401、无数据、Session challenge；FR-007/API-14 |
| HTTP-02 | 外部user/role/tenant/Bearer/trace头 | 均不能建立身份；API-14/15 |
| HTTP-03 | 未知账号登录 | 401统一凭据错误、无Cookie、无密码；FR-001 |
| HTTP-04 | 登录/注册必填字段缺失或空串 | 400、public field violations、无Session；FR-001/003 |
| HTTP-05 | 三种损坏JSON | 400 COMMON_INVALID_ARGUMENT；API-08 |
| HTTP-06 | foreign/null/缺失Origin，无替代证明 | 403且不发Session；ADR-002§5.6 |
| HTTP-07 | text/plain、form、multipart | 不兼收为JSON认证，415；ADR-002§5.6 |
| HTTP-08 | 非法邀请码 | 400 INVITATION_INVALID；FR-003 |
| HTTP-09 | 四类字符各缺一类，真实可用码 | 拒绝弱密码、保留邀请码，合法重试成功；FR-018/003 |
| HTTP-10 | Aa1!四字符密码 | 注册并登录成功，不擅加长度下限；PRD注册规则 |
| HTTP-11 | 合法注册 | 201、Location、字符串ID、不创建Session、显式登录才成功；FR-017/API-05/11 |
| HTTP-12 | 重复使用已消费邀请码 | 409 INVITATION_ALREADY_USED，失败账号无法登录；FR-003/API错误表 |
| HTTP-13 | 八个账号争抢同码 | 1成功/7冲突，只有胜者能登录；FR-003 |
| HTTP-14 | 已存在账号+新码 | 409，码仍可由新账号消费；FR-002/003 |
| HTTP-15 | 同账号+两个独立码并发 | 一个账号胜出，失败码仍可用；FR-002/003 |
| HTTP-16 | 已知账号错密码、未知账号 | 相同公开失败提示，不泄露存在性；PRD登录规则 |
| HTTP-17 | Disabled账号 | 拒绝且不发Session；FR-010 |
| HTTP-18 | 连续12次错误后正确登录 | 不锁定、不429，正确登录成功；FR-013 |
| HTTP-19 | 已登录，连续恢复 | 同一用户ID与账号，无密码/SID；FR-005/008 |
| HTTP-20 | 登录、成功认证活动 | Cookie一小时并同步续期；ADR-001§3.4 |
| HTTP-21 | 已登录退出 | 204无正文、清Cookie、重放401；FR-009/API-05 |
| HTTP-22 | 退出与12次读取并发 | 退出后旧Cookie不可用；压力测试，不代替确定性交错证明 |
| HTTP-23 | 登录携带攻击者预设SID | 新Session不采用预设SID；ADR-001随机SID |
| HTTP-24 | SID通过Bearer/header或伪造query提交 | 不能替代Cookie；ADR-001/002 |
| HTTP-25 | 匿名生成码/重置密码 | 401，无管理能力；FR-012/016 |
| HTTP-26 | 普通用户附伪造ALL/Admin头 | 403；未授权重置不改变原密码；FR-012/016 |
| HTTP-27 | Bootstrap生成邀请码 | 201且能用一次，再次409；FR-016 |
| HTTP-28 | Bootstrap重置专用账号 | 不返回明文，旧密码失败，固定新密码成功；FR-012 |
| HTTP-29 | 管理员带合法Cookie/token，恶意Origin | 拒绝写入；API-14 |
| HTTP-30 | 有Cookie但无Origin/Referer/token | 拒绝写入；API-14 |
| HTTP-31 | 恶意来源退出 | 拒绝且原Session仍有效；API-14 |
| HTTP-32 | 两请求携带同一自选request-id | 服务端生成不同关联ID；API-15 |
| HTTP-33 | null/数字/对象/数组/布尔类型替代字符串 | 400并指出对应字段；API-09，绑定待002评审 |
| HTTP-34 | 注册写入管理员/权限/ID/未知字段 | 明确拒绝，不耗码，重试后仍是普通用户；ADR-002§8 |
| HTTP-35 | 匿名HEAD读取当前会话 | 401、无正文、challenge；API-05/14 |

所有普通JSON场景复用的断言覆盖精确四键、状态码、稳定业务code、message/meta形状、普通错误data=null、401 challenge及X-Request-Id。此检查**不是完整OpenAPI schema校验**。

## 存储和故障 Given / When / Then

代码：[integration.test.mjs](../../tests/acceptance/integration.test.mjs)。观察器接口与反伪造要求见[observer-contract.md](../../tests/acceptance/observer-contract.md)。当前观察器未实现，19项均BLOCKED；不声称已验证真实事务或TTL。

| 测试组 | Given / When | Then / 来源 |
| --- | --- | --- |
| STORE-01 | 新Session，读取Redis PTTL | 原生TTL为3600秒，容差只含实测耗时；FR-006 |
| STORE-02 | 缩短原生TTL再成功认证 | 恢复至3600秒；FR-006 |
| STORE-03/04 | CSRF失败/普通用户管理动作失败 | 原生TTL不向后延长；FR-006 |
| STORE-05 | 原生Redis过期后读取 | 401，不重建已过期key；FR-008 |
| STORE-06 | 创建时间一周前但当前TTL有效 | 不附加绝对最长寿命；FR-006 |
| STORE-07 | 独立用户登录后禁用 | 已有Session失效；ADR-001§3.4 |
| STORE-08/09 | 真实断开Redis/PG后读资源/登录 | 503系统错误，无受保护数据/新Session；PRD fail closed、API错误表 |
| STORE-10 | Redis断开后退出 | 不假报撤销成功；FR-009 |
| STORE-11 | 注册事务提交前注入故障 | 无用户、未耗码；解除后重试成功且恰好一份状态；FR-003 |
| STORE-12 | 八请求同码并发后独立SQL计数 | 一用户、一凭据、一消费，无部分写入；FR-003 |
| STORE-13 | 两个同密码账号 | 存储不含明文，哈希独立；算法/成本另待确认，不能据此证明完整密码安全 |
| STORE-14 | 成功和失败登录 | 审计含用户/IP/UA/时间/结果，审计和应用日志无凭据；FR-014 |
| STORE-15 | 备份→退出→正式恢复流程 | 已失效Session不复活；PRD发布回滚规则 |
| STORE-16 | 读请求暂停touch→退出删除→恢复touch | 不重建key，旧Cookie不可用；FR-009 |
| STORE-17 | 管理员重置独立账号后读原始状态和日志 | 不含旧密码/固定新密码/SID；FR-012/014 |

## 浏览器覆盖

[web.spec.ts](../../tests/acceptance/web.spec.ts)保留8个原始需求场景并补27个参数化场景，覆盖：登录/注册字段与label；显示/隐藏密码；四项表单与确认密码不匹配；每项必填；四类密码缺失；密码强度反馈；邀请码URL填充、一次解码、可编辑、文本安全与真实服务端复核；无找回流程；匿名路由；错误凭据；注册不登录、密码重新输入；刷新恢复、安全Cookie、存储不泄露、退出；Enter登录；登录/注册挂起防重复；网络故障与凭据失败区分；Disabled用户、重复账号修正后重试、已用邀请码、Redis过期返回登录与前端日志去敏。

挂起测试只延迟后继续真实请求，故障测试使用网络abort；没有route.fulfill伪造业务成功。trace/screenshot/video关闭，但未来有真实fixture时仍需007审查错误上下文与reporter去敏，不能把凭据场景报告直接上传。

## 仍不能称为完整的项目

| 缺口 | 原因与下一步 | owner |
| --- | --- | --- |
| 逐接口全量schema/错误码/DTO、正式3.2.1工具兼容与差异检查 | contracts尚无已评审接口；不写空schema假通过 | 002 |
| 账号大小写/首尾空格/Unicode/长度界限 | Notion明确待评审，尚无本次确认 | 产品+002/003 |
| 物理字段、约束/索引/外键、迁移空库/升级/回退 | 数据模型仍待评审，未读字段记录不冒充已核对；评审后逐记录下钻 | 003 |
| 精确3599999/3600000ms边界、确定性交错退出/续期 | 需004真实存储和受控时钟/调度屏障；当前PTTL与并发压力不代替 | 004 |
| 重置撤销所有旧Session、重新启用不复活、重置/登录竞态 | 属新增待评审策略，不能当已接受规则 | 产品+004/005 |
| Seed幂等、不覆盖密码/不提升同名普通用户 | 数据设计建议尚未接受，未编写草案SQL实现 | 003 |
| 哈希算法/参数、邀请码摘要、完整存储字段去敏 | 安全目标已明确，具体存储规范需003评审；STORE-13仅部分断言 | 003/005 |
| 审计保留/清理周期、应用日志所有失败分支去敏 | 周期未决；现有STORE-14只测登录成功/失败，不冒称全日志覆盖 | 产品+005 |
| 浏览器已写场景的真实全栈执行、完整跨页故障恢复 | 需独立fixture、HTTPS和前端；不能把缺口改成人工项 | 006/007 |
| 同源HTTPS、真实观察器/fixture、上述测试CI装配、reporter去敏 | 当前默认8080/4173仅底座；产品依赖未实现 | 007 |
| 生产TLS、备份RPO/RTO、安全债务签署 | 需授权目标环境与负责人，不代签 | 008 |

FR-015为范围约束：不凭空增加Native、SSO、设备中心、权限平台测试。没有真实跨服务RPC，API-16..19中的RPC互操作不擅自变成当前实现任务。人工设计验收仍需负责人。

## 执行与隔离

每轮由003产生新fixture，user/admin/disabled/resetTarget/disableTarget/storageResetTarget必须互相隔离。resetTarget不能预先使用固定重置密码。uiInvitations每浏览器独立，并额外提供`${project}-reentry/-duplicate/-reuse/-pending`，不可跨用例共用一次性码。API新场景通过真实Bootstrap生成自己的邀请码，不复用其他测试消费过的数据。接入真实故障注入/恢复后必须串行执行文件与浏览器，或为每worker提供独立全栈实例，禁止一边全局断开依赖一边执行另一文件。

```sh
node --test --test-concurrency=1 tests/acceptance/api.test.mjs tests/acceptance/integration.test.mjs
pnpm exec playwright test tests/acceptance/web.spec.ts
```

fixture放在忽略的.work目录，未授权远程目标不运行可变更数据的测试。当前acceptance.yml只调用api.test.mjs；007接手必须显式加integration.test.mjs和真实observer，不得声称本PR已经启用全部CI验收。
