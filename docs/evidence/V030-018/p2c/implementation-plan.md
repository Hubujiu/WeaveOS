## P2c 后端正式流程目录与版本门禁（Root 技术合同，2026-10-04 12:49 UTC）
沿用FLOW-01/03/04/05/06/12。本包为正式应用库持久目录，不直接对浏览器开放内部函数；实际引擎部署与当前会话／管理权限由随后服务层组合。不能将内部ActorID参数当作授权证明。后端优先，完成后继续服务及HTTP接线。

### 接口与事务
包 services/bff/internal/workflowcatalog，固定 applications schema，所有函数接收调用方pgx.Tx，不开启／提交事务、不网络调用。Catalog为空结构。写入锁序为applications.apps行→logical_tables行→workflow_definitions行，和既有管理／schema Save门禁一致。调用方仍负责当前Session、manager或实际任务授权；函数验证资源归属、版本与状态。错误统一 ErrInvalid／ErrMissing／ErrConflict／ErrNotReady／ErrClosing；已返回错误的外层事务必须回滚。

VersionInput：AppID,TableID,ViewID,FlowID,ActorID,Name string；ExpectedRevision,ExpectedSchemaVersion int64；Graph flowgraph.Graph；AllowWithdraw bool。
PutVersionInTx(ctx,tx,input) (Head,error)：新FlowID要求ExpectedRevision=0，创建disabled定义、revision=1、version=1。已存在则精确CAS revision，名称与图保存到新版本，revision+1、current_version+1；table/view绑定不可变。拒绝closing中的版本发布。验证真实table/view/app归属、schema_ready以及schemaVersion；字段上下文从当前fields_json读取，不信请求传字段类型。调用flowgraph.Validate与CompileBPMN，graph和BPMN由服务器生成／验证；非法不留下任何行。每个版本使用独立version_id UUID作为BPMN process key，避免不同结构被引擎同名覆盖。新版本deployment_id初始NULL。
为避免把尚未部署的结构提前用于新发起，PutVersion不能在enabled状态直接替换current_version：enabled定义的新版本可以持久化，但current_version仍指向旧已确认版本，直到ConfirmDeployment成功；version按该flow最大version+1分配，最新候选记candidate_version。Head包含 FlowID,AppID,TableID,ViewID,Name,State string；Revision,CurrentVersion,CandidateVersion int64（尚未确认时CurrentVersion=0）。

ConfirmDeploymentInTx(ctx,tx,appID,flowID,version int64,deploymentID string) (Head,error)：只由持久引擎部署回执驱动，非用户API。部署ID非空且限200字节；给指定不可变version登记，完全同回执幂等，不同回执冲突。若该version仍等于candidate_version，则在同一锁内重读当前schema重新Validate图；兼容才提升current_version并revision+1。过时候选可保留部署事实但不能覆盖较新candidate/current；其状态更新不能伪造新候选已发布。名称以定义最新名称为准。该登记本身不自动启用disabled定义。
EnableInTx(ctx,tx,appID,flowID,expectedRevision) (Head,error)：要求当前候选已成功确认并成为current_version，重验当前schema及版本图；CAS后enabled，revision+1。重复同状态且匹配当前revision返回不增加revision；旧revision冲突。可重新开启closing以取消等待关闭，迟到FinalizeClose必须被revision拦住。
RequestCloseInTx同参数：CAS后停止新发起；未终态实例存在则closing，否则disabled；实际状态变化revision+1，同状态且当前revision幂等。
FinalizeCloseInTx同参数：只能对closing且同revision；未终态仍存在则返回原Head，不假报关闭；已排空则disabled且revision+1。enabled/disabled或旧revision ErrConflict。
GetInTx(ctx,tx,appID,flowID) (Head,error)：仅查持久目录，资源错误ErrMissing。调用方控制可见性。

