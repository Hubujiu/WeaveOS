# V030-014 → V030-012 Shell 接线合同

从 `apps/web/src/applications/forms/index.ts` 导入 `ApplicationStructurePanel`、`FormDesigner` 及其 props。Shell 负责应用与视图路由；目录面板只通过 `onOpenForm(viewId)` 请求打开设计器，设计器只通过 `onBack()` 请求返回。

| Prop | Shell 提供及处理 |
| --- | --- |
| `appId`、`viewId` | 当前已授权应用和表单视图的 ID；目录面板无需 `viewId`。切换作用域后组件会重新 GET 验证。 |
| `actorId` | 当前已验证会话的 `user.id`，不可用路由参数、缓存显示名或固定测试值代替。所有模块 API 请求均带 `X-Expected-Actor-Id`。 |
| `onOpenForm(viewId)` | 目录面板点击表单时切换至该应用下的视图路由。 |
| `onBack()` | 设计器已处理自己的脏输入弹窗和显式放弃后调用；Shell 切换回工作台。这个回调即使延迟或缺省，放弃慢预检也会在组件内取消，不能再发 PUT。 |
| `onDirtyChange(dirty)` | 维护**当前已挂载作用域**的导航保护。目录输入、设计器草稿、预检中和结果未知的写入都会报告 `true`；组件卸载时会报告 `false`，Shell 应同步清除该视图的全局 dirty guard。不要因此删除模块保留的未知操作包。外部 tab、应用切换、关闭及浏览器 Back 需由 Shell 自行询问当前 dirty guard。 |
| `onUnauthorized()` | 收到 401 时走 Shell 的登录失效处理。模块立即遮罩已缓存的服务端内容和草稿；认证恢复后同一 actor 重挂载会先实时 GET，再恢复本地未保存输入。 |
| `onIdentityMismatch()` | 收到 `AUTH_SESSION_CHANGED` 时重新确认会话身份并切换到相应账号作用域；不要把 A 的草稿重新绑定到 B。模块对 A/B 按 actor/app/view 隔离，并遮罩不再有效的数据。403 是应用权限失败，由模块遮罩，不调用全局登出。 |

目录和设计器的未知写入保留原 `operationId` 与不可变请求包。用户回到**同一个 SPA、同一个 actor/app/view** 后，先经过实时 GET 验证，再使用界面的“查询原操作结果”或“按原请求重试”。Shell 卸载或清除 dirty guard 不得把未知包当作已回滚；浏览器整页刷新不属于当前内存恢复范围，不能承诺跨刷新恢复。保存定义只通过 `preflight → 影响确认 → 原 token 的 PUT` 生效，本地预览不创建记录，也没有发布动作。

模块侧证据见 [V030-014 任务页](../../tasks/V030-014.md)；最终同源 HTTPS Shell 接线、导航和身份流验收由 V030-012 在隔离整合分支完成。
