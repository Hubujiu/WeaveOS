## J. P1b 精确执行合同：持久命令账本（主负责人冻结）
在 P1a 之后实现 services/bff/internal/flowcommands/ledger.go；只增加同包 ledger_test.go（Root 编写）、本任务文档和证据。此包为产品使用的持久事务原语，不是另造流程引擎；不改正式 migration、runtime 权限或 HTTP。正式装配将在 P2 把已有应用事务／记录 fence／审计接到同一事务，P1b 不宣称该装配已经完成。
### 接口
Ledger{Namespace string} 仅配置代码内可信 schema，固定表名 workflow_commands、workflow_dispatch；SQL 标识符必须 pgx.Identifier 安全引用，不能将用户输入拼成 SQL。Namespace 非空，tx 非 nil；调用方必须持有既有产品锁、完成授权和记录版本检查。该包没有对外端点。
Entry{Command Command,State string,Receipt *Receipt}。状态 pending/success/no_effect。
AcceptInTx(ctx,tx,command) (Entry,error)：先 P1a 校验并算指纹；INSERT command ON CONFLICT DO NOTHING 以真实唯一键串行。存在时比较完整规范 envelope 和 hash，同语义返回原 Entry，不再插发送任务；不同 ErrConflict。首次插入命令和发送任务在调用方同一事务完成。
GetInTx(ctx,tx,commandID) (Entry,error)：按唯一键读取、校验反序列化，不存在 ErrMissing；禁止把不存在视为失败。
ApplyInTx(ctx,tx,command,receipt,currentSequence,apply func(context.Context,pgx.Tx,ApplyPlan)error) (Entry,error)：参数有效、callback必需；锁 command 行，核对完整 envelope／hash；已应用则用原持久 receipt 做 P1a 精确重放校验，等价重复不调用 callback。pending 时按 P1a 校验序列和回执，调用 callback 在同一 tx 内完成业务投影／审计／占用释放，再保存终态与 receipt、删除发送任务。callback 出错不能保存终态；调用方必须回滚整个 tx。函数不得 Begin/Commit/Rollback，不缓存 pgx.Tx，不启动后台 goroutine。
成功 Entry 仅代表 tx 内写完，不代表 COMMIT 已确认；HTTP成功仍由外层提交分类控制。传输超时没有写失败终态的接口。禁止清理命令历史，避免旧命令再次生效。
### 存储合同（本段由 Root 的隔离 PG fixture 创建，不执行生产 DDL）
workflow_commands：command_id uuid PK、command_json jsonb NOT NULL、command_hash bytea NOT NULL且32字节、state text CHECK(pending/success/no_effect)、receipt_json jsonb nullable；pending当且仅当receipt_json为空；created_at timestamptz NOT NULL默认now()。Envelope JSON属于协议metadata，不是业务记录 JSONB。
workflow_dispatch：command_id uuid PK且FK指向workflow_commands RESTRICT、created_at timestamptz NOT NULL默认now()。待发命令按索引读取，后续调度不得删除未确认命令来代表失败。
隔离 fixture 的 audit/projection/fence 表仅用于验证真实 callback 与账本共享回滚，不伪装成现有产品模型。
### Root 测试与执行
真实 PostgreSQL 18：首次接受／重放／异负载冲突、接受后回滚两表全无、发送任务插入失败不残留；应用后终态和callback效果同事务、外层rollback全回滚；callback异常无终态、丢重放不重复审计；原命令不存在、错误回执、乱序回执不产生效果。连接断开重连后重新Get和重放可读原持久事实；并发测试后续与记录gate完整接线一并补充，不能在没有测试时宣称覆盖。
所需 isolated DSN 由执行环境原有测试机制提供，不使用任何真实业务数据，不把秘密写日志。Root测试不可改；运行后返回原始日志／SHA。namespace隔离fixture不是正式migration安全验收。
复杂度：主键操作O(log C)，单命令常量行数和内存；历史O(C)，待发O(U)。callback成本另计。完整数据库授权／索引与真实锁序仍在P2验收。