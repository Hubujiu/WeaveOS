Source: https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f
Edited: 2026-10-03T00:45:56.302Z
Read: 2026-10-03 UTC; complete bounded PLAN, read through authorized Notion connector.

## 2026-10-03｜B4b 内部权限判定核心 PLAN（有限执行）
**目的与授权范围**：在已确认应用权限规则下，先实现可独立并行的纯 Go apppolicy evaluator 与不变量测试，不等待前端。此包是内部权限判定核心，不是完整权限系统；不接 HTTP、生产 schema 或真实 grant，不执行账号授权或安全配置变更。整页审批状态不变。
### B4b.1 输入信任与边界
- API 接受调用者提供的一致 policy snapshot 与 resource context；actor ID、可信 Bootstrap 标志和 creator／owner 关系只能由可信服务端调用者注入，不接受请求自行声明
- resource 的实际 app／form／view 归属及存在性由服务端调用者核实。错误 app、不存在资源或伪造 appID 不能绕过判定；不能先按客户端提供的 appID 放行再检查资源归属
- 有效授权配置、撤权 revision／CAS、存储 hook 及跨请求更新机制留给主负责人后续设计与接线；本包不自建真实角色库、权限缓存或全局锁
### B4b.2 受控判定规则
- 可信 Bootstrap 拥有全局与应用内部权限；owner 拥有其应用完整能力。creator → owner 关系显式输入，不靠名称或猜测归属
- createApp 是独立 capability，只控制创建应用；没有隐含其他应用的进入、配置或数据管理权。没有 owner 关系的普通创建权限持有者仍按对应应用 grant 判定
- 普通组只合并同应用的有效 grant；grant 绑定 resource／action／row predicate／field mask，完整授权取 OR。未勾选只是无 grant，不产生 DENY；空有效授权不授予普通成员能力
- 本包内部只支持 all／own 行谓词；own 明确取不可变 record.createdBy 与 actor ID 比较，不使用 lastEditor。首版不加入下属或 explicit deny
- 菜单进入与数据动作／字段权限分别 evaluate；最终产品入口需同时执行相关门禁。隐藏 UI 不构成鉴权，单独通过菜单检查也不等于通过数据操作检查
- 字段可读与可编辑分别判定；多组字段并集只在同一 record／action 且匹配 row predicate 的 grant 中计算，不能把一个组的行范围、另一个组的动作及第三个字段掩码拼成新权限
- 审批任务操作资格、节点字段白名单与 recordVersion 是独立业务约束；data.edit 不自动授予 approve、反审核或反完成。Bootstrap／owner 的权限不绕过数据类型校验、schema 依赖保护或流程状态合法性
### B4b.3 明确不在本包实现的部分
角色／权限组 CRUD、所有权转移、默认授权、未来资源继承、导入导出数据 API、HTTP 和生产授权存储均不实现。截图按钮仍仅参考，不将未确认的逆向审批动作加入能力范围。真实 source／records hooks、持久化 revision／CAS 与撤权接线由主负责人另给计划。
### B4b.4 RED／GREEN 不变量测试
先建立能证明缺陷的目标 RED，再实现至同一断言 GREEN；结果区分通过、失败与未运行，不以演示代替测试。
- 跨应用 grant 不可串用；错误 app、伪造 appID 与不存在的 resource 不可获得能力
- 同应用有效 grant OR；未勾选不抵消别组授权，空 grant 拒绝普通请求，不引入 DENY 优先
- all／own 正确，own 取 record.createdBy；lastEditor 改变不改变所有者范围
- 读／编辑字段分离；own-edit-X 与 other-read-Y 合并不产生 other-edit-X／Y，拒绝 field mask 混淆及跨组拼接
- Bootstrap／owner 完整应用能力、非 owner 的边界、create-only 可创建但不能管理别人应用；可信身份与资源 context 不从请求伪造
- 菜单与数据门禁各自测试；字段隐藏不作为授权依据
- no-approve-from-edit：编辑数据权限不能自动通过审批动作资格、节点白名单或版本／状态校验
测试范围仅覆盖 evaluator 的判定不变量；真实会话、HTTP、持久化撤权／CAS、UI 及生产权限更改没有因此通过验收。
### B4b.5 执行顺序与交付
1. 先只读核对仓库任务登记、分支和工作树，选择未占用任务编号并记录精确 scope；不任意占用共享文件或碰撞既有任务
2. 本 PLAN 写入并回读后，由主负责人放行独立分支内部实现；先 RED，再纯 Go evaluator 与 GREEN，不等待 Figma
3. 不修改 PR #22–25、共享 migration／HTTP／权限配置，不执行生产权限 SQL，也不授予真实用户任何权限
4. 可本地提交，报告任务／分支、local SHA、RED／GREEN 命令与结果、信任边界、复杂度与未实现项；无 push、新 PR、merge 或 deploy
5. 主负责人负责契约和验收。后续生产接线另行计划，不能把本包通过称作完整权限系统上线或用户验收完成
**当前状态**：B4b 仅完成执行 PLAN，尚无实现／测试完成回报；属于本轮限定内部模块工作，不改变整份 PRD 待评审、ADR-009 拟议中状态。

