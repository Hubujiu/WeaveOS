# v0.1.0 任务索引

- [V030-057 · 发布统计实际落盘验证](V030-057.md) — 同步安全基线 develop `d9c2e60b`，新增报告到结果文件的真实行为测试，PR65 重新验收中。

- [V030-058 · Go安全补丁与独立回滚候选](V030-058.md) — [PR #66](https://github.com/Hubujiu/WeaveOS/pull/66) 已合入 develop `d9c2e60b`；Go1.27.2与必要依赖修补、独立回滚候选及完整CI已验收。

- [V030-056 · BPMN 固定图边完整映射验证](V030-056.md) — task/V030-056-bpmn-edge-oracle / ../WeaveOS-worktrees/V030-056；基线 develop `a6f04840`，只加强既有固定样例的边/default集合，当前 in_progress。

- [V030-055 · 磁盘准备真实拒绝与调用边界](V030-055.md) — [PR #63](https://github.com/Hubujiu/WeaveOS/pull/63) 已合入 develop `a6f04840`；四个原文字漏检已由真实Bash调用矩阵检出，最终三workflow20jobs首轮通过，原安全守卫保留。

- [V030-054 · 登录响应敏感字段契约校验](V030-054.md) — [PR #62](https://github.com/Hubujiu/WeaveOS/pull/62) 已合入 develop `0ab6986c`；真实 Schema 校验、四层反例及完整 CI 已验收，首轮 WebKit 准备失败与同头复跑保留，冻结文档保留验收前状态。

- [V030-053 · BFF 构建成本测量](https://app.notion.com/p/3f32f5a9e648818eb300f21b1f4ccbf2) — 无代码改动/PR；九次暖构建实测后两次重复合计中位约 3.620 秒，当前保留原独立 fixture。完整脚本与原始结果已在 Notion 归档，不宣称端到端 CI 提速。

- [V030-052 · 完整密码 Schema 校验](V030-052.md) — [PR #61](https://github.com/Hubujiu/WeaveOS/pull/61) 已合入 develop `c54c5402`；标准校验器、JSON 类型矩阵及完整 CI 已验收，首次字体超时与限定同头复跑证据保留。

- [V030-051 · 生产筛选管理器覆盖迁移与旧测试退役](V030-051.md) — [PR #60](https://github.com/Hubujiu/WeaveOS/pull/60) 已合入 develop `440d171e`；完整CI及两轮真实产品验收通过，默认组件490项，最终事实与归档见本批Notion。冻结任务保留验收前记录。

- [V030-050 · 剩余重复测试删减与回执分层](V030-050.md) — [PR #59](https://github.com/Hubujiu/WeaveOS/pull/59) 已于 2026-10-08 合入 develop `3a33e710`，最终 source `e41ab9e0`；重复人员布局与创建回执矩阵已精简，冻结任务保留合入前记录。剩余生产筛选覆盖迁移见 V030-051。

- [V030-049 · CI 提速与完整测试身份汇总](V030-049.md) — [PR #58](https://github.com/Hubujiu/WeaveOS/pull/58) 已于 2026-10-08 合入 develop `771732e9`，最终 source `e404b4cc`；缓存/preflight/组件四片及实际指针刷新回归已交付，冻结任务保留合入前记录。

Java 审批命令身份 A 段：[V030-030](V030-030.md)，task/V030-030-engine-execution / ../WeaveOS-worktrees/V030-030；精确 V029 80e4151 基线及 Root f869911 测试，[draft PR40](https://github.com/Hubujiu/WeaveOS/pull/40) 叠在 V029。云环境真实 Java8/独立门禁12 RED→GREEN，旧部署18/RPC9/互操作1/proof24通过；Root 提供全部测试及 CI 枚举，执行者仅实现 CommandEnvelope 和受测独立门禁，复用指定安全补丁。最终远端 CI 待查询，B/C 合同待 Root，前端暂停，无 main 合并或部署。

后台外壳修复：[V010-020](V010-020.md)，依赖已验收019；task/V010-020-admin-shell / ../WeaveOS-worktrees/V010-020。用户2026-09-30要求全视口背景、图标水平折叠及arca-ui动效参考，单Agent。

后端精简剩余需求：[V010-018](V010-018.md)，依赖已验收016/017；task/V010-018-backend-simplification / ../WeaveOS-worktrees/V010-018，[PR19](https://github.com/Hubujiu/WeaveOS/pull/19)。用户2026-09-29确认采用剩余全部方案，单Agent实施；日志留存参数见任务文档及Notion Q20。

公网入口：[V010-015](V010-015.md)，依赖已验收011；task/V010-015-public-https / ../WeaveOS-worktrees/V010-015。用户2026-09-28明确部署授权，ADR004已接受；单Agent实施，不触碰014界面工作树。

自动交付：[V010-016](V010-016.md)，依赖已验收008/011；task/V010-016-auto-deploy / ../WeaveOS-worktrees/V010-016。用户2026-09-28授权main通过检查后自动部署，包含已验收兼容迁移与配置。单Agent独立工作树。

后端边界：[V010-017](V010-017.md)，依赖已验收005；task/V010-017-auth-boundary / ../WeaveOS-worktrees/V010-017。[PR18](https://github.com/Hubujiu/WeaveOS/pull/18)实现用户已确认的同进程Web适配与认证用例分离，不新增微服务或改变部署。

状态以每个任务文件和GitHub PR/远程main为准，索引不复制第二份可能漂移的完成状态。

| 任务 | 工作内容 | 依赖 | 分支 / 工作树 |
| --- | --- | --- | --- |
| [V010-001](V010-001.md) | 工程底座、CI、测试框架、交接治理 | 无 | task/V010-001-foundation / ../WeaveOS-worktrees/V010-001 |
| [V010-002](V010-002.md) | 接口契约与未决项 | 001 | task/V010-002-contracts / ../WeaveOS-worktrees/V010-002 |
| [V010-003](V010-003.md) | 数据迁移、持久化、验收Seed | 002 | task/V010-003-persistence / ../WeaveOS-worktrees/V010-003 |
| [V010-004](V010-004.md) | Session、CSRF、身份 | 002、003 | task/V010-004-session / ../WeaveOS-worktrees/V010-004 |
| [V010-005](V010-005.md) | 注册登录、邀请、管理员、审计 | 002、003、004 | task/V010-005-auth / ../WeaveOS-worktrees/V010-005 |
| [V010-006](V010-006.md) | 登录/注册UI | 002；可与后端并行 | task/V010-006-web / ../WeaveOS-worktrees/V010-006 |
| [V010-007](V010-007.md) | 真实全栈集成和E2E | 003、004、005、006 | task/V010-007-acceptance / ../WeaveOS-worktrees/V010-007 |
| [V010-008](V010-008.md) | 发布、恢复、安全债务验收 | 007 | task/V010-008-release / ../WeaveOS-worktrees/V010-008 |
| [V010-011](V010-011.md) | 已验收成品服务器运行（SSH隧道） | 008 | task/V010-011-server / ../WeaveOS-worktrees/V010-011 |
| [V010-010](V010-010.md) | Notion 阻塞问题集中答复与来源规则 | 002 | task/V010-010-source-truth / ../WeaveOS-worktrees/V010-010 |
| [V010-012](V010-012.md) | Figma 自适应布局与手工浏览器视口修复 | 006 | task/V010-012-responsive / ../WeaveOS-worktrees/V010-012 |
| [V010-013](V010-013.md) | PR 收尾规则与遗留工作树清理 | 001 | task/V010-013-cleanup / ../WeaveOS-worktrees/V010-013 |
| [V010-014](V010-014.md) | 同步更新后的 Figma 登录注册布局 | 012 | task/V010-014-figma-refresh / ../WeaveOS-worktrees/V010-014 |

只领取依赖已经验收的任务。后续任务尚未启动，分支和工作树在领取时创建，不伪造Agent运行。共享契约由002维护；存储归003；Session归004；领域/HTTP装配归005；视觉/组件归006；跨层验证归007；运行配置归008。变更他人目录先登记协调，不跨任务覆盖。

新增用户授权的测试先行任务：[V010-009](V010-009.md)，依赖已验收001；task/V010-009-test-first / ../WeaveOS-worktrees/V010-009。用户2026-09-27追加授权完成PR #3：扩展HTTP/浏览器已通过，剩余19项真实STORE已本机通过并纳入CI；等待最终head完整远端复验，不提前accepted。正式来源和002–008验收不被本任务替代。

本轮完整接入：[V010-019](V010-019.md)，依赖已验收018；task/V010-019-complete-app / ../WeaveOS-worktrees/V010-019。用户要求补齐最新登录注册、Home导航与已批准人员管理R3；Q24确认按Figma原版复刻，组件优化后置。单Agent。

最新尺寸、后台动画与Arca原版表格：[V010-020](V010-020.md)，依赖已验收019；task/V010-020-admin-shell / ../WeaveOS-worktrees/V010-020，[PR21](https://github.com/Hubujiu/WeaveOS/pull/21)。固定56顶栏/176侧栏与全视口材质、Q32六处Select/四页签Pill；Q35身份复用模板卡片+详情，成员/操作记录直接消费固定22模块Arca源码/完整样式Motion，启用当前页排序/拖列/列宽/真实服务端页大小，少零数据空网格填满/分页贴底。Notion/Figma已同步重读，严格多阶段真实RED→GREEN；最终143完整组件/54源三引擎、类型构建/132治理/官方依赖audit通过，永久原字节/哈希/必要补丁及截图在q35。ready可审阅；提交后的实际HEAD归档安全与五项CI另按PR实时核对，不冒充accepted/合并/部署。root代码唯一写者，子Agent只读复审与独立项目Figma同步。

受控部署 P1：[V030-022](V030-022.md)，Root 合同 737daa4、未合 V020 基线 d99b8eb；[draft PR32](https://github.com/Hubujiu/WeaveOS/pull/32) base 为 task/V030-020-workflow-http。实际 RED／GREEN 与未改旧 proof 回归见任务证据，待 Root 验收，不代表 P2 或整版完成。

新风格表单设计器：[V030-023](V030-023.md)，Root设计/测试/验收；基于 `c935510` 叠在 `task/V030-021-monochrome`，[draft PR34](https://github.com/Hubujiu/WeaveOS/pull/34)。限现有设计器及共享字段/弹窗视觉，不改业务/API/Shell，不合main或部署；实际证据与验收状态见任务文档。

已验前后端独立整合：[V030-024](V030-024.md)，Root冻结精确后端 `24d07a5` 与前端 `3900bca`，独立 `task/V030-024-integrate-verified`，[draft PR35](https://github.com/Hubujiu/WeaveOS/pull/35) 叠在 V022；仅任务索引冲突，保留原侧 blob 与完整历史，联合验证后交 Root 审查，不合 main 或部署。

部署RPC：[V030-026](V030-026.md)，task/V030-026-deployment-rpc / ../WeaveOS-worktrees/V030-026；基线为独立整合候选21a7912，Root合同与测试，执行者仅实现部署通信，无main合并/部署。

审批命令版本与历史兼容：[V030-029](V030-029.md)，task/V030-029-command-v2 / ../WeaveOS-worktrees/V030-029；从已验证V026 a093af1独立创建，Root06ead合同与12项测试，[draft PR39](https://github.com/Hubujiu/WeaveOS/pull/39)，云环境真实RED/GREEN已保存，12新测试／46全包／21正式Ledger-fence通过；仅协议切片，非全链路完成。

V030-030 B 段独立合同 RED：`task/V030-030-engine-actions` / `../WeaveOS-worktrees/V030-030-engine-actions`，Root权威测试提交66b8862；test-compile exit0，真实Java33 RED（5failures/27errors/0skip，exit1），Python16 RED（15拒绝通过/1占位错误，exit1）。[原始证据](../evidence/V030-030/execution-registry-red/README.md)。未实施，等待Root审查；无全CI/push/PR/部署，A530与兼容性5保留。

V030-030 B 段受权实现待Root复核：`task/V030-030-engine-actions`，冻结Root33+兼容5真实GREEN、独立gate16/38通过，旧部署18/解析8/RPC9/互操作1与governance240通过。[实现证据与边界](../evidence/V030-030/execution-registry-green/README.md)。完整head CI等源码复核后运行；SQL仍为隔离测试fixture，未接HTTP/bootstrap，不宣告全链路交付。

V030-030 Root复核修复：补测4086e897的3新例先实际RED，再修取消/身份判定先后与严格UTF8；最终36+兼容5及gate16/41全绿，旧18/8/9+互操作1/governance240重跑通过。[修复证据](../evidence/V030-030/execution-review-fix/README.md)。等待Root复核后完整远端CI，无main/部署或产品HTTP交付。

V030-030 B 段交付：[Draft PR42](https://github.com/Hubujiu/WeaveOS/pull/42)，base冻结A分支`task/V030-030-engine-execution`530d877，PR40不动。Root已完成17dd0b0源码/证据复核并放行最终metadata head完整远端CI；行为/测试源码不变，无status-only反复提交。仍未接产品执行RPC/HTTP、业务权限/投影/触发或正式bootstrap，非全链路交付。

Go执行负载与结果验证：[V030-031](V030-031.md)，task/V030-031-execution-codecs / ../WeaveOS-worktrees/V030-031；Root合同/15测试/8固定向量，[draft PR41](https://github.com/Hubujiu/WeaveOS/pull/41)，独立基线530d877；仅有界codec，云端RED/GREEN及基准已保存，待Root源码审查和最终CI，不代表完整审批或全链路完成。

执行RPC：[V030-032](V030-032.md)，task/V030-032-execution-rpc / ../WeaveOS-worktrees/V030-032；精确JavaB b6b10080 + Go codec888ac429，[Draft PR43](https://github.com/Hubujiu/WeaveOS/pull/43) base为JavaB。Root亲写合同/测试并已核验真实RED、三文件源码及本地GREEN；冻结测试/契约保持不变，等待最终固定head全部8项远端CI。未整合V027运行接线，无main合并/部署。

显式发布与可靠恢复：[V030-027](V030-027.md)，draft [PR#38](https://github.com/Hubujiu/WeaveOS/pull/38)，task/V030-027-publication / ../WeaveOS-worktrees/V030-027；叠在冻结 V026，Root合同与19项真实存储/HTTP测试，执行者实现持久发布编排。后台同请求重试和最终权限/结构/关闭复查已明确确认；生产配置、main合并与部署另行授权。
业务执行结果原子投影：[V030-033](V030-033.md)，task/V030-033-execution-projection；整合执行RPC与显式发布，Root亲写契约/测试并负责debug，执行者只按指令实现；最终head远端CI待核验，无main合并或部署。
执行回执原始证据接线：[V030-034](V030-034.md)。

持久命令派发与断线恢复：[V030-035](V030-035.md)，task/V030-035-execution-recovery；原负载持久接受、短事务队列与真实Java恢复已有限验证，最终CI待核验；用户HTTP与完整审批依据另行接线。

审批依据重建与字段复用：[V030-036](V030-036.md)，task/V030-036-approval-evidence；不可变字段复用、真实数据采集、有界成本与备份验证完成，最终0b8969be的9项CI由Root核验全部通过。仍不代表用户HTTP或完整后端交付，未合main/部署。

当前任务预览与审批操作接线：[V030-037](V030-037.md)，task/V030-037-approval-admission；核心30/38与真实HTTPS4/15、契约登记6及旧契约15通过，真实Java/备份/完整回归与最终CI待验收。Root亲写测试、debug及审查；当前为未完成检查点，前端暂停。

Flowable原生版本与流程层精简：[V030-038](V030-038.md)，Root本人设计/实现/测试/debug，不委派；原生版本/catalog接线与节点经过替换已取得分段真实GREEN，完整回归、成本观察及后续状态精简仍待。流程删除但独立操作日志保留，旧逻辑删除解读已撤回，不宣称整版交付。

- [V030-039 · source-map-js安全补丁与回滚候选](V030-039.md) — b28精确11项CI通过，代码已集成develop；main未推广

- [V030-040 · develop集成与main审批分离](V030-040.md) — PR50最终11项CI通过并squash合入develop c33d49c；main保留用户批准，旧main-only工具限制已说明

- [V030-041 · 独立Flowable启动与后台生命周期](V030-041.md) — 内部不使用TLS；Root先行runner八项RED，身份无关切片实施中，正式服务与迁移仍待后续验收

- [V030-044 · 记录触发与并行流程](V030-044.md) — 主助手自有环境已接通手动/新增/实际修改触发、持久启动恢复及安全流程发现；PR69实现候选2b25d701全CI通过，最后文档提交复验后集成；完整返工重审等后续任务仍待
- [V030-043 · 审批节点局部保存](V030-043.md) — 基于已合develop的8ecd39a2；Root亲写RED与验收，节点局部Save、六身份正式引擎组合及同记录多流程/并发审批已实测通过；develop [draft PR#53](https://github.com/Hubujiu/WeaveOS/pull/53)与精确head CI待审，公开触发/返工重审为后续，前端暂停。

- [V030-045 · 撤回与实际节点退回](V030-045.md) — 当前资格、原子命令接受及HTTP分层通过；真实Flowable和最终CI待验收

- [V030-046 · 记录关联流程摘要查询](V030-046.md) — 服务8项、HTTPS4项及完整service174主/93子通过，Root复核无关编辑权限不刷新；等待完整CI/性能索引

- [V030-047 · 最后实例确认后的自动关闭收尾](V030-047.md) — 七项真实PG、累计八身份正式进程及最终13项CI通过，PR56已合develop0a65e58f；完整删除仍后续

- [V030-048 · 精确行为断言与测试成本整改](V030-048.md) — [PR #57](https://github.com/Hubujiu/WeaveOS/pull/57) 已于 2026-10-08 合入 develop `3272446d`。本批覆盖原 84 组中的 58 组整改，其余 26 组保留/暂缓/历史范围，不能称 84 组全部完成；冻结任务内的待验收文字是合入前记录。

- [V030-059 · 按改动影响范围分流 CI](V030-059.md) — 用户明确要求建立分流；Root 先行测试与失败闭合，main/发布全量，不部署。

- [V030-060 · 实测纯文档 CI 分流](V030-060.md) — V059 全量已通过并合 develop；仅 Markdown 的真实 PR 验证重测试跳过及汇总门禁，结果待实跑。

- [V030-061 · 当前用户跨应用审批待办](V030-061.md) — 主助手直接执行；当前规范和先行测试，未实现、未验收。
