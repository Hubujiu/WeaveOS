# Accepted task ADR appendix B
Source: https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f
Last edited: 2026-10-03T10:02:37.297Z; actual fetch/readback in task before implementation.

## 附录 B｜应用域预期账号负向 guard（2026-10-03 主负责人冻结）
来源：V030-012 的 0f881b1 尚未通过主负责人审查，发现跨账号请求和写入结果未确认的保护缺口。本附录只冻结后端 guard 的受控修复；前端恢复及验收规则见 <mention-page url="https://app.notion.com/p/3ee2f5a9e648811382aced0b7e3db41c"/> 第6节。V030-013 是共享 HTTP／OpenAPI／错误码及后端接线唯一 owner，本附录回读后可实施，不新增业务审批循环。
### B.1 请求头与拒绝顺序
- 请求头为 `X-Expected-Actor-Id`，新应用域前端每次请求必须携带该视图或 operation 已验证的 actor UUID，不能临时改成另一账号去重放旧请求。
- 范围是 `/api/v1/applications` 应用域及其子路径（包含创建、业务读取和写入），以及 `/api/v1/application-operations/*`。省略此头的旧客户端保持兼容；新前端不得依赖省略来绕过账号绑定。
- 服务端先完成真实 Session 认证，再以该请求的真实 principal 原子比较预期 actor；比较必须在任何业务读取、写入、operation claim 或 replay 之前完成。不得先返回另一账号的数据再由前端隐藏。
- 头格式错误返回 400 `COMMON_VALIDATION_FAILED`；合法 UUID 与实际 actor 不同返回 409 `AUTH_SESSION_CHANGED`。不泄露另一账号资料，也不把 expected actor 作为身份来源。
- 该头只是负向约束：匹配不会授予身份或权限。正常 Session、账号 active／auth_version、资源权限、Origin／CSRF、policy／record CAS 仍全部生效，不修改登录 Cookie 或现有认证机制。
### B.2 接线与验证
V030-013 在已协调的共享入口／合同文件中单点接线，V030-012 只绑定请求 actor 并清理错误账号 UI。不能由前端直接改后端权限或将此 guard 复制成多套不一致入口。
先 RED 再修复：验证新前端请求带头、缺省旧调用兼容、非法格式400、跨 actor409，读写与 claim／replay 均在业务访问前拒绝；相同 actor 仍执行原 Session／CSRF／资源权限检查。覆盖 operation 查询／重放与创建，保证不会读出其他账号数据或意外执行其旧 packet。
真实 create-only、owner、已授菜单成员及跨 app 集成验证须补齐。当前前端 90＋24 项为 mock 组合，6 项 real 为同一 Bootstrap／无 grant 的有限场景，不能据此宣称全权限验收通过。完成后记录真实 RED／GREEN、SHA 和限制；本附录冻结不等于实现或最终产品验收。
</content>
</page>
