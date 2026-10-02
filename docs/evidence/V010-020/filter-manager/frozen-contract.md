# Filter manager: authoritative frozen contract readback

Read 2026-10-02 after root's stage-gate release. PRD approved, ADR008 accepted;
PRD archive flag remains false by design. Source lastEdited: 2026-10-02T02:58:52.013Z.
Source: https://app.notion.com/p/3ed2f5a9e64881639ba2c434fb6c7c34 .
No Notion source was edited by this executor.

+## 附录 A｜主负责人冻结的执行契约（2026-10-02）
本附录是已批准范围内的主负责人执行设计，覆盖前文“精确契约待冻结”的占位。它不新增用户产品选择，不授权执行者作新重大选型。schema 或仓库现有结构如与本附录冲突，仅记录冲突并交主负责人处理，不擅自改冻结契约。
### A1. 个人方案数据表与限制
- 独立表 `personnel.table_presets`：UUID `id`；`owner_id` 由 Session 解析并 FK `auth.users`；`view_key` 仅 members／events；`name`；`slot`；可空 `filter_json`；JSON 数组 `hidden_column_ids`；`schema_version=1`；正整数 CAS `version`；`created_at`／`updated_at`。
- name 首尾去空白，1–100 Unicode 字符，保留大小写；同 owner／view／name 精确重复拒绝。不引入额外大小写折叠或名称改写。
- slot 为 1–20；唯一约束 `(owner_id,view_key,slot)` 保证硬配额，`(owner_id,view_key,name)` 保证同表重名约束。配额并发算法复用既有草稿 slot 已验证的事务模式，不引入全局锁。
- filter 规范 UTF-8 JSON 最多 16KiB；整个方案内容规范 UTF-8 最多 32KiB；HTTP raw body 最多 64KiB。三个上限各自独立测字节边界，不用字符数替代 UTF-8 字节数，也不用规范化大小替代原始 body 上限。
- hiddenColumnIds 不重复且必须在当前 view 白名单内。无业务行、Session 或 queryVersion 存储；无到期自动删除。
### A2. 固定 API、结果与错误
- 基础路径 `/api/v1/personnel/table-presets`。GET `?view=members|events` 返回 items 全量、最多 20，顺序 updatedAt DESC、id ASC。
- POST 输入 `{view,name,filter?,hiddenColumnIds,schemaVersion:1}`，返回 201 及完整对象。
- `/{id}` GET 执行 owner 隔离。PUT 输入 `{version,name,filter?,hiddenColumnIds,schemaVersion:1}`，返回 200 和新 version；owner／view 不可改。
- `/{id}?version=N` DELETE 返回 204，执行 CAS。
- 保留既有 envelope、Session、CSRF／Origin 与 permission 检查。404 不暴露他人方案；其他 400／401／403 沿用现有规则。
- 独立登记冲突码：`PERSONNEL_PRESET_NAME_CONFLICT`、`PERSONNEL_PRESET_LIMIT_REACHED`、`PERSONNEL_PRESET_CONFLICT`，分别表示重名、数量限制和版本冲突。
- 不支持的 schema 拒绝 400 并明确反馈，不自动放宽、丢弃条件或降级查询。
### A3. AST、字段与显隐
- AND 行／OR 块只构造根 OR 加 AND 子组，或单 AND 组；保存前验证可编辑形状。零条件编码为 `filter=null`，不存空中间组。
- 复用已有类型、NULL、relations、date／time zone 语义与 20 叶验证；保留 3 层／16KiB 条件基线，查询引擎递归能力不改，不做通用 DNF 转换。
- 当前 view 的业务字段必须已知，visible 至少 1。成员业务列：`account`、`departments`、`identities`、`personnelManage`。事件业务列：`occurredAt`、`actorAccount`、`action`、`object`、`detail`、`outcome`。
- 隐藏 ID 仅用于前端 columns 可见性，不放入 query 请求，不改变指纹依赖、排序、筛选或权限。筛选字段菜单按 view 复用现有合法筛选字段，不能机械照搬全部 display 列。
### A4. 应用快照、保存与查询
- 当前应用方案保留本页应用快照 `id/version/filter/hiddenIds`。保存该方案新版本不会替换已应用快照，显示“已修改，待应用”。
- 重新打开页面没有 active preset，方案 list 仍持久保留。
- 应用时获取该方案最新版本并验证字段／引用，再走现有 queryVersion 旧查询校验。结果变化或上下文过期不自动 refresh，等待用户显式刷新。
- 切换新条件回 page 1 并清 selection；hidden-only 变更不必另发 query，也不能绕过原 context。权限撤销由现有 app 层处理。
### A5. 取消、删除与显隐 baseline
- 取消应用／删除 active 的确认流程：解除自定义 filter，恢复应用第一套方案前的显隐 baseline，保留当前搜索、快筛、正常排序、列宽与顺序。
- 按已批准规则回第一页重新查询；需要旧 queryVersion 校验时仍遵守原错误流程，不偷换为自动刷新。
- A → B 不把 A 的显隐当 baseline。baseline 在首次进入方案模式时捕获，取消才清除。
- 保存／删除失败保持状态；已成功 CRUD 与随后查询失败必须准确分别反馈。
### A6. 受控 Table 与居中 Dialog
- 显隐接入既有受控 Table，不建立 PersonnelTable 样式适配层。稳定字段 ID 保存；隐藏不删除宽度／顺序状态。
- manager 和 editor 都以 viewport 居中 Dialog 呈现：manager 约 440px、editor 约 640px，均受 viewport−24px 约束；max-height 为 viewport−24px，主体滚动、footer 固定。
- 保留项目背景视觉、原 reduced-motion、WAAPI fallback、最后意图竞态修复及装饰 SVG 按压稳定保护，不另选新动效架构。
### A7. 迁移、最小权限与真实验收
- 新迁移使用 00005，实施写入前再查空位；不改 00001–00004。固定基线的只读目录核对另记，不替代实施时检查并行改动。
- 同步精细 CRUD roles 权限、固定 roles SHA、compatibility 新迁移 SHA 和全部 latest-schema fixture；旧历史 schema 升级 fixture 保持历史意义。
- 仅授予 presets 必需列 UPDATE，不允许改 owner／view／slot／id。
- 自动回归覆盖新表备份／恢复、无业务 revision 副作用、owner／view 隔离、20 并发硬上限、CAS、重名、独立大小字节边界及未知字段拒绝。
- 其余实施阶段和验收链保持：独立 RED → CRUD／隔离／CAS → Dialog／显隐 → 旧查询和草稿回归 → 实际截图和 Chromium／Firefox 录屏 → 同一最终 head CI／真实 PG／Redis／Nginx／backup／upgrade／packaging → 主负责人最终审查。未通过、跳过、未运行分别记录，不合并或部署。
只读目录核对：固定基线 d912bb64409f807ddde79bad125b6837938f95a0 的 db/migrations 只有 00001–00004 和 README，00005 未占用；写入前仍复查实际 head／工作树，避免并行占位冲突。

