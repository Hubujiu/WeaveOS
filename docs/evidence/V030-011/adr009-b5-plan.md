# Source: ADR009 actual readback

## 2026-10-03｜B5 应用创建与权限组存储／HTTP 接线 PLAN（入口已确认，有限实现放行）
**目标与状态**：打通“创建应用 → owner 管理权限组／成员／完整 grant → 成员进入应用”的有限纵向链路，包含真实存储与 HTTP。用户已确认入口规则，本 PLAN 回读后可放行对应的独立本地实现与隔离验证；没有完整 records／Flowable、逆审批或 owner transfer 的新增授权。B4b 局部通过不等于 B5 已完成，整页仍待评审。
### B5.1 应用入口与创建权限分配（用户已确认）
主负责人问题 Sentinel_043b28ab93388191ac6cf09dca309027：owner 将成员加入权限组并勾选菜单后，是否自动取得该应用入口，无需另经全局身份／模板再授予入口？
用户 Sentinel_66026e5820bc81918229633c34f431bd 于 2026-10-03 01:51:09 UTC 对上述唯一待确认问题回复“确定”；主负责人已通过 Sentinel_0daeff4c28c48191b9d1f5be883208b8 复述确认。本次确认的执行语义如下：
- 普通成员在同 app 拥有有效 membership，且至少有一个有效菜单 grant，即可发现并进入该 app；只可进入被授予的菜单，不自动获得数据动作或字段能力
- 仅有 membership、没有菜单 grant，不授予 app 入口；owner／Bootstrap 本身具有完整应用能力
- 不再增加中央身份／template 的第二次入口 grant 门槛；app catalog 仍用于存在性、可用性与服务端资源归属校验，不是额外身份授权步骤
- 创建 capability 的分配沿用既有 personnel.manage／权限模板规则；create-only 不包含委托全局 capability 的能力，也不授予别人应用的内部全权
其余技术契约由主负责人负责核实与冻结；执行者按本 PLAN 实现，不自行扩展未批准业务。
普通委托管理、owner transfer、删除权限组的影响、未来资源继承及候选成员搜索范围均未纳入本阶段，不在实现中补选默认规则。
### B5.2 集成基线、任务与文件所有权
- 候选任务号 V030-011，开工前必须再次查询唯一性；这里没有宣称已占用或已创建执行任务
- 指定集成基线：[PR #21](https://github.com/Hubujiu/WeaveOS/pull/21) fixed 74cc5824ee4336dd76c72874f0cc4cf38fe9e889；[B0 PR #22](https://github.com/Hubujiu/WeaveOS/pull/22) head 5571420c8a50fe41909d75790509b9e5b6b5dbd5；[B1 PR #23](https://github.com/Hubujiu/WeaveOS/pull/23) head 67f19c03218568a08b5dafd829c9d3db39f301a4；B4b local 2e600d606bb5127065c2efb819f8680cc789b1e5
- 仅在独立本地集成分支普通 merge，禁止强推，不改原分支或 main；没有取得本阶段新 PR／push 授权
- B5 独占新 `internal/applications` 的 usecase／store／HTTP，以及本次必要的 `personnel/access` 事务 capability gate、auth dispatcher／config、OpenAPI／错误码／测试、`roles.sql`、升级 pin／compatibility manifest 和冷热迁移
- 共享文件由 B5 唯一 owner 按主负责人冻结契约修改，其他包不并发抢改；不能直接复用 `personnel.AuthorizeWrite` 并假定其人员域前提适用于应用域
### B5.3 迁移空位与兼容约束
主负责人已独立读取目录纠正基线：main 热库目前仅 00001／00002，但 PR21 fixed 74cc5824 的热迁移 00003 drafts、00004 writers、00005 presets 已占用。
- 热库预留 `00006_apps_policy.sql`，不可再使用热库 00003，不修改旧迁移
- 冷库仍为 00001／00002，预留 `00003_apps_audit.sql`
- 冷 00003 先于热 00006；迁移编号与内容在实际开工前再次核对，保留历史 schema 升级夹具的意义
- 完整保留 PR21 的 roles、drafts／presets／revision 权限及 compatibility 记录，不能由旧 main 基线覆盖新权限
- 新 roles／migration hash 与兼容清单由主负责人复核登记；相关批准仅用于代码和隔离测试，不授权生产迁移、角色授权或部署
### B5.4 存储与受控 capability 草案
- 关系表覆盖 apps、permission groups、members 和完整 grant tuples；apps 的 `owner_user_id` 固定，`policy_revision` 独立
- group、member／grant 及资源引用使用同 app 的复合约束，不能只验证单个 ID 存在而允许跨应用拼接
- 独立创建 capability 的拟议编码为 `applications.create`，中文“应用管理”，category＝system、appID＝null；通过精确 catalog whitelist 与新迁移声明。该编码仍是草案，不声称已经注册或发布
- 创建权限分配沿用既有 `personnel.manage`／权限模板规则已获确认，不改成 root-only；Bootstrap 拥有全部权限。create-only 不授予别人应用内部全权，也不授予委托全局 capability 的能力
- actor 由 Session、账号状态与 auth_version 等既有可信链取得，保留 Origin／CSRF 校验；新应用 UUID 由服务端生成，owner＝actor，不允许客户端指定他人为 owner
### B5.5 创建原子性与幂等
- 创建 operation 的 scope 绑定 actor＋幂等 key＋canonical payload hash；重复操作与同 key 不同 payload 必须按冻结契约区分
- app、中央入口登记、审计和 operation result 在同一事务提交，任一失败不能留下部分创建结果
- commit 结果未知时核查原 operation，不盲目重试或误报成功／确定回滚；结果查询绑定当前主体，不能拿到别人的 operation 结果
- 不把权限或创建结果复制为 Cookie／Session 的长期缓存；既有会话与权限事实源继续有效
### B5.6 权限组写入、快照与锁顺序
- 组成员／grant 写入携带 `expectedPolicyRevision`，CAS 冲突返回 409；members／grants 采用明确的完整替换语义，不将遗漏字段解释成悄悄追加或保留
- 事务内复检当前授权，经共同的 app 写入门禁协调组变更；短只读 RR 事务构造一致 policy snapshot
- app policy revision 与既有 auth_version／personnel version 分开，不把它们混成一个全局策略版本
- 严格遵循 PR21 写锁顺序：query revision locks → 账号／授权／依赖 → app policy 锁 → catalog registration。禁止先 INSERT catalog 触发 revision，再倒序获取前面的锁
- catalog runtime 当前只有 SELECT；写入登记必须采用受限登记函数或等效最小权限实现，具体方案由主负责人冻结，不能授予任意 catalog CRUD
### B5.7 既有审计与冷热兼容
- 沿用既有 15 列审计结构，扩展 event type／object summary 的约束；不新增列，避免冷热 JSON 的 NULL key 不等导致复制校验差异
- 保留 archive 的复制／校验／commit 后删除 hot 算法，失败保留 hot 数据，不为新事件绕过验证
- 真实 roles、备份／恢复和 upgrade checks 只作用于代码与隔离测试；不得应用到生产环境
### B5.8 拟议 HTTP 范围
本轮路由范围为 `/api/v1/applications`，以及嵌套 permission-groups／members／grants 与 access。入口判定按已确认 B5.1 实现；正式方法、DTO 与错误码由主负责人按本范围冻结，不由执行者增加额外入口门槛或业务动作。
不新增 records 写入、审批、反审核、反完成或其他截图动作。本阶段也不实现委托管理、所有权转移、未来资源继承、未定义的组删除影响或候选成员搜索范围。
### B5.9 RED／GREEN 与集成验证计划
在真实临时 PostgreSQL／Redis 上验证 HTTP＋Session＋CSRF，使用合成账号／数据；未开始的测试不能写通过。
- create-only、Bootstrap、owner、非 owner、跨 app／伪造资源归属的允许与拒绝；有效 membership＋菜单 grant 可发现／进入且仅授权菜单，membership 无菜单拒绝入口，无中央身份／template 二次 grant；数据动作仍独立校验，create-only 不可委托全局 capability
- app／entry／audit／result 原子失败回滚；幂等重复、payload 冲突与 commit unknown 的原 operation 核查
- 完整 grant OR，不把行范围／动作／字段拆成笛卡尔积；首版无下属能力
- membership 撤权与写入并发、expectedPolicyRevision CAS、事务内授权复检和一致 policy snapshot
- 热旧 00001–00005 回归、冷热升级兼容、角色最小权限及 compatibility／hash；保留旧 drafts／presets／revision 能力
- 冷审计约束先升级、新事件冷热复制校验、备份／恢复及归档失败保留 hot
- 实际 API 错误和退出状态可见，不删测试／skip／放宽 guard 过关；记录真实 RED／GREEN、未运行和阻塞项
### B5.10 放行条件与交付
入口规则已获用户确认，本 PLAN 回读后可按限定纵向范围启动独立本地实现及隔离测试。开工仍须核实任务号唯一性、指定集成基线和迁移空位；具体契约由主负责人协调冻结，未定依赖只暂停受影响部分，不允许执行者自行选新业务。
执行者交付本地 SHA、基线与 merge 记录、代码／测试范围、事务与锁顺序证据、权限／冷热回归结果及未实现项。新 PR／push 不沿用旧四包授权，merge／deploy／生产权限变更仍禁止。
**当前状态**：B5 入口规则已确认，完成本 PLAN 回读后可放行有限存储／HTTP 纵向实现；尚无 B5 完成或验收结果。B4b 只提供有限 evaluator 基础。暂不 push／创建新 PR，不执行生产 DDL／授权变更、merge 或 deploy。整页 PRD 待评审、ADR-009 拟议中保持，用户尚未接受整版 v0.3.0。
