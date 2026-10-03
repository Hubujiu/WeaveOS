# V015 shared integration source readback

Actual authorized connector reads. ADR revision 2026-10-03T11:08:25.612Z; PRD revision 2026-10-03T09:44:31.464Z. Parent explicitly instructed V013 to continue shared wiring, retain trusted BFF/Redis boundary and reject new DB actor HMAC credentials. §10.2 runtime reference/quick-search/history/grant-dependency DTO directions still require exact review; §8 is frozen. Worker fixed input is origin/v015-input @34b3e2db6d3360fe714f60b409dc6aceae672acd. Its actor-HMAC proposal is superseded by ADR §10.1; not implemented.

Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14 as of 2026-10-03T11:08:25.612Z:
<page url="https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14" icon="📐">
<ancestor-path>
<parent-data-source url="collection://235f72c0-b039-40d8-9dcc-7abff28b9406" name="架构决策 ADR"/>
<ancestor-2-database url="https://app.notion.com/p/f4b3520a9eb84c8280f33f35142ca388" title="架构决策 ADR"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e64881b9b005dc78a252c08f" title="Architecture & ADR"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"ADR 标题":"V030-015 ADR｜记录权限、查询算法与草稿事务审查","ADR 编号":"","date:决策日期:is_datetime":0,"date:决策日期:start":"2026-10-03","url":"https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14","关联迭代":["https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a"],"决策领域":"数据","影响范围":"模块级","最后更新时间":"2026-10-03T11:08:25.612Z","状态":"已接受"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"📐"}</iconMetadata>
<content>
**A 算法及 record／query／draft 核心合同已由主负责人冻结，详见第8节；回读后可释放限定核心实现。** 快搜 DTO 与业务旧值／新值 history 合同仍待技术审查，用户四项业务问题保持独立。本任务未完成验收，执行者不得自行扩展范围。
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
</content>
</page>

Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a as of 2026-10-03T09:44:31.464Z:
<page url="https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a" icon="📋">
<ancestor-path>
<parent-data-source url="collection://73e57a4a-3b61-4a72-88a0-a1f5b3887865" name="迭代与 PRD"/>
<ancestor-2-database url="https://app.notion.com/p/a6a6c92db4cb41fc98c917bd465cc0e2" title="迭代与 PRD"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e6488037975bc303bee464b5" title="PRD"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"date:排期:is_datetime":0,"url":"https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a","已冻结":"__YES__","最后更新时间":"2026-10-03T09:44:31.464Z","版本号":"v0.3.0","版本类型":"功能迭代","迭代/需求名称":"V030-015 PRD｜真实记录、数据权限、查询与草稿","迭代状态":"审批通过"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"📋"}</iconMetadata>
<content>
**A 算法及 record／query／draft 核心已由主负责人冻结，回读后可放行限定实现。** 本任务是完整 v0.3.0 的后端切片；精确方法、DTO、错误与权限语义见配套 ADR 第8节。快搜与旧值／新值业务 history 仍待技术合同审查，用户四项业务问题不变，当前没有实现验收完成声明。
总需求：<mention-page url="https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa"/>。数据结构前置合同：<mention-page url="https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f"/>。本页不替代其字段、同表 gate 或真实类型存储规范。
## 目标与功能边界
- 真实 PostgreSQL 新增、编辑、读取记录；稳定 id、不可变 createdBy、recordVersion、createdAt／updatedAt
- 请求／返回的字段值按 fieldId 映射，底层仍为 V030-013 的真实类型列，不以 JSONB／EAV 保存业务行
- 正式写入、审计与 operation result 同事务；响应丢失查询原 operation，不换 key 盲重试
- 未来审批节点 Save 复用同事务端口，保存记录不自动推进流程
- 记录删除、永久删除／恢复政策不在本切片擅自决定；不因此停止新增、编辑和查询规划
## 数据权限
资源、动作、行 predicate、字段 mask 的完整 grant 取 OR；owner／Bootstrap 全权仍要求资源真实存在。own 使用 immutable createdBy，首版无下属、deny 或隐式目录继承。
字段读／写能力只在匹配同 record 与 action 的完整 grant 中合并，不能把不同组拆开交叉授权。服务器先授权，再用户 filter，再 COUNT／分页。没有字段权限时不能借筛选、排序或计数泄露其值。隐藏列仅视觉，不改变权限、筛选和既有查询逻辑。
## 统一查询与刷新
支持服务端嵌套 AND／OR、字符串等于／不等于、数字／时间比较与排序、稳定 ID 次序及任意页码跳转。复用既有统一表格抽象，不新增迁就旧人员域实现的适配层，不引入文字排序。
只有与查询相关的数据变化才要求刷新并回第一页；不能每次同表写入都无条件 invalidate，不能把完整结果搬到客户端，也不能宣称 deep OFFSET 为 O(1)。主负责人已选择 A：共享 Q36 生命周期、同 RR 的完整获授权投影指纹，观测版本变化时有界流式重算。B 旧值查询 journal 不实现；初始／变化检查 O(M×W) 与 COUNT／deep OFFSET 成本须诚实报告。
引用遵循已定 registry／tombstone／PF：同一只读 RR 快照，引用谓词在 COUNT／LIMIT 前生效。真实源 hooks 与多部门语义必须按实际人员来源接入，不能把 B4a 内部单部门模块冒充已有 HTTP。
## 显式草稿
草稿由用户明确保存，所有者由可信 Session 主体确定，绑定资源、schemaVersion、baseRecordVersion、draftVersion。保存草稿不改正式记录、不触发流程；正式提交与草稿消费同事务。冲突／失权／schema 改变必须显式反馈，不能暗中丢弃或提交草稿。
## 本阶段可做与不可做
独立合同已审查并冻结核心；回读后按 ADR 第8节实施 record／query／draft 与独立 RED／GREEN。共享 migration／OpenAPI／roles／入口仍由 V030-013 owner 集成，本任务不并写，未实现 API 不得宣称可用。快搜和业务值 history 先提合同，不自行扩展。
任务独占候选模块 apprecords／appquery／appdrafts／appaccess；扩展 apppolicy 必须唯一 owner 协调。共享入口、契约文件和迁移仍由 V030-013 集成 owner 管理。
## 验收计划（未执行）
- [ ] 真实权限与跨账号／app／资源边界；同 record／action 完整字段 mask 并集，筛选／排序／COUNT 不泄露未授权字段
- [ ] 并发 recordVersion CAS、审计与结果回滚、已提交响应丢失和同 key 恢复，不重复写入
- [ ] query 相关与不相关变更分别验证，条件成员变化／排序／来源更新等由冻结算法给出可核查结果
- [ ] 嵌套条件、NULL、数值／时间排序及稳定次序、任意页和深页，与原统一查询语义一致
- [ ] 百万量级性能与内存，说明样本、冷热缓存、索引／扫描／journal 存储与复杂度，不编造常数时间
- [ ] 草稿显式保存、版本冲突、提交消费原子性；无正式记录或流程副作用
- [ ] 源修改／删除、tombstone 历史显示、真实多部门引用及同名新账号不得接管旧引用
- [ ] 流转 pending record fence 的事务端口必须明确拒绝受保护写入，缺少保护器不能默认允许
本任务核心冻结后可按主负责人放行实施，真实结果尚待交付；全版仍未完成。不得因合同冻结创建新 PR 或扩大生产／合并／部署权限。
配套算法与事务审查 ADR：<mention-page url="https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14"/>。核心冻结与限定放行以配套 ADR 第8节为准；快搜及业务 history 的待审边界保留。
## 2026-10-03 核心合同冻结后的验收补充
- 数据授权按 form view，物理行共享 table。查询投影包含 table 当前全部获授权字段，不随 layout／隐藏缩小；filter／sort 字段 scope 不覆盖全部可见行时整请求 403，不悄悄过滤掉可见行。无可读字段行不 COUNT；单行无 read／跨 app 返回 404，入口无权 403
- defaults-only create 允许明确空 create mask。记录新建与 PATCH 返回最小 MutationResult，operation 不保存业务 values；本人合法查询仅确认状态，不泄露失权旧字段
- RecordEdit 使用封闭 PATCH changes；直接编辑可省略 queryVersion，列表发起必须携带。仅 record search 获 ADR-002 的 POST＋任意页码窄例外，draft list 继续 cursor
- search：pageSize 1–100、3 层／20 leaves、raw 64 KiB／canonical filter 16 KiB；record／draft body 1 MiB。multi_select eq／neq 为去重集合相等／不等，NULL 沿 SQL 约定，不是 contains；快搜未启用模糊默认
- 草稿允许缺必填与未完成文本，提交时才严格业务校验。Draft 返回必填 conflicts，Summary 返回 hasConflicts；失权值不返回但 stored 保留，整个资源无权仍 403
- 草稿 PATCH 使用 changes＋removeFieldIds，不能重叠，省略保留；可显式移除本人草稿中失效／失权键而不读旧值，不改正式记录。空修改 no-op 不增 draftVersion；schema／base 冲突不暗 rebase，显式另存新草稿，旧稿不自动删除
- node Save 的 task-scoped 可信授权与普通 record.edit 分开，不擅自要求二者交集；共同遵守 fence／CAS。同表真实空 fence 可以证明无 pending，禁止硬编码 no-op
- A 初次／变化检查流式处理完整投影 O(M×W)，不把完整结果搬到客户端；实际 10k／100k／1m、1／20／200 contexts 性能与内存须独立审查，禁止扩大 timeout 掩盖失败
- 财务旧值／新值变更日志仍须支持，独立 apprecordchange／当前行字段权限 history 合同待主负责人另冻；raw values 不放全局 personnel 认证审计。此项和快搜不阻塞已冻结核心，但未完成时不能宣称完整财务用例已交付
配套 ADR 已提供完整 DTO／错误／事务与尺度要求。所有本轮测试仍需实际执行和证据，不把技术冻结记作验收通过；完整 v0.3.0、main merge／deploy 状态不变。
</content>
</page>
