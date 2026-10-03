# B5a.18 bounded implementation release

## PRD readback

Source: https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa?pvs=204
Last edited: 2026-10-03T06:53:46.766Z

### B5a.18 筛选管理器关闭后的焦点竞态修复 PLAN（主负责人已冻结，2026-10-03）
**诊断结论与证据**：B5a.17 中的 WebKit 假设已由执行者诊断证实。诊断基线 `apps/web/src/TablePresetManager.tsx` 第 58 行附近，较晚到达的退出动画完成回调第二次恢复 trigger 焦点，夺走用户新聚焦 checkbox 的焦点；CI trace 中随后按 Space 重新打开 panel。未改动的原测试在本地通过，说明该缺陷依赖时序，不能据此否定 CI 中的真实问题。
测试专用 commit：fe125489f4a1490039aff636292d27acae70c338。确定性测试控制真实动画完成通知的交付，不使用 sleep；同一断言在 Chromium／Firefox／WebKit 三个引擎均出现目标 RED。[WeaveOS-PR26-filter-focus-diagnosis-RED.zip](https://chatgpt.com/api/library/files/libfile_5263a69f96dc8191a94e2e92b0432bd9/download) 保存本轮诊断证据。本节冻结修复计划，尚不表示代码修复或 GREEN 已完成。
**主负责人冻结的实现约束**：
- 每次关闭建立独立的焦点恢复授权，以单调递增的生命周期 token 判定是否仍有效；fallback、原生动画完成回调与 `finalFocus` 共用同一授权状态，避免重复或过期恢复
- 仅当前关闭且焦点仍需要恢复时，才恢复 trigger。用户较新的主动聚焦／导航使旧恢复授权失效；重新打开与组件卸载也必须使旧完成回调失效
- 因 DOM 移除而自动落到 body 的焦点，不得被误判为用户主动选择而破坏正常关闭恢复
- 本轮不改变点击关闭的业务行为。保留 Escape／outside close、正常回到 trigger、Space 操作 checkbox、重复开关，以及 reduced motion／fallback／native 动画路径
- 不改变 UI、布局、样式或动画曲线；不加任意等待，不 skip 测试，不放宽原断言
**文件范围与执行协调**：
- 仅在独立分支修改 `apps/web/src/TablePresetManager.tsx`、直接相关的焦点测试、证据与任务文档
- 共享 motion helper 不在默认修改范围；若确有必要，先提交具体原因和路径，由主负责人另行批准
- 交付补丁供 B5 owner 集成，不改 PR #21 的远端分支；主负责人负责最终 review，不由执行者扩展其他前端行为
**RED／GREEN 与验收要求**：
1. 保持本次确定性 RED 的原断言不变，修复后在三个浏览器引擎变为 GREEN
2. 覆盖正常关闭恢复、较新的主动焦点、重新打开、卸载及旧通知晚到的边界；同时覆盖 reduced motion／fallback／native 路径
3. 运行完整相关测试、typecheck 与 build，逐项报告通过、失败、skip 和未运行；不能将原测试偶然通过作为竞态已修复的证据
4. 至少交付焦点状态证据。若实际触及视觉样式，必须补现有 panel 关闭／打开／editor 状态截图，并先处理超出默认范围的变更批准；不以截图替代焦点与键盘断言
5. 交付实际文件差异、RED／GREEN、local SHA 与未跑项，由主负责人审查后再交 B5 owner 集成及检查同 HEAD CI
**放行结论**：本 PLAN 回读后可释放上述限定实现，取代 B5a.17 当时“仅诊断与测试准备”的限制。整页 PRD 待评审、ADR-009 拟议中保持；没有整版 v0.3.0 验收、main merge 或 deploy 授权。


## ADR009 readback

Source: https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f?pvs=204
Last edited: 2026-10-03T06:53:53.535Z

### B5a.18 筛选管理器关闭后的焦点竞态修复 PLAN（主负责人已冻结，2026-10-03）
**诊断结论与证据**：B5a.17 中的 WebKit 假设已由执行者诊断证实。诊断基线 `apps/web/src/TablePresetManager.tsx` 第 58 行附近，较晚到达的退出动画完成回调第二次恢复 trigger 焦点，夺走用户新聚焦 checkbox 的焦点；CI trace 中随后按 Space 重新打开 panel。未改动的原测试在本地通过，说明该缺陷依赖时序，不能据此否定 CI 中的真实问题。
测试专用 commit：fe125489f4a1490039aff636292d27acae70c338。确定性测试控制真实动画完成通知的交付，不使用 sleep；同一断言在 Chromium／Firefox／WebKit 三个引擎均出现目标 RED。[WeaveOS-PR26-filter-focus-diagnosis-RED.zip](https://chatgpt.com/api/library/files/libfile_5263a69f96dc8191a94e2e92b0432bd9/download) 保存本轮诊断证据。本节冻结修复计划，尚不表示代码修复或 GREEN 已完成。
**主负责人冻结的实现约束**：
- 每次关闭建立独立的焦点恢复授权，以单调递增的生命周期 token 判定是否仍有效；fallback、原生动画完成回调与 `finalFocus` 共用同一授权状态，避免重复或过期恢复
- 仅当前关闭且焦点仍需要恢复时，才恢复 trigger。用户较新的主动聚焦／导航使旧恢复授权失效；重新打开与组件卸载也必须使旧完成回调失效
- 因 DOM 移除而自动落到 body 的焦点，不得被误判为用户主动选择而破坏正常关闭恢复
- 本轮不改变点击关闭的业务行为。保留 Escape／outside close、正常回到 trigger、Space 操作 checkbox、重复开关，以及 reduced motion／fallback／native 动画路径
- 不改变 UI、布局、样式或动画曲线；不加任意等待，不 skip 测试，不放宽原断言
**文件范围与执行协调**：
- 仅在独立分支修改 `apps/web/src/TablePresetManager.tsx`、直接相关的焦点测试、证据与任务文档
- 共享 motion helper 不在默认修改范围；若确有必要，先提交具体原因和路径，由主负责人另行批准
- 交付补丁供 B5 owner 集成，不改 PR #21 的远端分支；主负责人负责最终 review，不由执行者扩展其他前端行为
**RED／GREEN 与验收要求**：
1. 保持本次确定性 RED 的原断言不变，修复后在三个浏览器引擎变为 GREEN
2. 覆盖正常关闭恢复、较新的主动焦点、重新打开、卸载及旧通知晚到的边界；同时覆盖 reduced motion／fallback／native 路径
3. 运行完整相关测试、typecheck 与 build，逐项报告通过、失败、skip 和未运行；不能将原测试偶然通过作为竞态已修复的证据
4. 至少交付焦点状态证据。若实际触及视觉样式，必须补现有 panel 关闭／打开／editor 状态截图，并先处理超出默认范围的变更批准；不以截图替代焦点与键盘断言
5. 交付实际文件差异、RED／GREEN、local SHA 与未跑项，由主负责人审查后再交 B5 owner 集成及检查同 HEAD CI
**放行结论**：本 PLAN 回读后可释放上述限定实现，取代 B5a.17 当时“仅诊断与测试准备”的限制。整页 PRD 待评审、ADR-009 拟议中保持；没有整版 v0.3.0 验收、main merge 或 deploy 授权。
