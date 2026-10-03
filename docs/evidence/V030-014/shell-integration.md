# V030-014 → V030-012 Shell 接线合同

从 `apps/web/src/applications/forms/index.ts` 导入 `ApplicationStructurePanel`、`FormDesigner` 及其 props。Shell 负责应用与视图路由；目录面板只通过 `onOpenForm(viewId)` 请求打开设计器，设计器只通过 `onBack()` 请求返回。

按冻结 ADR 第 I 节，两组件现在都要求 `registerLeaveGuard(scope,controller):()=>void`。共同的 `LeaveScope`、`LeaveStatus`、`LeaveDecision`、`LeaveResult`、`LeaveController`、`RegisterLeaveGuard` 与 `LeaveGuardProps` 类型从同一 `index.ts` 导出。Shell 返回的 unsubscribe 必须绑定**该次注册实例**；同 scope 旧卸载或 StrictMode 旧 cleanup 不能删去新的 controller。

Shell 弹离开确认前调用 `controller.getStatus()`；确认后调用 `prepareLeave('discard'|'retain_operation')`，先得到 `{ok:true}` 才能卸载或切路由。首次 `getStatus()` 锁定这次确认的状态与输入快照；弹窗期间重复读取只返回当前显示状态，不替换首次确认快照。若返回 `{ok:false,status}`，说明确认期间状态或输入已变，须按新 status 重新说明、再次调用 `getStatus()` 并重新确认。`clean` 可直接离开；`draft`（含未发 PUT 的影响包）与 `preflight` 只允许 `discard`；`write_in_flight` 与 `unknown` 只允许 `retain_operation`。控制器同步取消预检或同步保留原请求包，模块内部“返回工作台”使用同一控制器。`onDirtyChange` 只供兼容显示，不能代替确认后的 `prepareLeave`。

| Prop | Shell 提供及处理 |
| --- | --- |
| `registerLeaveGuard(scope, controller)` | 每次注册返回绑定本次实例的 unsubscribe；Shell 按当前 scope 聚合状态、确认后调用 `prepareLeave`，只有 `{ok:true}` 才离开。 |
| `appId`、`viewId` | 当前已授权应用和表单视图的 ID；目录面板无需 `viewId`。切换作用域后组件会重新 GET 验证。 |
| `actorId` | 当前已验证会话的 `user.id`，不可用路由参数、缓存显示名或固定测试值代替。所有模块 API 请求均带 `X-Expected-Actor-Id`。 |
| `onOpenForm(viewId)` | 目录面板点击表单时切换至该应用下的视图路由。 |
| `onBack()` | 设计器已处理自己的脏输入弹窗和显式放弃后调用；Shell 切换回工作台。这个回调即使延迟或缺省，放弃慢预检也会在组件内取消，不能再发 PUT。 |
| `onDirtyChange(dirty)` | 兼容显示**当前已挂载作用域**的脏状态。目录输入、设计器草稿、预检中和结果未知的写入会报告 `true`；组件卸载时报告 `false`。这只撤销显示 guard，不清草稿或未知操作包。外部 tab、应用切换、关闭和浏览器 Back 必须走上面的 controller 确认。 |
| `onUnauthorized()` | 收到 401 时走 Shell 的登录失效处理。模块立即遮罩已缓存的服务端内容和草稿；认证恢复后同一 actor 重挂载会先实时 GET，再恢复本地未保存输入。 |
| `onIdentityMismatch()` | 收到 `AUTH_SESSION_CHANGED` 时重新确认会话身份并切换到相应账号作用域；不要把 A 的草稿重新绑定到 B。模块对 A/B 按 actor/app/view 隔离，并遮罩不再有效的数据。403 是应用权限失败，由模块遮罩，不调用全局登出。 |

目录和设计器的未知写入保留原 `operationId` 与不可变请求包。用户回到**同一个 SPA、同一个 actor/app/view** 后，先经过实时 GET 验证，再使用界面的“查询原操作结果”或“按原请求重试”。Shell 卸载或清除 dirty guard 不得把未知包当作已回滚；浏览器整页刷新不属于当前内存恢复范围，不能承诺跨刷新恢复。保存定义只通过 `preflight → 影响确认 → 原 token 的 PUT` 生效，本地预览不创建记录，也没有发布动作。

模块侧证据见 [V030-014 任务页](../../tasks/V030-014.md)；最终同源 HTTPS Shell 接线、导航和身份流验收由 V030-012 在隔离整合分支完成。
