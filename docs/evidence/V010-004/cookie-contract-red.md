# Q7 Cookie 协议 RED → GREEN

来源：用户 Q7 原文；2026-09-26 实际同步并重读 Redis Session §3、ADR-002 与身份认证底账。Session 名称 `__Host-session`，可读 CSRF 名称 `__Host-csrf`，请求头 `X-CSRF-Token`，登录 JSON 无 token；保留原已批准的哈希绑定/Origin/Lax/3600 秒属性。

测试提交 `78a1c3f`；可恢复源码 `cookie-contract-red.test.mjs` 和旧契约 `cookie-contract-before.json`。本机原始 `node --test contracts/openapi.contract.test.mjs` 退出 1：API-02 实际旧名称不等于新名称，API-10 Set-Cookie 描述缺新名称，8 passed / 2 failed。随后最小契约更新，原命令退出 0，10 passed。

安装本锁定依赖后执行固定 Redocly 2.54.2 lint、bundle、generate-client 及 tsc 编译均退出 0。lint 仍有既存 license 元数据警告；Cookie header 生成客户端不用于浏览器。初次 lint 因未安装 CLI 失败是环境问题，不算 RED。

旧 HTTP 验收 login helper 已按用户明确修订预期：getSetCookie 检查两种 Cookie 的属性与独立值，后续携带两个 Cookie 和 header，不从 JSON 取 token。真实产品 HTTP suite 仍 NOT RUN，需 V010-005 装配与隔离 fixtures，不把契约 GREEN 当业务通过。
