## I. P1a 精确执行合同：回执校验核心（主负责人冻结）
本段为首个可独立验证的小包，主负责人已冻结技术合同，不等待整版 UI 或其它触发器。不更改本页仍待后续细化的持久存储／服务接线状态。新增三项业务范围已于 03:04 UTC 获用户确认。

范围：仅 services/bff/internal/flowcommands/protocol.go、主负责人提供的 protocol_test.go 与 V030-018 文档／证据。基线 db475e0dc00e2ac7c18fc5023cbc9039300711ac，任务分支 task/V030-018-approval-commands。不改 HTTP、数据库、依赖、其它模块或生产配置。该纯函数模块不表示审批已接通，紧接着实施真实持久存储及服务接线。

Command 固定字段：ProtocolVersion int、CommandID/AppID/TableID/RecordID/InstanceID/TaskID/ActorID/Action string、RecordVersion/FenceEpoch/TaskEpoch/ExpectedSequence int64、PayloadHash [32]byte。ProtocolVersion=1；动作 start/agree/reject/withdraw；所有 ID 为小写 canonical UUID，非零。start 的 TaskID 为空且 TaskEpoch=0；其余 TaskID 必填且 TaskEpoch>=1。RecordVersion/FenceEpoch>=1，ExpectedSequence>=0；所有数字不超过 9007199254740991。实例 ID 在 start 前由应用生成，仍必填。PayloadHash 非全零。
Fingerprint 对固定 struct 字段按 encoding/json.Marshal 的确定字段序列编码，计算 SHA256；编码即本段 protocolVersion=1 的规范，仅服务内部使用。每个字段必须参与绑定，不依赖 map 顺序。非法命令返回 ErrInvalid，不输出有效指纹。

Receipt 固定字段：CommandID string、CommandHash [32]byte、Outcome string、Sequence int64、ProofID string、ResultHash [32]byte。Outcome 仅 success/no_effect，ProofID 为非零 canonical UUID，ResultHash 非零。回执必须精确匹配 CommandID 和 Fingerprint。只有经过内部受控服务来源校验的持久回执可进入此核心，HTTP 客户端输入不能充当证据。
PlanReceipt(command,receipt,currentSequence,previous) 返回 ApplyPlan{Outcome string,Sequence int64,Duplicate bool} 或错误。previous 为该命令已经原子应用的持久回执：完全相同则 Duplicate=true，可在实例后续 sequence 已增长时返回；不同则 ErrConflict。无 previous 时 currentSequence 必须等于 command.ExpectedSequence。success 只能恰好 currentSequence+1，no_effect 只能 currentSequence，跳号／旧号返回 ErrOutOfOrder；非法枚举／数值返回 ErrInvalid，身份／哈希绑定错误返回 ErrConflict。不修改输入；没有 I/O、锁、时钟、租约或状态落库。
输出只是调用方事务的计划，不允许据此在提交前向用户报成功或释放 fence。后续应用事务负责原子投影／审计／inbox／fence。该函数也不自行相信网络超时／404，可由未知结果推断 no_effect 的 API 不存在。

测试由主负责人亲自编写并先提交无行为占位，以编译通过后的断言失败记录 RED。执行者只实现 protocol.go，不改测试；所有新增测试误差先报主负责人。验证命令 go test -race ./internal/flowcommands、go vet ./internal/flowcommands，保留 RED/GREEN 原始日志和测试 SHA256。复杂度：固定大小 envelope/receipt 校验 O(1) 时间和空间，不含后续数据库和结果 payload 解析。