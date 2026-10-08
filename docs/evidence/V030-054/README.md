# V030-054 可恢复证据

## 来源与范围
- [PRD](https://app.notion.com/p/3f32f5a9e6488163b0bbde935938b58b) / [ADR](https://app.notion.com/p/3f32f5a9e64881c7ba77d2a26309237b)。
- 基线 develop c54c540200a1534d365e3fadb94e55b06e5dc5cd，固定 Node24.14.0 / pnpm10.28.2 / Ajv8.20.0。
- Q7、ADR-002 §5.1/5.6 与 Redis Session §3 决定 JSON 不交付认证/Session/CSRF 秘密，普通 Envelope 保持封闭。独立合成输入不来自真实凭据或被测响应。
- 只加强 API-10 的 JSON 契约 oracle；原 Set-Cookie 文档声明、X-CSRF-Token required 检查和真实 HTTP/Cookie 验收保留。测试数量仍为原十项；没有新增浏览器用例或改生产 Schema。

## 文件与还原
- baseline-test.mjs.txt：旧测试原字节，SHA256 96122e4a4ffab2fb0a4dea97c72325aca712d78831094d36985fc2a032f7fac6。
- execution-records.json：工作树外原始证据文件的路径、字节数、SHA256 和完整 UTF-8 文本。text 字段可按原相对路径还原；manifest.json 对每个文件逐项校验。无依赖缓存、二进制或隔离复制树。
- 包含 before/after-v2 原始命令、时间、退出码、完整 TAP；首次 strict 兼容失败、被哈希拦截的转录错误、原测量脚本与 v2、原安装和各本地检查均保留。
- 正式 OpenAPI 从基线到候选逐字未变，可用基线或本候选 contracts/openapi/openapi.json 重现。回放需新的隔离输出目录，使用归档中的脚本；回放时间不冒充原执行时间。

## 原漏检与实际反例
原全文件基线/恢复均 10 PASS。保持英文 description 的情况下，隔离逐层 additionalProperties=true：
- 原 API-10 四层均 PASS。
- 完整旧文件 top 变异由已有 API-04 捕获；data/meta/pagination 仍各 10 PASS。没有抹去已有顶层保护。

强化测试先验证两个合法合成 Envelope，再验证 top/data/meta/pagination 四层各四个禁止字段，共 18 个样本，输入不可被改写。v2 baseline-full 与 restored-source-full 各 10 PASS；四个变异分别到达对应 forbidden csrfToken 的 ERR_ASSERTION、expected:false、actual:true。这些是实际行为反例，不是导入或编译失败。

## 工具适配和非行为失败
初版注册完整 OpenAPI，after baseline-full 9 PASS/1 FAIL，strict 报 unknown keyword openapi；原件和日志保留，不计行为 RED。
v2 只将原 components.schemas 放入标准 $defs 容器，allOf 使用实际 login201 Schema，只重定位本地组件 $ref。strict/meta-schema 和输入保持均不关闭，不手写部分 validator，不宣称 Ajv 验证整份 OpenAPI3.2.1。
一次补丁漏字在应用前因 hash 不匹配被拦截，修正后完整 SHA 匹配才应用。check-tasks 首次缺 Progress/Handoff 标题失败，Root 只修正文档，后续三项结构复验通过。失败未删除或改成“首次全绿”。

## 局部验证与待验收
契约/底座/治理 390 PASS，typecheck、lint、build、verify-repo、check-tasks、diff 检查成功；12 条既有 lint warning 与 chunk 提示保留，未降低门槛。OpenAPI/package/lock 不变。
本归档写入时，提交后的固定 HEAD 秘密扫描和精确最终候选完整 CI 尚待执行，不能将局部结果当作已合 develop、main 审批或部署。最终事实写回 Notion 与 PR，不为写“通过”再改变冻结代码候选。
