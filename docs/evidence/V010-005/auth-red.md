# V010-005 认证业务 RED → GREEN 记录

## 来源与预期

2026-09-26 Asia/Shanghai 实际读取 Notion 项目、PRD入口、v0.1.0正文（最后编辑2026-09-26T01:55:01.066Z；需求编写中/未冻结）、Architecture、已接受ADR001/002（002最后编辑01:55:00.056Z）、登录/身份认证功能正文、相关设计/DDL、5对象与31个PG字段。Q1–Q12已答复并回写；不改审批状态。FR001–003/005–010/012–018及DDL事务§5/6是预期来源。API唯一路径/状态来自002批准的OpenAPI3.2.1与errors/codes.json，已使用邀请是409。Q13特殊符号边界尚未批准，相关候选规则不得作为正式验收。

## 环境 / 命令

本机Windows + WSL Docker Linux；PostgreSQL18.0、Redis8.2.1（隔离DB15）、golang:1.25.7，模块Go1.24。CI Go1.27.1。Go容器使用专用PG容器网络命名空间，测试只允许weaveos_数据库；不运行生产库，不上传fixture。存储包顺序执行 `go test -race -p 1 ./... -count=1`；随后 `go vet ./...`、`go build -o /tmp/bff ./cmd/bff`。PG迁移由003批准的Goose3.28.0预先应用。

## 实际观察到的阶段

| 测试版本 / 命令（services/bff中） | 有效RED，均退出1 | 后续结果 |
| --- | --- | --- |
| 5b212ca；go test ./cmd/acceptance-seed -run TestAcceptanceSeedProduces -count=1 | 四类邀请独立解码哈希匹配行数0，预期1 | 修正seed两处，完整seed测试退出0 |
| f7c3081；go test ./internal/auth -count=1 | 四类密码被占位拒绝、哈希相同/空、独立PHC被拒绝；正确路由注册501/201、并发501×2；匿名当前501/401 | 密码与基础登录实现；仍逐步达到其他目标 |
| 5df5071；同上 | 当前用户501/200、普通用户调用管理员501/403、审计失败仍201/503、健康Ready失败 | 当前恢复/管理员拒绝/成功审计/真实Ping通过 |
| 5df5071；go test ./internal/session -run TestLifecycle -count=1 | 健康真实Redis Ping错误 | 实际Ping/Close后GREEN |
| d47ca55；go test ./internal/auth -count=1 | 已使用邀请400/409；并发[201,400]/[201,409]；退出501/204；管理员生成501/201；未知账号审计缺行 | 区分邀请冲突、退出撤销、独立HMAC、管理员邀请事务通过 |
| 3439582；同上 | 管理员重置501/200；审计故障重置501/503；未知/尾随/重复JSON字段201/400；错误媒体201/415；审计与响应requestId不同 | 严格DTO、事务重置/版本失效/回滚及可信ID后认证包GREEN |
| 3d82af6；go test -p 1 ./cmd/bff ./internal/platform/httpserver -count=1 | 健康readiness503/200；装配业务404/201；部分配置错误放行 | 配置装配与平台路由接线后GREEN |
| 7c4d709；go test ./cmd/bff -run TestReadAuthentication -count=1 | 配置未读取/密钥未解码 | 显式配置读取后GREEN |
| cd0d144；go test ./internal/auth -run 'TestUnknownAPI\|TestRevoked' -count=1 | 未知API501/404；已撤销Session无session_invalid记录；修复后二次运行独立观察注册回滚缺failure审计 | 去敏事件及安全404后认证包GREEN |
| 0ed860e；go test ./internal/invitation -count=1 | 解码32字节独立SHA256向量不匹配，生成值空且重复 | 独立邀请领域Generate/Digest后GREEN |

只记录实际到达的断言；被前置登录失败遮蔽的管理员/审计断言没有提前算RED。17字节错误fixture盐、一次返回值数量编译错误、main局部close遮蔽内建close导致编译失败，都不是有效业务RED。错误盐修成16字节是独立fixture纠错，没有改产品预期。最初自行把中文/空格归为不算特殊符号的两条断言缺来源支持，已明确移除并集中Q13待答复，旧测试仍保留于快照。

## 永久证据与回放

本目录保留seed/password/HTTP/协议/装配/环境/邀请的测试源码、最小无行为占位以及中间实现；snapshots.sha256按Git的LF字节归一计算。恢复时在隔离副本把对应快照还原至其原模块路径，使用该阶段测试（不混入后续测试）与上述隔离PG/Redis运行；不要覆盖共享工作树。重放时间不能冒充本轮原始观察时间。旧HTTP测试的17字节盐问题见纠错快照http-corrected-fixture-test.go。

## 已验证与限制

真实PG/Redis事务、一次性邀请并发、精确大小写唯一、注册不创建Session、登录/禁用/恢复/撤销、CSRF/来源、Bootstrap生成/重置、全部旧版本会话失效、审计故障登录拒绝与Session补偿撤销、重置/邀请审计事务回滚、退出审计失败不恢复Session、独立HMAC/IP/UA去敏均通过。TLS httptest.NewTLSServer与真正HTTPS客户端/Cookie jar登录→恢复→退出验证直接GREEN，测试既有接线，不伪称新RED。真实PG查询屏障验证“读取旧凭据→管理员重置→登录不能取得新版本Session”直接GREEN。

69项仓库/契约/治理测试通过；这些不证明产品或人工验收。最终CI需按最终head另核对。三浏览器产品E2E、冷归档/恢复/本地运行发布矩阵仍由007/008完成。Q13未答复，不合并005或宣称整版完成。

## 运行配置

BFF读取WEAVEOS_DATABASE_URL、WEAVEOS_REDIS_URL、WEAVEOS_PUBLIC_ORIGIN（HTTPS origin）、WEAVEOS_SESSION_GENERATION、WEAVEOS_AUDIT_KEY_ID及Base64编码WEAVEOS_AUDIT_HMAC_KEY。密钥至少32字节，keyId为1–16个ASCII字母数字/下划线/横线。全部未配时平台readiness保持503；部分配置、非法配置或真实依赖不可用拒绝初始化。错误日志只给通用初始化失败，不输出连接字符串/密钥。此处是本地装配说明，非生产部署批准。
