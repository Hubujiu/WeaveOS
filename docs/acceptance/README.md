# v0.1.0 验收说明

自动产品验收、运行恢复已通过；2026-09-27用户Q10/Q14三项人工签署已确认，见docs/evidence/V010-008/user-signoff.md。最终验收仍以008最终head CI、远程main任务文档与merged PR事实为准。

## 可重复产品验收

```sh
node infra/acceptance/run.mjs
```

要求 Docker Linux daemon、Node 和 OpenSSL（Windows 使用 Git 自带 OpenSSL），只运行本地隔离环境。流程创建独立 Compose 项目，真实 PostgreSQL18、Redis8.2、Go BFF、Nginx TLS 与正式前端构建；仅入口127.0.0.1:19443发布端口。另有临时 test-redis 为现有本地地址测试提供隔离，单元结束停止；运行 Redis 是独立服务。数据库和运行账号最小权限、冷归档与恢复由008落实，此验收栈的测试数据库管理账号不能当生产运行账号。

顺序：空库 Goose Up/重复Up → Go race（-p1，避免共享测试库清理冲突）/vet/build → 新产品库迁移与私有随机 Seed → 14契约/57治理底座/5拓扑 → OpenAPI3.2.1 lint/类型/构建 → 23隔离组件 → 真实HTTPS HTTP及逐响应schema → Chromium/Firefox/WebKit → 实际Redis/PG停止/重启与503/readiness恢复。

已发布00001迁移含用户/审计数据，不提供破坏性Down；回退采用008的隔离备份恢复/已验证制品回滚。不能为了测试修改已发布迁移。

组件测试的API mock只证明独立页面行为；产品API与三浏览器连接真实BFF/PG/Redis，没有API mock。三浏览器的忽略证书错误仅用于短期自签名隔离证书，不修改Cookie Secure/HttpOnly/Lax、不扩展系统信任库。Windows WebKit在006出现SameSite None；严格Lax断言保留，已在批准的Linux目标通过，未声称Windows全部通过。

私有 .work/acceptance 存放随机密码/邀请码、TLS私钥、HMAC、数据库URL和fixture，禁止入仓库或上传。现有环境拒绝覆盖，失败保留现场并停止自建容器，不删除卷。公开artifact仅允许 .work/acceptance/public/result.json；不得上传fixture、转储、完整Cookie、私钥、认证trace或带输入的截图。

## 自动化层级与证据

| 层级 | 路径 | 验证边界 |
| --- | --- | --- |
| 契约 | contracts/*.test.mjs、OpenAPI3.2.1 lint | 来源约束、五路径/七操作、DTO/错误/Cookie；不能代替实际响应 |
| 数据 | internal/persistence 与 cmd/acceptance-seed | 真实约束、事务/同码并发、失败不消费、Bootstrap幂等、不提升同名普通用户、私有随机fixture |
| Session | internal/session 与 internal/security | 真实Redis一小时TTL、边界/续期、失败不续、退出竞态不复活、generation、当前用户/版本、CSRF |
| 认证 | internal/auth | 注册/登录/管理、审计原子失败、重置读竞态、禁用、去敏、字段错误、可信代理、401 challenge/人类message |
| 页面 | apps/web/src/*.component.spec.ts | 23独立表单/错误/Loading/键盘/强度/路由用例，API mock限本层 |
| 产品HTTP | tests/acceptance/api.test.mjs、response-schema.mjs | 25用例；按真实状态核对OpenAPI的成功/错误/violations、请求ID、缓存、无正文、401头 |
| 产品浏览器 | tests/acceptance/web.spec.ts | 30用例，三浏览器真实注册/登录/刷新/退出与Cookie/CSRF/前端存储 |
| 依赖故障 | infra/acceptance/faults.test.mjs | 3用例；实际停止/重启Redis、PG，匿名身份伪造、API404不回SPA、故障503不发Cookie及恢复 |
| 底座smoke | tests/e2e | 仅平台探针，不计产品用例 |
| 发布门禁 | scripts/check-release.mjs、v0.1.0.json | 全部自动与用户人工证据；pending不能视为通过 |

永久RED/源代码快照与哈希：docs/evidence/V010-002..007；007完整运行证据和最终CI链接见 docs/evidence/V010-007/full-stack.md（形成后登记）。执行结果以实际运行和对应任务最终head为准，不以本表数量自动判PASS。

## 需求 → 独立用例

| PRD | 用例 |
| --- | --- |
| FR-001/005 | API正确/错误登录、GoLoginCurrentLogout、Web恢复与Cookie |
| FR-002 | PG大小写唯一/事务冲突、API重复不耗码/case与trim后长度、前端空格拦截 |
| FR-003 | PG RegisterConcurrent/事务回滚，API缺失/无效/已用/并发邀请，邀请码哈希 |
| FR-004 | 三浏览器URL预填；服务端仍验码 |
| FR-006 | Session CreateLoad一小时PTTL、Touch滑动、失败不续、真实过期，无需等待一小时 |
| FR-007/008 | API/Web匿名保护；Go真实expiry、禁用/版本/generation拒绝；页面401回登录 |
| FR-009 | API/Web退出闭环；GoTouchRevoke并发不复活、审计失败不恢复退出 |
| FR-010 | API disabled登录；Go已存在会话读取当前状态失效 |
| FR-011/015 | Q12额外入口灰色disabled无交互/存储，无找回/SSO/设备/Native扩展 |
| FR-012 | Admin reset、无明文响应、PHC独立样例/盐、全旧版本失效与reset/login读竞态 |
| FR-013 | API连续失败后仍可登录；无失败锁定/限流规则 |
| FR-014 | Go真实审计字段/HMAC/UA/可信IP、凭据去敏、事务审计失败；读取/年度冷归档权限归008 |
| FR-016 | BootstrapSeed幂等/不提升普通人，Admin与member生成/重置权限，事务内复核当前Admin |
| FR-017 | API注册无Cookie、Web返回登录且不能直接进app |
| FR-018 | Go/OpenAPI/API/组件的ASCII四类/控制/非ASCII边界；Aa1!成功且无额外下限 |

实际HTTP schema核对覆盖201/200/204/400/401/403/404/409/415/503及HEAD；未知schema约束会让验收oracle失败，不静默忽略。trace/RPC仅在真正跨服务采用时验证，本期单进程不伪造RPC。

## 用户最终验收（Q10）

| 项目 | 具体审核材料 | 当前责任 |
| --- | --- | --- |
| MAN-UI | Figma Login13:2/Register40:2与登录/注册desktop/mobile截图、键盘/状态路径 | 用户；2026-09-27 Q10实际确认 |
| MAN-OPERATIONS | 本机WSL Docker网络/TLS/权限、冷归档/到期删除、加密独立备份恢复、失效Session与制品回滚的去敏结果/实测耗时 | 用户；2026-09-27 Q10实际确认本机结果及模拟边界 |
| MAN-RISK | 固定重置密码、无登录限流/锁定；本期仅本地，公网发布仍未批准 | 用户；2026-09-27 Q10/Q14实际确认，接受基础镜像/工具漏洞用于本地开发 |

生产域名、证书续期、真实异机故障域、RPO/RTO目标与部署未授权；本地模拟结果不能宣称生产恢复能力或公网安全。007产品CI不替008的完整release门禁；008必须检查全部记录证据并取得实际用户签署，再运行check-release。

每任务最终head检查通过再squash；远程main同路径同任务ID全完成且对应PR已merged才接受。版本完成还须008与矩阵全部证据，Notion/Figma始终定义产品预期，测试或本文件不反向新增规则。
