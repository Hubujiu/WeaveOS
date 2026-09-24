# 测试先行与独立验收

## 顺序与预期

已确认需求→验收用例→测试代码→实际目标RED→最小实现GREEN→重构→回归。代码、配置、迁移、脚本同样适用；禁止先实现再补测试。每用例记录来源/需求ID、Given/When/Then、层级、文件。预期来自PRD、已接受ADR、契约、数学/协议定义及独立样例，不来自实现输出或复制实现算法。

阅读实现可定位缺陷与入口，不能决定应该怎样。Characterization只能标记现状，不代替需求验收。真实缺陷必须先有失败复现。改预期只有在独立来源变化或确认测试有误时，说明依据/审查并重新RED，不能为当前实现变绿。

## 有效RED与证据

记录实现前测试版本、命令、环境、退出码、失败用例及断言原因。运行器要成功加载并到达目标断言；缺接口仅可最小无行为占位。导入/语法/依赖/网络故障不是有效RED。环境不足报告NOT RUN/BLOCKED，不伪造日志、时间、截图或审批。

每个任务独立branch/worktree，工作分支先保存RED再GREEN；按用户要求squash前在docs/evidence/<task>/保留可恢复源码/占位/补丁、命令与结果。之后重放必须标REPLAY，不能冒充原始时间。见V010-001证据示例。

## 分层

单元测试验证领域可观察结果，不锁死无关私有调用顺序；只mock外部边界，不mock被测对象。可控时钟与随机来源避免sleep式TTL测试。

HTTP依据OpenAPI/ADR-002验证状态、Envelope及例外、身份、错误/字段；生成类型不代替实现一致性测试。真正跨服务时才增加Proto兼容、错误详情与互操作。

真实PostgreSQL/Redis集成验证事务、唯一性、邀请码并发、TTL、退出/续期/撤销；内存fake不能证明存储语义。迁移检查空库/升级/约束索引/评审回退策略，禁止生产数据库跑测试。

组件/浏览器依PRD和原型验证可见结果、键盘/Loading/错误；不把当前DOM/截图自动批准为正确。最终E2E必须真实BFF与存储；API mock仅适用于隔离组件测试。视觉质量还需产品审阅。

## 禁止伪绿

禁止删断言、弱化正确预期、skip/only、passWithNoTests、continue-on-error掩盖失败、用自身输出生成expected、未经来源核对接受snapshot、降低门槛或用覆盖率替代验收。缺少自动测试列待完成，不转成“GitHub不支持”。纯文档格式可TDD:N/A并说明适用校验。

## 当前命令与状态

```sh
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
node scripts/verify-repo.mjs
node scripts/check-tasks.mjs
(cd services/bff && go test -race ./... && go vet ./...)
pnpm typecheck && pnpm build
pnpm exec playwright test tests/e2e
```

这些是当前工程底座验证。产品用例见tests/acceptance（HTTP与浏览器），需未来已评审契约、Seed、真实业务/存储/HTTPS装配；当前不能PASS。整版矩阵 `node scripts/check-release.mjs`当前预期失败。具体自动/人工条目、FR映射和待补用例见docs/acceptance/README.md。

结构检查只证明文件/规则/状态格式，不证明实际Notion阅读、TDD时序、业务正确或人工签署。任务最终完成须打开真实CI和来源证据核对，再squash至main；远程main同ID任务文档全完成加merged PR才代表验收。
