## P2e 正式流程管理HTTP与当前权限（Root冻结，2026-10-04 15:00 UTC）
后端优先，作为V030-020从P2c稳定检查点独立并行执行，不依赖P2d测试帮助函数。只增加流程定义保存／读取／启用／关闭真实管理接口；实际部署确认只接受后续受控引擎回执，不能由浏览器伪造。审批任务推进RPC与待办是后续包，本包不宣称已经接通。

### 路由与DTO
统一BFF入口，路径基准 /api/v1/applications/{appId}/forms/{viewId}/workflows/{flowId}。flowId为非零canonical UUID，首次保存expectedRevision=0创建，其后按CAS更新。
PUT /definition：严格JSON字段operationId、name、expectedRevision、expectedSchemaVersion、graph、allowWithdraw。graph为version/nodes/edges，节点id/kind及按类型限定approval或condition，approval为mode/assigneeIds/editableFieldIds，边from/to及可选branch。严格拒绝额外字段、重复JSON键、尾随JSON、null必填、非法数值、超过1MiB、错误Content-Type；复用flowgraph的有界图规则，不接受BPMN/XML、actor、scope、权限或deploymentId输入。
第一次成功201，其余200；data只含operationId,id,revision,currentVersion,candidateVersion,state。该成功仅说明定义已持久化，不说已部署/启用。GET /definition返回200及id,name,revision,currentVersion,candidateVersion,state,schemaVersion,allowWithdraw,graph（规范camelCase）；读取当前candidate版本供管理者编辑，不返回内部SQL/BPMN/deployment凭据。
POST /enable 与 /close：严格JSON仅operationId、expectedRevision，成功200同最小data。enable必须部署已确认且当前定义/候选/权限可用；close有在途时state=closing，仅表示接受关闭请求，不冒充disabled。版本CAS/重放语义沿用Catalog。
查询参数一律拒绝；未知路由/方法404。新诊断使用既有code/message/data/meta、Cache-Control:no-store、requestId。错误：COMMON_INVALID_ARGUMENT 400；AUTH_UNAUTHENTICATED 401；COMMON_CSRF_REJECTED/APPLICATION_FORBIDDEN/WORKFLOW_APPROVER_FORBIDDEN 403；APPLICATION_NOT_FOUND 404；WORKFLOW_CONFLICT/WORKFLOW_NOT_READY/WORKFLOW_CLOSING/APPLICATION_OPERATION_CONFLICT 409；APPLICATION_OPERATION_UNCONFIRMED 503带operationId。其他服务失败503，不暴露SQL细节。

### 授权与幂等事务
新包appworkflows，Application{Pool,*pgxpool.Pool; Limits appschema.Limits}，Service{Application *Application; Authenticator session.Authenticator; TrustedProxyHosts []string}实现http.Handler。
调用现有Prepare、Authenticate、ExpectedActor；cookie/CSRF/活会话/预期账号规则不变。只有应用owner或bootstrap admin可管理；中央“创建应用”权或普通数据权限不自动获得管理权。
GET使用同一RR事务读取当前auth版本、注册、owner/bootstrap及真实app/view归属。写入使用现有applications.BeginManagerWrite的活Session及personnel/app门禁，不自行复制弱权限事务。operationId由现有applications.operations按当前actor去重：先Replay，再Claim，catalog变更、审计、Complete同事务，最后Commit确认才响应成功；同键异内容409，无盲重试未知COMMIT。
新migration16只扩展ck_operation_kind三个值workflow.definition.save、workflow.enable、workflow.close，保留之前所有值与其他CHECK。Down先锁operations，存在三个新kind任何记录即55000拒绝，不能删除历史。更新兼容hash；旧1–15不改。本包不新增凭据/服务账号或生产权限。

### 审批人候选校验
FLOW-06：保存与启用时，所有approval节点的assignee必须是当前active账号，且已有当前表单菜单进入及至少一项非空字段data.read授权；owner/bootstrap依既有规则全权。own范围允许作为配置候选，但绝不解释成all；实际记录生成任务及审批时仍要行范围复核。禁用组、不属于组、缺menu、空字段读、账号回收都不能通过。配置editableFieldIds不会授予编辑能力，运行时仍取原权限交集，不要求所有候选一定拥有整个编辑白名单。
从当前数据库读取完整grant tuples，复用appaccess/apppolicy语义，不接受客户端传入任何权限事实，不用某条grant的scope拼另一条的fields。当前包是配置权限检查，不能伪装运行记录授权已实现。

### 审计与装配
管理操作复用既有application_structure_changed审计，object_type=form、object_id=viewId，change_summary包含appId,flowId,operationId,revision,state与动作，不存业务字段值。这是表单的流程配置变更，不新增未知event_type或扩大日志读权限。同一operation重放不新增版本/审计。
applications.Service增加Workflows http.Handler并在泛forms Definitions路由前分流精确workflows路径；cmd/bff的buildHandler装配新Service。没有配置的nil handler保持不可用，不回退直接Flowable REST。OpenAPI3.2.1与错误登记同步。前端保持原样。

### Root验收
真实Session/CSRF和PG覆盖保存读取、最小结果/未部署禁启用、同键重放和异负载、manager与普通账号隔离、候选无权限/own范围/撤权、过期schema/revision、异常输入、部署后启用、关闭等待、审计一次与真实宿主路由。Root先提供可编译503占位，观察行为RED后执行者实现；测试不得修改。Root后续单独覆盖真实Flowable RPC与审批流程，不能因这些HTTP通过宣称全后端已交付。
