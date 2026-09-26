# 真实 PG/Redis 认证与续期 RED → GREEN

独立来源：已重新读取 Redis Session §4–7、auth.users.status/auth_version 数据字典、ADR-001/002、Q7。无行为声明及测试提交 `59eeb63`；源码 authenticate-stub.go / authenticate-red_test.go 永久保留。

原始 Docker golang:1.25.7 + PostgreSQL18.0 / Redis8.2.1：`go test ./internal/session -run TestAuthentication -count=1 -v` 退出1，合法真实Session仍被stub拒绝（unauthenticated）。加载PG当前用户、精确active/version验证及写请求防护后，原命令继续到 Renew 未实现的目标RED，成功活动不能续发两Cookie；提交19b2dd3及源码 authenticate-before-renew.go保存此阶段。随后原子Touch与同步Cookie实现。

边界复现：`go test ./internal/session -run TestUninitialized -count=1 -v` 在零值Store.Load发生nil pointer panic，退出1；测试提交65a3113及源码 authenticate-boundary-red_test.go / store-before-zero-value.go保留。最小修复拒绝无客户端Store。

真实测试覆盖：当前PG身份（伪造外部头无效）、active/版本、禁用/重新启用撤销、缺CSRF/恶意Origin、失败不续期、成功滑动真实TTL并同步Cookie、DEL后Renew不复活、恢复generation不接受旧Cookie、实际Redis过期、PG关闭及Redis连接拒绝均fail closed并区分系统故障。身份context由独立identity测试证明只接受可信注入；业务HTTP装配将在005调用此边界。

本机环境：Linux Docker Desktop，PostgreSQL18.0（官方image），Redis8.2.1。Goose3.28.0经GOTOOLCHAIN=auto使用Go1.26.8迁移到1；应用测试Go1.25.7。最初Goose因工具链版本不足、首次完整suite因seed保护拒绝非localhost数据库都是环境问题，不算RED，未削弱保护。将Redis sidecar放入隔离PG网络命名空间后，二者均localhost。

完整命令：

```powershell
docker run --rm --network container:weaveos-v010-test-postgres --mount 'type=bind,src=D:\Workspace\WeaveOS-worktrees\V010-004\services\bff,dst=/src' --mount 'type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod' -e WEAVEOS_TEST_REDIS_URL=redis://127.0.0.1:6379/15 -e WEAVEOS_TEST_DATABASE_URL=postgres://weaveos_test:isolated-local-only@127.0.0.1:5432/weaveos_ci_test?sslmode=disable -w /src golang:1.25.7 go test -race -p 1 ./... -count=1
```

完整suite首次正确环境退出0；新增实际过期/失败TTL断言后再执行最终回归。生产凭据未使用。模块GREEN不代表005/007HTTP/三浏览器已完成，也不代表008备份恢复演练已验收。