ReserveInput：AppID,FlowID,InstanceID,RecordID,ActorID string；ExpectedRevision,ExpectedSchemaVersion,ExpectedRecordVersion int64。
ReserveInTx(ctx,tx,input) (Instance,error)：同锁序，必须enabled且current_version已有deployment_id；新候选未确认期间继续使用旧current_version，不因编辑草稿中断旧已发布流程；CAS revision/schema/真实record_version，查真实typed record归属存在；保存starting实例，固定definition_version。返回Instance含ID,FlowID,AppID,TableID,ViewID,RecordID,InitiatorID,State string；DefinitionVersion,Sequence int64。实例不存固定业务recordVersion。相同InstanceID、同身份/资源/绑定定义的重放返回原行，即使以后current_version变化；异身份资源冲突。重放不得绕过外层当前权限。不同流程可为同一record各有实例。正式服务会把此预留和ledger/fence放在同一事务，独立预留测试不代表已经接通跨服务。
本包不新增结束实例的外部接口。workflow_instances的starting/active为未终态；completed/rejected/withdrawn/no_effect为确认终态。后续受控Apply投影才可更新，测试可由owner模拟已确认终态来验排空；网络超时不写终态。

### DDL与最小角色
新migration15，旧1–14不改。workflow_definitions：id uuid PK、app_id/table_id/view_id uuid、name varchar(100)非空、revision bigint正安全整数、state disabled/enabled/closing、current_version bigint>=0、candidate_version bigint>=1、created_at/updated_at。app/table/view用复合FK防串app，并校验view属于同table。
workflow_versions：(app_id,flow_id,version)PK；version_id uuid UNIQUE；schema_version bigint>=1；graph_json jsonb object、bpmn_xml text非空；allow_withdraw boolean；created_by uuid、created_at；复合FK定义，immutable内容由列级权限保护。deployment_id独立存于workflow_deployments：(app_id,flow_id,version)PK/FK，deployment_id text非空、confirmed_at，不允许改旧回执。
workflow_instances：id uuid PK；app_id/flow_id/definition_version复合FK版本，table/view/record/initiator UUID，state上述枚举，sequence bigint>=0安全整数，created_at/updated_at；复合scope FK保证table/view与该定义一致；建立(app_id,flow_id,id) WHERE state IN ('starting','active')用于排空，(app_id,table_id,definition_version)同样未终态索引用于兼容扫描。
auth_app：四表SELECT/INSERT；definitions只UPDATE(name,revision,state,current_version,candidate_version,updated_at)；instances只UPDATE(state,sequence,updated_at)；versions/deployments无UPDATE/DELETE/TRUNCATE。auth_backup仅SELECT，reader/maintenance无新增权限，PUBLIC无授权。不新增服务账号或生产凭据。本包仅隔离DB。
Down先按固定顺序ACCESS EXCLUSIVE锁四表；任一非空55000拒绝，不擦历史以回退。空库允许。迁移后roles再安装的顺序必须可用；更新兼容SHA和现有roleHash，backup fixture由Root追加15。

### 兼容性查询
CheckCompatibilityInTx(ctx,tx,appID,tableID,proposed []appquery.Field) ([]Conflict,error)：
检查enabled/closing的current_version，以及starting/active实例绑定的各个历史definition_version，去重。disabled且无在途的历史不阻止结构修改；未确认候选不作为运行依赖，Confirm/Enable时再校验。
Conflict字段FlowID,NodeID,FieldID,Reason string，Version int64。针对删除editable字段、条件字段不存在或原比较在新类型下不可执行报告；保留兼容number→money、重命名、精度调整等可继续比较的变化，不一刀切拦类型不同。条件逐节点复用appquery.Compile；输出确定排序。当前只验证Field的ID/Kind条件与编辑能力，选项语义/字段完整配置需schema服务层进一步核验，不声称本包代替完整schema Save。
复杂度：按app/table索引定位运行定义与在途不同版本，O(K*(V+E+F))加编译固定上限成本；排空EXISTS索引O(log I)，预留PK O(log I)。无扫描全历史业务记录、无跨网络长事务。

### Root测试与衔接
Root亲写真实PG测试覆盖不可变版本、部署确认前禁止启动、同回执重放／异回执拒绝、旧候选不覆盖新候选、在途固定结构且后续新实例使用新结构、schema CAS、closing挡新发起并等待starting、关闭完成与重开CAS、A/B独立、内外事务回滚、跨app/view拒绝、精确兼容与历史依赖，以及DB权限。执行者只实现Root规定文件，保留真实RED／GREEN。
下一包必须把catalog挂入当前管理Session服务、schema Save/preflight和正式HTTP，补真实接口测试；不能以目录存储通过宣称后端全部完成。