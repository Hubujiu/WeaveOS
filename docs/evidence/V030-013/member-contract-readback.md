# Accepted task ADR appendix A
Source: https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f
Last edited/read back: 2026-10-03T09:55:41.530Z
Read ADR002 §5.4 directly; meta.pagination={nextPageToken:string|null,hasMore:boolean}; pageToken omitted/empty starts first page; no total is promised.

## 附录 A｜权限管理成员候选与成员显示读接口（2026-10-03 主负责人冻结）
本附录补齐 V030-012 真实 B5 权限管理界面的成员选择与显示能力。接口、最小数据范围和接线归属由主负责人确定，本附录读回后即可实施，不属于仍待用户答复的四项业务默认规则。只增加受控读合同，不据此扩大既有授权或宣称接口已交付。
### A.1 活跃成员候选
- 方法与路径：`GET /api/v1/applications/{appId}/member-candidates`
- 仅当前固定应用 owner 或可信 Bootstrap 可用。每次由服务端验证真实 app、当前 Session、账号 active 状态与 auth_version；只有 `applications.create` 能力不能枚举他人 app 的候选成员。
- 当前单组织模型内只返回活跃账号。查询参数 `q` 可省略，TrimSpace 后按公开显示账号 account 的前缀搜索，最长 100 字符；不是模糊包含搜索。使用参数绑定，不能将输入拼接为 SQL。
- `pageSize` 为 1–50，默认 20；`pageToken` 可省略，为绑定 actor、app 与规范化 q 的不透明 cursor。顺序稳定为 account、id；正确校验 cursor 失效、篡改与作用域变化，不能全量拉到客户端分页。
- 响应使用现有 envelope：`data.items` 为下列最小候选数组，分页沿现有 `meta.pagination` 契约，不另造分页结构。
```typescript
type MemberCandidate = {
  id: UUID;
  label: string;
  status: "active";
};
```
- label 使用既有人员公开显示账号。不返回邮箱拆分、凭据、Session、权限模板或完整人员 DTO。错误与 cursor 失败沿公共 Session／应用授权／校验／分页合同，不放宽原错误或访问语义。
### A.2 已登记的权限组成员显示
- 既有 `GET /api/v1/applications/{appId}/permission-groups/{groupId}/members` 保留 `memberIds`，增加 `members` 数组。每项包含 `id`、`label`、`status`、`selectable`，只覆盖本组实际成员。
- status 必须来自真实人员来源的当前状态，不能伪造 active。inactive 成员仍显示已有身份信息，selectable 为 false，不作为新增选择；活跃候选按 A.1 提供。
- 软回收后的历史成员显示遵既有 source 保留规则，不得因来源失活而空白或静默丢失成员。稳定 ID 不按同名账号重新绑定。
- 成员写入继续使用 B5 已有真实账号校验、完整替换、operationId 和 expectedPolicyRevision CAS。候选读结果不授予写权限，提交时仍须重验当前 actor、资源和成员状态。
### A.3 唯一接线 owner 与范围
V030-013 是本附录共享 BFF HTTP、OpenAPI 与必要服务端测试的唯一接线 owner，沿本页合同 7 的实际共享文件协调执行。V030-012 只消费已冻结的候选／成员标签合同实现选择器，不另造或并发修改后端接口。
不新增永久人员管理权限，也不修改现有 personnel-only 接口的授权。普通记录编辑者的成员引用选择由 V030-015 后续专用合同定义；本 owner-only 管理候选接口不得直接开放给所有 record 编辑者。没有新的生产权限变更或凭据配置授权。
### A.4 必须验证的行为
- owner／Bootstrap 正常分页与前缀搜索；普通成员、他人 app 的 create-only、失活 actor 和失效 auth_version 被拒绝。
- q 首尾空格、长度边界、特殊字符参数绑定；pageSize 默认／边界；稳定 account／id 排序；cursor 跨 actor／app／q、失效与篡改拒绝。
- 响应只含冻结的最小显示字段与标准分页，不暴露完整人员数据；读取不会添加 membership 或 grant。
- 本组 memberIds 兼容；active／inactive 显示与 selectable 正确，软回收后身份信息保留；并发状态变化下写入仍按 B5 校验与 CAS。
- 真实 API 集成到 V030-012 选择器，加载／空结果／分页／无权限／失败／失活已选成员均有可见状态。测试和截图未完成前，不把本附录冻结等同功能验收。
</content>
</page>
