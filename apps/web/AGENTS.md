# Web 前端

先读取根 `AGENTS.md`、`docs/notion-router.md` 和 `docs/testing.md`。本文件不得弱化 Notion 门禁与测试先行要求。

读当前 PRD 的流程、异常与 Figma 节点，再读 ADR-003 的当前状态和 ADR-002/实际 HTTP 契约。React + Vite 是方向；没有评审不要自行安装整套候选库、指定版本或加入 SSR/微前端运行时。

先基于用户行为写组件/交互测试并观察 RED，再实现页面。不得按当前 DOM、组件内部 state 或自动快照倒推需求；成功、错误、Loading、键盘操作及密码可见性均按原型/需求验证。UI 与业务 API 之间不能自行发明 DTO。

必须主动读取 `Hubujiu/React-` 的 AGENTS、消费规范及目标组件批准登记。只能在明确批准范围内消费；缺少匹配组件报告 `COMPONENT_GAP` 并停止该组件实现。收藏不是批准，不自写通用组件，不擅改 sources 原件，也不默认拥有任意样式/适配权限。

浏览器不处理密码哈希或认证决策；不把 Session ID/认证 Token 放入 localStorage/sessionStorage。路由保护只是交互层，不能替代服务端认证。

当前目录只定义边界，没有前端应用、依赖锁文件、运行/构建或浏览器测试结果。
