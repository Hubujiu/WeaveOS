# V010-007 真实缺陷 RED → GREEN

2026-09-26，单 Agent；依赖 PR6/8/9/5 实际 merged 到远程 main 后领取。需求源重新实际读取项目/PRD/Architecture/ADR001–004/数据库设计与字典/Redis/功能正文；Q13 已写回正式来源，PRD仍需求编写中且未冻结。

## 字段错误、可信代理、Q13 Schema

RED 测试和无行为字段声明提交 `7aa81a7`，永久源码快照为本目录 `*-red-test.*`、`auth-before-validation-proxy.go`、`config-before-proxy.go`、`openapi-before-q13.json`。

实际 `node --test contracts/password-ascii.contract.test.mjs` exit1：Schema 接受非 ASCII 密码；账号原始 maxLength254 与用户批准的 trim 后长度语义冲突。旧 API-07 的 rejectInternalWhitespace 预期根据已批准字典修正为普通空格；NBSP 允许，并未减少真实格式限制。

实际 Docker Go1.25.7、PG18/Redis8.2，隔离数据库 weaveos_ci_test、Redis DB15，执行：

```sh
go test -race -p 1 ./internal/auth ./cmd/bff -run 'TestFieldValidationUsesApprovedEnvelope|TestAuditClientIPOnlyTrustsConfiguredProxy|TestConfiguredProxyHosts' -count=1
```

exit1：四个字段错误缺 COMMON_VALIDATION_FAILED/violations；可信 localhost 代理发来的单一198.51.100.9被记录为127.0.0.1；显式代理配置未加载。URL非法配置断言因前一个 fatal 未执行，未声称单独 RED。

预期来自已接受 ADR-002 的400/公开字段/VALIDATION_REQUIRED与密码策略例子，数据库设计与 ADR004 的可信代理规则；地址都是文档测试地址。新增字段级错误登记见 contracts/README.md。

最小实现后同命令 exit0；`node --test contracts/*.test.mjs`14/14 exit0；完整 `go test -race -p 1 ./...`、`go vet ./...`、`go build ./cmd/bff` 均 exit0。可信代理配置缺省不信任任何转发头，DNS解析失败/多个地址/无效地址都退回TCP peer，注册与审计共用该解析。无新增公开端点。

## 验收拓扑与CI

空合法 Compose 与 Nginx 配置先加入；`node --test tests/acceptance/topology.test.mjs` 三用例实际 exit1：缺 PG18/Redis8.2/loopback入口；旧CI未调用可重现HTTPS流程；空Nginx没有覆盖XFF/清身份头/cache/API路由。快照 `compose-before.json`、`nginx-before.conf`、`workflow-before.yml`、`topology-red-test.mjs`；配置后3/3 exit0。结构 GREEN 仅证明配置约束，实际栈验收单独记录。

第一次流程试跑误用了 Goose Down：已发布初始迁移明确没有破坏性 Down，Down显示EMPTY后重跑Up失败。未改迁移、未把该环境错误算产品 RED；改为Up重复应用并将恢复回退交008。第二次试跑被已有Redis本地隔离检查拒绝，保留检查，把测试Redis与PG放在同一个隔离容器网络命名空间。第三次试跑发现Goose可执行文件未在短生命周期容器间保留，改到私有工作目录。三次失败环境均停止，私有现场与独立PG volume保留供核查，不声称通过。

自动产品测试与最终发布签署分阶段：PR必跑产品栈，008完成后必须执行包含全部人工证据的check-release。没有修改release-policy或将pending改成passed；本次产品流程成功不等于整版发布门禁通过。

纯正文、错误登记和证据：TDD:N/A，文档一致性和Git路径检查；没有修改Notion审批/冻结/上线状态。

## 完整协议审查补缺

再次读取ADR002正文确认每个401必须有 `Session realm="enterprise-management-system"`，message是可公开的人类说明。`TestAcceptedUnauthorizedChallengeAndPublicMessages` 在真实PG/Redis下exit1：登录和匿名current均缺challenge，message均重复机器code。测试先提交 `67a21b4`，源码快照challenge-red-test.go/auth-before-challenge.go；最小统一reply修复后同命令race exit0，不根据message判定业务行为。HTTP验收也加强相同断言。

第五次整栈：Go race/vet/build、14契约+57治理底座+3拓扑（74/74）、23组件、20真实HTTP与逐响应schema、30三浏览器均通过；真实Redis故障与恢复通过。PG故障拒绝访问通过，但恢复后就绪仍503，整栈exit1，不记PASS。原配置把测试Redis与运行Redis混为同一个PG网络命名空间，PG重启后Redis停留在旧命名空间；保留compose-before-fault.json与fault-recovery-red-test.mjs后，将运行Redis保持独立服务名网络，单独test-redis只用于已有本地地址单元隔离，单元结束停止它。第六次整栈用于验证该修复和最终协议断言。
