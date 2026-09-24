# v0.1.0 架构落地与模块边界

当前完成基础宿主，不是业务完成。Node24.14.0、pnpm10.28.2、Go1.27.1为本任务CI锁定值；React19.3.0、Vite8.0.0、TypeScript5.9.3、Playwright1.63.0通过锁文件/实际构建核对。不是宣称行业最新，也不批准ADR-003全部候选库。Go module的1.23.0是语言最低版本，CI/建议运行时由.go-version锁定，不推荐用旧运行时发布。

浏览器 → React/Vite静态构建 → 同源HTTP入口 → Go BFF。当前cmd/bff装配平台宿主，internal/platform/httpserver实现健康/就绪/错误边界；persistence、session、identity、auth、invitation、audit由各任务补齐。没有额外Node BFF，没有为同进程模块制造RPC。

GET/HEAD /health/live报告宿主存活；依赖未装配时/health/ready返回503，未来只能真实必要依赖检查成功才转200。平台健康不套业务Envelope；未知API返回404四字段且不回退SPA；不信任外部requestId作为入口ID。

`cd services/bff && go run ./cmd/bff`默认127.0.0.1:8080，BFF_ADDR由受控环境配置。宿主设置HTTP超时与SIGTERM关闭。前端`pnpm --filter @weaveos/web dev`；preview只用于CI/development，正式Nginx/TLS/Compose按运行任务落实。

只创建真实需要的宿主和编译入口，不生成空AuthService/UserService，不默认MQ/搜索/对象存储/Kubernetes。密码仅在BFF认证边界，业务消费统一身份上下文；SQL归认证数据所有者，不在HTTP Handler堆领域SQL。接口和数据取舍须与已确认Notion、contracts和migrations一致。

目录归属、依赖和接手动作见任务索引；尚未实现的模块不能用假的成功返回、内存用户库或无条件ready掩盖。
