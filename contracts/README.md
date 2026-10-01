# v0.1.0 认证 HTTP 契约

权威描述：[openapi/openapi.json](openapi/openapi.json)（OpenAPI 3.2.1）；稳定业务错误映射：[errors/codes.json](errors/codes.json)。这两个文件描述 v0.1.0 的 Web 认证 API，不表示对应处理器、数据库或 Session 已实现。`/api/v1` 是接口兼容代际，和产品版本、OpenAPI 格式版本分别管理。

| 操作 | 路径 | 成功 | 身份 |
| --- | --- | --- | --- |
| 注册 | `POST /api/v1/registrations` | 201，用户 ID、账号及 Location；不创建 Session | 公开 |
| 登录 | `POST /api/v1/sessions` | 201，用户基础信息和安全 Cookie | 公开 |
| 当前用户 | `GET /api/v1/sessions/current` | 200，用户基础信息 | Web Session |
| 当前用户探测 | `HEAD /api/v1/sessions/current` | 200，无正文 | Web Session |
| 退出 | `DELETE /api/v1/sessions/current` | 204，无正文，清 Cookie | Web Session |
| 生成邀请码 | `POST /api/v1/invitations` | 201，邀请码只展示一次 | Bootstrap Admin |
| 重置密码 | `POST /api/v1/users/{userId}/password-reset` | 200，`data: null`，不返回固定密码 | Bootstrap Admin |

全部普通 JSON 响应使用 `code/message/data/meta` 四字段，HTTP 状态保持真实语义；HEAD/204 无正文。401 带 `Session realm="enterprise-management-system"` challenge。所有响应声明服务端生成的 `X-Request-Id`，外部同名头不能建立身份。Session 只通过 `__Host-session` Cookie 传送，须 Secure、HttpOnly、SameSite=Lax、Path=/、Max-Age=3600 且无 Domain；独立 `__Host-csrf` Cookie 使用相同属性但非 HttpOnly。前端读取后通过 `X-CSRF-Token` 提交；服务端验证 Cookie/header、Session 哈希绑定及可信 Origin，登录 JSON 不返回 csrfToken。注册与登录为公开操作，仍须来源校验。凭据写入仅收 `application/json`。该命名及交付由用户 Q7 于 2026-09-26 确认并已写回正式 Notion；属于发布前协议调整。

账号规则依据用户确认：后端去首尾普通 ASCII 空格，保留大小写，规范化后为 1–254 Unicode 码点，拒绝内部普通空格及控制字符；前端拒绝普通空格，NBSP 保留。`Alice` 与 `alice` 是两个不同账号。OpenAPI 的 `x-max-length-after-trim` 表达规范化后的限制，不对原始字符串设置 254 长度上限。密码仅允许 U+0020–U+007E，须有 A–Z、a–z、0–9 和可见 ASCII 标点各一个；空格不计作标点，不新增额外长度下限，不 trim 密码。邀请码一次成功消费，失败与并发路径遵守原子性。

## 字段校验错误登记（ADR-002）

字段错误 HTTP 400、`COMMON_VALIDATION_FAILED`，`data.violations` 使用公开字段名与 `location: body`，不回显输入。JSON 语法、未知字段、媒体类型等协议错误仍按对应协议错误返回。

| 字段级 code | 字段 | 含义 |
| --- | --- | --- |
| VALIDATION_REQUIRED | account / password / invitationCode | 必填内容为空；账号先按已确认规则 trim |
| COMMON_INVALID_ARGUMENT | account | 账号违反已确认格式或规范化后长度 |
| AUTH_PASSWORD_POLICY_VIOLATION | password | 注册密码违反已确认 Q13 字符和四类规则 |

登录密码只要求非空并精确验证现有凭据，不把注册策略附加为登录限制。

## V010-018 已确认故障语义

2026-09-29用户Q19接受PRD FR04与ADR005 D8，正式Notion功能/Redis/DDL说明已同步。邀请码或密码重置的PG事务已明确提交后，Session续期失败仍返回201/200及原业务结果，清除`__Host-session`和`__Host-csrf`两种浏览器Cookie，要求后续重新认证；清Cookie不是Redis已撤销的证明。提交前依赖故障或COMMIT结果不确定仍返回503，不返回成功结果，不自动重放操作；校验和授权失败保持各自4xx语义。

响应结构、路径和错误登记不变。Request-ID由Nginx生成并覆盖外部同名头，响应头、普通JSON的meta.requestId、认证审计和入口访问日志使用同值；非可信直连由Go重新生成。日志不包含明文邀请码、Cookie、CSRF、密码或带邀请码的查询串。

## 工具验证

2026-09-25 在 Node 22.23.1 下使用固定 `@redocly/cli@2.54.2`：

```sh
node --test contracts/openapi.contract.test.mjs
pnpm exec redocly lint contracts/openapi/openapi.json
pnpm exec redocly bundle contracts/openapi/openapi.json --output .work/openapi-bundled.json
pnpm exec redocly generate-client contracts/openapi/openapi.json --output .work/generated-client.ts
pnpm exec tsc --noEmit --target ES2022 --module ESNext --moduleResolution Bundler --lib 'ES2022,DOM' .work/generated-client.ts
```

解析、lint、bundle、生成和编译已运行通过。lint 报一项尚未决定仓库许可证的元数据警告，不影响规范有效性。生成器提示浏览器不能手动设置 Cookie header；前端不直接采用该生成客户端的 Cookie 注入逻辑，后续按同源 `credentials` 与实际 Cookie 行为验收。使用 `swagger-ui-dist@5.32.2` 从本机提供 bundle，Playwright 浏览器看到标题为 `OAS 3.2` 的七个操作且控制台 0 错误、0 警告；Redoc 2 静态页虽显示内容，但出现 React 控制台错误，未选为文档展示器。实际 HTTP 请求/响应符合度须在 V010-007 的真实服务验收验证，不能由本页的文档 lint 代替。

## Q36 发布前人员查询与持久草稿

已批准的新增协议、DTO、迁移结构和执行边界见 [query-drafts.md](query-drafts.md)。这是阶段 A 契约基线；HTTP 业务实现与完整存储/界面验收仍待后续阶段，不能单独部署。
