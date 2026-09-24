# 测试目录

先读根 AGENTS 和 `docs/testing.md`，再实际读取被测行为的 PRD/ADR/契约。测试也受 Notion 门禁、独立预期和先 RED 后实现约束。

测试预期来自来源及独立样例，不来自实现输出、复制的算法或被测对象自己的 mock。现有代码可以帮助定位接口，不定义正确答案。修 bug 先写能够重现问题的失败测试。

不得删断言、无依据改 expected、skip/only、降低门槛或盲目接受快照。真实存储语义需要真实隔离存储验证；缺失环境报告 BLOCKED，不能假装通过。

`governance/policy.test.mjs` 覆盖仓库结构检查器，不是产品登录测试，也不证明 Notion 阅读或 TDD 时序真实性。其独立需求位于 `docs/repository-checks.md`；fixture 不导入实现常量作为预期来源。
