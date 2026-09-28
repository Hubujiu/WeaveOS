# V010-017 · 真实测试先行记录

## 原始执行

- 测试提交：d3474d5c844291c72861bf2a8ad5b52edd140e68；基线 main 88ce2da3091ad8e29c9ce65fa7d54863c281dd2f。
- PR18 的 Actions 实际检出 merge ref 4142839d50968fbe5b01b9db4199cf66af74a99a（上述两提交合并测试树）。
- [CI run 36410278046 / Go job 108888585328](https://github.com/Hubujiu/WeaveOS/actions/runs/36410278046/job/108888585328)。Ubuntu24.04，Go1.27.1，隔离PG18.6及Redis8.2.10；现有热/冷迁移均成功。
- 命令顺序：gofmt检查 → go vet ./... → go test -race -p 1 -count=1 -coverprofile=coverage.out ./... → go build ./cmd/bff。
- 10:36:27 UTC，四个Application用例在最初成功断言处失败，另一个结构边界测试失败；10:36:37 UTC测试退出1。格式、编译和依赖初始化没有阻止用例执行；后续build因set -e未执行。
- 尚未执行到的同一测试后续断言不冒充已观察RED。其他原有包测试通过不等于新边界已完成。治理任务因首次创建PR时任务pr字段为null失败，是文档登记问题，不算TDD RED；后续填18，未改门禁。

## 原始输出节选

下列是实际日志的连续用例失败片段，保留原时间戳；不是重新执行的模拟结果。

```text
2026-09-28T10:36:27.1758093Z --- FAIL: TestApplicationRegistrationWithoutHTTP (0.09s)
2026-09-28T10:36:27.1767943Z     application_test.go:38: registration must work without HTTP and normalize account
2026-09-28T10:36:27.1769503Z --- FAIL: TestApplicationLoginWithoutHTTP (0.09s)
2026-09-28T10:36:27.1771515Z     application_test.go:64: login must return authenticated identity and session material without HTTP
2026-09-28T10:36:27.1773102Z --- FAIL: TestApplicationInvitationWithoutHTTP (0.09s)
2026-09-28T10:36:27.1774795Z     application_test.go:87: administrator invitation use case must work without HTTP
2026-09-28T10:36:27.1776714Z --- FAIL: TestApplicationPasswordResetWithoutHTTP (0.16s)
2026-09-28T10:36:27.1778920Z     application_test.go:109: administrator reset use case must work without HTTP
2026-09-28T10:36:27.1780549Z --- FAIL: TestHTTPFunctionsDoNotExecuteAuthenticationStorage (0.01s)
2026-09-28T10:36:27.1828641Z ##[error]    boundary_test.go:61: service.go: HTTP function login bypasses the application boundary
2026-09-28T10:36:27.1847271Z ##[error]    boundary_test.go:54: service.go: HTTP function login performs credential work
```

其余结构诊断分别指出register/createInvitation/event/reset中的SQL调用、密码计算和BeginTx/Commit/Rollback仍在HTTP函数里；可在原job查看全部输出。没有在本机或服务器冒充执行此测试。

## 可恢复源码

本目录按原blob保存无行为Application声明、两份测试和重构前service/validation；扩展名.txt避免参与构建。测试/声明原Git blob分别为2280b5a6cda7fe439b847555b151d25ec58c435c、5e55e64c5879372385e6141eaf8fce7ff8497e55、dc27f50b0cbce24c06cb280f73140369968c1717。可在固定基线工作树把这些快照复制到原路径，使用同隔离环境复现；回放时间不得冒充原执行时间。

## 预期来源

已确认ADR-005 D1 / PRD FR-01要求同进程用例不接收HTTP对象；注册/登录/邀请/重置规则来自现行PRD、ADR001、已接受API规范与DDL/Session规范。测试独立检查注册不登录、邀请码不被失败消费、权限查当前数据库、版本递增与既有密码规则。不以新实现返回值生成expected。
