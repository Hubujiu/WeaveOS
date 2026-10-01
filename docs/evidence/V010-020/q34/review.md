# Q34 三处 Arca Data Table

用户指定公开 Data Table 并确认成员、身份、操作记录三个表格都替换；身份列表转为四列表格，详情编辑保留。继续 V010-020 / PR21，不改权限模板、认证表单、后端/契约/迁移、依赖或外部组件源码。主Agent独占工作树写入；公开源和代码入口子Agent只读，另一个子Agent仅负责项目Figma同步，产物由主Agent验证SHA256后导入。

## 独立正式来源

项目、PRD入口、当前人员R3、登录功能底账、Architecture与适用ADR、React-消费及批准登记已实际读取。R3§5.7已回写并实际重读2026-10-01T04:02:23.429Z，登录底账03:56:15.803Z；全部相关正文完整、无报告截断。审批/冻结/上线属性未改。集中问题清单Q34已记答复和正式来源同步。source-and-scope.json是03:57的检查点，不冒充后来center更新；figma/final-notion-source.json保留最后的完整§5.7。

公开官网真实浏览器与官方源码固定c0319d887e10775c8968a7d6a451f248858726ec已核对，截图arca-official.png。全库没有已核对许可文件，不声明MIT或复制整套源码。延续Q24/Q32的项目内页面控件授权，原创原生React/CSS适配；不引官方示例排序、拖列/缩放、客户端分页或新批量行为。已批准React-表格的Potlab视觉不同，不能假装此Arca是同一个批准登记。

Figma保留三根161:441、163:3461、164:3505，新表279:1768/1896/1944。三个get_design_context都包含完整非sparse代码和工具截图；完整字段读回验证Noto字体、40行高、14/20正文、12/20行号、列宽与原业务详情。已有静态SVG素材复用，模板/外壳/Q32保留。身份footer279:1894明确48px及水平16px；成员原型正文为14/20，没有独立账号状态caption，R3保留业务要求因此保留既有正常/已停用文字并采用14/20。Code Connect端点受席位限制，不能宣称服务端已证明无映射；上下文未供应映射，现有项目内适配授权适用。

## 本轮真实时序

- f969e5d只记录已确认范围，产品仍是8c4b439。独立9项在未改产品时全RED，f4a05a3实际提交推送；原产品/旧测试/新测试/配置/退出码/完整失败日志和规范LF SHA256在red/。
- center补充3项RED在00ad1f9，结果正文样式补充1项RED在21bec68，两者先推送后实施。首批最小表格10项GREEN在97a753a，焦点几何复现1失败/反向键盘导航1通过亦保存于该提交；没有把通过项说成RED。
- 97a753a推送后将表内按钮focus outline绘到内部，同断言12项GREEN与124完整组件、153三引擎GREEN；51810bf保留焦点修复和新身份footer16px≠0px的实际RED。
- Figma独立正文样式澄清后，状态font14px≠12px的实际RED在10ff513，推送后才最小调整状态字号与身份footer内边距。footer-red/、status-red/都保存对应产品/测试/哈希/命令/退出码，不仅引用将被squash删除的分支提交。

三次中间首批失败日志保留。窄屏测试准备先纵滚200再看第一行、或将192px操作列滚到最右端却要求最左按钮可见，均与真实滚动需求不符；分别恢复纵向第一行并滚入两个操作按钮，保留所有原断言且强化第二按钮/横滚检查。详见test-review.md，未改产品来满足错误准备、未删断言或降低门槛。截图仅输出test.info路径；日志空白规范及来源/证据记录属于TDD:N/A。

## 需求与用例

| 独立来源要求 | 实際用例 |
|---|---|
| 三处白灰14px圆角、40行高、14/20表头/正文、center、固定列布局 | Q34三tab geometry；结果body typography；成员status typography |
| 当前页48px选择/混合ARIA/选中行，选择不写API | Q34 mixed selection；column geometry/page selection |
| 身份四字段、名称键盘编辑、原选中/脏输入保护 | Q34 identity table preserves edit entry；既有R3保存/影响/失败等完整回归 |
| 双轴滚动、固定表头、窄屏行内操作、页面无溢出 | Q34 dense table1920/320；reverse keyboard+sticky |
| 完整焦点可见、空表保留标题、减少动效 | Q34 focus ring clip geometry；empty/reduced motion |
| 已确认20服务端分页、筛选查询及footer占位/16内边距 | Q34 identity pagination；既有R3 identity/member/activity查询用例 |

## 恢复与证据完整性

figma-original-byte-archive.zip包含Git LF规范前完整Figma目录；import-sha256.json是外部原字节SHA256，解压ZIP核对，而非对规范后文本伪称同字节。实际22/22导入哈希、初始RED6/6和焦点RED4/4已核对。恢复画布按figma/recovery.md及before-editable.json/manifest，不盲目重跑或覆盖后续编辑。临时签名资产URL不进入永久证据；保留文字除这些URL的明确脱敏外完整。

测试配置实际依赖链在configs/。在任务工作树将对应*.ts.txt恢复为.work/同名*.ts；它们引用项目现有apps/web/playwright.component.config.ts，4174/4175均strictPort，不占在用4173。初始RED恢复测试与两份旧产品；焦点、footer、status分别恢复各自snapshot，不把多个阶段混成同一基线。下一条验证命令pnpm exec playwright test --config .work/q34.config.ts，三引擎用q34-cross.config.ts与日志记录的grep。

本机浏览器均合成API边界组件验证，不是真实BFF/PG/Redis产品验收。4173恢复旧命名审核窗口，合成fixture来自已测试样例；HTTP200、实际56/176和40/14表格几何已读回，截图local-preview.png。开放PR/在用服务及恢复资料保留，未合并/accepted/部署；验收后清理按仓库规则另执行。最终完整126/126 Chromium通过（3.9m），最终Chromium/Firefox/WebKit63/63通过（3.2m），根pnpm typecheck/build退出0。较广153/153基线通过（7.4m）亦保留。132基础/治理通过；结构/记录及最终HEAD archive安全检查随后复验，推送后五项CI只引用精确head实际状态。21张最终三引擎截图已人工对照白灰圆角、行列/编辑/焦点与安全业务文案。
