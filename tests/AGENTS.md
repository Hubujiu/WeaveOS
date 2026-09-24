# 测试目录

先读根AGENTS、HANDOFF、任务文档和docs/testing.md，再实际读取被测行为PRD/ADR/契约。测试也受Notion、独立预期和RED先行约束。

预期来自独立来源/样例，不来自实现结果、复制算法或被测对象自己的mock。代码只帮助定位接口。修缺陷先写复现失败用例，不删断言/无依据改expected/skip/only/降低门槛/盲目批准快照。真实存储语义使用真实隔离实例；缺环境报告BLOCKED。

governance覆盖原仓库结构；foundation覆盖工程/任务/发布规则；e2e当前仅三浏览器底座smoke；acceptance为产品HTTP/浏览器预期，业务尚未完成。完整映射和未完成自动化见docs/acceptance/README.md。不得把12个底座smoke写成登录系统已验收。

API/页面测试绑定在V010-002/006对照来源确认，不以新实现的当前响应作为改预期理由。验收矩阵结构检查不证明证据真实，不能删除未完成条目假装整版完成。

每任务保存独立RED/GREEN及长久可恢复证据；squash后不能只依赖到期artifact或已删分支。分支/worktree和远程main验收规则遵循根AGENTS。
