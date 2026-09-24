# Web 前端

先读根AGENTS、HANDOFF、任务文档、Notion路由和测试规范。实际读PRD流程/异常、Figma目标节点、ADR-003状态以及ADR-002/已确认HTTP契约；不自行安装全部候选库或新增SSR/微前端运行时。

先基于用户行为写组件/交互测试并观察RED，再实现页面。当前DOM、内部state和未审阅快照不是正确答案；成功/错误/Loading/键盘/密码可见性均按来源验收，UI不自行发明DTO。

必须实际读取Hubujiu/React-的AGENTS、消费规范及目标组件批准登记。仅按批准范围消费；缺口报COMPONENT_GAP并停止对应组件实现。收藏不是批准，不自写替代通用组件，不擅改sources原件或假定任意适配授权。

浏览器不哈希/校验密码作为认证决策，不将Session或认证Token放localStorage/sessionStorage。路由保护不能替代后端认证。

当前React/Vite/TypeScript入口、锁文件、类型检查/构建已建立，但src/main.tsx仅为空挂载，不是登录UI。V010-006负责批准组件组合；tests/e2e仅底座smoke，产品浏览器验收见tests/acceptance。最终真实E2E不能mock API。

每任务独立branch/worktree，更新同ID任务文档；squash与远程main验收/清理遵循根规则。
