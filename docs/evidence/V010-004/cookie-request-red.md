# Cookie 与请求防护 RED → GREEN

来源：Q7 已回写 Redis Session §3/§6、ADR-002，读取时间 2026-09-26。安全属性、独立 CSRF 与服务端 hash 绑定、Origin/Referer 来源规则来自正式文档。

无行为声明与测试提交 `23703f0`；永久源码 cookie-red_test.go、cookie-stub.go、request-red_test.go、request-stub.go。Docker golang:1.25.7 上 `go test ./internal/session ./internal/security -run 'TestHostCookies|TestSource|TestCSRFRequires' -count=1 -v` 退出 1：已知合法凭据无法签发 Cookie、合法 Origin/Referer 被拒、正确 Cookie/header/hash 被拒，三个目标有效 RED。

最小实现：两种 Host Cookie 签发与清除；固定 HTTPS 可信来源匹配及 Referer 回退，拒外部/空/null 来源；CSRF Cookie/header 常量时间相等并匹配 Session 哈希。后续重复值拒绝实现尚需独立边界复现。真实 Redis 8.2.1 / golang:1.25.7 `go test -race ./internal/session ./internal/security ./internal/identity -count=1 -v` 退出 0（含六项真实存储测试），不代表 PG 当前状态验证或 HTTP 用例已接线。
