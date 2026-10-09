# V030-059 · 分流验证与边界

独立需求来自本轮 PRD/ADR 与用户 2026-10-09 明确要求；不是根据旧工作流运行结果生成 expected。

## 真实顺序
1. 04:36 UTC 创建并回读 PRD/ADR。Root 写两份无分流/无汇总的最小加载占位与独立用例。
2. 04:37 UTC 执行 `node --test tests/governance/ci-routing.test.mjs tests/governance/ci-selection.test.mjs`，exit 1：文档/Go/前端仍全量、缺失候选字段、应拒绝的失败/取消/skip 被接受。源码在 RED commit e3b1955；归档保留完整测试/占位。此处是新需求对无行为接口的 RED，不声称生产存在这些接口。
3. `python3 tests/governance/root_ci_routing_test.py` exit 1：现有工作流不存在 selection-gate/输出。缺字典键的 error 只作为声明缺失，不冒充有效行为 RED；Node 的精确断言失败是核心 RED。
4. 最小实现后 110 项 Node、2 项 YAML 接线通过。旧图测试拒绝合法只汇总 always；Root 先补独立有效/无效样例，实际观察同一约束失败，再只豁免固定结果验证器。3 项路由与 11 项原吞吐/磁盘测试通过。
5. 补充三个防回归观测：真实删除、符号链接和真实 CLI 文件输出/候选不匹配，最终路由/门禁 113 项通过。它们是追加回归，不冒充实现前原始 RED。

## 验证层次
- 分类矩阵独立覆盖 Markdown、两个有限 Go 包、前端、混合/未知/共享边界、4001 个路径等。
- 原生临时 Git 仓库真实执行多提交、三点比较、合并树、重命名/删除、特殊 mode、缺失 refs、SHA 绑定；不是 mock Git。
- 汇总器检查确切 needs 集与每个应跑 job 的 success，拒绝 failure/cancelled/skipped/缺失；未选中 skipped 不算测试通过。
- YAML 解析/对象比较只证明配置接线，不冒称 GitHub scheduler 实际执行。三个工作流全部既有 step 对象、矩阵、服务、环境变量、timeout 与权限和原基线完全相同。
- 447 项治理/基础 + 62 项契约共 509 项 Node、本地 typecheck/build、实际 OpenAPI lint 通过（保留原 warning）。Python 14 项通过。
- 本地无 Docker，现有原生秘密扫描用于确切候选的本地扫描；原固定 Docker 秘密/漏洞、全部实际后端/浏览器/产品链依赖最终 GitHub CI，不冒称本地全量。

## 范围限制
文档仍跑七项快速门禁及治理，不是零 CI；源码注释仍按所属代码域。内部 Go 白名单仅 flowgraph 与 flowcommands，不能说所有 Go 都不跑浏览器。前端仍保留完整 product（其中也执行后端），此版没有拆解产品 runner。发布/手动/main/未知全量；不修改分支保护、main、部署权限。

原字节记录在 execution-records.json，各记录有 UTF-8 长度及 SHA256；manifest 校验整包。原始环境文件时间来自容器，不作为 UTC 执行时间依据。完整远端 CI 后在 PR/Notion 汇报，不为完成文字重复生成新候选。
