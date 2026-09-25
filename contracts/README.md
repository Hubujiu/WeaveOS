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

全部普通 JSON 响应使用 `code/message/data/meta` 四字段，HTTP 状态保持真实语义；HEAD/204 无正文。401 带 `Session realm="enterprise-management-system"` challenge。所有响应声明服务端生成的 `X-Request-Id`，外部同名头不能建立身份。Session 只通过 `__Host-weaveos_session` Cookie 传送，Cookie 须 Secure、HttpOnly、SameSite=Lax、Path=/；注册与登录为公开操作，仍须来源/CSRF 校验。凭据写入仅收 `application/json`。

账号规则依据本轮用户确认修订：去首尾普通空格、保留大小写，账号内不允许空格；前端同时拦截，后端仍须独立验证。`Alice` 与 `alice` 是两个不同账号，注册唯一性及登录查找均区分大小写，不能采用 Notion 草案的 lowercase 唯一键。密码规则来自 PRD：至少分别包含大写英文字母、小写英文字母、数字和特殊符号；不新增长度下限。邀请码一次成功消费，失败与并发路径遵守原子性。

## 工具验证

2026-09-25 在 Node 22.23.1 下使用固定 `@redocly/cli@2.54.2`：

```sh
node --test contracts/openapi.contract.test.mjs
npx --yes @redocly/cli@2.54.2 lint contracts/openapi/openapi.json
npx --yes @redocly/cli@2.54.2 bundle contracts/openapi/openapi.json --output .work/openapi-bundled.json
npx --yes @redocly/cli@2.54.2 generate-client contracts/openapi/openapi.json --output .work/generated-client.ts
pnpm exec tsc --noEmit --target ES2022 --module ESNext --moduleResolution Bundler --lib 'ES2022,DOM' .work/generated-client.ts
```

解析、lint、bundle、生成和编译已运行通过。lint 报一项尚未决定仓库许可证的元数据警告，不影响规范有效性。生成器提示浏览器不能手动设置 Cookie header；前端不直接采用该生成客户端的 Cookie 注入逻辑，后续按同源 `credentials` 与实际 Cookie 行为验收。使用 `swagger-ui-dist@5.32.2` 从本机提供 bundle，Playwright 浏览器看到标题为 `OAS 3.2` 的七个操作且控制台 0 错误、0 警告；Redoc 2 静态页虽显示内容，但出现 React 控制台错误，未选为文档展示器。实际 HTTP 请求/响应符合度须在 V010-007 的真实服务验收验证，不能由本页的文档 lint 代替。
