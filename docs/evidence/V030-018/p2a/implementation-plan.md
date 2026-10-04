## P2a 正式记录保护：精确存储合同（Root 冻结）
目标：把持久命令占用接到现有真实记录写入保护，不创建模拟业务表。新增未发布迁移 db/migrations/00013_record_command_fence.sql（实施前核实编号仍空），只修改 infra/runtime/roles.sql 增加两项受控函数EXECUTE，Root新增 services/bff/internal/apprecordservice/root_command_fence_test.go。原1–12迁移与普通记录业务源码不改。后续产品审批接受命令时在同一应用事务中组合这些能力与已验证Ledger。
### 字段／索引
applications.record_command_fence_epoch_seq：独立bigint sequence，START/MINVALUE 1、MAXVALUE 9007199254740991、NO CYCLE、CACHE 1。迁移owner拥有，auth_app不授USAGE/SELECT/UPDATE；只由受控Acquire函数nextval。全局单调代次不是recordVersion/schemaVersion，不影响查询数据revision。事务回滚允许内部序列留空号，不产生业务写入或成功回执；避免既有logical_tables宽UPDATE权限继承到可重置计数列。
applications.record_command_fences.fence_epoch bigint NOT NULL DEFAULT 0，CHECK同范围。0仅兼容旧测试／既存占位；新受控函数只能创建正数epoch，旧epoch0不会被新release误释放。
record_command_fences 新增唯一(app_id,table_id,record_id)及唯一(command_id)。升级发现冲突必须失败，不静默删旧记录。仍只有pending状态；确定终态时在外层应用事务内精确删除占用，历史回执保留在命令ledger。
### Acquire函数
applications.acquire_record_command_fence(app_uuid uuid,table_uuid uuid,view_uuid uuid,record_uuid uuid,command_uuid uuid,expected_schema bigint,expected_record bigint) RETURNS bigint。
SECURITY DEFINER，固定search_path=pg_catalog，PUBLIC无执行权限，auth_app只有该有限函数执行权，仍不可直接INSERT/UPDATE/DELETE fence。所有UUID不得null/零；schema 0..安全上限，record 1..安全上限，非法SQLSTATE23514。
按现有锁序在调用方已完成Session/权限及app门禁后调用：函数先锁同app logical_tables行，验证form_view/menu归属、schema_ready及schemaVersion，再锁受控物理记录行，校验recordVersion。错归属／缺记录23503，schema/record版本冲突40001，未ready或其他pending占用55000。
相同command/record/version且正epoch的重复Acquire返回原epoch，不分配新代次；不同pending命令拒绝。新Acquire在同一事务调用受限sequence.nextval，插pending fence，返回正epoch；计数溢出拒绝，不复用旧代次。物理标识符只能由已核实UUID生成并安全quote，不接收任意表名或SQL。不得Begin/Commit，也不持跨服务网络事务。
### Release函数
applications.release_record_command_fence(app_uuid uuid,table_uuid uuid,record_uuid uuid,command_uuid uuid,fence_epoch bigint,expected_record bigint) RETURNS boolean。
同样有限SECURITY DEFINER与PUBLIC撤权；参数严格校验。先锁table及当前记录，校验recordVersion，只有app/table/record/command/epoch/expected_version全部匹配才删除并返回true；不匹配false且不改变当前占用。epoch0或空参数非法。没有TTL到期自动释放或按超时解锁功能。
调用方只可在持久引擎结果核实后，通过Ledger.ApplyInTx的同一个pgx.Tx完成投影、审计、终态与release。函数自己不证明授权或引擎成功，正式RPC/HTTP接线前必须保留该边界。
### schema并发保护
logical_tables 新增BEFORE UPDATE OF schema_version触发器：版本实际变化且该表存在pending record_command_fences时，拒绝SQLSTATE55000，固定constraint名 active_workflow_command_blocks_schema_change。这防止DDL/default/conversion绕过正在确认的记录占用。调用方事务中的早先DDL或数据改变随失败一起回滚；纯view layout不改变schema_version，不因此被拦。未来preflight显示需接入同一事实源，不伪报本包已有前端提示。
### Down与角色边界
Down若该sequence的is_called为true或有正epoch占用则拒绝，避免历史代次重用；不能把回退等同清空命令证据。空数据库允许Down/Up。只移除本迁移新增函数/trigger/index/columns及该专用sequence，不删除旧fence表或逻辑表。
roles.sql只追加对两函数的EXECUTE。默认auth_app已有fence SELECT保留，无通用写入、DDL或schema所有权扩张。本包只操作隔离测试DB，不授权生产DDL/凭据/权限变更。
### Root测试与执行
复用newRecordFixture的真实Session主体、app/table/view、原生类型列、现有记录Service.Edit和Redis。Root先提供仅用于RED隔离数据库的函数无行为声明，使SQL入口可调用、返回0/false；声明不进入正式migration。先实际观察功能断言失败，再由执行者写migration13与有限roles增补，重建隔离DB或移除Root声明后执行正式migration。
覆盖占用生效、同键重放、不同命令拒绝、错误epoch不能释放、ABA旧令牌不能释放新占用、事务回滚、旧record/schema版本和跨app拒绝、普通真实Edit被阻止且其他记录可编辑、schema修改不能绕过、runtime直接写入仍拒绝。函数返回未知/超时不自动解锁由上层协议继续验收。测试不由执行者改写。
复杂度：table PK与record PK查找O(log T+log R)，record-fence唯一索引O(log U)，常量行更新；同table短事务门禁沿用既有架构，不跨网络持锁。其他记录可能短暂等同table门禁，不会因某记录持续未知而长期锁住；不能宣称零锁等待。

### 精确配套登记
新增migration13须追加到infra/server/deploy/compatibility.json并记录真实SHA256，旧migration条目/字节不变；roles.sql的精确两函数EXECUTE增补后，仅更新infra/server/deploy/personnel-upgrade.mjs的roleHash常量匹配审核后的角色文本，不改验证逻辑、不放宽固定角色校验。以上仅仓库源文件/隔离测试；不安装到真实服务器。Root最终检查差异及哈希。
