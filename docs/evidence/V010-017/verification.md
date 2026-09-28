# V010-017 · 验证检查点

## 已实际通过的代码版本

代码head：dcd72c61f3511b1b33f3e9f66f5644149470fb52；基线main：88ce2da3091ad8e29c9ce65fa7d54863c281dd2f。PR18对应测试合并引用为a2e41945fe6ab0bf53aea41e1eabbee7aafcf73c。以下记录来自GitHub实际run/job结果，不是本机模拟输出。

[CI run36411796614](https://github.com/Hubujiu/WeaveOS/actions/runs/36411796614)于2026-09-28T10:52:29Z更新为completed/success。

| 检查 | 实际结果 |
| --- | --- |
| Go job108893489241：隔离PG/Redis和冷热迁移 | success |
| gofmt检查、go vet ./...、go test -race -p 1 -count=1 -coverprofile=coverage.out ./...、go build ./cmd/bff | 所在步骤success，shell正常完成；不是以编译代替测试 |
| 浏览器job108893489056：OpenAPI校验、类型检查、前端构建、真实浏览器到BFF smoke | success |
| 治理job108893489700：规则测试、verify-repo、check-tasks | success |
| 独立Repository governance run36411796714 | completed/success |

四个新增Application用例与结构边界测试均由未修改的go test ./...命令实际纳入；没有skip、替换断言或调低门槛。新增测试使用真实隔离存储，不要求HTTP请求对象即可完成注册、登录、管理员邀请与重置。已有HTTP、CSRF、会话、事务、审计与并发回归继续执行。RED源码、命令和原始失败节选见red.md。

此表是run/job状态及其命令的核对摘要，不冒充完整原始stdout。完整stdout可通过相应Go job日志读取；未向永久文档复制容器日志中的合成凭据等无关内容。

## 尚待最终结果

记录时[产品验收run36411796598](https://github.com/Hubujiu/WeaveOS/actions/runs/36411796598)仍在执行，其完整HTTPS产品、镜像、恢复、回滚、安全与交付检查未宣称通过。任何后续提交（包括纯文档）都需重新读取它自己的最终head CI，不能拿本表的旧绿灯替代合并前检查。最终复验可在PR18补充不可变run/head记录，不因此改写本检查点时间。

首次PR治理失败为任务pr字段登记为空，不算RED；初次实现提交79cb7b0的远端wiring.go与本机gofmt对齐不同，已用dcd72c6修正，不修改行为、不绕过格式门禁。

## 本地与部署边界

本机只能对编辑快照执行gofmt及独立标准库AST边界测试，不能连接GitHub进行完整clone，也没有要求的Go工具链/PG/Redis/Docker。上述完整测试确实在Actions运行。未修改生产、未在服务器运行测试、未合并main。

回滚本PR只需恢复上一应用代码/镜像组合；本PR没有schema、迁移、Session generation、Cookie、Nginx或Cron变化。D8提交后续期失败语义仍保留原状，不将本次模块分离宣传为该窗口已修复。

## 2026-09-28 最终head复核

PR18原head`16a1bf0c52a12a7a61ef032c793b799003fa95ed`的CI/Repository governance/产品验收共5项检查实际均SUCCESS。产品run36413324451的headSha与该值一致；job108898454276中“migrations, HTTPS and real product acceptance”、“runtime configuration, crypto, failure handling and dependency security”、“immutable runtime, restore, rollback and browsers”、“automatic deployment migrations with isolated real PostgreSQL”、“deployment packaging”步骤均SUCCESS。`check-release`步骤在PR条件下SKIPPED，不记作已执行。之后任务文档或合并main会产生新head，必须重验，不能用本检查点替代新head结果。
