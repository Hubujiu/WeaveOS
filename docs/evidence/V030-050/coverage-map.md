# V030-050 覆盖映射

基线 `771732e904e996e288eceaffd63eb6ef68a75caa`。这是 Root 审查后的覆盖映射；定向与受影响文件验证已经执行，最终精确候选完整 CI 仍待完成。

## 布局重复

删除 personnel.component.spec.ts 的 `Root Mono narrow personnel navigation remains reachable`。
保留 `Root Mono personnel keeps reachable navigation and full-bleed surface at 320`：同为先设置 320×844、进入 admin、调用 assertMonochromeLayout，且额外检查不存在折叠按钮。
原宽度矩阵 2504/1920/900/390/320 全部保留。另保留 dirty 离开保护、reduce/no-preference、1000 边界、1440 图标居中、resize 后 reload、任意宽高增长、键盘滚动及人员权限拒绝。

## 九种创建回执

九种原样例都在 tests/foundation/application-create-receipt.test.mjs 中保留，调用实际 applicationApiEnvelope 和生产共用 validCreatedApplication。

| 原浏览器变体 | 实际拒绝层 | 新低层覆盖 | 浏览器处理 |
| --- | --- | --- | --- |
| 201 empty JSON | HTTP envelope | 精确 ApplicationError/unconfirmed | 下移，JSON null 为该层代表 |
| 201 JSON null | HTTP envelope | 精确 ApplicationError/unconfirmed | 保留 |
| 201 error envelope | HTTP envelope | 精确 ApplicationError/unconfirmed | 下移，JSON null 为该层代表 |
| 201 missing data | HTTP envelope | 精确 ApplicationError/unconfirmed | 下移，JSON null 为该层代表 |
| 201 malformed application | 实体 | API 原值及生产谓词 false | 下移，wrong owner 为该层代表 |
| 201 wrong owner | 实体 | API 原值及生产谓词 false | 保留 |
| 201 invalid application UUID | 实体 | API 原值及生产谓词 false | 下移，wrong owner 为该层代表 |
| 201 wrong initial revision | 实体 | API 原值及生产谓词 false | 下移，wrong owner 为该层代表 |
| 200 valid application body | 预期 HTTP 状态 | 精确 ApplicationError/unconfirmed | 保留 |

另有正确 201、当前 owner、初始 revision=1 的低层成功对照。浏览器保留项仍实际渲染组件，经 hook 显示未确认和核查入口，点击同操作重试后确认完整请求体一致；不修改这些断言。原网络丢失后重试成功、核查 404 不作回滚、身份换人、401 重认证、禁止跨 actor 重放，以及后台真实原子性/并发用例均保留。

原九个浏览器变体不各自证明重试后成功清理：它们第二次仍返回坏回执。原相邻 lost-response 用例证明重试后成功关闭。本批保持该边界说明，不夸大替代覆盖。

## 计数口径

- 布局删除 1 个静态声明和 1 个展开 Chromium 身份。
- 回执仍 1 个参数化浏览器声明，展开变体 9→3，减少 6 个 Chromium 身份。
- Node 新增 10 个展开用例：5 个 HTTP/envelope 失败、4 个实体失败、1 个有效成功对照。
- 同环境实际 --list 为基线 500→候选 493，精确身份差集仅为上述七项，没有新增身份；基线定向 12 PASS、候选 5 PASS，两完整文件 120 PASS（32+88），均无重试、跳过或错误。
- 不将上述不同层级相减为全仓净删数量，也不从减少数量推断提速百分比。

## 本批明确保留

SCOPE003/COST008：旧 QueryFilterPanel 只有夹具引用，但共享 Table、键盘焦点、延迟 opening 提交、晚帧像素恢复等尚有独有覆盖。未接替前不删除整文件、不直接缩短观测窗口。DUP005 的其他几何测试前提不同，本批不以相同 helper 为由删除。原 84 组意见中的其他保留/暂缓/历史范围继续按 V030-048 处置，不能将本批完成等同全面整改完毕。
