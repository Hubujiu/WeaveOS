## 2026-10-03｜V030-012 原版 AppShell 与应用管理前端接线 PLAN（主负责人已冻结）
**交付定位与放行**：本任务是完整 v0.3.0 的首个可执行前端切片，不是可独立声称功能齐全的最终版本。用户已要求按调整前 Figma 交付，本节回读后可按限定范围开始实现，不等待尚未定案的流程细节，也不等待所有后端完成。完整表单、记录、审批与模板等已确认目标继续保留，后续由主负责人按缺口审查冻结其他前后端任务与接口。
### V030-012.1 已冻结视觉与接线基线
- 原版主页：[Home 358:18205](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=358-18205)
- 原版应用中心：[327:2132](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=327-2132)
- 原导航／侧栏／数据表：[108:151](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=108-151)，保留顶部 56、侧栏 176 与连续 Chrome 的风格／交互
- [WaveOS Original Figma Implementation Handoff.zip](https://chatgpt.com/api/library/files/libfile_0ae036f216cc8191a5315b8e5ced9071/download)，主负责人回报归档 v0，并已核实为 React＋Vite 的高保真交接内容；本任务按原 Figma 基线接线，不实施 Precision／Taste 改版
- 实施前先检查现有 routing、Session／auth 及人员页面边界，再决定具体接入点，不能先重建框架绕开既有边界
### V030-012.2 本切片实际实现范围
1. 建立可复用、长期挂载的原版全局 AppShell，接入应用标签、导航与现有路由；保留现有 auth、personnel 和统一 Table 组件
2. 使用现有 B5 真实 API 实现应用列表、详情、access 与创建。应用元数据来自既有 GET／POST applications，不使用本地伪应用列表冒充保存或权限通过
3. 实现权限组、成员与菜单授权管理，按 B5 当前 API 的能力和 owner／Bootstrap 管理门禁接线
4. 当前可配置菜单 grant 仅为真实 application 根入口。数据权限 API 尚未开放完整字段／记录能力，不能把数据权限 UI 画成已可用，也不能把当前根菜单授权宣称为完整权限矩阵
5. 为后续表单模块保留真实 form tabs／内容布局接口，不创建假的表单项目或可点击但没有真实动作的按钮来计入完成
6. 应用内分组、记录与表单的新后端契约由缺口审查后的其他 owner 接续；本切片中的“权限组”不等于应用目录分组，不混用对象或伪造接口
### V030-012.3 状态、写入与权限规则
- 创建入口是否可见由服务器可信 capability 决定，不能靠前端硬编码角色名；实际创建和权限配置仍以服务器当前授权结果为准
- 成员／菜单配置遵现有 B5 owner／Bootstrap 权限、完整替换、operationId 与 policy revision 契约；服务器拒绝时前端展示真实错误，不显示假成功
- 展示真实 loading、empty、unauthorized、error、retry 状态。只有服务器已确认写入成功才提示成功；未确认结果按既有 operation 查询／恢复语义处理，不将网络异常冒充回滚或静默成功
- 保留未保存内容的关闭保护；跨应用切换、重新导航与异步返回不得将旧结果覆盖到错误应用
- 个人 pin、临时标签与跨应用保留表单／位置的已确认方向保持；跨设备 pin 与刷新／重开后的临时状态恢复仍待用户集中规则答复。可以准备组件和状态接口，但不能自行选 localStorage 或其他持久化 fallback 作为产品规则
### V030-012.4 文件归属与实施约束
- 前端 owner 独占本任务的 `apps/web` 中 apps 域组件、hooks、routes 与必要 AppShell 接线，具体文件路径经现有结构检查后写入 task scope
- `apps/web/src/TablePresetManager.tsx` 当前由 PR #26 CI 修复 owner 管理，本任务不得修改
- 保留既有登录、权限、人员、表格功能，不通过大范围迁移或样式重做扩大本任务
- 不引入新框架或依赖；若确有技术必要，先将原因、选型和影响写入契约，由主负责人处理，不由执行者自行新增
- 独立任务分支；共享导航、API 契约、迁移等冲突由主负责人协调唯一 owner。本节只放行指定实现，不代替 main merge／deploy 授权
### V030-012.5 验收与交付
- [ ] 真实 API 集成验证应用列表／详情／access／创建，以及权限组成员和根菜单授权：owner、Bootstrap、普通成员、create-only、无权限与跨 app 场景符合 B5 契约
- [ ] 创建成功后真实持久化并可重读；失败、冲突、未确认与重试均展示实际响应，无 mock success
- [ ] 权限变更后实际菜单访问与服务器一致；仅 membership 无菜单不授入口，菜单授权不虚构数据能力
- [ ] 原版 Shell、应用中心及本切片所有新增页面／弹窗／加载／空／错误／无权限状态提供实际截图，与指定 Figma 基线逐项核对
- [ ] 键盘操作、焦点、减少动态效果、快速导航／切换和未保存关闭保护通过；不以任意 sleep、skip 或放宽断言过关
- [ ] 运行相关集成测试、typecheck／build 与受影响旧功能回归，记录真实命令、SHA、结果、截图及未运行项
- [ ] 本切片完成时明确报告仍未接入的表单／记录／流程／模板范围，不以原型占位或当前 API 子集宣称整版可交付
**完成判断**：主负责人据真实接口、视觉与行为证据验收本切片，再与后续工作流集成。整页 PRD 仍待评审、ADR-009 仍拟议中；完整 v0.3.0 功能与最终用户验收门槛不缩减。
