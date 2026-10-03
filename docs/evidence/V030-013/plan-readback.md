# V030-013 PLAN readback

Source: PRD https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa
page_last_edited_at: 2026-10-03T08:40:01.401Z
Source: ADR009 https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f
page_last_edited_at: 2026-10-03T08:40:07.467Z

Read with the authorized Notion connector after the lead's release message.
No tool truncation/unknown-block indicators were returned. The two applicable
PLAN sections compare byte-identically after removing closing page markup.
PRD remains pending review; ADR009 proposed. This freezes only the finite
technical slice, not the whole version's acceptance or production authorization.

## 2026-10-03｜V030-013 应用结构、表单定义与真实 schema Save 后端 PLAN（主负责人技术冻结）
**目标与放行范围**：在既有 B5 应用域与 B2 schema Save 核心上，实现真实应用目录、逻辑数据表、表单定义、常用字段类型和受控结构保存。这是完整 v0.3.0 的独立后端工作流，可在本 PLAN 回读后开始字段核心与 schema 实现，不依赖当前待答的流程／导航集中问题。主负责人在用户既有交付目标及算法取舍授权内制定下列技术选择；执行者不自行改变业务。精确 HTTP DTO／路由产物仍须主负责人审查冻结，前端不能提前消费未冻结的形状。
### V030-013.1 元数据与权限边界
- 扩展既有 B5 应用域元数据：logical tables、fields、form views 均有各自稳定 UUID；目录使用 parent UUID、同 app 约束、无环校验及顺序
- 逻辑表 `schemaVersion` 与视图布局 `viewVersion` 分开管理；多个 form views 引用同一逻辑表，不为每个视图重复创建物理表
- 本切片结构变更仅由当前 owner／可信 Bootstrap 发起；普通成员读取必须基于真实登记的 resource 校验。在后续菜单／数据授权接线完成前，不能从未经校验的资源或伪 appID 放行读取
- 不自动继承目录 grant；权限体系已确认的完整 grant 并集规则不改变。本切片未完成的普通成员资源／数据接线按 fail-closed 处理，不冒充完整产品权限已经可用
### V030-013.2 字段、布局与物理类型
字段 kinds 固定为 `text`、`multiline`、`number`、`money`、`date`、`datetime`、`single_select`、`multi_select`、`boolean`、`member`、`department`。系统字段 `id`、`createdBy`、`createdAt`、`updatedAt`、`recordVersion` 为只读。
- 布局 group／divider／description 独立于业务 SQL 列，不把装饰布局组件建为数据字段
- 选项有稳定 option UUID，多选值去重。删除已被使用的选项须提供明确映射／新值处理，不允许通过删除选项静默擦除原数据；这是保存时的安全校验约束
- 真实 PostgreSQL 业务列使用 text／numeric／date／timestamptz／boolean／uuid／uuid\[\] 等类型。元数据和布局可使用 JSON，业务值不转为 JSONB／EAV
- reference 保存稳定 UUID，新增选择验证当前活跃源对象；源删除保留引用与最后显示的既定语义由后续真实 source adapter 接线，不以外键级联删除历史引用或记录
**数值契约**：
- number／money 在线路中传十进制字符串，不经 float 传递
- 可配置 precision，最大 38；scale 为 0–18
- 处理位数覆盖小数后 18 位至整数侧 18 位，允许十／百等位置；精确 DTO 表达由主负责人合同审查固定
- 舍入枚举为 `HALF_UP`、`HALF_EVEN`、`TOWARD_ZERO`、`FLOOR`、`CEILING`，默认 HALF_UP
- money 默认 scale＝2，number 默认 scale＝0，均可配置；拒绝非有限值与溢出，不静默截断
**日期时间契约**：
- date 使用精确 YYYY-MM-DD
- datetime 接收含显式 offset 的 RFC3339，规范存储为 UTC；可配置 minute／second／millisecond 精度
- 展示时区属于 presentation metadata，不混同于规范存储值
上述是本轮主负责人冻结的技术契约，不伪称用户逐项指定了枚举或默认值。
### V030-013.3 Preview、预检与 Save 事务
- 表单设计器 Save 是结构与布局唯一提交点；preview 是纯预览，不产生 DDL
- Save 携带 `operationId`、`expectedSchemaVersion`、`expectedViewVersion` 与完整 fields／layout
- preview／preflight 返回 change plan 及破坏性影响确认 token；token 绑定 actor、app、table、schema revision、data revision、dependency revision 和规范化 plan hash，不能只绑定行数
- token 按服务端策略过期，精确期限与错误表达须在合同中记录，不由执行者偷设产品承诺
- 提交时持锁重新验证上述事实；计划、数据、依赖或版本变化使旧确认失效，不能用过期确认继续删列
- 新列对旧数据采用配置默认值或 NULL；新增必填列必须先有默认值／补齐方案。任一 cast 失败导致整次事务回滚
- 删除有值列必须持当前有效确认 token；启用流程／在途实例引用字段时，删除或改型必须阻止并列出依赖，普通删除确认不能绕过
- 复用并扩展 B2 真实 `appschema` 执行器至上述常用类型；DDL、metadata、layout、audit 与 operation result 在同一 PostgreSQL 事务提交
- COMMIT 结果未知时按原 operation 核查与幂等恢复；不能换新 key 盲重试，不能把响应丢失报告为确定回滚
### V030-013.4 产品 HTTP、共享文件与集成端口
本任务负责产出产品 HTTP 与 OpenAPI 合同，并实现实际 migration／roles 的代码及隔离验证；禁止应用到生产。
主负责人给出的路由组织候选如下，具体方法／DTO／错误码须经过合同审查后再供前端接入：
- `/api/v1/applications/{appId}/structure`
- 同一 app 下的 `/directories`、`/tables`、`/forms`
- `/forms/{viewId}/definition`、definition preflight 与 definition save
这些候选不被写成已完成或已冻结的全部 HTTP 接口。字段核心和 schema 可以先按本节冻结规则实现；前端消费必须等待精确合同通过主负责人审查。
- 现有共享 BFF wiring、迁移与相关 roles 由 V030-013 单一 owner 协调；不得与 B5 runtime 修复并发修改同一文件
- 开工先扫描实际 HEAD 与迁移目录，再选择未占用编号；不重写既有迁移，不凭旧 main 状态猜新编号
- records／query／Flowable 后续工作流通过导出的事务端口接入，不自行绕过本包 schema／依赖／版本保护
- 实施前登记实际分支、基线与文件 scope；共享变更冲突由主负责人裁决，不扩大为整仓重构
- 不任意调整生产 timeout 或放宽既有权限／安全 guard
### V030-013.5 RED／GREEN 与隔离验收
- [ ] 真实隔离 PostgreSQL、受限运行角色验证 schema／metadata 保存，非法跨 app 引用、目录环、伪资源与 owner／Bootstrap 权限边界正确
- [ ] 十进制字符串、所有舍入规则、负数半值、整数十／百等位置、precision／scale 边界、非有限值及溢出拒绝
- [ ] 日期 offset、UTC 规范化及 minute／second／millisecond 精度契约一致
- [ ] 稳定选项、多选去重；多选转单选存在多个值时阻止整次保存；使用中的选项删除不得静默丢值
- [ ] 并发 schemaVersion／viewVersion CAS，数据与依赖变化使删除确认失效；不能仅凭相同行数复用旧 token
- [ ] enabled／in-flight 流程依赖阻止删列／改型，默认值／NULL／必填补齐与 cast 失败符合已确认规则
- [ ] DDL、metadata、layout、audit、operation result 全部成功或全部回滚；审计／元数据失败不能留下部分已改 schema
- [ ] 真实已提交但响应丢失的 operation 恢复、同 key 重放不重复变更；未知结果不误报确定失败
- [ ] 保留旧迁移／roles 与升级兼容回归；报告实际 RED／GREEN、版本、命令、SHA、复杂度、锁及未测规模风险，未运行项明确列出
**范围与完成声明**：本包未包含完整业务记录 CRUD／query、普通成员的数据字段 grant 全量接线或 Flowable 产品集成，这些属于后续工作流。本包不能单独验收为完整表单平台或 v0.3.0 已交付；当前只冻结并放行上述独立实现。整页 PRD 仍待评审、ADR-009 仍拟议中，main merge／deploy 与生产 DDL／授权未获放行。
