Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14 as of 2026-10-03T14:47:12.842Z:
<page url="https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14" icon="📐">
<ancestor-path>
<parent-data-source url="collection://235f72c0-b039-40d8-9dcc-7abff28b9406" name="架构决策 ADR"/>
<ancestor-2-database url="https://app.notion.com/p/f4b3520a9eb84c8280f33f35142ca388" title="架构决策 ADR"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e64881b9b005dc78a252c08f" title="Architecture & ADR"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"ADR 标题":"V030-015 ADR｜记录权限、查询算法与草稿事务审查","ADR 编号":"","date:决策日期:is_datetime":0,"date:决策日期:start":"2026-10-03","url":"https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14","关联迭代":["https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a"],"决策领域":"数据","影响范围":"模块级","最后更新时间":"2026-10-03T14:47:12.842Z","状态":"已接受"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"📐"}</iconMetadata>
<content>
**A 算法及 record／query／draft 核心合同已冻结；RuntimeView、quickSearch、记录保存历史与共享核心修正见第11节，Q36 抽取范围见第12节。** 中性抽取已通过主负责人源码审查；第13节冻结 records consumer 共同基线和限定接线计划，并更新早期端口等待状态。compact-A 仍仅隔离实验，百万 full-A 容量问题开放；用户四项业务问题保持独立，本任务尚未产品验收。 V017 消费所需的 draft 最小写回执、普通引用候选、history 显示及私人预设依赖已在第14节冻结，相关旧DTO表述以第14节为准。
任务 PRD：<mention-page url="https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a"/>。前置精确结构合同：<mention-page url="https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f"/>。总架构：<mention-page url="https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f"/>。
## 1. 已定边界
复用 V030-013 字段规范化、稳定 ID、真实物理类型表与同表 gate。业务行保留 id、不可变 createdBy、recordVersion、createdAt、updatedAt；wire 按 fieldId 映射值，存储不变成 JSONB／EAV。
写入＋audit＋operation result 一个事务；未知结果恢复原 operation。为未来审批节点 Save 提供 caller-owned 事务端口，节点保存不推进流程。流转 pending record fence 接口必须明确失败／阻止，不能缺省允许或绕开。
## 2. 权限编译约束
完整 resource／action／row predicate／field mask tuple OR，先服务器权限，再用户 filter，再 COUNT／page。字段 mask 在同一记录与 action 命中的 grants 内合并，禁止跨组笛卡尔积。owner／Bootstrap 不绕过资源存在性与业务状态约束；own＝immutable createdBy，无下属、deny、目录隐式继承。
没有字段权限时，不能通过 filter、sort、count 旁路观察字段。隐藏列只改变显示，不缩小既定鉴权或查询语义。精确授权与接口以第8节冻结合同为准；按 form view 授权、投影不随布局／隐藏缩小、filter／sort scope 不覆盖时整请求 403。
## 3. 初始算法审查要求（选择结论见第8节）
需求：嵌套 AND／OR；字符串仅等于／不等于；数字／时间比较和排序；稳定 ID 次序、任意页码；相关变化才触发刷新并回第一页。沿统一表格抽象，不另建适配层。
执行者须先提交清晰的 journal 方案或既有 engine 泛化方案，说明：
- 怎样判定行值、筛选成员、排序、权限和引用变化与当前查询相关，怎样证明无关变化不全表 invalidate
- 生命周期、版本／事件与查询 context 的一致性、并发和失效边界，不能依靠客户端装下完整结果
- N 行、条件数、匹配数、页深、活跃 context、事件留存等实际维度下的时间／空间复杂度；明确最坏内存界限和持续增长项
- SQL 与索引计划、COUNT／页查询／引用解析成本、百万量级与深页实测方案；不把 OFFSET 当常数时间，不只用一次快样本定案
- 与现有人员查询的语义复用／安全隔离方式，避免把动态字段绑死旧人员白名单
主负责人已选择 A 的完整授权投影指纹方案，B journal 未选、不实现。下述初始比较要求与历史分析保留；当前执行和规模验收以第8节为准，不宣称已有通用 engine 或已证明百万性能。
## 4. 引用与一致快照
沿已定 E 轻量 registry＋tombstone＋PF：同一只读 RR snapshot，无引用谓词时 count／page 后批量解析当前页去重引用；引用条件在 COUNT／LIMIT 前按真实 registry 过滤。避免 N＋1、读时逐行缓存写入或偷偷引入实验全局锁。
真实 source hooks、display label 和多部门投影必须引用实际人员代码来源并提出明确合同。B4a 只有内部模块，不能视作真实人员 HTTP／hooks 已完成。源删除保留最后显示、不复用 ID，同名账号不能继承旧引用。
## 5. 草稿事务
显式保存，owner 取可信 Session 主体，绑定实际资源、schemaVersion、baseRecordVersion、draftVersion；不保存 Session 凭据。草稿保存不改变正式行也不触发流程。提交正式记录与消费草稿同一事务；失权、schema／record 版本冲突、未知结果与重放必须有明确合同。
不自行确定永久删除／恢复政策；记录删除留独立业务范围决定，新增／编辑／查询计划继续。
## 6. 文件 owner 与本阶段输出
本任务新模块为 `apprecords`、`appquery`、`appdrafts`、`appaccess`，真实完整路径在代码核查后登记。修改 `apppolicy` 须协调唯一 owner。OpenAPI、errors、migrations、roles、BFF 共享入口归 V030-013 集成 owner；V030-015 先交增量合同／迁移提案，不并发写这些文件。
初始审查产物已形成，主负责人已按第8节冻结核心 DTO／方法／错误与算法。回读后可执行限定核心；共享文件仍交 V030-013 owner，快搜和业务值 history 只提合同待审。
## 7. 后续验收要求与风险
真实 PG 的授权／跨账号／app／字段；CAS／未知提交；相关与不相关变化；百万级性能和内存；草稿无副作用及提交消费原子性；源删除旧引用；缺失 fence fail-closed。所有测试须区分未运行／失败／skip／通过，实验与正式接线分别报告。
本任务核心按第8节冻结，待审扩展不纳入自动放行；不得创建新 PR、合并或部署，不将任务技术冻结称作完整 v0.3.0 完成。
## 8. 2026-10-03 主负责人冻结：record／query／draft 核心
依据已完整阅读的 [2da21b2a 合同与算法提案](https://github.com/Hubujiu/WeaveOS/blob/2da21b2a/docs/evidence/V030-015/contract-and-algorithms.md)，主负责人选择 A 算法，并冻结以下方法、DTO、错误、授权和草稿语义。本节覆盖原提案中的 DECIDE、PUT record／draft、混合字段静默排除行、节点白名单强制交普通 edit 等旧候选。固定来源链接仍是提案历史，不声称已含这些修正。
**有限放行**：本页读回后由主负责人释放已冻结 record／query／draft 核心。快搜 DTO 与业务旧值／新值 history 合同仍待另行审查，不阻塞其余核心，也不视为已实现。用户待答的流程／导航四项保持独立。
### 8.1 查询选择与复杂度
采用 **A：共享 Q36 生命周期与同一 RR 快照，完整获授权投影指纹在观测版本变化时重算**。提取共同 criteria/context／Redis CAS／Session 绑定／显式刷新生命周期；应用域按 V030-013 动态字段编译类型化 SQL，前端继续使用统一 Table，不增加旧人员适配层。B 旧值查询 journal 未选，不实现其留存／delta 重放。
规范投影 P 包括全部匹配且可读的记录 ID、确定顺序、COUNT、该物理 table 当前所有获授权字段的值及必要引用显示；**不因 form layout 或 hiddenColumnIds 缩小**。同一 table 的多个 form 共用物理记录，权限资源仍按实际 form view。
- 初次查询建立完整投影指纹。数据／policy／schema／相关 registry 观测版本变化后，在同一 RR 中有界流式重算 digest＋count；只有实际 P 改变才返回 QUERY_CHANGED，提示显式刷新并回第一页
- P 未变则 CAS 推进 context 的观测版本，不误报刷新；版本未变走既定页查询路径。policy 丢失立即按当前权限拒绝，context 丢失另报过期，不能拿旧投影当授权
- 全量指纹由服务器流式计算，不把完整结果保存在浏览器或为每个 context 缓存全部行
- 同表无关写入可能触发重算，但不能无条件要求用户刷新；当前页以外的匹配、获授权字段变化仍可能相关
- 同 RR 批量引用解析遵 E＋PF／tombstone，引用谓词在 COUNT／LIMIT 前。缺失源版本 hook／registry 不能猜 unchanged
设 N 为表行数、M 为匹配获授权行数、W 为投影行字节、F 为字段／条件规模、B 为有界 stream batch、p 为页长、o 为 offset、C 为活跃 context。初始／变更检查的投影传输为 O(M×W)，数据库还承担 scan(N,F)＋sort(M)；应用工作内存目标 O(B×W＋p×W＋F)，context 元数据 O(C×criteria)，不保存 O(C×M) 全结果。PG 排序／hash 仍可能使用 O(M) 内存或磁盘临时空间，不能把应用 RSS 当全部查询内存。
可用有序索引时深页仍有 seek＋o＋p 成本；COUNT 通常扫描候选／索引，动态无索引排序可能接近 O(N log N)。无关写入后 C 个 context 各自再访问可能产生 C 次完整验证。百万规模性能尚未证实。
**强制实测**：实施后测 N＝10k／100k／1m、C＝1／20／200，选择性与宽查询、own／all、0／1／20 条件、p＝20／100、浅／深页、变更率与引用扇出。分别报告 COUNT、page、完整 rehash、EXPLAIN／BUFFERS、内存、临时空间、索引／WAL、重复样本分位数与锁等待；主负责人独立审查。不得扩大 timeout 掩盖失败或声称深分页常数时间。
### 8.2 权限与投影边界
- data grants 的 resource 是真实 **form view**；record 仍是共享 table 行。owner／Bootstrap 全支持能力仍要求资源存在；普通 grants 只合并同 app 完整 resource/action/row/field 元组
- own＝不可变 createdBy。无 subordinate、explicit deny、隐式目录继承；字段 mask 仅在同 record／action 命中的 grants 内 OR
- 没有任何可读字段的行不进入 COUNT／page。GET 单行无 read 或跨 app record 返回 404；无入口权限返回 403
- filter／sort 所需字段的可读 row scope 必须覆盖全部 visible-row scope，按 all／own 谓词覆盖关系检查；否则整请求 403。不能为了评估秘密字段或避免报错而静默排除本应可见的行
- 先权限，再用户 filter，再 COUNT／page。SQL 不能把未授权值送到 Go 或客户端作筛选／排序；优化器重排不能绕过字段权限
- data.create 支持明确空 field mask 的 defaults-only create；客户端提交的字段仍必须在有效 create mask 中。沿提案本切片 create grant 使用 all scope，新 createdBy 固定为 actor
- 普通 POST／PATCH 重新检查当前权限、schema／record CAS、reference 与 fence。node Save 后续通过独立 task-scoped 可信授权端口验证实际任务与节点白名单，**不擅自增加“节点白名单 AND 普通 data.edit”业务门槛**；两种入口共同遵守 fence／CAS／同事务保存
### 8.3 HTTP 方法与明确例外
路由除 operation 外均相对 `/api/v1/applications/{appId}`。viewId、实际 tableId、owner 从真实资源加载，不信任 body。沿 ADR-002 envelope、meta.requestId、no-store、Session／Origin／CSRF；UUID canonical lowercase，严格 JSON 拒绝未知／重复键与错误形状。
- POST `/forms/{viewId}/records`：RecordCreate → **MutationResult，201＋Location**
- GET `/forms/{viewId}/records/{recordId}`：Record，200，仅当前可读字段
- **PATCH** `/forms/{viewId}/records/{recordId}`：封闭 RecordEdit JSON → **MutationResult，200**；取消原 PUT 候选，不采用任意 JSON Patch
- POST `/forms/{viewId}/records/search`：RecordSearch → RecordPage，200
- POST `/forms/{viewId}/drafts`：DraftCreate → Draft，201＋Location
- GET `/forms/{viewId}/drafts`：pageSize／pageToken cursor query → DraftSummary 数组，200＋meta.pagination
- GET `/forms/{viewId}/drafts/{draftId}`：Draft，200
- **PATCH** `/forms/{viewId}/drafts/{draftId}`：DraftUpdate → Draft，200；不再完整覆盖 values
- DELETE `/forms/{viewId}/drafts/{draftId}`：query 为 operationId／expectedDraftVersion → 204，无响应 body；内部持久最小 operation result
- GET `/api/v1/application-operations/{operationId}`：既有 route 增加 record／draft 结果类型；不返回历史业务 values
**ADR-002 的窄例外**：仅本次 record search route 明确允许只读 POST＋page/pageSize 任意页码。原 ADR-002 的默认 cursor 与原人员域例外历史不改写、不向其他接口扩展；draft list 继续 cursor。
search 固定 pageSize 1–100、最多 3 层 group／20 leaves、raw 64 KiB／canonical filter 16 KiB；record／draft body 上限 1 MiB。queryVersion 可在直接打开编辑时省略；从列表发起的操作必须传，服务器按提供的 context 验证，不把它当权限证据。
### 8.4 精确 DTO（输入与结果分开）
除 ? 字段外均必填；Version 为安全 JSON 整数 0..2\^53−1，page 与 pageSize 再施加对应正数约束。Values 的键须实际稳定 fieldId，不能利用 string 索引类型传 system／unknown／tombstoned ID。DraftValues 是独立的未完成输入载体，不能使用正式 Values 的完整业务校验器。
```typescript
type UUID = string; type Version = number; // safe JSON integer 0..2^53-1
type FieldValue = string | boolean | string[] | null;
// Exact kind drives validation: decimal is a decimal string; date YYYY-MM-DD;
// datetime explicit-offset RFC3339 normalized UTC; option/ref stable UUID.
// Only multi_select uses string[]; null is explicit clear.
type Values = { [fieldId: string]: FieldValue };
type DraftValues = { [fieldId: string]: string | boolean | string[] | null };
type Record = {id:UUID; appId:UUID; tableId:UUID; viewId:UUID;
  createdBy:UUID; createdAt:string; updatedAt:string;
  recordVersion:Version; schemaVersion:Version; values:Values};
type DraftRef = {id:UUID; draftVersion:Version};
type RecordCreate = {operationId:UUID; expectedSchemaVersion:Version;
  values:Values; draftRef?:DraftRef; queryVersion?:string};
type RecordEdit = {operationId:UUID; expectedSchemaVersion:Version;
  expectedRecordVersion:Version; changes:Values;
  draftRef?:DraftRef; queryVersion?:string};
type FilterGroup = {operator:"and"|"or";
  children:(FilterGroup|FilterCondition)[]};
type FilterCondition = {fieldId:UUID|"createdAt"|"updatedAt";
  operator:"eq"|"neq"|"gt"|"gte"|"lt"|"lte";
  value:FieldValue};
type Sort = {fieldId:UUID|"createdAt"|"updatedAt";
  direction:"asc"|"desc"} | null;
type RecordSearch = {page:Version; pageSize:Version;
  filter:FilterGroup|null; sort:Sort; queryVersion?:string};
type RecordPage = {items:Record[]; total:Version; page:Version;
  pageSize:Version; sort:Sort; queryVersion:string;
  schemaVersion:Version; viewVersion:Version};
type DraftSummary = {id:UUID; viewId:UUID; tableId:UUID;
  targetRecordId:UUID|null; schemaVersion:Version;
  baseRecordVersion:Version|null; draftVersion:Version;
  createdAt:string; updatedAt:string; hasConflicts:boolean};
type DraftConflict = {fieldId:UUID|null;
  reason:"FIELD_REMOVED"|"FIELD_PERMISSION_REVOKED"|"SCHEMA_CHANGED"|"BASE_RECORD_CHANGED"};
type Draft = DraftSummary & {values:DraftValues; conflicts:DraftConflict[]};
type DraftCreate = {operationId:UUID; targetRecordId:UUID|null;
  schemaVersion:Version; baseRecordVersion:Version|null; values:DraftValues};
type DraftUpdate = {operationId:UUID; expectedDraftVersion:Version;
  changes:DraftValues; removeFieldIds:UUID[]};
type DraftDeleteQuery = {operationId:UUID;
  expectedDraftVersion:Version};
type MutationResult = {operationId:UUID; id:UUID; recordVersion:Version;
  schemaVersion:Version; createdAt:string; updatedAt:string};
```
### 8.5 记录写入、幂等与最小 operation 结果
Create 为 sparse values，省略字段取 V030-013 规范化常量默认，应用默认后 required 须满足。PATCH changes 省略字段保留原值，null 只清可空字段；空 changes 是 no-op，保存最小 operation／操作审计，不增加 recordVersion。系统字段不可写，两个 view 看到同一行及版本。schemaReady=false 返回 409／APPLICATION_SCHEMA_NOT_READY，不提供 JSONB fallback。
MutationResult 仅含 operationId、id、recordVersion、schemaVersion、createdAt、updatedAt；operation 持久化最小结果，不保存业务 values。当前合法 actor 可查询本人 operation 确认状态，不返回已失权旧字段，不因业务字段失权泄露旧数据。
共用 actor＋operationId namespace；指纹覆盖 method、kind、实际 app/view/table、规范化 body、expected versions、可选 queryVersion／draftRef。同 key 不同 payload 冲突；确认重放先验证当前 actor 及最小结果可读性，再处理旧 CAS／草稿消费，避免已提交响应丢失无法恢复。COMMIT／响应丢失保留原 key、exact body；404／timeout 不是回滚证明，不自动换 key 重试。
### 8.6 筛选、排序与 NULL
text／multiline 仅 eq／neq；number／money／date／datetime 可 eq／neq／gt／gte／lt／lte；boolean／single_select／member／department 仅 eq／neq。option／reference 比较稳定 ID，不按名称或过期缓存值。
multi_select 的 eq／neq 为去重后的集合相等／不等，**不是 contains**。非 null 的 neq 排除 NULL；eq null＝IS NULL，neq null＝IS NOT NULL。无文字排序。
默认 createdAt DESC、id DESC；显式 numeric／time sort 追加同方向 id，NULLS LAST。offset＝(page−1)×pageSize 必须安全算术；可访问超过 total 的合法页并返回空 items，不默默 clamp。若未来暴露相对日期，先冻结绝对范围到 context。**quick search 尚待独立技术 DTO 补充，本版此合同不暗中启用模糊搜索。**
### 8.7 草稿部分输入、冲突与 PATCH
DraftValues 允许缺少必填值与未完成文本；保存仅按字段 ID、当前授权、JSON 形状和长度约束验证，不能在草稿阶段套正式 required／完整业务值校验。正式 commit 才执行 V030-013 的严格类型、默认、required、reference 和版本校验。
Draft 的 conflicts 为必填数组，无冲突为空；元素不含旧值。FIELD_REMOVED／FIELD_PERMISSION_REVOKED 指向字段，SCHEMA_CHANGED／BASE_RECORD_CHANGED 可以 fieldId=null。DraftSummary 仅增加 hasConflicts，不暴露值。
- 返回草稿值只限当前可读／可编辑范围；失权字段的 stored 值保留且不返回，通过 conflicts 提示。整个资源权限丢失仍 403，不能用 conflict 绕过入口
- PATCH body 为 operationId、expectedDraftVersion、changes、removeFieldIds；changes 与 remove 不能重叠，省略字段保持
- 显式 remove 只清本人的草稿键，不修改正式记录；允许移除本人草稿中已失效／失权旧键，但不返回其旧值。写新值仍需当前权限
- 空 changes＋空 remove 是 no-op，保留 draftVersion，持久最小 operation 结果
- schema／base 冲突不自动 rebase；保留旧草稿，UI 可以基于当前版本显式另存新草稿，不自动删除旧稿，不新增隐式恢复／rebase API
- 正式提交消费 exact owner/app/table/view/schema/base/draftVersion，记录＋审计＋operation＋消费同事务。较新草稿不被旧版本提交删除；不匹配明确冲突
- 纯草稿保存不改正式记录／业务 table revision，不记正式记录变更、不触发流程、不调用 record fence；草稿列表使用有索引的 owner／cursor 查询，不能一次载入全部草稿
- discard 对外 204，内部最小持久 operation 结果用于未知提交恢复，不给 204 塞 body
### 8.8 已冻结错误码
沿现有公共 envelope／Session／media 状态；不暴露敏感值或 raw PostgreSQL 错误。
- COMMON_VALIDATION_FAILED：400，既有 body-field 校验格式
- APPLICATION_FORBIDDEN：403，当前入口／resource/action/field 无权，或 filter/sort 读 scope 不覆盖全部 visible rows
- APPLICATION_NOT_FOUND：404，实际 app/view/record 不匹配，以及 GET record 无 read；避免跨 app 枚举
- APPLICATION_RESOURCE_INVALID：400，输入字段／reference 无效或跨 app
- APPLICATION_SCHEMA_NOT_READY：409，定义尚未首次 Save
- APPLICATION_SCHEMA_CONFLICT：409，仅对获授权主体提供 currentSchemaVersion
- APPLICATION_RECORD_CONFLICT：409，仅对当前可读主体提供 currentRecordVersion
- APPLICATION_RECORD_FENCED：409，pending command 或受保护写入
- APPLICATION_QUERY_CHANGED：409，完整获授权投影真实变化，须显式刷新第一页
- APPLICATION_QUERY_CONTEXT_EXPIRED：409，context 丢失／驱逐／无效，不宣称数据一定变更
- APPLICATION_DRAFT_CONFLICT：409，draftVersion 冲突，不静默覆盖
- APPLICATION_DRAFT_BASE_CONFLICT：409，base record／schema 改变，保留草稿
- APPLICATION_OPERATION_CONFLICT：409，同 actor/key 不同 fingerprint
- APPLICATION_OPERATION_UNCONFIRMED：503，带原 operationId 供核查
- COMMON_SERVICE_UNAVAILABLE：503，必需 guard/source/registry 不可用或 COMMIT 前 timeout
### 8.9 事务端口、fence 与真实来源
严格沿 V030-013 的 PR21/B5 锁序：事务局部限额 → query revision locks → AccessForWrite／排序 source locks → app/policy＋operation → structure／同 table gate → row lock/CAS/fence → value／audit／result → 唯一 COMMIT。record writer 先取同表 gate 再改行，不能普通成员误用 owner-only BeginManagerWrite。
共享 owner 提供或批准允许 caller 注入 action policy 的 BeginRecordWrite／Replay／Claim／Complete／Commit 生命周期，共用 ledger 与 unknown-COMMIT 分类。RecordWriter.ApplyInTx、DraftConsumer.ConsumeInTx 接收 caller pgx.Tx，不 Begin／Commit／Rollback、不在 PG 锁内调用远端 Flowable。TrustedRecordContext 只能来自事务内 Session／真实资源／live policy；实际端口方法与参数沿已读提案 §4，RecordResult 返回以上最小 MutationResult。
RecordFence 必须真实检查；可用且确为空的持久 fence 表可证明当前无 pending，不能硬编码 no-op。缺失／不可用返回服务不可用，pending 返回 RECORD_FENCED。未来 task-scoped node Save 独立于普通 CRUD 授权，复用同表 gate、recordVersion CAS、fence 与保存事务，不能自动 complete 或触发其他流程。
新 reference 值要求活跃权威源。实际人员使用多 DepartmentIDs，不能任取第一个；registry／member-department edges、source version hooks 必须引用真实代码并接线。nullable 保留 NULL，缺 registry 明确完整性／不可用错误；源删除保留稳定 ID 与最后显示，不让同名账号复用。此处不声称 B4a 已提供生产 HTTP。
### 8.10 尚待技术补充与共享 owner
**不阻塞已冻结核心的两项**：
- 快搜的具体 DTO／文本匹配语义待单独冻结；不能把精确 eq/neq 改模糊搜索
- 财务例子要求旧值／新值业务变更日志。执行者另提独立 apprecordchange 表及按当前 row／field 权限读取 history 的合同，交主负责人冻结；不能把 raw values 放到全局 personnel 认证审计。此项尚未批准详细存储／API，不能声称完整财务审计已交付
普通记录写入现阶段保留最小安全审计：actor/app/table/record/version/operation／changed field IDs 与必要 correlation；不得用这份最小审计冒充旧值／新值日志。
本任务独占 apprecords／appquery／appdrafts／appaccess，apppolicy 扩展须协调唯一 owner。OpenAPI／errors／migration／roles／BFF 共享入口仍由 V030-013 集成 owner 管理，V030-015 不并写。索引及候选热8／冷5等仅提案，实际编号需扫描并由 owner 分配；不创建查询 journal，不改旧 migration 或门禁。
实际 RED／GREEN 依提案 §7 加本节修正：权限 scope 覆盖拒绝、defaults-only create、最小 MutationResult／operation、防失权字段泄露、草稿冲突／PATCH保留／移除／no-op／显式另存、真实 empty fence、所有类型／CAS／未知提交、相关性及规模矩阵。源提案 B 算法专项测试不纳本次实现。尚未运行不能写通过，性能不能靠扩大 timeout 过关。
**当前结论**：A 算法及上述 record/query/draft 核心合同已由主负责人接受，回读后按限定范围放行；快搜与业务 history 技术细节、用户四项业务问题保持待决。没有新 PR／main merge／deploy 或生产 DDL／权限放行，整版 v0.3.0 未验收。
## 9. 查询核心容量结果与受控 compact-A 实验（2026-10-03）
### 9.1 已测事实与尚未通过的容量门槛
已读取 [固定 34b3e2d 的 scale-core-report.md](https://github.com/Hubujiu/WeaveOS/blob/34b3e2d/docs/evidence/V030-015/scale-core-report.md)。报告使用隔离 PostgreSQL 18.6、真实类型列与索引，逐行流式 JSON 指纹；没有把完整结果装进 Go 内存。
- 百万行每 context 流式 JSON 为 144.78 MB；C＝1 约 3.24–3.56 秒，C＝20 顺序总耗时 45.3 秒。
- 百万行宽范围 C＝200 在整矩阵与独立宽范围两次运行中均超过未修改的 600 秒期限，未取得成功 REHASH 结果。该容量问题保持未通过，不能扩大期限后称已过。
- C 表示顺序重复观测的假设 contexts，不是并发 Redis／API context。样本为 warm 本地测试，早期与后续运行的硬件／缓存状态有变化，不能将组合数值当统一 SLA。
- 27–29 MiB 仅为后续限定／过滤样本的 BFF 测试进程最大 RSS，不含 PG、缓存或完整服务链。报告未覆盖真实 Session、Redis Q36 CAS、受限角色、持久权限、reference display、HTTP／浏览器或并发写锁，不能据此验收生产容量。
原 A 仍是第8节已冻结基线。表 revision 变化后的不相关写即使最终不提示用户刷新，也可能引起 O(C×M) 重算成本；这一成本不能藏在页长20／100之下。
### 9.2 主负责人放行的隔离实验，尚未替换生产算法
只放行 compact-A 的隔离验证与比较，不直接替换 A。前提是全部真实 writer 强制维护 recordVersion，schema 变化由 schemaVersion 完整覆盖；这些前提必须验证，不能只依赖实验 fixture。
- 候选指纹由稳定 record ID／version、逐行 read-mask，以及真正可见 reference display 的去重 digest 构成。与完整可观察 projection oracle 做等价校验，先证明业务观测一致，再比较成本。
- recordVersion／updatedAt 已返回给合法读者，属于可观察 P；匹配行这些值改变算相关。其他不匹配行或无关资源仍不得无条件要求用户刷新。
- 比较同 revision、同 criteria、同有效授权 scope 的 digest／count 有界缓存与 singleflight；必须同时测200个不同条件的最坏情况，不能只用共享缓存命中展示改善而掩盖不同条件成本。
- 不存全量记录，不引入旧值查询 journal；报告正确性 oracle、成本、内存界限和实际未覆盖链路。新实验不使原 A 默默失效，最终选型待主负责人根据实测决定。
完整链路和百万规模容量尚未通过，记录／权限／草稿已冻结核心接线继续，不把实验阶段写成生产算法已验收。
## 10. 数据库信任边界与待补齐合同方向（2026-10-03）
### 10.1 拒绝新增数据库 HMAC actor 凭据体系
终端用户授权边界保持 Redis Session＋BFF，auth_app 为可信服务角色。共享数据库 role 无法独立证明最终用户 Session，不新增 DB HMAC actor 凭据体系，也不声称已有这种身份隔离。
typed DML 仅作用于真实登记的表／列和 canonical 值，保护系统列与 CAS；不提供 generic SQL 或任意 DDL。BFF 必须在同事务内强制 current Session、CSRF、完整 grants 和 fence。V030-013 是共享实现的唯一 owner；不由执行者自行引入第二套凭据或生产安全配置。
### 10.2 下列方向补齐精确合同后由主负责人最终核对 DTO
这些是下一步合同方向，不将尚未完成的接口或权限配置宣称已交付，也不阻止第8节已冻结核心接线。
- **运行时定义与引用显示**：普通运行时 view／schema 接口和 Record 的最小 reference display 不能缺失，须给出对应真实权限、方法、DTO 与来源契约。现有 owner-only 定义／候选接口不能直接开放给普通 record 编辑者。
- **quickSearch**：显式指定1–20个 text fields；TrimSpace 后文本长度1–160字符；字面 substring，ASCII A–Z 不区分大小写，其余 Unicode 按字面；参数化且无 wildcard 语义。所有搜索字段的可读 scope 须覆盖整个 visible-row scope，否则403。具体 DTO 和错误细节待主负责人最终核对，不改变既有 eq／neq 或引入文字排序。
- **history**：新增独立 data.history 能力，默认无 grant。须同时满足当前 row／field 的 read 与 history 权限；隐蔽事件先过滤后分页。旧值／新值保存在独立值表，不放全局 authentication audit、operation 或日志。暂不自动 purge、不新增保留天数；沿备份原加密和最小角色原则。task 详情按实际权限展示，不能以任务存在绕过字段权限。worker 还须补历史旧类型／option 展示策略和精确 DTO，主负责人另行冻结。
- **字段引用的权限保护**：字段仍被 data grant 引用时，删除须阻止并列出依赖；必须显式先撤销 grant，schema Save 不暗改权限。具体错误／依赖表达与共享接线由主负责人审查精确合同。
用户四项待答业务问题与以上技术补充区分；没有新的 main merge／deploy 或生产权限／DDL 放行，整版 v0.3.0 未验收。
## 11. RuntimeView、quickSearch、可观察投影与记录保存历史冻结（2026-10-03）
本节由主负责人完成 [1633361 shared-integration-proposal](https://github.com/Hubujiu/WeaveOS/blob/1633361e69060011e25d780f39c0ec68dbab3512/docs/evidence/V030-015/shared-integration-proposal.md) 与 [同 SHA contract-and-algorithms](https://github.com/Hubujiu/WeaveOS/blob/1633361e69060011e25d780f39c0ec68dbab3512/docs/evidence/V030-015/contract-and-algorithms.md) 审查后冻结。以下修正优先于固定源码和本页第8／10节的旧待审表述；不声称该固定源码已回写修正或接口已经实现。用户四项业务问题不变，整个任务尚未验收，compact-A 不用于生产选型。
### 11.1 普通运行时视图与最小引用显示
新增 `GET /api/v1/applications/{appId}/forms/{viewId}/runtime`，200返回 RuntimeView，沿公共 envelope。服务端解析真实 form→table 及其对应 menu resource，先验证实际菜单的 menu.enter，再要求该 form 至少一项当前 `data.read`／data.create／data.edit。menu-only为403；具有所需菜单且只有 data.create 的主体可以加载。owner／Bootstrap仍须真实 form／table 存在，不放宽原 owner-only definition route。
字段只返回当前 create／read／edit 授权的并集；完整 grant tuple 计算各动作 scope，不拆成笛卡尔组合。layout移除无授权字段节点并裁去空group，只保留经整理的安全展示／system节点，不泄露无授权字段名、defaults、options、内部workflow／DDL／依赖配置。default只向对该字段有create权限者提供；query能力仍须覆盖整个可见行scope。
RuntimeField.timePrecision必须包含minute，修正源提案遗漏；input与V013实际规范保持一致。以下是冻结的新增DTO，现有 FieldKind／LayoutNode／FieldValue／Version沿第8节及V013定义：
```typescript
type Scope = "none" | "own" | "all";
type RuntimeField = {
  id: UUID; name: string; kind: FieldKind; required: boolean;
  presentation: {helpText: string | null; displayTimeZone: string | null};
  input: {
    decimal?: {precision:number; scale:number; roundingPlaces:number;
      roundingMode:"HALF_UP"|"HALF_EVEN"|"TOWARD_ZERO"|"FLOOR"|"CEILING"};
    timePrecision?: "minute" | "second" | "millisecond";
    options?: {id:UUID; label:string}[];
    referenceKind?: "member" | "department";
  };
  default?: FieldValue;
  access: {read:Scope; create:boolean; edit:Scope};
  query: {operators:("eq"|"neq"|"gt"|"gte"|"lt"|"lte")[];
    sortable:boolean; quickSearchable:boolean};
};
type RuntimeView = {
  appId:UUID; tableId:UUID; viewId:UUID;
  schemaVersion:Version; viewVersion:Version; policyRevision:Version;
  fields:RuntimeField[]; layout:LayoutNode[];
  capabilities:{create:boolean; read:Scope; edit:Scope;
    search:boolean; draftCreate:boolean; draftEdit:boolean};
};
type ReferenceDisplay = {id:UUID; label:string; deleted:boolean};
// Added to existing Record; existing system/version/values members stay.
type RecordReferenceDisplays = {
  referenceDisplays:{[fieldId:UUID]:{[sourceId:UUID]:ReferenceDisplay}};
};
```
GET Record与RecordPage每行增加referenceDisplays，只覆盖该行readable values内实际引用的ID，按当前registry／tombstone取最小label与deleted状态，不返回全量候选或完整人员DTO。无引用谓词时先获授权COUNT／page，再在同一RR批量解析去重ID；有引用谓词仍在COUNT／LIMIT前核权威registry。缺失registry明确完整性／不可用错误，不伪造空label。已删除源保稳定ID及最后权威显示，不允许同名接管。
RuntimeView作为V014已有FieldRenderer的受控输入，不再造第二套renderer；schemaReady=false沿409 APPLICATION_SCHEMA_NOT_READY。
### 11.2 quickSearch精确输入与旧context规则
```typescript
type QuickSearch = {term:string; fieldIds:UUID[]};
// Additive to the existing RecordSearch:
type RecordSearch = {
  page:Version; pageSize:Version; filter:FilterGroup|null;
  sort:Sort; queryVersion?:string; quickSearch?:QuickSearch;
};
```
quickSearch可省略，显式null拒绝。term按Unicode首尾空白Trim后要求1–160 Unicode字符、有效Unicode且无NUL，空term拒绝。fieldIds为1–20个distinct当前text／multiline字段ID；每个都须在该实际table／form内且可读scope覆盖全部visible rows，否则整请求403，不缩减可见行逃避权限检查。
选中字段之间OR，再与结构filter AND。匹配为字面substring：ASCII A–Z双方转小写，其余Unicode字面；参数绑定，%／_／反斜杠不作为LIKE模式，NULL不匹配。不默认搜全字段、系统字段、引用标签、历史或布局说明；隐藏列仍只是展示，不据hiddenColumnIds改变服务器读权限。canonical criteria排序fieldIds并规范ASCII term，不引入文字排序或模糊排序。
UI改变条件回page1，但允许携带旧queryVersion。服务器先按保存的旧criteria验证旧context，再按新criteria创建新context并使用合法requested page；不偷偷clamp为1，不仅因条件变化＋page非1额外返回400。已有context时，只有用户显式刷新才省略旧token并回第一页；首次请求本来没有token。旧criteria失效／权限丢失沿原QUERY_CHANGED／403规则。
substring可能扫描候选文本，现有BTree不证明有效索引；实测与EXPLAIN仍是验收要求，不由本合同批准额外索引或放宽timeout。
### 11.3 P与控制版本分离
完整可观察P包含匹配获授权行的确定次序／数量，以及每行：
- id、createdBy、createdAt、updatedAt、recordVersion
- 当前全部获授权business values，不受layout或隐藏列缩小
- 仅真正可见reference display，包括fieldId／sourceId／label／deleted
P不包含schemaVersion、viewVersion、policyRevision等控制令牌。它们及其他依赖revision变化触发同一RR内对旧criteria的完整旧P重算；仅P的digest／count真正变化才返回QUERY_CHANGED。旧criterion已无法编译或不再有合法scope时仍按原错误规则，不能为计算旧P泄露已失权字段。
匹配行recordVersion／updatedAt变化既然可见，就属于相关变化；不匹配行及无关资源仍不得强制用户刷新。DTO返回当前控制版本；UI发现schema／view版本变化要重取RuntimeView，保留dirty输入供显式核对，不自动rebase或重发写入。不能因为P不含控制版本而忽略结构校验。
### 11.4 “记录保存历史”的范围、存储与读取
对外定名为“记录保存历史”。record.create、record.edit与未来task Save的真实值变化，在记录事务内写canonical old／new差量；no-op无值delta。保存事件、差量、业务记录、草稿消费（若有）、最低审计及operation结果同一事务，不把独立Save与后续Agree混成一笔跨服务事务。
schema Save只原子记录变更规则、actor、time和前后schema版本的审计；不逐行业务history、不逐行增加recordVersion／updatedAt。有损转换的旧逐行值因此无法从“记录保存历史”还原，preflight影响确认必须明示，不能宣传完整合规审计或历史全量快照。
采用独立应用历史事件／值关系，业务表仍真实类型列。精确字段沿源合同，必要存储为：
- record_change_events：id、app_id、table_id、record_id、source_view_id、actor_user_id、operation_id、record_version_before／after、origin（ordinary／task_save）、可空opaque_task_ref、occurred_at
- record_change_values：event_id、field_id、event-time field_kind、canonical old_value／new_value（JSON值或JSON null），event＋field唯一；operation＋record防重放重复事件
- 保留field／option tombstone最后label／kind，delta保留写入时kind。option／reference值保稳定ID；读取时的当前／tombstone标签不冒充事件时点标签。无可用标签则显式标识缺失并保ID，不猜测
- 值不进入全局authentication audit、operation result、普通context或日志；备份延用原加密与最小角色，不给全局认证审计读取者业务值权限。无自动purge或新retention天数；增长成本持续为全部变更canonical字节及事件／delta／索引开销之和，容量限制仍须记录
读取路由相对app：`GET /forms/{viewId}/records/{recordId}/history?pageSize=&pageToken=`，标准data.items与meta.pagination；cursor沿既有ADR-002规则并绑定actor／Session／form／record／policy与schema版本。
```typescript
type HistoryChange = {
  fieldId:UUID; fieldKind:string; before:FieldValue; after:FieldValue;
};
type HistoryEvent = {
  id:UUID; recordVersionBefore:Version; recordVersionAfter:Version;
  actorId:UUID; occurredAt:string; origin:"ordinary"|"task_save";
  changes:HistoryChange[];
};
```
先验证实际menu.enter，再以当前record.createdBy和当前完整grant求current row `data.read`及逐field `data.read` AND data.history；data.history默认无grant。无row read为404，无history action为403。无任何允许delta的隐蔽事件在LIMIT／cursor前过滤；混合事件只返回有权delta及该事件actor／time，不泄露隐藏field ID、数量或隐藏事件header。task详情另验实际任务权限，source_view仅provenance，不能绕当前view授权。
owner／Bootstrap在真实form／table仍存在时可看已删除field的既存history，按保留tombstone与event-timekind解释；普通actor没有当前field grant就不可见。删除field前必须先显式撤销引用它的data grant，schema Save在app／table gate内列依赖并阻止删除，不暗改权限；与并发grant新增／撤销及record写入做一致锁序回归。
### 11.5 共享核心放行边界与有限typed DML
Session／Redis＋BFF是终端用户身份授权边界；auth_app是可信服务角色，数据库共享role不能独立证明Session。不新增HMAC actor凭据体系，不给泛SQL、任意DDL或业务表owner／宽泛UPDATE权限。
普通记录使用独立非manager生命周期；真实Session／CSRF／当前完整grants／schema／fence在caller-owned同事务中验证，operation replay／claim／minimum result与原unknown-COMMIT语义保持。typed DML只访问真实登记表列及canonical值、保护system列／immutable createdBy／CAS，必须包含受控LockHeader，不能因SELECT FOR UPDATE需要权限而授整个动态表UPDATE。
```go
type TypedDML interface {
  Insert(context.Context, pgx.Tx, Table, Create, []string) (StoredHeader, error)
  LockHeader(context.Context, pgx.Tx, Table, string) (StoredHeader, error)
  UpdateCAS(context.Context, pgx.Tx, Table, Edit, []string) (StoredHeader, error)
}
type StoredHeader struct {
  ID, CreatedBy string
  RecordVersion int64
  CreatedAt, UpdatedAt time.Time
}
```
真实metadata／同表gate／fence／operation／audit与业务变更保持同事务；writer不自行Begin／Commit，不持PG锁调用远端Flowable。所有生产适配仍须真实受限auth_app及恶意直接调用测试，不能拿owner-role临时测试当通过。
共享OpenAPI／errors／migration／roles／BFF／人员源hooks由V013唯一owner协调；实际migration编号先查重，不重写历史。Q36抽取的独立实施计划随后由主负责人按第12节冻结；只有该节列明的文件归属例外可执行，不能将提案第4节整张路径表视为无条件改共享文件授权。抽取尚未完成。
### 11.6 必须新增的验收与当前容量事实
覆盖runtime真实form→menu解析、menu-only403／create-only可加载、字段/layout最小投影、minute精度；quickSearch省略／null／空term／distinctfields／ASCII与Unicode字面／特殊字符／全scope403；旧token＋新criteria＋合法深页、显式refresh及control-only变化P不变；schema/view变化保留dirty核对。
记录保存历史须验证ordinary／task_save同事务、no-op无delta、有损schema确认且不伪造逐行history／版本、删field／option tombstone、owner与普通actor差异、隐蔽事件先LIMIT过滤、撤权与cursor、task详情鉴权及备份最小角色。另验typedDML LockHeader、受限auth_app、CAS／fence／operation响应丢失、Save与grant依赖竞态。
最新compact-A百万行、200个不同条件约4分02秒，仅隔离warm顺序实验；不是完整Redis／Session／权限／HTTP链路容量通过，也未批准替换生产A。原完整投影百万×200超时事实保留，最终容量和算法决断待实测审查。本节是合同冻结，不是整体验收或merge／deploy放行。
## 12. Q36 单套公共生命周期抽取实施计划（2026-10-03，主负责人冻结）
主负责人已审查并冻结 [12fa3608 q36-extraction-plan.md](https://github.com/Hubujiu/WeaveOS/blob/12fa3608381f54d7f00cebeafd3be7fe622cf52c/docs/evidence/V030-015/q36-extraction-plan.md)。本节读回后可执行限定抽取；源文件中的“等待Notion冻结”是此前状态，不表示已经实现。没有新增records HTTP接线或扩大benchmark的授权。
### 12.1 唯一文件归属
V030-015独占：
- `services/bff/internal/personnel/query_context.go`
- `services/bff/internal/personnel/query_engine.go`
- `services/bff/internal/personnel/query_write_guard.go`
- 上述对应的Q36 context／projection／write-guard／HTTP回归测试，已有测试范围登记到V015 task scope
- 新 `services/bff/internal/querycontext/`
这是对原“所有personnel共享文件归V013”的精确例外。其它personnel文件、真实source hooks、共享迁移／API／roles／应用lifecycle仍由V013 owner负责；不得顺带修改。`query_projection.go`的业务SQL／指纹framing不迁移，commitBusiness与草稿清理业务行为保留。
### 12.2 必须保持的兼容与顺序
- 单套Redis／Lua／RR／receipt实现移到中性querycontext，不复制第二套引擎。人员prefix精确保留为 `ems:personnel:query:<generation>:v1:{sha256(lowercase sessionRef)}:`。
- token仍为随机32字节base64url；Redis hash只有data与revision，data原JSON字段名／顺序和protocolVersion1不变，revision保持People／Configuration／Activity原JSON bytes。无需Redis迁移，旧token可由新实现Load／Advance，新token亦通过旧实现兼容测试。
- 每Session共享members／events的20条LRU、30分钟idle TTL、成功Create／Load／Advance更新访问分数；53bit安全总数、64字符小写SHA256 fingerprint、64KiB元数据限制及封闭领域验证保持，不能退化为任意JSON检查。
- 先Load token再开启只读RR；即使Load失败，仍先执行live Session／授权，再返回token过期／无效错误。授权、criteria、revision、旧／新完整投影、COUNT、page与hydration都在同一RR内。
- RR COMMIT后才Redis Publish／Create／Advance CAS；Advance只更新revision，不改criteria／count／fingerprint，使用原revision和不可变fingerprint比较。并发CAS失利及其他Redis错误按原人员语义处理，不提前发布或声称PG／Redis原子事务。
- 原人员10秒read deadline、COMMON_QUERY／QUERY_BUSY HTTP错误映射、路由签名、错误优先级、锁顺序保留；有效超出total页返回空items，不clamp。
### 12.3 中性接口与领域Prepare
Store的Metadata／Policy／Create／Load／Advance，以及Strategy／Execute／ValidateSavedRead／Receipt依固定计划的最小接口实现；中性层不接业务行、filter AST、SQL、grants或物理表名。
Strategy由Resource、OpenRead、Prepare、Revisions、Observe、Page组成。Prepare保留领域的合法性和错误顺序：人员维持原incoming normalization优先行为；records按已冻结规则先验保存的旧criteria。不能为了统一顺序改变旧人员API。
Observe返回完整获授权P的digest／count加请求页；Page只用于criteria及revision不变路径。页引用hydration始终使用传入RR。ValidateSavedRead交回同RR及Receipt，调用者完成其它读取、RR提交后才允许Redis CAS。
人员导出QueryContext、QueryRevisions、NewQueryContextStore、SearchMembers、SearchEvents、BeginQueryWrite及error identity保留为兼容wrapper／alias，Application.Queries和HTTP组合不变。未来应用域是同一套Store／Execute／Receipt的第二消费者，不能另造人员适配层或复制Lua；本阶段不接records HTTP。
### 12.4 写receipt与安全边界
先在RR验证旧context并完成read commit，再进入RC业务事务，先personnel.lock_query_revisions再actor授权及revision重验。最多3次重试仅限pre-mutation验证；业务mutation与COMMIT执行一次，未知COMMIT不得重试。
原本要求queryVersion的HTTP继续要求，原本允许直接无token的服务方法仍保留。receipt／锁顺序／草稿清理不被抽取改写；不将queryVersion当权限凭证。records后续实际revision vector与source counters需共享能力就绪，缺失不能视作零或未变化。
### 12.5 RED／GREEN实施与回归
1. 用真实Redis记录旧实现prefix、data／revision bytes、20＋1驱逐、Session隔离、TTL和CAS fixture；先获得新中性store／parity失败RED。
2. 移动单套Lua／storage，保留人员wrapper；真实Redis证明old→new、new→old token双向Load／Advance与byte兼容。
3. 先为RR old／new／Page路径及receipt发布顺序建立RED，再抽取单套lifecycle；保留业务projection SQL和领域Prepare行为。
4. 真实PG＋Redis回归context、projection、HTTP、changed-criteria RR、events引用／归档、固定filter、Unicode、write guard、revision锁序、草稿清理与lost-COMMIT；至少race覆盖personnel／querycontext，另跑vet和实际HTTP测试。
5. 记录实际SHA、RED／GREEN、兼容fixture与未运行项。保持原测试断言／限额／deadline，不扩大benchmark；V013共享字段／grants／sources／事务端口就绪及后续计划冻结后再接records消费者与真实HTTP。
本节冻结的是抽取实施计划，尚未通过实现验收；compact-A保持隔离未选，整版v0.3.0未验收，没有main merge／deploy或生产权限／DDL放行。
## 13. Records consumer 接线与共同基线执行计划（2026-10-03，主负责人冻结）
主负责人已审查并批准 [V015 records-consumer-wiring-plan](https://github.com/Hubujiu/WeaveOS/blob/5235c2e079d9b6c5c20b090a99a6b4d7a77bffbd/docs/evidence/V030-015/records-consumer-wiring-plan.md) 和 [V013 integration-guide](https://github.com/Hubujiu/WeaveOS/blob/46cf01b0d7b79cebe4d50dca2667083ab9b850d9/docs/evidence/V030-013/integration-guide.md)。已通过 GitHub commit 回读将 5235c2e 核实为完整 SHA：`5235c2e079d9b6c5c20b090a99a6b4d7a77bffbd`。
本节是下一段可执行接线计划，取代此前“等待共享 source／事务端口”的阻塞状态。中性 Q36 抽取已经主负责人源码审查通过；该结论仅限抽取，不等于 records 产品、HTTP 或容量验收。原百万完整投影 A 超时问题仍未关闭。
### 13.1 共同开发基线与合并边界
V015 在自己的 task 分支整合两个固定输入：自身 `5235c2e079d9b6c5c20b090a99a6b4d7a77bffbd` 与 V013 `46cf01b0d7b79cebe4d50dca2667083ab9b850d9`。登记整合后的完整 HEAD、父提交与实际 task scope，保持来源可追溯。
不修改 V013 分支、PR26 或 main。冲突涉及共享语义、接口、migration／roles 或其它 owner 路径时暂停该冲突并报主负责人，不自行选择一边覆盖。开发分支整合不代表测试通过或新发布权限；原外部操作授权范围不扩大。
### 13.2 唯一文件与职责归属
**V015：** 新内部 service 包拟为 `services/bff/internal/apprecordservice/`；若采用同等合理名称，先登记实际路径与 task scope。继续独占 apprecords、appquery、appdrafts、appaccess 四核心，及第12节中性 querycontext／三个 personnel 兼容 wrapper 和对应测试。
V015 负责记录查询 Strategy、完整 P 流式观测、page 路径、引用显示的只读 SQL／同 RR hydration、typed DML 适配、live policy／source／fence 组合、record 与 draft service 方法，以及第11节已冻结的 quickSearch SQL。引用读 SQL 可消费真实源表／ports，不取得源写入 hooks 的所有权。
**V013：** 共享 HTTP／OpenAPI／errors、migrations／roles、config／dispatcher、source hooks，以及历史事件／值表和历史存储端口继续唯一归属 V013。由 V013 把记录保存历史写入 port 注入同一事务，并挂接 history／runtime／record／search／draft HTTP 路由。V015 提供 service 接口和精确增量，不并写这些共享文件。
新 service 的依赖需避免 appstructure／applications 循环引用；由 V013 的 config／dispatcher 组合接口。继续使用唯一 querycontext Store／Execute／ValidateSavedRead，不复制第二套 Redis／Lua／receipt，也不新造人员域适配层。
### 13.3 已具备的共享输入
固定 V013 输入已提供真实 `applications.BeginRecordWrite`、RecordWriteOptions（SourceGuard、Authorize、operation fingerprint 和限额）、RecordContext（live actor、真实 form→table／schema／fields、完整 grant tuples），以及 RecordWrite 的 Replay／Claim／Complete／Commit／Rollback／Tx。
同时已有受限角色的 `appstructure.RecordDML`（Insert／LockHeader／UpdateCAS）、RecordGate、RecordFence、最小 RecordAudit，hot8 的 grants／drafts／fences／源 registry 与事务性源 revision hooks。真实 member、department、member→departments 边和 tombstone 已作为接线输入提供，不再把早期 B4a 单部门内部包当成实际来源。
这些端口消除了本段适配器的硬等待；仍不能宣称 record HTTP、runtime 或旧／新值保存历史已接通。最小 RecordAudit 只含 ID／版本／字段 ID／requestId，不替代第11节的记录保存历史。
### 13.4 先建立受限真实测试，再实现读取消费者
1. 在隔离 PG／Redis、已迁移的实际结构与受限 auth_app 下建立测试，先证明尚未接线的目标失败。记录实际失败原因；已有能力直接回归为 GREEN 时如实标记，不制造 RED。
2. 实现 `querycontext.Strategy[RecordPageItems]`，resource 绑定实际 app／table／view，namespace 为 applications，SessionRef 只能来自可信主体，使用封闭 metadata／revision policy。
3. OpenRead 验证 live Session、真实 form/menu、当前读取权限与来源可用性，并在同一只读 RR 内固定授权、结构、源与查询事实。即使 token 失效也不得绕过 live auth。Prepare 先验证保存的旧 record criteria 及完整字段 read scope，再验证 incoming；失权字段为403，仍有入口权限但旧条件字段已 tombstone 时为 QUERY_CHANGED。
4. 同 RR 读取实际 table data/dependency、app policy、schema、view 和引用源 revisions；缺失 hooks／counter／必要源数据须明确不可用或完整性错误，不能当零／未变化。
5. SQL 中先完整授权，再用户 filter／quickSearch，再 COUNT／page。Observe 以有界流式方式计算第11.3节完整 P：确定次序、行 id／createdBy／createdAt／updatedAt／recordVersion、获授权业务值和可见引用 id／label／deleted。控制版本不进入 P；不能因 layout／hidden 缩小投影。
6. Page 路径使用同样授权、mask 和 RR hydration。无引用条件时先获授权 count／page，再对页中 distinct source IDs 批量解析；引用条件须基于 registry／多部门边在 COUNT／LIMIT 前过滤。Observe 的完整 P 同样按批次处理可见引用，不能只 hash 当前页或把完整结果攒在内存。
7. queryVersion／criteria／revision 未变只读请求页；revision 变则重观测旧完整 P，criteria 变先验证旧 P 再观测新 P。按第11.2节保留合法 requested page 和显式刷新语义。返回实际控制版本，不使用 compact-A／cache 实验替代本版 A。
quickSearch 只实现已冻结的字段 scope、ASCII 字面 substring 与参数化 SQL；不新增文字排序或擅改条件语义。
### 13.5 Record／draft 写入与恢复
列表来源写入先经同一 ValidateSavedRead／receipt 验证旧 context，read commit 与 Redis receipt 完成后才进入业务 mutation。调用 BeginRecordWrite，传可信 Session principal、实际 form、原 operationId／fingerprint、现有限额及 source lock／policy callbacks；RR receipt 不是授权，RC 锁／gate 内仍复检当前权限与 revision。
Replay 必须在 Claim 和新 schema／record／draft／fence 等陈旧状态检查前；已确认重放遵既有活跃 actor 最小结果恢复规则，不因旧 CAS 或后来撤权伪造失败。新的操作仍完整授权；缺少必需 policy／source／fence 不能默认放行。
用精确中性结构转换，将 apprecords.TypedDML 适配至 V013 RecordDML；保持原生错误身份／注册 code。动态物理标识符由实际登记信息推导，不相信调用方 Namespace 或展示名。Writer 不自行 Begin／Commit，也不直接绕过受控 DML 写业务表。
当前值按 appfields.NormalizeValue 规范化；新选引用验证 active source；完整 RecordContext.Grants 转换为 appaccess.Policy，在 caller-owned 同事务内执行 row／field 权限、CAS、source 和 fence。草稿提交检查精确 owner／schema／baseRecordVersion／draftVersion，正式记录、准确草稿消费、最小审计、operation result 原子提交；历史 port 就绪后由 V013 同事务注入，不用空实现冒充完整历史。
业务 COMMIT 结果未知保留原 key，经 operation 查询恢复，不自动换 key 或重复 mutation。记录最小结果、draft 最小结果及 discard 外部204无 body 沿已冻合同，业务值不进 operation。
### 13.6 验证顺序与失败门槛
**第一段由 V015 证明 service 消费：**
- 真实 auth_app typed create／edit，owner／Bootstrap 与普通主体、跨 app／form 拒绝，all／own 完整 grant tuple 的 COUNT／field mask，whole-visible-scope 拒绝。
- live Session、source 同快照、member 多部门／部门改名／移动／删除与 tombstone；缺失源不能静默丢行或伪造显示。
- 空／pending fence、record CAS、operation 同 key 重放／payload 冲突／已提交响应丢失恢复，draft owner／schema／base／version 冲突及提交消费回滚。
- 最小审计／operation／record 原子性；记录保存历史接线前明确未覆盖，不把最小审计当全量值历史。
- 真实 Redis token／CAS、相关与不相关变化、control-only 变化、旧条件失效、page-only 与完整重算路径；quickSearch／分页／引用条件先后顺序。
**第二段由 V013 组合真实 HTTP，再联合验收：** 在 consumer 完成后接实际 Session／CSRF／actor guard、runtime／history／record／search／draft HTTP，验证真实 PG／Redis／受限角色与完整端点错误行为。源 guide 的已存在端口不能作为这些端点已通过的证据。
**第三段执行既定容量矩阵：** 沿 10k／100k／1m、1／20／200 contexts 测量真实链路，分别报告流式内存、完整 P 重算、COUNT／deep OFFSET 与查询页成本。原百万 full-A 超时继续开放；不改用 compact-A，不扩大既定 timeout，不 skip／删断言或用 mock／owner role 掩盖失败。
交付整合 HEAD、RED／GREEN、命令与原始证据、owner 接线清单和未运行项。若共享语义冲突、必要 port 缺失、授权／事务／未知结果保护失败，相关步骤不得通过；独立项可继续。开发整合、中性抽取通过与记录产品验收分别报告；完整 v0.3.0、main merge／deploy 状态保持。
## 14. V030-017消费前的窄DTO修订与跨任务依赖（2026-10-03，主负责人冻结）
本节覆盖第8／11节及历史提案中与本节不同的draft写回执、runtime history能力和history标签形状，其余record／query／draft语义保持。它补齐真实记录前端的必要合同，不表示HTTP已完成、compact-A已生产采用或V015／v0.3已验收。
消费任务：<mention-page url="https://app.notion.com/p/3ee2f5a9e64881cca5e6eb314c6ec899"/>；<mention-page url="https://app.notion.com/p/3ee2f5a9e6488179816fc7f286376d5e"/>。共享API／OpenAPI／migration由V013唯一owner实现，V015提供真实service结果；前端各owner按下述边界接线。
### 14.1 草稿写回执统一为最小结果
Draft新建POST201与修改PATCH200的正常执行和confirmed replay均返回完全相同的最小结果：
```typescript
type DraftMutationResult = {
	operationId: UUID;
	id: UUID;
	draftVersion: Version;
};
```
不再将正常响应或重放响应声明为完整Draft。前端验证status／envelope／operationId／id／scope后，按当前权限GET完整Draft；失权则不补回旧业务值。operation GET恢复相同最小确认信息，ledger不增加values、conflicts或字段显示内容。
DELETE draft仍204、无body；ledger保留内部最小结果供operation恢复，不对204执行JSON解码。原Draft／DraftSummary读取、conflicts、changes＋removeFieldIds、no-op、schema／base冲突、draftRef原子消费规则均不变。最小对象不能强转FullDraft；旧源提案含draftId或updatedAt等不同结果形状时以本节为准。
V012负责唯一operation／请求层支持PATCH、DELETE204、精确状态和validator以及只读POST search分类；V017消费，V015／V013使service与HTTP正常／重放一致。不能将只读search失败写入业务mutation unknown状态。
### 14.2 普通引用候选
冻结路由：GET /api/v1/applications/\{appId\}/forms/\{viewId\}/reference-candidates。
请求query为fieldId、action=create或edit；edit必须recordId；q、pageSize、pageToken沿V013候选规范。q可省略，TrimSpace后最长100 Unicode字符，实际成员账号／部门name前缀搜索；pageSize1–50默认20；稳定账号／名称和ID排序，cursor分页不全量下载。cursor必须绑定actor、app、view、field、action、recordId（适用时）及policy版本，校验失效／范围。
解析真实app／form／table／reference field，再验证实际menu.enter与当前field create，或实际record的field edit完整grant。readonly字段无候选枚举权限，create-only仅可列获create的字段，own编辑须命中实际record.createdBy。不能借owner-only候选绕过，也不能以客户端能力标志代替后端授权。
返回标准data.items及meta.pagination；每项只含active的id、label、status，department额外parentId，均为最小组织显示信息，不含完整人员DTO／权限模板／凭据。Save时在同事务重新验证source及当前权限，候选出现不代表永久允许选择。旧deleted引用按现有Record.referenceDisplays读取，不在active候选中重新选取。
未来task字段候选由独立可信task授权合同定义，不擅自要求普通data.edit作为额外门槛。本接口不自动扩大到task场景，也不改V013的owner-only定义候选权限。
### 14.3 RuntimeView与记录保存历史安全显示
沿Scope＝none／own／all，新增：
```typescript
// Additive members; the rest of RuntimeField and RuntimeView stay unchanged.
type RuntimeFieldAccess = {
	read: Scope; create: boolean; edit: Scope; history: Scope;
};
type RuntimeCapabilities = {
	create: boolean; read: Scope; edit: Scope; history: Scope;
	search: boolean; draftCreate: boolean; draftEdit: boolean;
};
type HistoryValueLabel = {
	label: string \| null;
	deleted: boolean;
	labelUnavailable: boolean;
};
type HistoryChange = {
	fieldId: UUID;
	fieldKind: string;
	before: FieldValue;
	after: FieldValue;
	fieldLabel: string;
	fieldDeleted: boolean;
	valueLabels: { \[stableId: string\]: HistoryValueLabel };
};
```
RuntimeField.access.history和RuntimeView.capabilities.history按当前read与history的交集给提示；原Scope类型与字段保留，history不是未经授权的额外字段枚举。服务器仍在真实form／record上验证当前row read以及完整field read／history tuple，不能只看按钮。
HistoryChange保留原第11.4节字段名fieldKind，记录event-time kind；新增fieldLabel和fieldDeleted来自允许访问的field／tombstone。valueLabels只覆盖本条允许delta中before／after实际值ID；禁止包含隐藏field／delta或整库候选。映射键是该值的稳定ID，选项与引用label使用当前或tombstone最后label，不声称事件发生时名称。
缺失引用信息仍明确labelUnavailable，不伪造label；缺旧option label允许label=null且labelUnavailable=true。不得为了显示标签读取或发送新撤权字段／值。已有field tombstone保存最后label，删除field的既存history仅owner／Bootstrap在真实form／table仍存在时可看；普通actor当前无grant则不能读，既有hidden事件先过滤后LIMIT规则保持。
本节不扩大history retention、全量业务快照或schema逐行历史；no-op无delta、有损结构转换的历史限制与告知沿第11.4节。
### 14.4 应用私人预设与查询边界
新增应用预设scope是可信ownerUserId＋appId＋formViewId。同table不同view不共享；真实menu＋`data.read`才可用，且保存／读取／应用均实时检查。逻辑列hidden只显隐，不缩小完整授权投影P。
V013负责新持久API／存储与追加migration，沿既有name／CAS／operation／limits规范；端点及完整DTO由V013按本节提交主负责人核读后消费。保留原personnel.table_presets、members／events API、原ID和旧migration，不迁移已有方案。
有效GET回原AND／OR、sort、hidden／width／order。字段删除或permission coverage失效，整套方案invalid，不得静默删条件；GET仅返回id／name／version／invalid及可公开reason，不返回失权fieldIDs／operands或泄露它们的摘要。原定义保留服务端；本人仍有入口＋read时可明确重配覆盖或删除，不自动rebase。候选apply必须实时schema／权限验证，再走原queryVersion；失败保留当前状态。
此持久化缺口未完成时，即使共用前端manager可显示，也不能宣称应用个人筛选方案交付。方案应用不绕开旧查询变化检测、whole-visible-scope、无文字排序或完整P规则。
### 14.5 跨task唯一归属与释放顺序
- V017唯一泛化前端TablePresetManager／TablePresetState／QueryFilterState／query-contracts／usePersonnelQuery及核实登记的最小调用点，保留personnel wire、相对日期、Q36和旧focus／motion回归。不复制第二manager，不改vendor Table；新公共路径先登记。
- V017另外独占applications/records与专属tests，提供真实资源／授权field descriptor和transport序列化，不能伪造owner Definition。
- V014唯一提供FieldRenderer窄runtime显示接口、readonly tombstone／分页引用selector和record／draft leave scope。V017不并写forms／motion／共享types。
- V012唯一扩展PATCH／DELETE204和只读POST请求分类、actor-bound operation恢复；唯一Shell路由owner区分/forms/:viewId运行记录和/design配置，明确处理旧链接redirect，不让普通用户进入owner API。
- V013唯一共享HTTP／OpenAPI／errors／migrations／roles／source hooks／history／候选／预设存储owner；V015保持record service、query与draft核心，协作提供本节最小结果，不并写共享入口。
准备基线frontend3e84662＋V014e60f717仅用于研究。正式实施／集成须主负责人接受V012 revision-skew修复后的新HEAD与V013完整HTTP固定版本。组件／contract工作可以先行，真实联调及验收须实际端点和真实Session／CSRF／actor guard，不用mock或旧service通过替代。
V017按共享最小扩展→runtime／records→填写／drafts→history／Shell→真实HTTPS三浏览器和截图执行。容量沿V015独立矩阵；百万完整A问题仍开放，不暗切compact-A或提高timeout。
### 14.6 新增验收
- 草稿POST201／PATCH200正常与confirmed replay同shape、丢响应operation恢复、DELETE204空body、当前失权后仅最小确认不泄露旧值；确认后GET完整草稿的权限和错误路径。
- 候选field/action/record/policy绑定：create-only、own编辑、只读拒绝、跨app／view／record／field、cursor失效、active与tombstone区分、保存前源删除／撤权。
- history能力read交集、fieldKind保留、当前／tombstone／unavailable标签、valueLabels只含许可delta ID、已删field owner特例和普通撤权无泄露、隐藏事件分页。
- 预设私人view scope、人员旧方案零迁移、实时权限／schema失效整套禁用与脱敏、明确重配／删除、CAS／operation／原查询上下文；显隐不缩小P。
- 共用前端人员全部相关旧回归、record安全renderer、actor／guard／未知结果、旧链接权限边界和三浏览器真实HTTPS。
所有新增实现仍待执行与证据审查；本节技术冻结不改变整版状态和外部发布权限。
</content>
</page>
