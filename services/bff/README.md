# Go Web Backend（现有部署名 bff）

这里是一个进程，包含 Web 适配与本地认证模块；不是纯接口转发 BFF，也不是多个认证微服务。保留 `services/bff`、`cmd/bff` 和镜像入口，避免代码边界重构影响发布。

## 从哪里读

- `internal/auth/service.go`：Web 路由、Cookie、来源检查、调用认证用例，以及会话恢复/退出的 Web 编排。
- `internal/auth/http_protocol.go`：JSON 解析、四字段响应、状态码和公开错误映射。
- `internal/auth/application.go`：不接收 HTTP 对象的注册、登录；管理员用例在 `application_admin.go`，审计在 `application_audit.go`。
- `internal/auth/validation.go`、`password.go`：现行账号和密码规则、密码哈希，不依赖 HTTP。
- `internal/auth/wiring.go`：兼容原 `Service` 装配入口，复用同一组存储资源；无通用依赖注入框架。
- `internal/persistence` 与 `authsql`：注册事务及已有类型化 SQL；管理员事务由认证用例在同一数据库内协调。
- `internal/session`：已有会话存储、原子续期和 Web 会话验证。此次未重写 Redis / Cookie 实现。

调用方向是 HTTP 适配 → 认证用例 → 现有持久化 / Session。`Application` 的方法接收 context、普通参数和可信来源已提取的 `RequestMetadata`；不读取请求头或写响应。管理员调用方必须先认证，事务仍复核数据库中的真实管理员状态与认证版本。

`LoginResult` 包含内部 SID/CSRF 材料，不得直接编码为 HTTP JSON；适配层只返回用户信息并按现有规范下发两种 Cookie。数据库错误也不直接回传，`Failure.Code` 由 HTTP 适配统一映射。

## 本次不做

不拆 Auth Service，不加 RPC / 框架 / 中间件，不改 Cookie 名、路径、密码策略、数据库结构或部署拓扑。Request-ID 入口迁移、Cron、日志留存与“提交成功但续期失败”的返回策略属于后续独立变更，不在 V010-017 顺手处理。

## 验证

真实隔离 PostgreSQL / Redis 下运行 `go test -race -p 1 -count=1 ./...`、`go vet ./...` 与 `go build ./cmd/bff`。新增 `application_test.go` 验证无需 HTTP 也能执行四个认证用例；`boundary_test.go` 只检查代码职责，不替代事务、Cookie、CSRF 和并发集成测试。已有测试预期继续保留。
